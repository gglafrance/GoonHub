package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/pkg/ffmpeg"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const maxMarkersPerVideo = 50
const markerThumbnailMaxDimension = 320
const markerThumbnailQuality = 75

type MarkerService struct {
	markerRepo         data.MarkerRepository
	videoRepo          data.VideoRepository
	markerThumbnailDir string
	logger             *zap.Logger
}

func NewMarkerService(markerRepo data.MarkerRepository, videoRepo data.VideoRepository, cfg *config.Config, logger *zap.Logger) *MarkerService {
	return &MarkerService{
		markerRepo:         markerRepo,
		videoRepo:          videoRepo,
		markerThumbnailDir: cfg.Processing.MarkerThumbnailDir,
		logger:             logger,
	}
}

func (s *MarkerService) ListMarkers(userID, videoID uint) ([]data.UserVideoMarker, error) {
	// Verify video exists before returning markers
	_, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("video", videoID)
		}
		s.logger.Error("failed to verify video exists", zap.Uint("videoID", videoID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to verify video", err)
	}

	markers, err := s.markerRepo.GetByUserAndVideo(userID, videoID)
	if err != nil {
		s.logger.Error("failed to list markers", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to list markers", err)
	}
	return markers, nil
}

func (s *MarkerService) CreateMarker(userID, videoID uint, timestamp int, label, color string) (*data.UserVideoMarker, error) {
	// Validate video exists and get duration
	video, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("video", videoID)
		}
		s.logger.Error("failed to get video", zap.Uint("videoID", videoID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get video", err)
	}

	// Validate timestamp is within video duration
	if timestamp < 0 {
		return nil, apperrors.NewValidationError("timestamp must be non-negative")
	}
	if video.Duration > 0 && timestamp > video.Duration {
		return nil, apperrors.NewValidationError(fmt.Sprintf("timestamp %d exceeds video duration %d", timestamp, video.Duration))
	}

	// Check marker limit
	count, err := s.markerRepo.CountByUserAndVideo(userID, videoID)
	if err != nil {
		s.logger.Error("failed to count markers", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to count markers", err)
	}
	if count >= maxMarkersPerVideo {
		return nil, apperrors.NewValidationError(fmt.Sprintf("maximum of %d markers per video reached", maxMarkersPerVideo))
	}

	// Validate color format (hex color)
	if color == "" {
		color = "#FFFFFF" // default white
	}
	if len(color) != 7 || color[0] != '#' {
		return nil, apperrors.NewValidationError("color must be a valid hex color (e.g., #FF4D4D)")
	}

	// Validate label length
	if len(label) > 100 {
		return nil, apperrors.NewValidationError("label must be 100 characters or fewer")
	}

	marker := &data.UserVideoMarker{
		UserID:    userID,
		VideoID:   videoID,
		Timestamp: timestamp,
		Label:     label,
		Color:     color,
	}

	if err := s.markerRepo.Create(marker); err != nil {
		s.logger.Error("failed to create marker", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to create marker", err)
	}

	// Generate thumbnail (best effort - marker is still useful without it)
	if err := s.generateThumbnail(marker, video); err != nil {
		s.logger.Warn("failed to generate marker thumbnail",
			zap.Uint("markerID", marker.ID),
			zap.Uint("videoID", videoID),
			zap.Error(err))
	}

	return marker, nil
}

func (s *MarkerService) UpdateMarker(userID, markerID uint, label *string, color *string, timestamp *int) (*data.UserVideoMarker, error) {
	marker, err := s.markerRepo.GetByID(markerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("marker", markerID)
		}
		s.logger.Error("failed to get marker", zap.Uint("markerID", markerID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get marker", err)
	}

	// Verify ownership
	if marker.UserID != userID {
		return nil, apperrors.NewForbiddenError("you do not own this marker")
	}

	// Update fields if provided
	if label != nil {
		if len(*label) > 100 {
			return nil, apperrors.NewValidationError("label must be 100 characters or fewer")
		}
		marker.Label = *label
	}

	if color != nil {
		if len(*color) != 7 || (*color)[0] != '#' {
			return nil, apperrors.NewValidationError("color must be a valid hex color (e.g., #FF4D4D)")
		}
		marker.Color = *color
	}

	var timestampChanged bool
	var video *data.Video

	if timestamp != nil {
		if *timestamp < 0 {
			return nil, apperrors.NewValidationError("timestamp must be non-negative")
		}

		// Validate against video duration
		var err error
		video, err = s.videoRepo.GetByID(marker.VideoID)
		if err != nil {
			s.logger.Error("failed to get video", zap.Uint("videoID", marker.VideoID), zap.Error(err))
			return nil, apperrors.NewInternalError("failed to get video", err)
		}
		if video.Duration > 0 && *timestamp > video.Duration {
			return nil, apperrors.NewValidationError(fmt.Sprintf("timestamp %d exceeds video duration %d", *timestamp, video.Duration))
		}
		if marker.Timestamp != *timestamp {
			timestampChanged = true
		}
		marker.Timestamp = *timestamp
	}

	if err := s.markerRepo.Update(marker); err != nil {
		s.logger.Error("failed to update marker", zap.Uint("markerID", markerID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to update marker", err)
	}

	// Regenerate thumbnail if timestamp changed
	if timestampChanged {
		// Delete old thumbnail
		if marker.ThumbnailPath != "" {
			oldPath := filepath.Join(s.markerThumbnailDir, marker.ThumbnailPath)
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("failed to delete old marker thumbnail",
					zap.Uint("markerID", marker.ID),
					zap.String("path", oldPath),
					zap.Error(err))
			}
		}

		// Fetch video if not already fetched
		if video == nil {
			var err error
			video, err = s.videoRepo.GetByID(marker.VideoID)
			if err != nil {
				s.logger.Warn("failed to get video for thumbnail regeneration",
					zap.Uint("videoID", marker.VideoID),
					zap.Error(err))
			}
		}

		// Generate new thumbnail
		if video != nil {
			if err := s.generateThumbnail(marker, video); err != nil {
				s.logger.Warn("failed to regenerate marker thumbnail",
					zap.Uint("markerID", marker.ID),
					zap.Error(err))
			}
		}
	}

	return marker, nil
}

func (s *MarkerService) DeleteMarker(userID, markerID uint) error {
	marker, err := s.markerRepo.GetByID(markerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperrors.NewNotFoundError("marker", markerID)
		}
		s.logger.Error("failed to get marker", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to get marker", err)
	}

	// Verify ownership
	if marker.UserID != userID {
		return apperrors.NewForbiddenError("you do not own this marker")
	}

	// Store thumbnail path before deleting marker
	thumbnailPath := ""
	if marker.ThumbnailPath != "" {
		thumbnailPath = filepath.Join(s.markerThumbnailDir, marker.ThumbnailPath)
	}

	// Delete DB record first (this is the critical operation)
	if err := s.markerRepo.Delete(markerID); err != nil {
		s.logger.Error("failed to delete marker", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to delete marker", err)
	}

	// Clean up thumbnail file after successful DB delete (best effort)
	if thumbnailPath != "" {
		if err := os.Remove(thumbnailPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("failed to delete marker thumbnail",
				zap.Uint("markerID", markerID),
				zap.String("path", thumbnailPath),
				zap.Error(err))
		}
	}

	return nil
}

func (s *MarkerService) GetLabelSuggestions(userID uint, limit int) ([]data.MarkerLabelSuggestion, error) {
	if limit <= 0 {
		limit = 50
	}
	suggestions, err := s.markerRepo.GetLabelSuggestionsForUser(userID, limit)
	if err != nil {
		s.logger.Error("failed to get label suggestions", zap.Uint("userID", userID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get label suggestions", err)
	}
	return suggestions, nil
}

func (s *MarkerService) GetLabelGroups(userID uint, page, limit int, sortBy string) ([]data.MarkerLabelGroup, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Validate sortBy parameter
	validSorts := map[string]bool{
		"count_desc": true,
		"count_asc":  true,
		"label_asc":  true,
		"label_desc": true,
		"recent":     true,
	}
	if !validSorts[sortBy] {
		sortBy = "count_desc"
	}

	groups, total, err := s.markerRepo.GetLabelGroupsForUser(userID, offset, limit, sortBy)
	if err != nil {
		s.logger.Error("failed to get label groups", zap.Uint("userID", userID), zap.Error(err))
		return nil, 0, apperrors.NewInternalError("failed to get label groups", err)
	}
	return groups, total, nil
}

func (s *MarkerService) GetMarkersByLabel(userID uint, label string, page, limit int) ([]data.MarkerWithVideo, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	markers, total, err := s.markerRepo.GetMarkersByLabelForUser(userID, label, offset, limit)
	if err != nil {
		s.logger.Error("failed to get markers by label", zap.Uint("userID", userID), zap.String("label", label), zap.Error(err))
		return nil, 0, apperrors.NewInternalError("failed to get markers by label", err)
	}
	return markers, total, nil
}

// generateThumbnail extracts a frame at the marker's timestamp and saves it as a thumbnail.
// This is a best-effort operation - the marker remains valid even if thumbnail generation fails.
func (s *MarkerService) generateThumbnail(marker *data.UserVideoMarker, video *data.Video) error {
	// Ensure marker thumbnail directory exists
	if err := os.MkdirAll(s.markerThumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create marker thumbnail directory: %w", err)
	}

	// Check if video file exists
	if video.StoredPath == "" {
		return fmt.Errorf("video has no stored path")
	}
	if _, err := os.Stat(video.StoredPath); os.IsNotExist(err) {
		return fmt.Errorf("video file not found: %s", video.StoredPath)
	}

	// Calculate dimensions preserving aspect ratio
	tileWidth, tileHeight := ffmpeg.CalculateTileDimensions(video.Width, video.Height, markerThumbnailMaxDimension)

	// Generate thumbnail filename: marker_{id}.webp
	thumbnailFilename := fmt.Sprintf("marker_%d.webp", marker.ID)
	thumbnailPath := filepath.Join(s.markerThumbnailDir, thumbnailFilename)

	// Convert timestamp to ffmpeg seek format (seconds)
	seekPosition := strconv.Itoa(marker.Timestamp)

	// Extract thumbnail with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ffmpeg.ExtractThumbnailWithContext(ctx, video.StoredPath, thumbnailPath, seekPosition, tileWidth, tileHeight, markerThumbnailQuality); err != nil {
		return fmt.Errorf("failed to extract thumbnail: %w", err)
	}

	// Update marker with thumbnail path
	marker.ThumbnailPath = thumbnailFilename
	if err := s.markerRepo.Update(marker); err != nil {
		// Clean up the generated thumbnail file
		os.Remove(thumbnailPath)
		return fmt.Errorf("failed to update marker with thumbnail path: %w", err)
	}

	return nil
}
