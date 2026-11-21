package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"syscall"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

const (
	// healthPathBatchSize is the number of scene paths read from the DB at a time by a task
	healthPathBatchSize = 1000
	// healthItemBatchSize is the number of check items buffered before they are written
	healthItemBatchSize = 500
	// healthReindexBatchSize is the number of scenes re-indexed at a time after fixing sizes
	healthReindexBatchSize = 100
	// healthProgressInterval is the minimum interval between progress writes to the DB
	healthProgressInterval = 2 * time.Second
	// healthDefaultConcurrency is used when the configured stat concurrency is not positive
	healthDefaultConcurrency = 4
	// healthMaxPageLimit caps the page size of the drill-down lists
	healthMaxPageLimit = 100
)

// Health preview actions that are not processing phases.
const (
	HealthActionMissingFiles = data.HealthCheckKindMissingFiles
	HealthActionFixSizes     = data.HealthCheckKindFixSizes
)

// healthPhases are the processing phases whose missing jobs the health page can queue.
var healthPhases = map[string]bool{
	"metadata":            true,
	"thumbnail":           true,
	"sprites":             true,
	"animated_thumbnails": true,
	"fingerprint":         true,
}

// HealthProcessingInfo is what the health service needs to know about processing.
// SceneProcessingService implements it.
type HealthProcessingInfo interface {
	IsDuplicationEnabled() bool
	GetProcessingQualityConfig() ProcessingQualityConfig
}

// StoragePathHealth is the health of one storage path: what the DB holds in it and the usage
// of its filesystem. ID is nil for the scenes in no storage path.
type StoragePathHealth struct {
	ID         *uint      `json:"id"`
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	SceneCount int64      `json:"scene_count"`
	TotalSize  int64      `json:"total_size"`
	DiskUsage  *DiskUsage `json:"disk_usage"`
}

// HealthTaskStatus is the state of the background health task, if one is running.
type HealthTaskStatus struct {
	Running bool                     `json:"running"`
	Task    *data.LibraryHealthCheck `json:"task,omitempty"`
}

// LibraryHealthSummary is the content of the library health page.
type LibraryHealthSummary struct {
	TotalScenes int64 `json:"total_scenes"`
	// Counts per category. missing_file and unreadable_file are absent until a missing-file
	// check has completed.
	Counts              map[string]int64         `json:"counts"`
	StoragePaths        []StoragePathHealth      `json:"storage_paths"`
	LastMissingCheck    *data.LibraryHealthCheck `json:"last_missing_check"`
	LastFixSizes        *data.LibraryHealthCheck `json:"last_fix_sizes"`
	Task                HealthTaskStatus         `json:"task"`
	FingerprintEnabled  bool                     `json:"fingerprint_enabled"`
	ScenePreviewEnabled bool                     `json:"scene_preview_enabled"`
}

