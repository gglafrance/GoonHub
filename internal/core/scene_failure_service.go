package core

import (
	"errors"
	"os"
	"sync"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// FailingPhaseSubmitter queues a phase job for a scene (implemented by SceneProcessingService).
type FailingPhaseSubmitter interface {
	SubmitPhaseWithPriority(sceneID uint, phase string, priority int) error
}

// failingStatWorkers bounds the parallel file stats when checking many counters.
const failingStatWorkers = 8

// SceneFailureService tracks the consecutive failures of each (scene, phase). A (scene, phase)
// is failing when its failures reach the threshold (processing.failing_threshold) and its file
// still has the size and modification time of the last failure: the "missing" bulk runs and the
// automatic retries skip it, the explicit actions (per-scene trigger, manual retry, the Failing
// view) still run it. It never touches a file: it only reads their size and modification time.
type SceneFailureService struct {
	repo      data.ScenePhaseFailureRepository
	sceneRepo data.SceneRepository
	threshold int
	logger    *zap.Logger

	// stat returns the size and modification time of a file (os.Stat; replaced in tests)
	stat func(path string) (int64, time.Time, error)

	submitterMu sync.RWMutex
	submitter   FailingPhaseSubmitter
}

// NewSceneFailureService creates the service with the threshold of the processing config.
func NewSceneFailureService(repo data.ScenePhaseFailureRepository, sceneRepo data.SceneRepository, cfg config.ProcessingConfig, logger *zap.Logger) *SceneFailureService {
	return &SceneFailureService{
		repo:      repo,
		sceneRepo: sceneRepo,
		threshold: cfg.FailingThreshold,
		logger:    logger.With(zap.String("component", "scene_failures")),
		stat:      osStat,
	}
}

func osStat(path string) (int64, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, time.Time{}, err
	}
	return info.Size(), info.ModTime(), nil
}

// SetSubmitter sets what queues the jobs of the "Retry" action.
func (s *SceneFailureService) SetSubmitter(submitter FailingPhaseSubmitter) {
	s.submitterMu.Lock()
	defer s.submitterMu.Unlock()
	s.submitter = submitter
}

func (s *SceneFailureService) getSubmitter() FailingPhaseSubmitter {
	s.submitterMu.RLock()
	defer s.submitterMu.RUnlock()
	return s.submitter
}

// Threshold returns the number of failures in a row from which a (scene, phase) is failing
// (0 or less: never).
func (s *SceneFailureService) Threshold() int {
	return s.threshold
}

// fileState returns the size and modification time of a file (microseconds, the precision of
// PostgreSQL), or nil when it cannot be read.
func (s *SceneFailureService) fileState(path string) (*int64, *time.Time) {
	if path == "" {
		return nil, nil
	}
	size, mod, err := s.stat(path)
	if err != nil {
		return nil, nil
	}
	mod = mod.UTC().Truncate(time.Microsecond)
	return &size, &mod
}

func sameInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// sameFile tells whether the file of the counter is unchanged since its last failure.
func (s *SceneFailureService) sameFile(row *data.ScenePhaseFailureRow) bool {
	size, mod := s.fileState(row.StoredPath)
	return sameInt64(size, row.FileSize) && sameTime(mod, row.FileModTime)
}

// overThreshold tells whether the counter reached the threshold (file not checked).
func (s *SceneFailureService) overThreshold(failures int) bool {
	return s.threshold > 0 && failures >= s.threshold
}

// failingRows keeps the counters whose file is unchanged, in the same order. The files are
// read in parallel by a bounded number of workers.
func (s *SceneFailureService) failingRows(rows []data.ScenePhaseFailureRow) []data.ScenePhaseFailureRow {
	keep := make([]bool, len(rows))
	indexes := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(failingStatWorkers, len(rows)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				keep[i] = s.sameFile(&rows[i])
			}
		}()
	}
	for i := range rows {
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	out := make([]data.ScenePhaseFailureRow, 0, len(rows))
	for i := range rows {
		if keep[i] {
			out = append(out, rows[i])
		}
	}
	return out
}

