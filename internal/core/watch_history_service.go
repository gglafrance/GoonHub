package core

import (
	"errors"
	"fmt"
	"goonhub/internal/data"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WatchHistoryService struct {
	repo      data.WatchHistoryRepository
	sceneRepo data.SceneRepository
	indexer   SceneIndexer
	logger    *zap.Logger
}

func NewWatchHistoryService(repo data.WatchHistoryRepository, sceneRepo data.SceneRepository, indexer SceneIndexer, logger *zap.Logger) *WatchHistoryService {
	return &WatchHistoryService{
		repo:      repo,
		sceneRepo: sceneRepo,
		indexer:   indexer,
		logger:    logger,
	}
}

type WatchHistoryEntry struct {
	Watch data.UserSceneWatch `json:"watch"`
	Scene *data.Scene         `json:"scene,omitempty"`
}

// RecordWatch records a watch event and increments view count if not viewed in last 24h.
// Uses atomic database operations to prevent race conditions from concurrent requests.
func (s *WatchHistoryService) RecordWatch(userID, sceneID uint, duration, position int, completed bool) error {
	// Verify scene exists
	_, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("scene not found")
		}
		return fmt.Errorf("failed to verify scene: %w", err)
	}

	// Record the watch session
	if err := s.repo.RecordWatch(userID, sceneID, duration, position, completed); err != nil {
		s.logger.Error("Failed to record watch",
			zap.Uint("user_id", userID),
			zap.Uint("scene_id", sceneID),
			zap.Error(err),
		)
		return err
	}

	// Atomically try to increment view count (handles 24h deduplication)
	incremented, err := s.repo.TryIncrementViewCount(userID, sceneID)
	if err != nil {
		s.logger.Warn("Failed to increment view count",
			zap.Uint("scene_id", sceneID),
			zap.Error(err),
		)
		// Don't fail the request for this
	} else if incremented {
		s.logger.Debug("Incremented view count",
			zap.Uint("scene_id", sceneID),
			zap.Uint("user_id", userID),
		)
		// Update search index with new view count
		if s.indexer != nil {
			scene, err := s.sceneRepo.GetByID(sceneID)
			if err == nil {
				if err := s.indexer.UpdateSceneIndex(scene); err != nil {
					s.logger.Warn("Failed to update scene in search index after view count increment",
						zap.Uint("scene_id", sceneID),
						zap.Error(err),
					)
				}
			}
		}
	}

	return nil
}

// GetResumePosition returns the position to resume from, or 0 if completed or not watched
func (s *WatchHistoryService) GetResumePosition(userID, sceneID uint) (int, error) {
	watch, err := s.repo.GetLastWatch(userID, sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}

	// If completed, don't resume
	if watch.Completed {
		return 0, nil
	}

	return watch.LastPosition, nil
}

// GetUserHistory returns paginated watch history with scene details
func (s *WatchHistoryService) GetUserHistory(userID uint, page, limit int) ([]WatchHistoryEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	watches, total, err := s.repo.ListUserHistory(userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	// Collect scene IDs
	sceneIDs := make([]uint, 0, len(watches))
	for _, w := range watches {
		sceneIDs = append(sceneIDs, w.SceneID)
	}

	// Fetch scenes
	scenes, err := s.sceneRepo.GetByIDs(sceneIDs)
	if err != nil {
		s.logger.Warn("Failed to fetch scenes for history",
			zap.Error(err),
		)
		// Continue without scene details
		scenes = nil
	}

	// Create scene map
	sceneMap := make(map[uint]*data.Scene)
	for i := range scenes {
		sceneMap[scenes[i].ID] = &scenes[i]
	}

	// Build result
	entries := make([]WatchHistoryEntry, 0, len(watches))
	for _, w := range watches {
		entry := WatchHistoryEntry{
			Watch: w,
			Scene: sceneMap[w.SceneID],
		}
		entries = append(entries, entry)
	}

	return entries, total, nil
}

// GetSceneHistory returns watch sessions for a specific scene
func (s *WatchHistoryService) GetSceneHistory(userID, sceneID uint, limit int) ([]data.UserSceneWatch, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ListSceneWatches(userID, sceneID, limit)
}

// computeSinceTime converts a day count to a start time.
// 0 means all time (uses year 2000 as sentinel).
func computeSinceTime(rangeDays int) time.Time {
	if rangeDays <= 0 {
		return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Now().UTC().AddDate(0, 0, -rangeDays)
}

// GetUserHistoryByDateRange returns watch history entries within a date range, enriched with scene data.
func (s *WatchHistoryService) GetUserHistoryByDateRange(userID uint, rangeDays, limit int) ([]WatchHistoryEntry, error) {
	if limit <= 0 {
		limit = 2000
	}

	since := computeSinceTime(rangeDays)

	watches, err := s.repo.ListUserHistoryByDateRange(userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list history by date range: %w", err)
	}

	// Collect scene IDs
	sceneIDs := make([]uint, 0, len(watches))
	for _, w := range watches {
		sceneIDs = append(sceneIDs, w.SceneID)
	}

	// Fetch scenes
	scenes, err := s.sceneRepo.GetByIDs(sceneIDs)
	if err != nil {
		s.logger.Warn("Failed to fetch scenes for history",
			zap.Error(err),
		)
		scenes = nil
	}

	// Create scene map
	sceneMap := make(map[uint]*data.Scene)
	for i := range scenes {
		sceneMap[scenes[i].ID] = &scenes[i]
	}

	// Build result
	entries := make([]WatchHistoryEntry, 0, len(watches))
	for _, w := range watches {
		entry := WatchHistoryEntry{
			Watch: w,
			Scene: sceneMap[w.SceneID],
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetDailyActivity returns daily activity counts for a user within a date range.
func (s *WatchHistoryService) GetDailyActivity(userID uint, rangeDays int) ([]data.DailyActivityCount, error) {
	since := computeSinceTime(rangeDays)
	counts, err := s.repo.GetDailyActivityCounts(userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily activity counts: %w", err)
	}
	return counts, nil
}
