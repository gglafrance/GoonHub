package core

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"
)

// ScanStatus represents the current state of a scan operation
type ScanStatus struct {
	Running      bool             `json:"running"`
	CurrentScan  *data.ScanHistory `json:"current_scan,omitempty"`
}

// ScanService handles scanning storage paths for new video files
type ScanService struct {
	storagePathService  *StoragePathService
	videoRepo           data.VideoRepository
	scanHistoryRepo     data.ScanHistoryRepository
	processingService   *VideoProcessingService
	eventBus            *EventBus
	logger              *zap.Logger
	indexer             VideoIndexer

	mu          sync.Mutex
	currentScan *data.ScanHistory
	cancelFunc  context.CancelFunc
}

// NewScanService creates a new scan service
func NewScanService(
	storagePathService *StoragePathService,
	videoRepo data.VideoRepository,
	scanHistoryRepo data.ScanHistoryRepository,
	processingService *VideoProcessingService,
	eventBus *EventBus,
	logger *zap.Logger,
) *ScanService {
	return &ScanService{
		storagePathService: storagePathService,
		videoRepo:          videoRepo,
		scanHistoryRepo:    scanHistoryRepo,
		processingService:  processingService,
		eventBus:           eventBus,
		logger:             logger.With(zap.String("component", "scan_service")),
	}
}

// SetIndexer sets the video indexer for search index updates
func (s *ScanService) SetIndexer(indexer VideoIndexer) {
	s.indexer = indexer
}

// RecoverInterruptedScans marks any scans left in running state as failed
func (s *ScanService) RecoverInterruptedScans() {
	if err := s.scanHistoryRepo.MarkInterruptedAsFailedOnStartup(); err != nil {
		s.logger.Error("Failed to recover interrupted scans", zap.Error(err))
	} else {
		s.logger.Info("Recovered interrupted scans on startup")
	}
}

// StartScan initiates a new scan of all storage paths
func (s *ScanService) StartScan(_ context.Context) (*data.ScanHistory, error) {
	s.mu.Lock()
	if s.currentScan != nil && s.currentScan.Status == "running" {
		s.mu.Unlock()
		return nil, fmt.Errorf("a scan is already running")
	}

	// Create new scan record
	now := time.Now()
	scan := &data.ScanHistory{
		Status:    "running",
		StartedAt: now,
		CreatedAt: now,
	}

	if err := s.scanHistoryRepo.Create(scan); err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("failed to create scan record: %w", err)
	}

	s.currentScan = scan

	// Create cancellable context from background - NOT from request context
	// The scan runs as a background job and should not be cancelled when the HTTP request completes
	scanCtx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.mu.Unlock()

	// Publish start event
	s.publishEvent("scan:started", scan)

	// Run scan in background
	go s.runScan(scanCtx, scan)

	return scan, nil
}

// CancelScan cancels the currently running scan
func (s *ScanService) CancelScan() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentScan == nil || s.currentScan.Status != "running" {
		return fmt.Errorf("no scan is currently running")
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	return nil
}

// GetStatus returns the current scan status
func (s *ScanService) GetStatus() ScanStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentScan != nil && s.currentScan.Status == "running" {
		return ScanStatus{
			Running:     true,
			CurrentScan: s.currentScan,
		}
	}

	return ScanStatus{Running: false}
}

// GetHistory returns paginated scan history
func (s *ScanService) GetHistory(page, limit int) ([]data.ScanHistory, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.scanHistoryRepo.List(page, limit)
}