// RecordFailure adds a failure of the phase to the counter of the scene and tells whether the
// (scene, phase) is failing now. Errors are logged: tracking never stops the job handling.
func (s *SceneFailureService) RecordFailure(sceneID uint, phase, errMsg string) bool {
	rec := data.ScenePhaseFailureRecord{
		SceneID:  sceneID,
		Phase:    phase,
		Error:    errMsg,
		FailedAt: time.Now(),
	}
	if scene, err := s.sceneRepo.GetByID(sceneID); err == nil && scene != nil {
		rec.FileSize, rec.FileModTime = s.fileState(scene.StoredPath)
	}
	row, err := s.repo.RecordFailure(rec)
	if err != nil {
		s.logger.Warn("Failed to record the failure of a scene phase",
			zap.Uint("scene_id", sceneID), zap.String("phase", phase), zap.Error(err))
		return false
	}
	failing := s.overThreshold(row.Failures)
	if failing && row.Failures == s.threshold {
		s.logger.Info("Scene phase failing, no longer retried automatically",
			zap.Uint("scene_id", sceneID), zap.String("phase", phase), zap.Int("failures", row.Failures))
	}
	return failing
}

// ClearFailure forgets the failures of a (scene, phase), after a success.
func (s *SceneFailureService) ClearFailure(sceneID uint, phase string) {
	if err := s.repo.Clear(sceneID, phase); err != nil {
		s.logger.Warn("Failed to clear the failures of a scene phase",
			zap.Uint("scene_id", sceneID), zap.String("phase", phase), zap.Error(err))
	}
}

// IsFailing tells whether the (scene, phase) is failing (automatic retries skip it).
func (s *SceneFailureService) IsFailing(sceneID uint, phase string) (bool, error) {
	if s.threshold <= 0 {
		return false, nil
	}
	row, err := s.repo.GetByScenePhase(sceneID, phase)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return s.overThreshold(row.Failures) && s.sameFile(row), nil
}

// FailingSceneIDs returns the scenes failing for the phase (a "missing" bulk run skips them).
func (s *SceneFailureService) FailingSceneIDs(phase string) (map[uint]bool, error) {
	ids := map[uint]bool{}
	if s.threshold <= 0 {
		return ids, nil
	}
	rows, err := s.repo.List(data.ScenePhaseFailureFilter{MinFailures: s.threshold, Phase: phase})
	if err != nil {
		return nil, err
	}
	for _, row := range s.failingRows(rows) {
		ids[row.SceneID] = true
	}
	return ids, nil
}

// FailingSceneItem is a failing (scene, phase) of the Failing view.
type FailingSceneItem struct {
	ID           uint      `json:"id"`
	SceneID      uint      `json:"scene_id"`
	SceneTitle   string    `json:"scene_title"`
	StoredPath   string    `json:"stored_path"`
	Phase        string    `json:"phase"`
	Failures     int       `json:"failures"`
	LastError    string    `json:"last_error"`
	LastFailedAt time.Time `json:"last_failed_at"`
	FileSize     *int64    `json:"file_size"`
}

// FailingSceneList is a page of the Failing view. Totals counts per phase the failing
// (scene, phase) matching the error filter, whatever the phase filter.
type FailingSceneList struct {
	Items     []FailingSceneItem `json:"data"`
	Total     int                `json:"total"`
	Page      int                `json:"page"`
	Limit     int                `json:"limit"`
	Totals    map[string]int     `json:"totals"`
	Threshold int                `json:"threshold"`
}

// FailingFilter selects failing (scene, phase): an optional phase and error text.
type FailingFilter struct {
	Phase      string
	ErrorQuery string
}

