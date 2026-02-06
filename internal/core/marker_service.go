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

const maxMarkersPerScene = 50

type MarkerService struct {
	markerRepo              data.MarkerRepository
	sceneRepo               data.SceneRepository
	tagRepo                 data.TagRepository
	markerThumbnailDir      string
	markerThumbnailMaxDim   int
	markerThumbnailQuality  int
	markerAnimatedDuration  int
	markerThumbnailType     string
	logger                  *zap.Logger
}

func NewMarkerService(markerRepo data.MarkerRepository, sceneRepo data.SceneRepository, tagRepo data.TagRepository, cfg *config.Config, logger *zap.Logger) *MarkerService {
	markerAnimatedDuration := cfg.Processing.MarkerAnimatedDuration
	if markerAnimatedDuration <= 0 {
		markerAnimatedDuration = 10
	}
	markerThumbnailType := cfg.Processing.MarkerThumbnailType
	if markerThumbnailType == "" {
		markerThumbnailType = "static"
	}
	return &MarkerService{
		markerRepo:             markerRepo,
		sceneRepo:              sceneRepo,
		tagRepo:                tagRepo,
		markerThumbnailDir:     cfg.Processing.MarkerThumbnailDir,
		markerThumbnailMaxDim:  cfg.Processing.MaxFrameDimension,
		markerThumbnailQuality: cfg.Processing.FrameQuality,
		markerAnimatedDuration: markerAnimatedDuration,
		markerThumbnailType:    markerThumbnailType,
		logger:                 logger,
	}
}

func (s *MarkerService) ListMarkers(userID, sceneID uint) ([]data.MarkerWithTags, error) {
	// Verify scene exists before returning markers
	_, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("scene", sceneID)
		}
		s.logger.Error("failed to verify scene exists", zap.Uint("sceneID", sceneID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to verify scene", err)
	}

	markers, err := s.markerRepo.GetByUserAndScene(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to list markers", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to list markers", err)
	}

	// Build result with tags
	result := make([]data.MarkerWithTags, len(markers))
	for i, m := range markers {
		result[i] = data.MarkerWithTags{
			UserSceneMarker: m,
			Tags:            []data.MarkerTagInfo{}, // default empty slice
		}
	}

	// Batch fetch tags for all markers
	if len(markers) > 0 {
		markerIDs := make([]uint, len(markers))
		for i, m := range markers {
			markerIDs[i] = m.ID
		}

		tagsMap, err := s.markerRepo.GetMarkerTagsMultiple(markerIDs)
		if err != nil {
			s.logger.Warn("failed to batch fetch marker tags", zap.Uint("sceneID", sceneID), zap.Error(err))
			// Continue without tags - not a critical failure
		} else {
			// Populate tags on each marker
			for i := range result {
				if tags, ok := tagsMap[result[i].ID]; ok {
					result[i].Tags = tags
				}
			}
		}
	}

	return result, nil
}

