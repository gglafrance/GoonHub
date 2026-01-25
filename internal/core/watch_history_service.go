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
	logger    *zap.Logger
}

func NewWatchHistoryService(repo data.WatchHistoryRepository, videoRepo data.VideoRepository, logger *zap.Logger) *WatchHistoryService {
	return &WatchHistoryService{
		repo:      repo,
		videoRepo: videoRepo,
		logger:    logger,
	}
}

type WatchHistoryEntry struct {
	Watch data.UserVideoWatch `json:"watch"`
	Video *data.Video         `json:"video,omitempty"`
}

// RecordWatch records a watch event and increments view count if not viewed in last 24h
func (s *WatchHistoryService) RecordWatch(userID, videoID uint, duration, position int, completed bool) error {
	// Verify video exists
	_, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("video not found")
		}
		return fmt.Errorf("failed to verify video: %w", err)
	}

	// Check if user has viewed this video in the last 24 hours
	hasViewed, err := s.repo.HasViewedWithin24Hours(userID, videoID)
	if err != nil {
		s.logger.Error("Failed to check view history",
			zap.Uint("user_id", userID),
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		return err
	}

	// Record the watch
	if err := s.repo.RecordWatch(userID, videoID, duration, position, completed); err != nil {
		s.logger.Error("Failed to record watch",
			zap.Uint("user_id", userID),
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		return err
	}

	// Increment view count if not viewed in last 24h
	if !hasViewed {
		if err := s.repo.IncrementVideoViewCount(videoID); err != nil {
			s.logger.Warn("Failed to increment view count",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
			// Don't fail the request for this
		} else {
			s.logger.Debug("Incremented view count",
				zap.Uint("video_id", videoID),
				zap.Uint("user_id", userID),
			)
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
