package core

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"
)

const (
	// scanBatchSize is the number of new scenes to collect before flushing to DB
	scanBatchSize = 50
	// progressDBInterval is the minimum interval between DB progress writes
	progressDBInterval = 2 * time.Second
	// progressEventInterval is the minimum interval between SSE progress events
	progressEventInterval = 2 * time.Second
	// progressEventBatchSize is the number of files between SSE progress events
	progressEventBatchSize = 100
)

// ScanStatus represents the current state of a scan operation
type ScanStatus struct {
	Running     bool             `json:"running"`
	CurrentScan *data.ScanHistory `json:"current_scan,omitempty"`
}

// pendingScene holds data for a new scene that has not yet been flushed to DB
type pendingScene struct {
	scene       *data.Scene
	storagePath string
}

// scanLookupIndex provides in-memory lookup structures built once before a scan
type scanLookupIndex struct {
	// knownPaths is the set of stored_path values for non-deleted scenes
	knownPaths map[string]struct{}
	// lookupByKey maps "size:filename" -> []ScanLookupEntry for move detection
	lookupByKey map[string][]data.ScanLookupEntry
}

func buildScanLookupKey(size int64, filename string) string {
	return fmt.Sprintf("%d:%s", size, filename)
}

// ScanService handles scanning storage paths for new scene files
type ScanService struct {
	storagePathService *StoragePathService
	sceneRepo          data.SceneRepository
	scanHistoryRepo    data.ScanHistoryRepository
	processingService  *SceneProcessingService
	eventBus           *EventBus
	logger             *zap.Logger
	indexer            SceneIndexer

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

// buildLookupIndex pre-loads all scene path and size/filename data into memory
// so that the walk loop can do in-memory lookups instead of per-file DB queries.
func (s *ScanService) buildLookupIndex() (*scanLookupIndex, error) {
	// Load known paths (non-deleted scenes)
	knownPaths, err := s.sceneRepo.GetAllStoredPathSet()
	if err != nil {
		return nil, fmt.Errorf("failed to load stored path set: %w", err)
	}

	// Load scan lookup entries (all scenes, including soft-deleted) for move detection
	entries, err := s.sceneRepo.GetScanLookupEntries()
	if err != nil {
		return nil, fmt.Errorf("failed to load scan lookup entries: %w", err)
	}

	lookupByKey := make(map[string][]data.ScanLookupEntry, len(entries))
	for _, e := range entries {
		key := buildScanLookupKey(e.Size, e.OriginalFilename)
		lookupByKey[key] = append(lookupByKey[key], e)
	}

	s.logger.Info("Scan lookup index built",
		zap.Int("known_paths", len(knownPaths)),
		zap.Int("lookup_entries", len(entries)),
	)

	return &scanLookupIndex{
		knownPaths:  knownPaths,
		lookupByKey: lookupByKey,
	}, nil
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

	// Pre-load lookup data into memory (eliminates ~80k+ per-file DB queries)
	lookupIdx, err := s.buildLookupIndex()
	if err != nil {
		s.completeScan(scan, "failed", fmt.Sprintf("failed to build lookup index: %v", err))
		return
	}

	var filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, scanErrors int
	lastProgressDBWrite := time.Now()
	lastProgressEvent := time.Now()

	// Phase 1: Detect missing files (scenes whose source files no longer exist)
	scenesRemoved = s.detectMissingFiles(ctx, scan, paths)
	if ctx.Err() != nil {
		s.completeScan(scan, "cancelled", "")
		return
	}

	// Pending batch for new scenes
	var pendingBatch []pendingScene

	// flushBatch writes pending scenes to DB, indexes them, and submits for processing
	flushBatch := func() {
		if len(pendingBatch) == 0 {
			return
		}

		batch := pendingBatch
		pendingBatch = nil

		// Collect scene pointers for batch create
		scenes := make([]*data.Scene, len(batch))
		for i := range batch {
			scenes[i] = batch[i].scene
		}

		// Batch create in DB
		if err := s.sceneRepo.CreateInBatches(scenes, scanBatchSize); err != nil {
			s.logger.Error("Failed to batch create scenes", zap.Error(err), zap.Int("count", len(scenes)))
			scanErrors += len(batch)
			return
		}

		// Add newly created paths to the lookup index so duplicates within
		// the same scan are correctly skipped
		for _, sc := range scenes {
			lookupIdx.knownPaths[sc.StoredPath] = struct{}{}
		}

		// Log each created scene and publish events
		for _, sc := range scenes {
			s.logger.Info("Scene record created",
				zap.Uint("scene_id", sc.ID),
				zap.String("stored_path", sc.StoredPath),
				zap.String("title", sc.Title),
			)
			s.publishEvent("scan:scene_added", map[string]any{
				"scene_id":   sc.ID,
				"scene_path": sc.StoredPath,
				"title":      sc.Title,
			})
		}

		// Batch index in search engine
		if s.indexer != nil {
			sceneValues := make([]data.Scene, len(scenes))
			for i, sc := range scenes {
				sceneValues[i] = *sc
			}
			if err := s.indexer.BulkUpdateSceneIndex(sceneValues); err != nil {
				s.logger.Warn("Failed to batch index scenes for search", zap.Error(err), zap.Int("count", len(scenes)))
			}
		}

		// Submit for processing
		if s.processingService != nil {
			for _, sc := range scenes {
				if err := s.processingService.SubmitScene(sc.ID, sc.StoredPath); err != nil {
					s.logger.Warn("Failed to submit scene for processing",
						zap.Uint("scene_id", sc.ID),
						zap.Error(err),
					)
				}
			}
		}
	}

	for _, storagePath := range paths {
		select {
		case <-ctx.Done():
			// Flush any remaining pending scenes before cancelling
			flushBatch()
			s.completeScan(scan, "cancelled", "")
			return
		default:
		}

		// Update current path (in-memory only, DB write is batched)
		s.updateScanProgressInMemory(scan, &storagePath.Path, nil, scan.PathsScanned, filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, scanErrors)

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
				scanErrors++
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

			// Batched progress: update in-memory always, write to DB periodically
			s.updateScanProgressInMemory(scan, &storagePath.Path, &currentFile, scan.PathsScanned, filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, scanErrors)
			if time.Since(lastProgressDBWrite) > progressDBInterval {
				s.flushScanProgressToDB(scan)
				lastProgressDBWrite = time.Now()
			}

			// In-memory check: does scene already exist at this path?
			if _, exists := lookupIdx.knownPaths[path]; exists {
				scenesSkipped++
				return nil
			}

			// Get file info from DirEntry (cached, no extra syscall)
			info, err := d.Info()
			if err != nil {
				s.logger.Warn("Error getting file info",
					zap.String("path", path),
					zap.Error(err),
				)
				scanErrors++
				return nil
			}

			// In-memory move detection: check if size+filename matches a known scene
			filename := filepath.Base(path)
			lookupKey := buildScanLookupKey(info.Size(), filename)
			if candidates, ok := lookupIdx.lookupByKey[lookupKey]; ok {
				if handled := s.handleMovedFile(candidates, path, info, &storagePath, &scenesMoved, &scanErrors); handled {
					// Also add the new path to knownPaths so we don't re-process it
					lookupIdx.knownPaths[path] = struct{}{}
					return nil
				}
			}

			// New scene: build record and add to pending batch
			scene := s.buildSceneRecord(path, info, &storagePath)
			pendingBatch = append(pendingBatch, pendingScene{scene: scene, storagePath: storagePath.Path})
			scenesAdded++

			// Flush batch if it's full
			if len(pendingBatch) >= scanBatchSize {
				flushBatch()
			}

			// Send batched SSE progress events
			if filesFound%progressEventBatchSize == 0 || time.Since(lastProgressEvent) > progressEventInterval {
				s.publishEvent("scan:progress", map[string]any{
					"files_found":    filesFound,
					"scenes_added":   scenesAdded,
					"scenes_skipped": scenesSkipped,
					"scenes_removed": scenesRemoved,
					"scenes_moved":   scenesMoved,
					"errors":         scanErrors,
					"current_path":   storagePath.Path,
					"current_file":   currentFile,
				})
				lastProgressEvent = time.Now()
			}

			return nil
		})

		if err != nil {
			if err == context.Canceled {
				flushBatch()
				s.completeScan(scan, "cancelled", "")
				return
			}
			s.logger.Error("Error scanning storage path",
				zap.String("path", storagePath.Path),
				zap.Error(err),
			)
			scanErrors++
		}

		scan.PathsScanned++
	}

	// Flush any remaining pending scenes
	flushBatch()

	// Update final stats
	scan.FilesFound = filesFound
	scan.VideosAdded = scenesAdded
	scan.VideosSkipped = scenesSkipped
	scan.VideosRemoved = scenesRemoved
	scan.VideosMoved = scenesMoved
	scan.Errors = scanErrors

	s.completeScan(scan, "completed", "")
}

