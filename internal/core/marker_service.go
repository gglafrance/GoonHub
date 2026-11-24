package core

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/compute"
	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const maxMarkersPerScene = 50

type MarkerService struct {
	markerRepo                  data.MarkerRepository
	sceneRepo                   data.SceneRepository
	tagRepo                     data.TagRepository
	markerThumbnailDir          string
	markerThumbnailMaxDim       int
	markerThumbnailQuality      int
	markerAnimatedDuration      int
	markerThumbnailType         string
	scenePreviewEnabled         bool
	scenePreviewSegments        int
	scenePreviewSegmentDuration float64
	scenePreviewDir             string
	scenePreviewMaxDim          int
	markerPreviewCRF            int
	scenePreviewCRF             int
	logger                      *zap.Logger

	// Marker thumbnails are made in the background (ffmpeg takes seconds, the new marker
	// must show at once). Nil queue (Start not called, as in tests): made inline.
	thumbnailWorkers int
	thumbnailQueue   chan markerThumbnailTask
	thumbnailWG      sync.WaitGroup
	startOnce        sync.Once
	stopOnce         sync.Once
}

// markerThumbnailTask asks for the thumbnail of a marker, made from its current row.
type markerThumbnailTask struct {
	markerID uint
}

// markerThumbnailQueueSize is how many thumbnails can wait for a worker; beyond it, the
// thumbnail is made inline (slower answer, but never lost).
const markerThumbnailQueueSize = 256