// listFailing returns the failing counters matching the error filter (all phases).
func (s *SceneFailureService) listFailing(errorQuery string) ([]data.ScenePhaseFailureRow, error) {
	if s.threshold <= 0 {
		return nil, nil
	}
	rows, err := s.repo.List(data.ScenePhaseFailureFilter{MinFailures: s.threshold, ErrorQuery: errorQuery})
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list failing scenes", err)
	}
	return s.failingRows(rows), nil
}

// List returns a page of the failing (scene, phase), most recent failure first.
func (s *SceneFailureService) List(filter FailingFilter, page, limit int) (*FailingSceneList, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	rows, err := s.listFailing(filter.ErrorQuery)
	if err != nil {
		return nil, err
	}
	totals := map[string]int{}
	matching := make([]data.ScenePhaseFailureRow, 0, len(rows))
	for _, row := range rows {
		totals[row.Phase]++
		if filter.Phase == "" || row.Phase == filter.Phase {
			matching = append(matching, row)
		}
	}
	start := min((page-1)*limit, len(matching))
	end := min(start+limit, len(matching))
	items := make([]FailingSceneItem, 0, end-start)
	for _, row := range matching[start:end] {
		items = append(items, FailingSceneItem{
			ID:           row.ID,
			SceneID:      row.SceneID,
			SceneTitle:   row.SceneTitle,
			StoredPath:   row.StoredPath,
			Phase:        row.Phase,
			Failures:     row.Failures,
			LastError:    row.LastError,
			LastFailedAt: row.LastFailedAt,
			FileSize:     row.FileSize,
		})
	}
	return &FailingSceneList{
		Items:     items,
		Total:     len(matching),
		Page:      page,
		Limit:     limit,
		Totals:    totals,
		Threshold: s.threshold,
	}, nil
}

// FailingIDs returns the IDs of every failing (scene, phase) matching the filter (bulk actions
// on "all matching").
func (s *SceneFailureService) FailingIDs(filter FailingFilter) ([]uint, error) {
	rows, err := s.listFailing(filter.ErrorQuery)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		if filter.Phase == "" || row.Phase == filter.Phase {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

// FailingActionResult is the outcome of a Retry or Clear action.
type FailingActionResult struct {
	Retried int   `json:"retried"`
	Cleared int64 `json:"cleared"`
	Errors  int   `json:"errors"`
}

// Retry forgets the counters and queues a job of their phase for their scene (elevated
// priority, as the other manual retries). IDs that no longer exist are ignored.
func (s *SceneFailureService) Retry(ids []uint) (*FailingActionResult, error) {
	submitter := s.getSubmitter()
	if submitter == nil {
		return nil, apperrors.NewInternalError("processing service not configured", nil)
	}
	rows, err := s.repo.GetByIDs(ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load failing scenes", err)
	}
	result := &FailingActionResult{}
	for _, row := range rows {
		if err := s.repo.Clear(row.SceneID, row.Phase); err != nil {
			s.logger.Error("Failed to clear the failures before a retry",
				zap.Uint("scene_id", row.SceneID), zap.String("phase", row.Phase), zap.Error(err))
			result.Errors++
			continue
		}
		result.Cleared++
		if err := submitter.SubmitPhaseWithPriority(row.SceneID, row.Phase, 1); err != nil {
			s.logger.Error("Failed to queue the retry of a failing scene",
				zap.Uint("scene_id", row.SceneID), zap.String("phase", row.Phase), zap.Error(err))
			result.Errors++
			continue
		}
		result.Retried++
	}
	s.logger.Info("Retried failing scenes",
		zap.Int("requested", len(ids)), zap.Int("retried", result.Retried), zap.Int("errors", result.Errors))
	return result, nil
}

// Clear forgets the counters without queuing anything.
func (s *SceneFailureService) Clear(ids []uint) (*FailingActionResult, error) {
	cleared, err := s.repo.DeleteByIDs(ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to clear failing scenes", err)
	}
	s.logger.Info("Cleared failing scenes", zap.Int("requested", len(ids)), zap.Int64("cleared", cleared))
	return &FailingActionResult{Cleared: cleared}, nil
}