func (s *MarkerService) CreateMarker(userID, sceneID uint, timestamp int, label, color string) (*data.UserSceneMarker, error) {
	// Validate scene exists and get duration
	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("scene", sceneID)
		}
		s.logger.Error("failed to get scene", zap.Uint("sceneID", sceneID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get scene", err)
	}

	// Validate timestamp is within scene duration
	if timestamp < 0 {
		return nil, apperrors.NewValidationError("timestamp must be non-negative")
	}
	if scene.Duration > 0 && timestamp > scene.Duration {
		return nil, apperrors.NewValidationError(fmt.Sprintf("timestamp %d exceeds scene duration %d", timestamp, scene.Duration))
	}

	// Check marker limit
	count, err := s.markerRepo.CountByUserAndScene(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to count markers", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to count markers", err)
	}
	if count >= maxMarkersPerScene {
		return nil, apperrors.NewValidationError(fmt.Sprintf("maximum of %d markers per scene reached", maxMarkersPerScene))
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

	marker := &data.UserSceneMarker{
		UserID:    userID,
		SceneID:   sceneID,
		Timestamp: timestamp,
		Label:     label,
		Color:     color,
	}

	if err := s.markerRepo.Create(marker); err != nil {
		s.logger.Error("failed to create marker", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to create marker", err)
	}

	// Apply label tags if the marker has a label
	if label != "" {
		if err := s.markerRepo.ApplyLabelTagsToMarker(userID, marker.ID, label); err != nil {
			s.logger.Warn("failed to apply label tags to marker",
				zap.Uint("markerID", marker.ID),
				zap.String("label", label),
				zap.Error(err))
		}
	}

	// Generate the appropriate thumbnail type (best effort - marker is still useful without it)
	if s.markerThumbnailType == "animated" {
		if err := s.generateAnimatedThumbnail(marker, scene); err != nil {
			s.logger.Warn("failed to generate animated marker thumbnail",
				zap.Uint("markerID", marker.ID),
				zap.Uint("sceneID", sceneID),
				zap.Error(err))
		}
	} else {
		if err := s.generateThumbnail(marker, scene); err != nil {
			s.logger.Warn("failed to generate marker thumbnail",
				zap.Uint("markerID", marker.ID),
				zap.Uint("sceneID", sceneID),
				zap.Error(err))
		}
	}

	return marker, nil
}

func (s *MarkerService) UpdateMarker(userID, markerID uint, label *string, color *string, timestamp *int) (*data.UserSceneMarker, error) {
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
	var scene *data.Scene

	if timestamp != nil {
		if *timestamp < 0 {
			return nil, apperrors.NewValidationError("timestamp must be non-negative")
		}

		// Validate against scene duration
		var err error
		scene, err = s.sceneRepo.GetByID(marker.SceneID)
		if err != nil {
			s.logger.Error("failed to get scene", zap.Uint("sceneID", marker.SceneID), zap.Error(err))
			return nil, apperrors.NewInternalError("failed to get scene", err)
		}
		if scene.Duration > 0 && *timestamp > scene.Duration {
			return nil, apperrors.NewValidationError(fmt.Sprintf("timestamp %d exceeds scene duration %d", *timestamp, scene.Duration))
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

		// Delete old animated thumbnail
		if marker.AnimatedThumbnailPath != "" {
			oldAnimPath := filepath.Join(s.markerThumbnailDir, marker.AnimatedThumbnailPath)
			if err := os.Remove(oldAnimPath); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("failed to delete old animated marker thumbnail",
					zap.Uint("markerID", marker.ID),
					zap.String("path", oldAnimPath),
					zap.Error(err))
			}
		}

		// Fetch scene if not already fetched
		if scene == nil {
			var err error
			scene, err = s.sceneRepo.GetByID(marker.SceneID)
			if err != nil {
				s.logger.Warn("failed to get scene for thumbnail regeneration",
					zap.Uint("sceneID", marker.SceneID),
					zap.Error(err))
			}
		}

		// Generate new thumbnail matching the current type
		if scene != nil {
			if s.markerThumbnailType == "animated" {
				if err := s.generateAnimatedThumbnail(marker, scene); err != nil {
					s.logger.Warn("failed to regenerate animated marker thumbnail",
						zap.Uint("markerID", marker.ID),
						zap.Error(err))
				}
			} else {
				if err := s.generateThumbnail(marker, scene); err != nil {
					s.logger.Warn("failed to regenerate marker thumbnail",
						zap.Uint("markerID", marker.ID),
						zap.Error(err))
				}
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

	// Store thumbnail paths before deleting marker
	thumbnailPath := ""
	if marker.ThumbnailPath != "" {
		thumbnailPath = filepath.Join(s.markerThumbnailDir, marker.ThumbnailPath)
	}
	animatedThumbnailPath := ""
	if marker.AnimatedThumbnailPath != "" {
		animatedThumbnailPath = filepath.Join(s.markerThumbnailDir, marker.AnimatedThumbnailPath)
	}

	// Delete DB record first (this is the critical operation)
	if err := s.markerRepo.Delete(markerID); err != nil {
		s.logger.Error("failed to delete marker", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to delete marker", err)
	}

	// Clean up thumbnail files after successful DB delete (best effort)
	if thumbnailPath != "" {
		if err := os.Remove(thumbnailPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("failed to delete marker thumbnail",
				zap.Uint("markerID", markerID),
				zap.String("path", thumbnailPath),
				zap.Error(err))
		}
	}
	if animatedThumbnailPath != "" {
		if err := os.Remove(animatedThumbnailPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("failed to delete animated marker thumbnail",
				zap.Uint("markerID", markerID),
				zap.String("path", animatedThumbnailPath),
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

	// Populate thumbnail marker IDs for cycling thumbnails
	if len(groups) > 0 {
		labels := make([]string, len(groups))
		for i, g := range groups {
			labels[i] = g.Label
		}

		thumbnails, err := s.markerRepo.GetRandomThumbnailsForLabels(userID, labels, 10)
		if err != nil {
			s.logger.Warn("failed to get thumbnail IDs for labels", zap.Uint("userID", userID), zap.Error(err))
		} else {
			for i := range groups {
				if ids, ok := thumbnails[groups[i].Label]; ok {
					groups[i].ThumbnailMarkerIDs = ids
				} else {
					groups[i].ThumbnailMarkerIDs = []uint{}
				}
			}
		}
	}

	return groups, total, nil
}

func (s *MarkerService) GetMarkersByLabel(userID uint, label string, page, limit int) ([]data.MarkerWithScene, int64, error) {
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

func (s *MarkerService) GetAllMarkers(userID uint, page, limit int, sortBy string) ([]data.MarkerWithScene, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Validate sortBy parameter
	validSorts := map[string]bool{
		"label_asc":  true,
		"label_desc": true,
		"recent":     true,
		"oldest":     true,
	}
	if !validSorts[sortBy] {
		sortBy = "label_asc"
	}

	markers, total, err := s.markerRepo.GetAllMarkersForUser(userID, offset, limit, sortBy)
	if err != nil {
		s.logger.Error("failed to get all markers", zap.Uint("userID", userID), zap.Error(err))
		return nil, 0, apperrors.NewInternalError("failed to get all markers", err)
	}
	return markers, total, nil
}

// GenerateMissingForScene finds all markers for a scene that lack thumbnails and generates them.
// This is best-effort: individual failures are logged and skipped.
// Implements jobs.MarkerThumbnailGenerator.
func (s *MarkerService) GenerateMissingForScene(ctx context.Context, sceneID uint) (int, error) {
	markers, err := s.markerRepo.GetBySceneWithoutThumbnail(sceneID)
	if err != nil {
		return 0, fmt.Errorf("failed to query markers without thumbnails: %w", err)
	}

	if len(markers) == 0 {
		return 0, nil
	}

	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		return 0, fmt.Errorf("failed to get scene for thumbnail generation: %w", err)
	}

	s.logger.Info("Generating missing marker thumbnails",
		zap.Uint("scene_id", sceneID),
		zap.Int("count", len(markers)))

	generated := 0
	for i := range markers {
		if ctx.Err() != nil {
			s.logger.Info("Marker thumbnail generation interrupted",
				zap.Uint("scene_id", sceneID),
				zap.Int("generated", generated),
				zap.Int("remaining", len(markers)-i))
			break
		}

		if err := s.generateThumbnail(&markers[i], scene); err != nil {
			s.logger.Warn("Failed to generate marker thumbnail",
				zap.Uint("marker_id", markers[i].ID),
				zap.Int("timestamp", markers[i].Timestamp),
				zap.Error(err))
			continue
		}
		generated++
	}

	return generated, nil
}

// generateThumbnail extracts a frame at the marker's timestamp and saves it as a thumbnail.
// This is a best-effort operation - the marker remains valid even if thumbnail generation fails.
func (s *MarkerService) generateThumbnail(marker *data.UserSceneMarker, scene *data.Scene) error {
	// Ensure marker thumbnail directory exists
	if err := os.MkdirAll(s.markerThumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create marker thumbnail directory: %w", err)
	}

	// Check if scene file exists
	if scene.StoredPath == "" {
		return fmt.Errorf("scene has no stored path")
	}
	if _, err := os.Stat(scene.StoredPath); os.IsNotExist(err) {
		return fmt.Errorf("scene file not found: %s", scene.StoredPath)
	}

	// Calculate dimensions preserving aspect ratio
	tileWidth, tileHeight := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, s.markerThumbnailMaxDim)

	// Generate thumbnail filename: marker_{id}.webp
	thumbnailFilename := fmt.Sprintf("marker_%d.webp", marker.ID)
	thumbnailPath := filepath.Join(s.markerThumbnailDir, thumbnailFilename)

	// Convert timestamp to ffmpeg seek format (seconds)
	seekPosition := strconv.Itoa(marker.Timestamp)

	// Extract thumbnail with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ffmpeg.ExtractThumbnailWithContext(ctx, scene.StoredPath, thumbnailPath, seekPosition, tileWidth, tileHeight, s.markerThumbnailQuality); err != nil {
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

// generateAnimatedThumbnail extracts a short MP4 clip at the marker's timestamp.
// This is a best-effort operation.
func (s *MarkerService) generateAnimatedThumbnail(marker *data.UserSceneMarker, scene *data.Scene) error {
	if err := os.MkdirAll(s.markerThumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create marker thumbnail directory: %w", err)
	}

	if scene.StoredPath == "" {
		return fmt.Errorf("scene has no stored path")
	}
	if _, err := os.Stat(scene.StoredPath); os.IsNotExist(err) {
		return fmt.Errorf("scene file not found: %s", scene.StoredPath)
	}

	animatedFilename := fmt.Sprintf("marker_%d.mp4", marker.ID)
	animatedPath := filepath.Join(s.markerThumbnailDir, animatedFilename)

	seekPosition := strconv.Itoa(marker.Timestamp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := ffmpeg.ExtractAnimatedThumbnailWithContext(ctx, scene.StoredPath, animatedPath, seekPosition, s.markerAnimatedDuration, s.markerThumbnailMaxDim); err != nil {
		return fmt.Errorf("failed to extract animated thumbnail: %w", err)
	}

	marker.AnimatedThumbnailPath = animatedFilename
	if err := s.markerRepo.Update(marker); err != nil {
		os.Remove(animatedPath)
		return fmt.Errorf("failed to update marker with animated thumbnail path: %w", err)
	}

	return nil
}

// GenerateMissingAnimatedForScene finds all markers for a scene that lack animated thumbnails and generates them.
// Implements jobs.AnimatedThumbnailGenerator.
func (s *MarkerService) GenerateMissingAnimatedForScene(ctx context.Context, sceneID uint) (int, error) {
	markers, err := s.markerRepo.GetBySceneWithoutAnimatedThumbnail(sceneID)
	if err != nil {
		return 0, fmt.Errorf("failed to query markers without animated thumbnails: %w", err)
	}

	if len(markers) == 0 {
		return 0, nil
	}

	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		return 0, fmt.Errorf("failed to get scene for animated thumbnail generation: %w", err)
	}

	s.logger.Info("Generating missing animated marker thumbnails",
		zap.Uint("scene_id", sceneID),
		zap.Int("count", len(markers)))

	generated := 0
	for i := range markers {
		if ctx.Err() != nil {
			s.logger.Info("Animated marker thumbnail generation interrupted",
				zap.Uint("scene_id", sceneID),
				zap.Int("generated", generated),
				zap.Int("remaining", len(markers)-i))
			break
		}

		if err := s.generateAnimatedThumbnail(&markers[i], scene); err != nil {
			s.logger.Warn("Failed to generate animated marker thumbnail",
				zap.Uint("marker_id", markers[i].ID),
				zap.Int("timestamp", markers[i].Timestamp),
				zap.Error(err))
			continue
		}
		generated++
	}

	return generated, nil
}

// GetMarkerThumbnailType returns the current marker thumbnail type setting
func (s *MarkerService) GetMarkerThumbnailType() string {
	return s.markerThumbnailType
}

// SetMarkerThumbnailType updates the marker thumbnail type setting
func (s *MarkerService) SetMarkerThumbnailType(thumbnailType string) {
	s.markerThumbnailType = thumbnailType
}

// SetMarkerAnimatedDuration updates the animated duration setting
func (s *MarkerService) SetMarkerAnimatedDuration(duration int) {
	s.markerAnimatedDuration = duration
}

// GetLabelTags returns the default tags for a label
func (s *MarkerService) GetLabelTags(userID uint, label string) ([]data.Tag, error) {
	if label == "" {
		return nil, apperrors.NewValidationError("label is required")
	}

	tags, err := s.markerRepo.GetLabelTags(userID, label)
	if err != nil {
		s.logger.Error("failed to get label tags", zap.Uint("userID", userID), zap.String("label", label), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get label tags", err)
	}
	// Ensure non-nil slice for JSON serialization
	if tags == nil {
		tags = []data.Tag{}
	}
	return tags, nil
}

// SetLabelTags sets the default tags for a label and syncs to all existing markers
func (s *MarkerService) SetLabelTags(userID uint, label string, tagIDs []uint) error {
	if label == "" {
		return apperrors.NewValidationError("label is required")
	}

	// Validate that all tag IDs exist
	if len(tagIDs) > 0 {
		tags, err := s.tagRepo.GetByIDs(tagIDs)
		if err != nil {
			s.logger.Error("failed to validate tags", zap.Uint("userID", userID), zap.Error(err))
			return apperrors.NewInternalError("failed to validate tags", err)
		}
		if len(tags) != len(tagIDs) {
			return apperrors.NewValidationError("one or more tags do not exist")
		}
	}

	if err := s.markerRepo.SetLabelTags(userID, label, tagIDs); err != nil {
		s.logger.Error("failed to set label tags", zap.Uint("userID", userID), zap.String("label", label), zap.Error(err))
		return apperrors.NewInternalError("failed to set label tags", err)
	}

	s.logger.Info("set label tags",
		zap.Uint("userID", userID),
		zap.String("label", label),
		zap.Int("tagCount", len(tagIDs)))

	return nil
}

// GetMarkerTags returns tags for a specific marker
func (s *MarkerService) GetMarkerTags(userID, markerID uint) ([]data.MarkerTagInfo, error) {
	// Verify ownership
	marker, err := s.markerRepo.GetByID(markerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("marker", markerID)
		}
		s.logger.Error("failed to get marker", zap.Uint("markerID", markerID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get marker", err)
	}

	if marker.UserID != userID {
		return nil, apperrors.NewForbiddenError("you do not own this marker")
	}

	tags, err := s.markerRepo.GetMarkerTags(markerID)
	if err != nil {
		s.logger.Error("failed to get marker tags", zap.Uint("markerID", markerID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get marker tags", err)
	}
	// Ensure non-nil slice for JSON serialization
	if tags == nil {
		tags = []data.MarkerTagInfo{}
	}
	return tags, nil
}

// SetMarkerTags sets individual (non-label-derived) tags on a marker
func (s *MarkerService) SetMarkerTags(userID, markerID uint, tagIDs []uint) error {
	// Verify ownership
	marker, err := s.markerRepo.GetByID(markerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperrors.NewNotFoundError("marker", markerID)
		}
		s.logger.Error("failed to get marker", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to get marker", err)
	}

	if marker.UserID != userID {
		return apperrors.NewForbiddenError("you do not own this marker")
	}

	// Validate that all tag IDs exist
	if len(tagIDs) > 0 {
		tags, err := s.tagRepo.GetByIDs(tagIDs)
		if err != nil {
			s.logger.Error("failed to validate tags", zap.Uint("markerID", markerID), zap.Error(err))
			return apperrors.NewInternalError("failed to validate tags", err)
		}
		if len(tags) != len(tagIDs) {
			return apperrors.NewValidationError("one or more tags do not exist")
		}
	}

	if err := s.markerRepo.SetMarkerTags(markerID, tagIDs); err != nil {
		s.logger.Error("failed to set marker tags", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to set marker tags", err)
	}
	return nil
}

// AddMarkerTags adds individual tags to a marker
func (s *MarkerService) AddMarkerTags(userID, markerID uint, tagIDs []uint) error {
	// Verify ownership
	marker, err := s.markerRepo.GetByID(markerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperrors.NewNotFoundError("marker", markerID)
		}
		s.logger.Error("failed to get marker", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to get marker", err)
	}

	if marker.UserID != userID {
		return apperrors.NewForbiddenError("you do not own this marker")
	}

	// Validate that all tag IDs exist
	if len(tagIDs) > 0 {
		tags, err := s.tagRepo.GetByIDs(tagIDs)
		if err != nil {
			s.logger.Error("failed to validate tags", zap.Uint("markerID", markerID), zap.Error(err))
			return apperrors.NewInternalError("failed to validate tags", err)
		}
		if len(tags) != len(tagIDs) {
			return apperrors.NewValidationError("one or more tags do not exist")
		}
	}

	if err := s.markerRepo.AddMarkerTags(markerID, tagIDs); err != nil {
		s.logger.Error("failed to add marker tags", zap.Uint("markerID", markerID), zap.Error(err))
		return apperrors.NewInternalError("failed to add marker tags", err)
	}
	return nil
}