// HealthActionPreview tells how many items an action from the health page would touch.
type HealthActionPreview struct {
	Action  string `json:"action"`
	Count   int64  `json:"count"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
}

// LibraryHealthService computes the library health figures and runs the background tasks
// that look at the media files: the missing-file check and the size fix. Both only read the
// files (stat); the only writes are to the database (results, and scene sizes for the fix).
type LibraryHealthService struct {
	repo         data.LibraryHealthRepository
	sceneRepo    data.SceneRepository
	storagePaths *StoragePathService
	processing   HealthProcessingInfo
	indexer      SceneIndexer
	concurrency  int
	logger       *zap.Logger
	// statFile looks at a file without opening it (os.Stat; replaced in tests)
	statFile func(name string) (fs.FileInfo, error)

	mu      sync.Mutex
	current *data.LibraryHealthCheck
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewLibraryHealthService creates the library health service. processing may be nil.
func NewLibraryHealthService(
	repo data.LibraryHealthRepository,
	sceneRepo data.SceneRepository,
	storagePaths *StoragePathService,
	processing HealthProcessingInfo,
	concurrency int,
	logger *zap.Logger,
) *LibraryHealthService {
	if concurrency <= 0 {
		concurrency = healthDefaultConcurrency
	}
	return &LibraryHealthService{
		repo:         repo,
		sceneRepo:    sceneRepo,
		storagePaths: storagePaths,
		processing:   processing,
		concurrency:  concurrency,
		logger:       logger.With(zap.String("component", "library_health")),
		statFile:     os.Stat,
	}
}

// SetIndexer sets the search indexer used to re-index scenes whose size was fixed.
func (s *LibraryHealthService) SetIndexer(indexer SceneIndexer) {
	s.indexer = indexer
}

// RecoverInterruptedChecks marks the tasks left running by a restart as failed.
func (s *LibraryHealthService) RecoverInterruptedChecks() {
	if err := s.repo.MarkInterruptedChecksFailed(); err != nil {
		s.logger.Error("Failed to recover interrupted library health tasks", zap.Error(err))
	}
}

// GetSummary returns the live counts, the last task results and the storage path usage.
func (s *LibraryHealthService) GetSummary() (*LibraryHealthSummary, error) {
	counts, total, err := s.repo.CountLiveCategories()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count library health categories", err)
	}

	lastMissing, err := s.repo.GetLatestCheck(data.HealthCheckKindMissingFiles, data.HealthCheckStatusCompleted)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get the last missing-file check", err)
	}
	if lastMissing != nil {
		itemCounts, err := s.repo.CountCheckItems(lastMissing.ID)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to count missing files", err)
		}
		counts[data.HealthCategoryMissingFile] = itemCounts[data.HealthItemReasonMissing]
		counts[data.HealthCategoryUnreadableFile] = itemCounts[data.HealthItemReasonUnreadable]
	}

	lastFix, err := s.repo.GetLatestCheck(data.HealthCheckKindFixSizes, "")
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get the last size fix", err)
	}

	storagePaths, err := s.storagePathHealth()
	if err != nil {
		return nil, err
	}

	summary := &LibraryHealthSummary{
		TotalScenes:      total,
		Counts:           counts,
		StoragePaths:     storagePaths,
		LastMissingCheck: lastMissing,
		LastFixSizes:     lastFix,
		Task:             s.GetTaskStatus(),
	}
	if s.processing != nil {
		summary.FingerprintEnabled = s.processing.IsDuplicationEnabled()
		summary.ScenePreviewEnabled = s.processing.GetProcessingQualityConfig().ScenePreviewEnabled
	}
	return summary, nil
}

func (s *LibraryHealthService) storagePathHealth() ([]StoragePathHealth, error) {
	paths, err := s.storagePaths.List()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list storage paths", err)
	}
	stats, err := s.repo.StoragePathSceneStats()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to group scenes by storage path", err)
	}

	byPath := make(map[uint]data.StoragePathSceneStats, len(stats))
	var orphan data.StoragePathSceneStats
	for _, st := range stats {
		if st.StoragePathID == nil {
			orphan.SceneCount += st.SceneCount
			orphan.TotalSize += st.TotalSize
			continue
		}
		byPath[*st.StoragePathID] = st
	}

	result := make([]StoragePathHealth, 0, len(paths)+1)
	known := make(map[uint]struct{}, len(paths))
	for _, p := range paths {
		id := p.ID
		known[id] = struct{}{}
		st := byPath[id]
		result = append(result, StoragePathHealth{
			ID:         &id,
			Name:       p.Name,
			Path:       p.Path,
			SceneCount: st.SceneCount,
			TotalSize:  st.TotalSize,
			DiskUsage:  s.storagePaths.GetDiskUsage(p.Path),
		})
	}
	// Scenes pointing at a storage path that no longer exists count as in no storage path
	for id, st := range byPath {
		if _, ok := known[id]; !ok {
			orphan.SceneCount += st.SceneCount
			orphan.TotalSize += st.TotalSize
		}
	}
	if orphan.SceneCount > 0 {
		result = append(result, StoragePathHealth{
			Name:       "No storage path",
			SceneCount: orphan.SceneCount,
			TotalSize:  orphan.TotalSize,
		})
	}
	return result, nil
}

// ListScenes lists the scenes of a health category, paginated.
func (s *LibraryHealthService) ListScenes(category string, page, limit int) ([]data.HealthSceneRow, int64, error) {
	if !data.IsHealthCategory(category) {
		return nil, 0, apperrors.NewValidationErrorWithField("category", "unknown library health category: "+category)
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > healthMaxPageLimit {
		limit = healthMaxPageLimit
	}

	var checkID uint
	if category == data.HealthCategoryMissingFile || category == data.HealthCategoryUnreadableFile {
		last, err := s.repo.GetLatestCheck(data.HealthCheckKindMissingFiles, data.HealthCheckStatusCompleted)
		if err != nil {
			return nil, 0, apperrors.NewInternalError("failed to get the last missing-file check", err)
		}
		if last != nil {
			checkID = last.ID
		}
	}

	rows, total, err := s.repo.ListCategoryScenes(category, checkID, page, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to list scenes", err)
	}
	return rows, total, nil
}

// PreviewAction counts what an action would touch, so the page can state it before the
// admin confirms: the jobs a "missing" bulk run of a phase queues, the scenes whose size the
// fix re-reads, or the files the missing-file check stats.
func (s *LibraryHealthService) PreviewAction(action string) (*HealthActionPreview, error) {
	preview := &HealthActionPreview{Action: action, Enabled: true}
	var err error
	switch {
	case healthPhases[action]:
		if action == "fingerprint" && (s.processing == nil || !s.processing.IsDuplicationEnabled()) {
			preview.Enabled = false
			preview.Reason = "fingerprint processing requires duplication detection to be enabled"
		}
		preview.Count, err = s.repo.CountScenesNeedingPhase(action)
	case action == HealthActionFixSizes:
		preview.Count, err = s.repo.CountScenePaths(data.HealthScenesSizeZero)
	case action == HealthActionMissingFiles:
		preview.Count, err = s.repo.CountScenePaths(data.HealthScenesAll)
	default:
		return nil, apperrors.NewValidationErrorWithField("action", "unknown action: "+action)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count the scenes of the action", err)
	}
	return preview, nil
}

// GetTaskStatus returns the running task, if any.
func (s *LibraryHealthService) GetTaskStatus() HealthTaskStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return HealthTaskStatus{Running: false}
	}
	snapshot := *s.current
	return HealthTaskStatus{Running: true, Task: &snapshot}
}

func selectionForKind(kind string) (string, bool) {
	switch kind {
	case data.HealthCheckKindMissingFiles:
		return data.HealthScenesAll, true
	case data.HealthCheckKindFixSizes:
		return data.HealthScenesSizeZero, true
	default:
		return "", false
	}
}

// StartTask starts a background task (missing_files or fix_sizes). Only one task runs at a time.
func (s *LibraryHealthService) StartTask(kind string) (*data.LibraryHealthCheck, error) {
	selection, ok := selectionForKind(kind)
	if !ok {
		return nil, apperrors.NewValidationErrorWithField("kind", "unknown task: "+kind)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		return nil, apperrors.NewConflictError("library health task", "a library health task is already running")
	}

	total, err := s.repo.CountScenePaths(selection)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count scenes", err)
	}
	now := time.Now()
	check := &data.LibraryHealthCheck{
		Kind:      kind,
		Status:    data.HealthCheckStatusRunning,
		StartedAt: now,
		CreatedAt: now,
		Total:     int(total),
	}
	if err := s.repo.CreateCheck(check); err != nil {
		return nil, apperrors.NewInternalError("failed to record the task", err)
	}

	// Not derived from the request context: the task outlives the HTTP request
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.current = check
	s.cancel = cancel
	s.done = done

	s.logger.Info("Library health task started",
		zap.Uint("check_id", check.ID),
		zap.String("kind", kind),
		zap.Int64("total", total),
		zap.Int("concurrency", s.concurrency),
	)
	go s.runTask(ctx, check, selection, done)

	snapshot := *check
	return &snapshot, nil
}

// CancelTask cancels the running task.
func (s *LibraryHealthService) CancelTask() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || s.cancel == nil {
		return apperrors.NewValidationError("no library health task is running")
	}
	s.cancel()
	return nil
}

// wait blocks until the running task (if any) has finished. Used by tests.
func (s *LibraryHealthService) wait() {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}

// fileState is the outcome of looking at one scene file.
type fileState int

const (
	fileOK fileState = iota
	fileMissing
	fileUnreadable
)

type fileStatResult struct {
	path   data.HealthScenePath
	state  fileState
	size   int64
	detail string
}

// statScenePath looks at a scene file without opening it.
func (s *LibraryHealthService) statScenePath(p data.HealthScenePath) fileStatResult {
	result := fileStatResult{path: p}
	info, err := s.statFile(p.StoredPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			result.state = fileMissing
		} else {
			result.state = fileUnreadable
			result.detail = err.Error()
		}
		return result
	}
	if !info.Mode().IsRegular() {
		result.state = fileUnreadable
		result.detail = "not a regular file"
		return result
	}
	result.state = fileOK
	result.size = info.Size()
	return result
}

// runTask stats the files of the selection with a bounded number of workers and records the outcome.
func (s *LibraryHealthService) runTask(ctx context.Context, check *data.LibraryHealthCheck, selection string, done chan struct{}) {
	defer func() {
		s.mu.Lock()
		if s.cancel != nil {
			s.cancel() // releases the context of a task that ended on its own
		}
		s.current = nil
		s.cancel = nil
		s.done = nil
		s.mu.Unlock()
		close(done)
	}()

	paths := make(chan data.HealthScenePath)
	results := make(chan fileStatResult)
	feedErr := make(chan error, 1)

	go func() {
		defer close(paths)
		var afterID uint
		for {
			batch, err := s.repo.ListScenePaths(selection, afterID, healthPathBatchSize)
			if err != nil {
				feedErr <- err
				return
			}
			if len(batch) == 0 {
				return
			}
			for _, p := range batch {
				select {
				case paths <- p:
				case <-ctx.Done():
					return
				}
			}
			afterID = batch[len(batch)-1].ID
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range paths {
				result := s.statScenePath(p)
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var items []data.LibraryHealthCheckItem
	var fixedIDs []uint
	lastProgress := time.Now()

	finish := func(status, message string) {
		if len(items) > 0 && status == data.HealthCheckStatusCompleted {
			if err := s.repo.InsertCheckItems(items); err != nil {
				status = data.HealthCheckStatusFailed
				message = fmt.Sprintf("failed to save the results: %v", err)
			}
		}
		s.reindexScenes(fixedIDs)
		s.finishCheck(check, status, message)
	}

loop:
	for {
		select {
		case result, ok := <-results:
			if !ok {
				break loop
			}
			item, fixed, err := s.recordResult(check, result)
			if err != nil {
				finish(data.HealthCheckStatusFailed, err.Error())
				return
			}
			if fixed {
				fixedIDs = append(fixedIDs, result.path.ID)
			}
			if item != nil {
				items = append(items, *item)
				if len(items) >= healthItemBatchSize {
					if err := s.repo.InsertCheckItems(items); err != nil {
						finish(data.HealthCheckStatusFailed, fmt.Sprintf("failed to save the results: %v", err))
						return
					}
					items = nil
				}
			}
			if time.Since(lastProgress) > healthProgressInterval {
				s.saveProgress(check)
				lastProgress = time.Now()
			}
		case <-ctx.Done():
			break loop
		}
	}

	select {
	case err := <-feedErr:
		finish(data.HealthCheckStatusFailed, fmt.Sprintf("failed to read the scenes: %v", err))
		return
	default:
	}
	if ctx.Err() != nil {
		finish(data.HealthCheckStatusCancelled, "")
		return
	}
	finish(data.HealthCheckStatusCompleted, "")
}

// recordResult counts one file outcome. For a missing-file check it returns the item to store;
// for a size fix it updates the scene size and reports whether it did.
func (s *LibraryHealthService) recordResult(check *data.LibraryHealthCheck, result fileStatResult) (*data.LibraryHealthCheckItem, bool, error) {
	var item *data.LibraryHealthCheckItem
	fixed := false

	switch result.state {
	case fileMissing, fileUnreadable:
		reason := data.HealthItemReasonMissing
		if result.state == fileUnreadable {
			reason = data.HealthItemReasonUnreadable
		}
		if check.Kind == data.HealthCheckKindMissingFiles {
			item = &data.LibraryHealthCheckItem{
				CheckID:    check.ID,
				SceneID:    result.path.ID,
				StoredPath: result.path.StoredPath,
				Reason:     reason,
				Detail:     result.detail,
			}
		}
	case fileOK:
		if check.Kind == data.HealthCheckKindFixSizes && result.size > 0 {
			updated, err := s.repo.UpdateSceneSize(result.path.ID, result.path.StoredPath, result.size)
			if err != nil {
				return nil, false, fmt.Errorf("failed to update the size of scene %d: %w", result.path.ID, err)
			}
			fixed = updated
		}
	}

	s.mu.Lock()
	check.Checked++
	switch result.state {
	case fileMissing:
		check.Missing++
	case fileUnreadable:
		check.Unreadable++
	case fileOK:
		if check.Kind == data.HealthCheckKindFixSizes {
			if fixed {
				check.Fixed++
			} else {
				check.Unchanged++
			}
		}
	}
	s.mu.Unlock()

	return item, fixed, nil
}

func (s *LibraryHealthService) saveProgress(check *data.LibraryHealthCheck) {
	s.mu.Lock()
	snapshot := *check
	s.mu.Unlock()
	if err := s.repo.UpdateCheck(&snapshot); err != nil {
		s.logger.Warn("Failed to save library health task progress", zap.Error(err))
	}
}

func (s *LibraryHealthService) finishCheck(check *data.LibraryHealthCheck, status, message string) {
	s.mu.Lock()
	now := time.Now()
	check.Status = status
	check.CompletedAt = &now
	if message != "" {
		check.ErrorMessage = &message
	}
	snapshot := *check
	s.mu.Unlock()

	if err := s.repo.FinishCheck(&snapshot); err != nil {
		s.logger.Error("Failed to save library health task result", zap.Uint("check_id", check.ID), zap.Error(err))
	}
	s.logger.Info("Library health task finished",
		zap.Uint("check_id", snapshot.ID),
		zap.String("kind", snapshot.Kind),
		zap.String("status", status),
		zap.Int("checked", snapshot.Checked),
		zap.Int("missing", snapshot.Missing),
		zap.Int("unreadable", snapshot.Unreadable),
		zap.Int("fixed", snapshot.Fixed),
		zap.String("error", message),
	)
}

// reindexScenes refreshes the search index of the scenes whose size changed.
func (s *LibraryHealthService) reindexScenes(ids []uint) {
	if s.indexer == nil || s.sceneRepo == nil || len(ids) == 0 {
		return
	}
	for start := 0; start < len(ids); start += healthReindexBatchSize {
		end := start + healthReindexBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		scenes, err := s.sceneRepo.GetByIDs(ids[start:end])
		if err != nil {
			s.logger.Warn("Failed to load scenes to re-index", zap.Error(err))
			continue
		}
		if err := s.indexer.BulkUpdateSceneIndex(scenes); err != nil {
			s.logger.Warn("Failed to re-index scenes after fixing sizes", zap.Error(err), zap.Int("count", len(scenes)))
		}
	}
}