func NewMarkerService(markerRepo data.MarkerRepository, sceneRepo data.SceneRepository, tagRepo data.TagRepository, cfg *config.Config, logger *zap.Logger) *MarkerService {
	markerAnimatedDuration := cfg.Processing.MarkerAnimatedDuration
	if markerAnimatedDuration <= 0 {
		markerAnimatedDuration = 10
	}
	markerThumbnailType := cfg.Processing.MarkerThumbnailType
	if markerThumbnailType == "" {
		markerThumbnailType = "static"
	}
	scenePreviewSegments := cfg.Processing.ScenePreviewSegments
	if scenePreviewSegments <= 0 {
		scenePreviewSegments = 12
	}
	scenePreviewSegmentDuration := cfg.Processing.ScenePreviewSegmentDuration
	if scenePreviewSegmentDuration <= 0 {
		scenePreviewSegmentDuration = 1.0
	}
	markerPreviewCRF := cfg.Processing.MarkerPreviewCRF
	if markerPreviewCRF <= 0 {
		markerPreviewCRF = 32
	}
	scenePreviewCRF := cfg.Processing.ScenePreviewCRF
	if scenePreviewCRF <= 0 {
		scenePreviewCRF = 27
	}
	thumbnailWorkers := cfg.Processing.MarkerThumbnailWorkers
	if thumbnailWorkers <= 0 {
		thumbnailWorkers = 2
	}
	return &MarkerService{
		thumbnailWorkers:            thumbnailWorkers,
		markerRepo:                  markerRepo,
		sceneRepo:                   sceneRepo,
		tagRepo:                     tagRepo,
		markerThumbnailDir:          cfg.Processing.MarkerThumbnailDir,
		markerThumbnailMaxDim:       cfg.Processing.MaxFrameDimension,
		markerThumbnailQuality:      cfg.Processing.FrameQuality,
		markerAnimatedDuration:      markerAnimatedDuration,
		markerThumbnailType:         markerThumbnailType,
		scenePreviewEnabled:         cfg.Processing.ScenePreviewEnabled,
		scenePreviewSegments:        scenePreviewSegments,
		scenePreviewSegmentDuration: scenePreviewSegmentDuration,
		scenePreviewDir:             cfg.Processing.ScenePreviewDir,
		scenePreviewMaxDim:          cfg.Processing.MaxFrameDimension,
		markerPreviewCRF:            markerPreviewCRF,
		scenePreviewCRF:             scenePreviewCRF,
		logger:                      logger,
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

// validateMarkerEnd checks that an optional marker end lies after the start and within the scene.
func validateMarkerEnd(timestamp int, endTimestamp *int, sceneDuration int) error {
	if endTimestamp == nil {
		return nil
	}
	if *endTimestamp <= timestamp {
		return apperrors.NewValidationError(fmt.Sprintf("end_timestamp %d must be greater than timestamp %d", *endTimestamp, timestamp))
	}
	if sceneDuration > 0 && *endTimestamp > sceneDuration {
		return apperrors.NewValidationError(fmt.Sprintf("end_timestamp %d exceeds scene duration %d", *endTimestamp, sceneDuration))
	}
	return nil
}

func (s *MarkerService) CreateMarker(userID, sceneID uint, timestamp int, endTimestamp *int, label, color string) (*data.UserSceneMarker, error) {
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
	if err := validateMarkerEnd(timestamp, endTimestamp, scene.Duration); err != nil {
		return nil, err
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
		UserID:       userID,
		SceneID:      sceneID,
		Timestamp:    timestamp,
		EndTimestamp: endTimestamp,
		Label:        label,
		Color:        color,
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

	// The thumbnail is made in the background (best effort - the marker is useful without it)
	s.queueThumbnail(marker, scene)

	return marker, nil
}

// UpdateMarker updates the provided fields of a marker. endTimestamp sets the end of the marker
// range; clearEnd removes it (endTimestamp is then ignored).
func (s *MarkerService) UpdateMarker(userID, markerID uint, label *string, color *string, timestamp *int, endTimestamp *int, clearEnd bool) (*data.UserSceneMarker, error) {
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

	if clearEnd {
		marker.EndTimestamp = nil
	} else if endTimestamp != nil {
		end := *endTimestamp
		marker.EndTimestamp = &end
	}
	if marker.EndTimestamp != nil && (timestamp != nil || endTimestamp != nil) {
		if scene == nil {
			var err error
			scene, err = s.sceneRepo.GetByID(marker.SceneID)
			if err != nil {
				s.logger.Error("failed to get scene", zap.Uint("sceneID", marker.SceneID), zap.Error(err))
				return nil, apperrors.NewInternalError("failed to get scene", err)
			}
		}
		if err := validateMarkerEnd(marker.Timestamp, marker.EndTimestamp, scene.Duration); err != nil {
			return nil, err
		}
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

		// New thumbnail matching the current type, in the background
		if scene != nil {
			s.queueThumbnail(marker, scene)
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

// normalizeMarkerSort validates the sort against the allowed values (falling back to def) and
// makes sure a random sort has a seed, so that pages of a random listing stay consistent.
func normalizeMarkerSort(opts data.MarkerListOptions, valid map[string]bool, def string) data.MarkerListOptions {
	if opts.SortBy != "random" && !valid[opts.SortBy] {
		opts.SortBy = def
	}
	if opts.SortBy == "random" && opts.Seed == 0 {
		opts.Seed = rand.Int63n(9007199254740991) + 1
	}
	return opts
}

func (s *MarkerService) GetLabelGroups(userID uint, page, limit int, opts data.MarkerListOptions) ([]data.MarkerLabelGroup, int64, error) {
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
	opts = normalizeMarkerSort(opts, validSorts, "count_desc")

	groups, total, err := s.markerRepo.GetLabelGroupsForUser(userID, offset, limit, opts)
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

		thumbnails, err := s.markerRepo.GetRandomThumbnailsForLabels(userID, labels, 10, opts)
		if err != nil {
			s.logger.Warn("failed to get thumbnail IDs for labels", zap.Uint("userID", userID), zap.Error(err))
		} else {
			for i := range groups {
				refs := thumbnails[groups[i].Label]
				groups[i].ThumbnailMarkerIDs = make([]uint, len(refs))
				groups[i].ThumbnailVersions = make(map[uint]int64, len(refs))
				for j, ref := range refs {
					groups[i].ThumbnailMarkerIDs[j] = ref.ID
					groups[i].ThumbnailVersions[ref.ID] = data.MarkerThumbnailVersion(ref.UpdatedAt)
				}
			}
		}
	}

	return groups, total, nil
}

func (s *MarkerService) GetMarkersByLabel(userID uint, label string, page, limit int, opts data.MarkerListOptions) ([]data.MarkerWithScene, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	markers, total, err := s.markerRepo.GetMarkersByLabelForUser(userID, label, offset, limit, opts)
	if err != nil {
		s.logger.Error("failed to get markers by label", zap.Uint("userID", userID), zap.String("label", label), zap.Error(err))
		return nil, 0, apperrors.NewInternalError("failed to get markers by label", err)
	}
	s.attachMarkerTags(markers)
	return markers, total, nil
}

func (s *MarkerService) GetAllMarkers(userID uint, page, limit int, opts data.MarkerListOptions) ([]data.MarkerWithScene, int64, error) {
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
	opts = normalizeMarkerSort(opts, validSorts, "label_asc")

	markers, total, err := s.markerRepo.GetAllMarkersForUser(userID, offset, limit, opts)
	if err != nil {
		s.logger.Error("failed to get all markers", zap.Uint("userID", userID), zap.Error(err))
		return nil, 0, apperrors.NewInternalError("failed to get all markers", err)
	}
	s.attachMarkerTags(markers)
	return markers, total, nil
}

// attachMarkerTags loads the tags of listed markers in one query (best effort: a failure only
// leaves the tags empty).
func (s *MarkerService) attachMarkerTags(markers []data.MarkerWithScene) {
	if len(markers) == 0 {
		return
	}
	ids := make([]uint, len(markers))
	for i, m := range markers {
		ids[i] = m.ID
	}
	tagsByMarker, err := s.markerRepo.GetMarkerTagsMultiple(ids)
	if err != nil {
		s.logger.Warn("failed to load marker tags", zap.Error(err))
		return
	}
	for i := range markers {
		markers[i].Tags = tagsByMarker[markers[i].ID]
	}
}

// GetMarkersForScenes returns the user's markers grouped by scene (for scene cards).
func (s *MarkerService) GetMarkersForScenes(userID uint, sceneIDs []uint) (map[uint][]data.UserSceneMarker, error) {
	result := make(map[uint][]data.UserSceneMarker)
	if userID == 0 || len(sceneIDs) == 0 {
		return result, nil
	}
	markers, err := s.markerRepo.GetByUserAndScenes(userID, sceneIDs)
	if err != nil {
		s.logger.Error("failed to get markers for scenes", zap.Uint("userID", userID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to get markers for scenes", err)
	}
	for _, m := range markers {
		result[m.SceneID] = append(result[m.SceneID], m)
	}
	return result, nil
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
// Start starts the background marker thumbnail workers. Safe to call once.
func (s *MarkerService) Start() {
	s.startOnce.Do(func() {
		s.thumbnailQueue = make(chan markerThumbnailTask, markerThumbnailQueueSize)
		for i := 0; i < s.thumbnailWorkers; i++ {
			s.thumbnailWG.Add(1)
			go func() {
				defer s.thumbnailWG.Done()
				for task := range s.thumbnailQueue {
					s.runThumbnailTask(task)
				}
			}()
		}
		s.logger.Info("Marker thumbnail workers started", zap.Int("workers", s.thumbnailWorkers))
	})
}

// Stop lets the workers finish the queued thumbnails, waiting at most timeout.
func (s *MarkerService) Stop(timeout time.Duration) {
	s.stopOnce.Do(func() {
		if s.thumbnailQueue == nil {
			return
		}
		close(s.thumbnailQueue)
		done := make(chan struct{})
		go func() {
			s.thumbnailWG.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(timeout):
			s.logger.Warn("Marker thumbnails still running at shutdown")
		}
	})
}

// queueThumbnail has the marker's thumbnail made by a worker, or inline when the workers
// aren't running or their queue is full.
func (s *MarkerService) queueThumbnail(marker *data.UserSceneMarker, scene *data.Scene) {
	if s.thumbnailQueue != nil {
		select {
		case s.thumbnailQueue <- markerThumbnailTask{markerID: marker.ID}:
			return
		default:
			s.logger.Warn("Marker thumbnail queue full, making it inline", zap.Uint("markerID", marker.ID))
		}
	}
	s.makeThumbnail(marker, scene)
}

// runThumbnailTask makes a queued thumbnail from the marker's current row (its time may
// have changed since it was queued; a deleted marker is skipped).
func (s *MarkerService) runThumbnailTask(task markerThumbnailTask) {
	marker, err := s.markerRepo.GetByID(task.markerID)
	if err != nil || marker == nil {
		return
	}
	scene, err := s.sceneRepo.GetByID(marker.SceneID)
	if err != nil {
		s.logger.Warn("failed to get scene for marker thumbnail", zap.Uint("markerID", marker.ID), zap.Error(err))
		return
	}
	s.makeThumbnail(marker, scene)
}

// makeThumbnail makes the thumbnail type the server is configured for.
func (s *MarkerService) makeThumbnail(marker *data.UserSceneMarker, scene *data.Scene) {
	if s.markerThumbnailType == "animated" {
		if err := s.generateAnimatedThumbnail(marker, scene); err != nil {
			s.logger.Warn("failed to generate animated marker thumbnail",
				zap.Uint("markerID", marker.ID), zap.Uint("sceneID", scene.ID), zap.Error(err))
		}
		return
	}
	if err := s.generateThumbnail(marker, scene); err != nil {
		s.logger.Warn("failed to generate marker thumbnail",
			zap.Uint("markerID", marker.ID), zap.Uint("sceneID", scene.ID), zap.Error(err))
	}
}

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

	// Update marker with thumbnail path (only that column: the user may have edited the
	// marker while its thumbnail was being made)
	marker.ThumbnailPath = thumbnailFilename
	if err := s.markerRepo.UpdateThumbnailPath(marker.ID, thumbnailFilename); err != nil {
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

	animatedFilename := workerproto.MarkerClipName(marker.ID)
	animatedPath := filepath.Join(s.markerThumbnailDir, animatedFilename)

	if err := compute.MarkerClip(context.Background(), compute.Env{}, scene.StoredPath, animatedPath,
		marker.Timestamp, s.markerAnimatedDuration, s.markerThumbnailMaxDim, s.markerPreviewCRF,
		workerproto.DefaultMarkerClipTimeoutSec*time.Second); err != nil {
		return err
	}

	marker.AnimatedThumbnailPath = animatedFilename
	if err := s.markerRepo.UpdateAnimatedThumbnailPath(marker.ID, animatedFilename); err != nil {
		os.Remove(animatedPath)
		return fmt.Errorf("failed to update marker with animated thumbnail path: %w", err)
	}

	return nil
}

// PrepareAnimatedThumbnails returns the animated thumbnails to make for a scene: a clip per
// marker that lacks one (every marker when forceTarget is "markers" or "both") and the scene
// preview when enabled and missing (or forceTarget is "previews" or "both"). Nil when there
// is nothing to make. Implements jobs.AnimatedThumbnailGenerator (the server-side preparation
// of the animated_thumbnails phase, local or remote).
func (s *MarkerService) PrepareAnimatedThumbnails(sceneID uint, forceTarget string) (*jobs.AnimatedThumbnailsPlan, error) {
	var markers []data.UserSceneMarker
	var err error

	if forceTarget == "markers" || forceTarget == "both" {
		markers, err = s.markerRepo.GetAllByScene(sceneID)
	} else {
		markers, err = s.markerRepo.GetBySceneWithoutAnimatedThumbnail(sceneID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query markers for animated thumbnails: %w", err)
	}

	previewEnabled := s.scenePreviewEnabled
	if len(markers) == 0 && !previewEnabled {
		return nil, nil
	}

	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if len(markers) > 0 {
			return nil, fmt.Errorf("failed to get scene for animated thumbnail generation: %w", err)
		}
		// Only the preview was wanted: best effort, as before
		s.logger.Warn("Failed to generate scene preview",
			zap.Uint("scene_id", sceneID),
			zap.Error(fmt.Errorf("failed to get scene for preview generation: %w", err)))
		return nil, nil
	}

	params := workerproto.AnimatedThumbnailsParams{
		SceneID:          sceneID,
		Markers:          make([]workerproto.MarkerClipSpec, 0, len(markers)),
		MarkerWidth:      s.markerThumbnailMaxDim,
		MarkerCRF:        s.markerPreviewCRF,
		MarkerTimeoutSec: workerproto.DefaultMarkerClipTimeoutSec,
	}
	for _, m := range markers {
		params.Markers = append(params.Markers, workerproto.MarkerClipSpec{
			MarkerID:     m.ID,
			TimestampSec: m.Timestamp,
			DurationSec:  s.markerAnimatedDuration,
			Output:       workerproto.MarkerClipName(m.ID),
		})
	}

	forcePreview := forceTarget == "previews" || forceTarget == "both"
	if previewEnabled && (scene.PreviewVideoPath == "" || forcePreview) {
		params.Preview = &workerproto.ScenePreviewSpec{
			Output:          workerproto.ScenePreviewName(scene.ID),
			Segments:        s.scenePreviewSegments,
			SegmentDuration: s.scenePreviewSegmentDuration,
			Width:           s.scenePreviewMaxDim,
			CRF:             s.scenePreviewCRF,
			WithAudio:       scene.AudioCodec != "",
		}
	}

	if len(params.Markers) == 0 && params.Preview == nil {
		return nil, nil
	}
	if len(params.Markers) > 0 {
		s.logger.Info("Generating missing animated marker thumbnails",
			zap.Uint("scene_id", sceneID),
			zap.Int("count", len(params.Markers)))
	}

	return &jobs.AnimatedThumbnailsPlan{
		Input:  scene.StoredPath,
		Video:  SceneVideoInfo(scene),
		Params: params,
	}, nil
}

// CommitAnimatedThumbnails moves the clips and preview found in artifactDir into the marker
// thumbnail and scene preview directories and records them. Implements
// jobs.AnimatedThumbnailGenerator.
func (s *MarkerService) CommitAnimatedThumbnails(sceneID uint, res *workerproto.AnimatedThumbnailsResult, artifactDir string) (int, bool, error) {
	return jobs.CommitAnimatedThumbnails(jobs.AnimatedThumbnailsCommit{
		Markers:    s.markerRepo,
		Scenes:     s.sceneRepo,
		MarkerDir:  s.markerThumbnailDir,
		PreviewDir: s.scenePreviewDir,
	}, sceneID, res, artifactDir)
}

// SceneVideoInfo returns the probed facts of a scene in the compute/worker protocol form.
func SceneVideoInfo(scene *data.Scene) workerproto.VideoInfo {
	return workerproto.VideoInfo{
		DurationSec: scene.Duration,
		Width:       scene.Width,
		Height:      scene.Height,
		FrameRate:   scene.FrameRate,
		VideoCodec:  scene.VideoCodec,
		AudioCodec:  scene.AudioCodec,
	}
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

// SetScenePreviewEnabled updates the scene preview enabled setting
func (s *MarkerService) SetScenePreviewEnabled(enabled bool) {
	s.scenePreviewEnabled = enabled
}

// SetScenePreviewSegments updates the scene preview segments setting
func (s *MarkerService) SetScenePreviewSegments(segments int) {
	s.scenePreviewSegments = segments
}

// SetScenePreviewSegmentDuration updates the scene preview segment duration setting
func (s *MarkerService) SetScenePreviewSegmentDuration(duration float64) {
	s.scenePreviewSegmentDuration = duration
}

// SetMarkerPreviewCRF updates the marker preview CRF setting
func (s *MarkerService) SetMarkerPreviewCRF(crf int) {
	s.markerPreviewCRF = crf
}

// SetScenePreviewCRF updates the scene preview CRF setting
func (s *MarkerService) SetScenePreviewCRF(crf int) {
	s.scenePreviewCRF = crf
}

// ApplyQualityConfig copies the marker thumbnail and scene preview settings from the
// processing quality config. It runs at startup, so settings saved to the DB survive a
// restart, and again whenever the config is updated. Zero values leave the current setting.
func (s *MarkerService) ApplyQualityConfig(cfg ProcessingQualityConfig) {
	s.SetScenePreviewEnabled(cfg.ScenePreviewEnabled)
	if cfg.ScenePreviewSegments > 0 {
		s.SetScenePreviewSegments(cfg.ScenePreviewSegments)
	}
	if cfg.ScenePreviewSegmentDuration > 0 {
		s.SetScenePreviewSegmentDuration(cfg.ScenePreviewSegmentDuration)
	}
	if cfg.MarkerThumbnailType != "" {
		s.SetMarkerThumbnailType(cfg.MarkerThumbnailType)
	}
	if cfg.MarkerAnimatedDuration > 0 {
		s.SetMarkerAnimatedDuration(cfg.MarkerAnimatedDuration)
	}
	if cfg.MarkerPreviewCRF > 0 {
		s.SetMarkerPreviewCRF(cfg.MarkerPreviewCRF)
	}
	if cfg.ScenePreviewCRF > 0 {
		s.SetScenePreviewCRF(cfg.ScenePreviewCRF)
	}
}

// GetScenePreviewEnabled returns whether scene preview generation is enabled
func (s *MarkerService) GetScenePreviewEnabled() bool {
	return s.scenePreviewEnabled
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