// runScan performs the actual scan operation
func (s *ScanService) runScan(ctx context.Context, scan *data.ScanHistory) {
	defer func() {
		s.mu.Lock()
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	// Get all storage paths
	paths, err := s.storagePathService.List()
	if err != nil {
		s.completeScan(scan, "failed", fmt.Sprintf("failed to get storage paths: %v", err))
		return
	}

	if len(paths) == 0 {
		s.completeScan(scan, "completed", "")
		return
	}

	var filesFound, videosAdded, videosSkipped, videosRemoved, videosMoved, errors int
	lastProgressUpdate := time.Now()
	progressBatchSize := 100

	// Phase 1: Detect missing files (videos whose source files no longer exist)
	videosRemoved = s.detectMissingFiles(ctx, scan, paths)
	if ctx.Err() != nil {
		s.completeScan(scan, "cancelled", "")
		return
	}

	for _, storagePath := range paths {
		select {
		case <-ctx.Done():
			s.completeScan(scan, "cancelled", "")
			return
		default:
		}

		// Update current path
		s.updateScanProgress(scan, &storagePath.Path, nil, scan.PathsScanned, filesFound, videosAdded, videosSkipped, videosRemoved, videosMoved, errors)

		err := filepath.WalkDir(storagePath.Path, func(path string, d os.DirEntry, walkErr error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if walkErr != nil {
				s.logger.Warn("Error walking path",
					zap.String("path", path),
					zap.Error(walkErr),
				)
				errors++
				return nil // Continue walking
			}

			if d.IsDir() {
				return nil
			}

			// Check if it's a video file
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if !isVideoExtension(ext) {
				return nil
			}

			filesFound++
			currentFile := path
			s.updateScanProgress(scan, &storagePath.Path, &currentFile, scan.PathsScanned, filesFound, videosAdded, videosSkipped, videosRemoved, videosMoved, errors)

			// Check if video already exists at this path
			exists, err := s.videoRepo.ExistsByStoredPath(path)
			if err != nil {
				s.logger.Warn("Error checking video existence",
					zap.String("path", path),
					zap.Error(err),
				)
				errors++
				return nil
			}

			if exists {
				videosSkipped++
				return nil
			}

			// Check if this might be a moved file (same size and filename exists elsewhere)
			info, err := os.Stat(path)
			if err != nil {
				s.logger.Warn("Error getting file info",
					zap.String("path", path),
					zap.Error(err),
				)
				errors++
				return nil
			}

			filename := filepath.Base(path)
			existingVideo, err := s.videoRepo.GetBySizeAndFilename(info.Size(), filename)
			if err != nil {
				s.logger.Warn("Error checking for moved file",
					zap.String("path", path),
					zap.Error(err),
				)
				// Don't fail - continue with normal add flow
			}

			if existingVideo != nil {
				// Check if video was soft-deleted (marked as missing) or if old path doesn't exist
				wasSoftDeleted := existingVideo.DeletedAt.Valid
				oldPathMissing := false
				if !wasSoftDeleted {
					if _, statErr := os.Stat(existingVideo.StoredPath); os.IsNotExist(statErr) {
						oldPathMissing = true
					}
				}

				// If the video was soft-deleted or its old path is missing, this is a moved/restored file
				if wasSoftDeleted || oldPathMissing {
					oldPath := existingVideo.StoredPath

					// Restore soft-deleted video first
					if wasSoftDeleted {
						if err := s.videoRepo.Restore(existingVideo.ID); err != nil {
							s.logger.Warn("Error restoring soft-deleted video",
								zap.Uint("video_id", existingVideo.ID),
								zap.Error(err),
							)
							errors++
							return nil
						}
					}

					// Update the stored path
					if err := s.videoRepo.UpdateStoredPath(existingVideo.ID, path, &storagePath.ID); err != nil {
						s.logger.Warn("Error updating moved video path",
							zap.Uint("video_id", existingVideo.ID),
							zap.String("old_path", oldPath),
							zap.String("new_path", path),
							zap.Error(err),
						)
						errors++
						return nil
					}

					// Re-index the video (it was removed from search when soft-deleted)
					if s.indexer != nil {
						existingVideo.StoredPath = path
						existingVideo.StoragePathID = &storagePath.ID
						existingVideo.DeletedAt.Valid = false // Clear for indexing
						if err := s.indexer.IndexVideo(existingVideo); err != nil {
							s.logger.Warn("Failed to re-index restored video",
								zap.Uint("video_id", existingVideo.ID),
								zap.Error(err),
							)
						}
					}

					videosMoved++
					s.logger.Info("Video file moved/restored detected",
						zap.Uint("video_id", existingVideo.ID),
						zap.String("old_path", oldPath),
						zap.String("new_path", path),
						zap.Bool("was_soft_deleted", wasSoftDeleted),
					)

					s.publishEvent("scan:video_moved", map[string]any{
						"video_id": existingVideo.ID,
						"old_path": oldPath,
						"new_path": path,
						"title":    existingVideo.Title,
					})

					return nil
				}
				// Old file still exists and video wasn't deleted - this is a copy, not a move. Create new record.
			}

			// Create new video record
			video, err := s.createVideoFromPath(path, &storagePath)
			if err != nil {
				s.logger.Warn("Error creating video from path",
					zap.String("path", path),
					zap.Error(err),
				)
				errors++
				return nil
			}

			videosAdded++

			// Publish video added event
			s.publishEvent("scan:video_added", map[string]any{
				"video_id":   video.ID,
				"video_path": path,
				"title":      video.Title,
			})

			// Send batched progress updates
			if filesFound%progressBatchSize == 0 || time.Since(lastProgressUpdate) > 2*time.Second {
				s.publishEvent("scan:progress", map[string]any{
					"files_found":     filesFound,
					"videos_added":    videosAdded,
					"videos_skipped":  videosSkipped,
					"videos_removed":  videosRemoved,
					"videos_moved":    videosMoved,
					"errors":          errors,
					"current_path":    storagePath.Path,
					"current_file":    currentFile,
				})
				lastProgressUpdate = time.Now()
			}

			return nil
		})

		if err != nil {
			if err == context.Canceled {
				s.completeScan(scan, "cancelled", "")
				return
			}
			s.logger.Error("Error scanning storage path",
				zap.String("path", storagePath.Path),
				zap.Error(err),
			)
			errors++
		}

		scan.PathsScanned++
	}

	// Update final stats
	scan.FilesFound = filesFound
	scan.VideosAdded = videosAdded
	scan.VideosSkipped = videosSkipped
	scan.VideosRemoved = videosRemoved
	scan.VideosMoved = videosMoved
	scan.Errors = errors

	s.completeScan(scan, "completed", "")
}

// detectMissingFiles checks all videos with storage paths and soft-deletes those whose files no longer exist
func (s *ScanService) detectMissingFiles(ctx context.Context, scan *data.ScanHistory, storagePaths []data.StoragePath) int {
	// Build a set of valid storage path prefixes
	validPrefixes := make(map[uint]string)
	for _, sp := range storagePaths {
		validPrefixes[sp.ID] = sp.Path
	}

	// Get all videos that have storage paths (excludes soft-deleted ones)
	videos, err := s.videoRepo.GetAllWithStoragePath()
	if err != nil {
		s.logger.Error("Failed to get videos for missing file detection", zap.Error(err))
		return 0
	}

	var videosRemoved int
	for _, video := range videos {
		select {
		case <-ctx.Done():
			return videosRemoved
		default:
		}

		// Skip videos without storage path ID (shouldn't happen but defensive)
		if video.StoragePathID == nil {
			continue
		}

		// Skip videos not in our scanned storage paths
		if _, ok := validPrefixes[*video.StoragePathID]; !ok {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(video.StoredPath); os.IsNotExist(err) {
			// File doesn't exist - soft-delete the video
			if err := s.videoRepo.MarkAsMissing(video.ID); err != nil {
				s.logger.Warn("Failed to soft-delete missing video",
					zap.Uint("video_id", video.ID),
					zap.String("stored_path", video.StoredPath),
					zap.Error(err),
				)
				continue
			}

			// Remove from search index
			if s.indexer != nil {
				if err := s.indexer.DeleteVideoIndex(video.ID); err != nil {
					s.logger.Warn("Failed to remove missing video from search index",
						zap.Uint("video_id", video.ID),
						zap.Error(err),
					)
				}
			}

			videosRemoved++
			s.logger.Info("Video file missing - soft deleted",
				zap.Uint("video_id", video.ID),
				zap.String("stored_path", video.StoredPath),
				zap.String("title", video.Title),
			)

			s.publishEvent("scan:video_removed", map[string]any{
				"video_id":   video.ID,
				"video_path": video.StoredPath,
				"title":      video.Title,
			})
		}
	}

	return videosRemoved
}

// createVideoFromPath creates a video record from a file path
func (s *ScanService) createVideoFromPath(path string, storagePath *data.StoragePath) (*data.Video, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	filename := filepath.Base(path)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	video := &data.Video{
		Title:            title,
		OriginalFilename: filename,
		StoredPath:       path,
		Size:             info.Size(),
		ProcessingStatus: "pending",
		Tags:             pq.StringArray{},
		Actors:           pq.StringArray{},
		StoragePathID:    &storagePath.ID,
	}

	modTime := info.ModTime()
	video.FileCreatedAt = &modTime

	if err := s.videoRepo.Create(video); err != nil {
		return nil, fmt.Errorf("failed to create video record: %w", err)
	}

	s.logger.Info("Video record created",
		zap.Uint("video_id", video.ID),
		zap.String("stored_path", video.StoredPath),
		zap.String("title", video.Title),
	)

	// Index video in search engine
	if s.indexer != nil {
		if err := s.indexer.IndexVideo(video); err != nil {
			s.logger.Warn("Failed to index video for search",
				zap.Uint("video_id", video.ID),
				zap.Error(err),
			)
		}
	}

	// Submit for processing synchronously - this is just a queue operation,
	// not the actual processing work, so it's safe to block briefly
	if s.processingService != nil {
		if err := s.processingService.SubmitVideo(video.ID, path); err != nil {
			s.logger.Warn("Failed to submit video for processing",
				zap.Uint("video_id", video.ID),
				zap.Error(err),
			)
			// Don't fail the scan - video is saved but processing won't start automatically
		}
	}

	return video, nil
}

// updateScanProgress updates the scan progress in the database
func (s *ScanService) updateScanProgress(scan *data.ScanHistory, currentPath, currentFile *string, pathsScanned, filesFound, videosAdded, videosSkipped, videosRemoved, videosMoved, errors int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scan.CurrentPath = currentPath
	scan.CurrentFile = currentFile
	scan.PathsScanned = pathsScanned
	scan.FilesFound = filesFound
	scan.VideosAdded = videosAdded
	scan.VideosSkipped = videosSkipped
	scan.VideosRemoved = videosRemoved
	scan.VideosMoved = videosMoved
	scan.Errors = errors

	if err := s.scanHistoryRepo.Update(scan); err != nil {
		s.logger.Warn("Failed to update scan progress", zap.Error(err))
	}
}

// completeScan marks the scan as complete
func (s *ScanService) completeScan(scan *data.ScanHistory, status string, errorMessage string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	scan.Status = status
	scan.CompletedAt = &now
	scan.CurrentPath = nil
	scan.CurrentFile = nil

	if errorMessage != "" {
		scan.ErrorMessage = &errorMessage
	}

	if err := s.scanHistoryRepo.Update(scan); err != nil {
		s.logger.Error("Failed to update scan completion status", zap.Error(err))
	}

	// Publish completion event
	eventType := "scan:completed"
	if status == "failed" {
		eventType = "scan:failed"
	} else if status == "cancelled" {
		eventType = "scan:cancelled"
	}

	s.publishEvent(eventType, scan)

	s.logger.Info("Scan completed",
		zap.Uint("scan_id", scan.ID),
		zap.String("status", status),
		zap.Int("files_found", scan.FilesFound),
		zap.Int("videos_added", scan.VideosAdded),
		zap.Int("videos_skipped", scan.VideosSkipped),
		zap.Int("videos_removed", scan.VideosRemoved),
		zap.Int("videos_moved", scan.VideosMoved),
		zap.Int("errors", scan.Errors),
	)
}

// publishEvent publishes an event to the event bus
func (s *ScanService) publishEvent(eventType string, data any) {
	if s.eventBus == nil {
		return
	}

	s.eventBus.Publish(VideoEvent{
		Type:    eventType,
		VideoID: 0, // Scan events are not video-specific
		Data:    data,
	})
}

// isVideoExtension checks if the extension is a valid video extension
func isVideoExtension(ext string) bool {
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".webm", ".wmv", ".m4v":
		return true
	default:
		return false
	}
}