// handleMovedFile checks lookup candidates and handles a moved/restored file.
// Returns true if the file was handled as a move (caller should skip creation).
func (s *ScanService) handleMovedFile(candidates []data.ScanLookupEntry, newPath string, info fs.FileInfo, storagePath *data.StoragePath, scenesMoved, scanErrors *int) bool {
	for _, candidate := range candidates {
		wasSoftDeleted := candidate.IsDeleted
		oldPathMissing := false
		if !wasSoftDeleted {
			if _, statErr := os.Stat(candidate.StoredPath); os.IsNotExist(statErr) {
				oldPathMissing = true
			}
		}

		if !wasSoftDeleted && !oldPathMissing {
			continue // Old file still exists - this is a copy, not a move
		}

		oldPath := candidate.StoredPath

		// Restore soft-deleted scene first
		if wasSoftDeleted {
			if err := s.sceneRepo.Restore(candidate.ID); err != nil {
				s.logger.Warn("Error restoring soft-deleted scene",
					zap.Uint("scene_id", candidate.ID),
					zap.Error(err),
				)
				*scanErrors++
				return true
			}
		}

		// Update the stored path
		if err := s.sceneRepo.UpdateStoredPath(candidate.ID, newPath, &storagePath.ID); err != nil {
			s.logger.Warn("Error updating moved scene path",
				zap.Uint("scene_id", candidate.ID),
				zap.String("old_path", oldPath),
				zap.String("new_path", newPath),
				zap.Error(err),
			)
			*scanErrors++
			return true
		}

		// Re-index the scene
		if s.indexer != nil {
			// Fetch full scene for indexing (moved files are rare, so individual fetch is acceptable)
			if scene, err := s.sceneRepo.GetByID(candidate.ID); err == nil {
				if err := s.indexer.IndexScene(scene); err != nil {
					s.logger.Warn("Failed to re-index restored scene",
						zap.Uint("scene_id", candidate.ID),
						zap.Error(err),
					)
				}
			}
		}

		*scenesMoved++
		s.logger.Info("Scene file moved/restored detected",
			zap.Uint("scene_id", candidate.ID),
			zap.String("old_path", oldPath),
			zap.String("new_path", newPath),
			zap.Bool("was_soft_deleted", wasSoftDeleted),
		)

		s.publishEvent("scan:scene_moved", map[string]any{
			"scene_id": candidate.ID,
			"old_path": oldPath,
			"new_path": newPath,
		})

		return true
	}

	return false
}

