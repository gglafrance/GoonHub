package core

import (
	"errors"
	"fmt"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WatchHistoryService struct {
	repo      data.WatchHistoryRepository
	videoRepo data.VideoRepository
	indexer   VideoIndexer
	logger    *zap.Logger
}

func NewWatchHistoryService(repo data.WatchHistoryRepository, videoRepo data.VideoRepository, indexer VideoIndexer, logger *zap.Logger) *WatchHistoryService {
	return &WatchHistoryService{
		repo:      repo,
		videoRepo: videoRepo,
		indexer:   indexer,
		logger:    logger,
	}
}

type WatchHistoryEntry struct {
	Watch data.UserVideoWatch `json:"watch"`
	Video *data.Video         `json:"video,omitempty"`
}

// RecordWatch records a watch event and increments view count if not viewed in last 24h.
// Uses atomic database operations to prevent race conditions from concurrent requests.
func (s *WatchHistoryService) RecordWatch(userID, videoID uint, duration, position int, completed bool) error {
	// Verify video exists
	_, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("video not found")
		}
		return fmt.Errorf("failed to verify video: %w", err)
	}

	// Record the watch session
	if err := s.repo.RecordWatch(userID, videoID, duration, position, completed); err != nil {
		s.logger.Error("Failed to record watch",
			zap.Uint("user_id", userID),
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		return err
	}

	// Atomically try to increment view count (handles 24h deduplication)
	incremented, err := s.repo.TryIncrementViewCount(userID, videoID)
	if err != nil {
		s.logger.Warn("Failed to increment view count",
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		// Don't fail the request for this
	} else if incremented {
		s.logger.Debug("Incremented view count",
			zap.Uint("video_id", videoID),
			zap.Uint("user_id", userID),
		)
		// Update search index with new view count
		if s.indexer != nil {
			video, err := s.videoRepo.GetByID(videoID)
			if err == nil {
				if err := s.indexer.UpdateVideoIndex(video); err != nil {
					s.logger.Warn("Failed to update video in search index after view count increment",
						zap.Uint("video_id", videoID),
						zap.Error(err),
					)
				}
			}
		}
	}

	return nil
}

// GetResumePosition returns the position to resume from, or 0 if completed or not watched
func (s *WatchHistoryService) GetResumePosition(userID, videoID uint) (int, error) {
	watch, err := s.repo.GetLastWatch(userID, videoID)
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

// GetUserHistory returns paginated watch history with video details
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

	// Collect video IDs
	videoIDs := make([]uint, 0, len(watches))
	for _, w := range watches {
		videoIDs = append(videoIDs, w.VideoID)
	}

	// Fetch videos
	videos, err := s.videoRepo.GetByIDs(videoIDs)
	if err != nil {
		s.logger.Warn("Failed to fetch videos for history",
			zap.Error(err),
		)
		// Continue without video details
		videos = nil
	}

	// Create video map
	videoMap := make(map[uint]*data.Video)
	for i := range videos {
		videoMap[videos[i].ID] = &videos[i]
	}

	// Build result
	entries := make([]WatchHistoryEntry, 0, len(watches))
	for _, w := range watches {
		entry := WatchHistoryEntry{
			Watch: w,
			Video: videoMap[w.VideoID],
		}
		entries = append(entries, entry)
	}

	return entries, total, nil
}

// GetVideoHistory returns watch sessions for a specific video
func (s *WatchHistoryService) GetVideoHistory(userID, videoID uint, limit int) ([]data.UserVideoWatch, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ListVideoWatches(userID, videoID, limit)
}
