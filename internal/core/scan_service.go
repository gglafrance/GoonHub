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

// ScanService handles scanning storage paths for new scene files
type ScanService struct {
	storagePathService  *StoragePathService
	sceneRepo           data.SceneRepository
	scanHistoryRepo     data.ScanHistoryRepository
	processingService   *SceneProcessingService
	eventBus            *EventBus
	logger              *zap.Logger
	indexer             SceneIndexer

	mu          sync.Mutex
	currentScan *data.ScanHistory
	cancelFunc  context.CancelFunc
}

// NewScanService creates a new scan service
func NewScanService(
	storagePathService *StoragePathService,
	sceneRepo data.SceneRepository,
	scanHistoryRepo data.ScanHistoryRepository,
	processingService *SceneProcessingService,
	eventBus *EventBus,
	logger *zap.Logger,
) *ScanService {
	return &ScanService{
		storagePathService: storagePathService,
		sceneRepo:          sceneRepo,
		scanHistoryRepo:    scanHistoryRepo,
		processingService:  processingService,
		eventBus:           eventBus,
		logger:             logger.With(zap.String("component", "scan_service")),
	}
}

// SetIndexer sets the scene indexer for search index updates
func (s *ScanService) SetIndexer(indexer SceneIndexer) {
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

	var filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, errors int
	lastProgressUpdate := time.Now()
	progressBatchSize := 100

	// Phase 1: Detect missing files (scenes whose source files no longer exist)
	scenesRemoved = s.detectMissingFiles(ctx, scan, paths)
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
		s.updateScanProgress(scan, &storagePath.Path, nil, scan.PathsScanned, filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, errors)

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
			s.updateScanProgress(scan, &storagePath.Path, &currentFile, scan.PathsScanned, filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, errors)

			// Check if scene already exists at this path
			exists, err := s.sceneRepo.ExistsByStoredPath(path)
			if err != nil {
				s.logger.Warn("Error checking scene existence",
					zap.String("path", path),
					zap.Error(err),
				)
				errors++
				return nil
			}

			if exists {
				scenesSkipped++
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
			existingScene, err := s.sceneRepo.GetBySizeAndFilename(info.Size(), filename)
			if err != nil {
				s.logger.Warn("Error checking for moved file",
					zap.String("path", path),
					zap.Error(err),
				)
				// Don't fail - continue with normal add flow
			}

			if existingScene != nil {
				// Check if scene was soft-deleted (marked as missing) or if old path doesn't exist
				wasSoftDeleted := existingScene.DeletedAt.Valid
				oldPathMissing := false
				if !wasSoftDeleted {
					if _, statErr := os.Stat(existingScene.StoredPath); os.IsNotExist(statErr) {
						oldPathMissing = true
					}
				}

				// If the scene was soft-deleted or its old path is missing, this is a moved/restored file
				if wasSoftDeleted || oldPathMissing {
					oldPath := existingScene.StoredPath

					// Restore soft-deleted scene first
					if wasSoftDeleted {
						if err := s.sceneRepo.Restore(existingScene.ID); err != nil {
							s.logger.Warn("Error restoring soft-deleted scene",
								zap.Uint("scene_id", existingScene.ID),
								zap.Error(err),
							)
							errors++
							return nil
						}
					}

					// Update the stored path
					if err := s.sceneRepo.UpdateStoredPath(existingScene.ID, path, &storagePath.ID); err != nil {
						s.logger.Warn("Error updating moved scene path",
							zap.Uint("scene_id", existingScene.ID),
							zap.String("old_path", oldPath),
							zap.String("new_path", path),
							zap.Error(err),
						)
						errors++
						return nil
					}

					// Re-index the scene (it was removed from search when soft-deleted)
					if s.indexer != nil {
						existingScene.StoredPath = path
						existingScene.StoragePathID = &storagePath.ID
						existingScene.DeletedAt.Valid = false // Clear for indexing
						if err := s.indexer.IndexScene(existingScene); err != nil {
							s.logger.Warn("Failed to re-index restored scene",
								zap.Uint("scene_id", existingScene.ID),
								zap.Error(err),
							)
						}
					}

					scenesMoved++
					s.logger.Info("Scene file moved/restored detected",
						zap.Uint("scene_id", existingScene.ID),
						zap.String("old_path", oldPath),
						zap.String("new_path", path),
						zap.Bool("was_soft_deleted", wasSoftDeleted),
					)

					s.publishEvent("scan:scene_moved", map[string]any{
						"scene_id": existingScene.ID,
						"old_path": oldPath,
						"new_path": path,
						"title":    existingScene.Title,
					})

					return nil
				}
				// Old file still exists and scene wasn't deleted - this is a copy, not a move. Create new record.
			}

			// Create new scene record
			scene, err := s.createSceneFromPath(path, &storagePath)
			if err != nil {
				s.logger.Warn("Error creating scene from path",
					zap.String("path", path),
					zap.Error(err),
				)
				errors++
				return nil
			}

			scenesAdded++

			// Publish scene added event
			s.publishEvent("scan:scene_added", map[string]any{
				"scene_id":   scene.ID,
				"scene_path": path,
				"title":      scene.Title,
			})

			// Send batched progress updates
			if filesFound%progressBatchSize == 0 || time.Since(lastProgressUpdate) > 2*time.Second {
				s.publishEvent("scan:progress", map[string]any{
					"files_found":    filesFound,
					"scenes_added":   scenesAdded,
					"scenes_skipped": scenesSkipped,
					"scenes_removed": scenesRemoved,
					"scenes_moved":   scenesMoved,
					"errors":         errors,
					"current_path":   storagePath.Path,
					"current_file":   currentFile,
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
	scan.VideosAdded = scenesAdded
	scan.VideosSkipped = scenesSkipped
	scan.VideosRemoved = scenesRemoved
	scan.VideosMoved = scenesMoved
	scan.Errors = errors

	s.completeScan(scan, "completed", "")
}

// detectMissingFiles checks all scenes with storage paths and soft-deletes those whose files no longer exist
func (s *ScanService) detectMissingFiles(ctx context.Context, scan *data.ScanHistory, storagePaths []data.StoragePath) int {
	// Build a set of valid storage path prefixes
	validPrefixes := make(map[uint]string)
	for _, sp := range storagePaths {
		validPrefixes[sp.ID] = sp.Path
	}

	// Get all scenes that have storage paths (excludes soft-deleted ones)
	scenes, err := s.sceneRepo.GetAllWithStoragePath()
	if err != nil {
		s.logger.Error("Failed to get scenes for missing file detection", zap.Error(err))
		return 0
	}

	var scenesRemoved int
	for _, scene := range scenes {
		select {
		case <-ctx.Done():
			return scenesRemoved
		default:
		}

		// Skip scenes without storage path ID (shouldn't happen but defensive)
		if scene.StoragePathID == nil {
			continue
		}

		// Skip scenes not in our scanned storage paths
		if _, ok := validPrefixes[*scene.StoragePathID]; !ok {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(scene.StoredPath); os.IsNotExist(err) {
			// File doesn't exist - soft-delete the scene
			if err := s.sceneRepo.MarkAsMissing(scene.ID); err != nil {
				s.logger.Warn("Failed to soft-delete missing scene",
					zap.Uint("scene_id", scene.ID),
					zap.String("stored_path", scene.StoredPath),
					zap.Error(err),
				)
				continue
			}

			// Remove from search index
			if s.indexer != nil {
				if err := s.indexer.DeleteSceneIndex(scene.ID); err != nil {
					s.logger.Warn("Failed to remove missing scene from search index",
						zap.Uint("scene_id", scene.ID),
						zap.Error(err),
					)
				}
			}

			scenesRemoved++
			s.logger.Info("Scene file missing - soft deleted",
				zap.Uint("scene_id", scene.ID),
				zap.String("stored_path", scene.StoredPath),
				zap.String("title", scene.Title),
			)

			s.publishEvent("scan:scene_removed", map[string]any{
				"scene_id":   scene.ID,
				"scene_path": scene.StoredPath,
				"title":      scene.Title,
			})
		}
	}

	return scenesRemoved
}

// createSceneFromPath creates a scene record from a file path
func (s *ScanService) createSceneFromPath(path string, storagePath *data.StoragePath) (*data.Scene, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	filename := filepath.Base(path)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	scene := &data.Scene{
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
	scene.FileCreatedAt = &modTime

	if err := s.sceneRepo.Create(scene); err != nil {
		return nil, fmt.Errorf("failed to create scene record: %w", err)
	}

	s.logger.Info("Scene record created",
		zap.Uint("scene_id", scene.ID),
		zap.String("stored_path", scene.StoredPath),
		zap.String("title", scene.Title),
	)

	// Index scene in search engine
	if s.indexer != nil {
		if err := s.indexer.IndexScene(scene); err != nil {
			s.logger.Warn("Failed to index scene for search",
				zap.Uint("scene_id", scene.ID),
				zap.Error(err),
			)
		}
	}

	// Submit for processing synchronously - this is just a queue operation,
	// not the actual processing work, so it's safe to block briefly
	if s.processingService != nil {
		if err := s.processingService.SubmitScene(scene.ID, path); err != nil {
			s.logger.Warn("Failed to submit scene for processing",
				zap.Uint("scene_id", scene.ID),
				zap.Error(err),
			)
			// Don't fail the scan - scene is saved but processing won't start automatically
		}
	}

	return scene, nil
}

// updateScanProgress updates the scan progress in the database
func (s *ScanService) updateScanProgress(scan *data.ScanHistory, currentPath, currentFile *string, pathsScanned, filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, errors int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scan.CurrentPath = currentPath
	scan.CurrentFile = currentFile
	scan.PathsScanned = pathsScanned
	scan.FilesFound = filesFound
	scan.VideosAdded = scenesAdded
	scan.VideosSkipped = scenesSkipped
	scan.VideosRemoved = scenesRemoved
	scan.VideosMoved = scenesMoved
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
		zap.Int("scenes_added", scan.VideosAdded),
		zap.Int("scenes_skipped", scan.VideosSkipped),
		zap.Int("scenes_removed", scan.VideosRemoved),
		zap.Int("scenes_moved", scan.VideosMoved),
		zap.Int("errors", scan.Errors),
	)
}

// publishEvent publishes an event to the event bus
func (s *ScanService) publishEvent(eventType string, data any) {
	if s.eventBus == nil {
		return
	}

	s.eventBus.Publish(SceneEvent{
		Type:    eventType,
		SceneID: 0, // Scan events are not scene-specific
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