// detectMissingFiles checks all scenes with storage paths and soft-deletes those whose files no longer exist.
// Uses lightweight ScenePathInfo instead of full Scene objects.
func (s *ScanService) detectMissingFiles(ctx context.Context, scan *data.ScanHistory, storagePaths []data.StoragePath) int {
	// Build a set of valid storage path IDs
	validPathIDs := make(map[uint]struct{})
	for _, sp := range storagePaths {
		validPathIDs[sp.ID] = struct{}{}
	}

	// Get lightweight scene path info (only id, stored_path, storage_path_id, title)
	sceneInfos, err := s.sceneRepo.GetScenePathsForMissingDetection()
	if err != nil {
		s.logger.Error("Failed to get scenes for missing file detection", zap.Error(err))
		return 0
	}

	var scenesRemoved int
	for _, info := range sceneInfos {
		select {
		case <-ctx.Done():
			return scenesRemoved
		default:
		}

		// Skip scenes not in our scanned storage paths
		if _, ok := validPathIDs[info.StoragePathID]; !ok {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(info.StoredPath); os.IsNotExist(err) {
			// File doesn't exist - soft-delete the scene
			if err := s.sceneRepo.MarkAsMissing(info.ID); err != nil {
				s.logger.Warn("Failed to soft-delete missing scene",
					zap.Uint("scene_id", info.ID),
					zap.String("stored_path", info.StoredPath),
					zap.Error(err),
				)
				continue
			}

			// Remove from search index
			if s.indexer != nil {
				if err := s.indexer.DeleteSceneIndex(info.ID); err != nil {
					s.logger.Warn("Failed to remove missing scene from search index",
						zap.Uint("scene_id", info.ID),
						zap.Error(err),
					)
				}
			}

			scenesRemoved++
			s.logger.Info("Scene file missing - soft deleted",
				zap.Uint("scene_id", info.ID),
				zap.String("stored_path", info.StoredPath),
				zap.String("title", info.Title),
			)

			s.publishEvent("scan:scene_removed", map[string]any{
				"scene_id":   info.ID,
				"scene_path": info.StoredPath,
				"title":      info.Title,
			})
		}
	}

	return scenesRemoved
}

// buildSceneRecord creates a Scene struct from file path and info without writing to DB.
func (s *ScanService) buildSceneRecord(path string, info fs.FileInfo, storagePath *data.StoragePath) *data.Scene {
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

	return scene
}

// updateScanProgressInMemory updates the in-memory scan state without writing to DB.
// This allows status queries to return current progress while batching DB writes.
func (s *ScanService) updateScanProgressInMemory(scan *data.ScanHistory, currentPath, currentFile *string, pathsScanned, filesFound, scenesAdded, scenesSkipped, scenesRemoved, scenesMoved, errors int) {
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
}

// flushScanProgressToDB writes the current in-memory scan state to the database.
func (s *ScanService) flushScanProgressToDB(scan *data.ScanHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
