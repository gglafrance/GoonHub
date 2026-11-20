package core

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Import API constants.
const (
	ImportAPIVersion         = 2
	MaxImportSummaryIDs      = 1000
	importDefaultMarkerColor = "#FFFFFF"
	importMaxMarkerLabelLen  = 100
	importMaxTitleLen        = 255
	importMaxStoredPathLen   = 512
	importMaxFilenameLen     = 255
	importReleaseDateLayout  = "2006-01-02"
)

// ImportFeatures lists the import API v2 features advertised by the capabilities endpoint.
var ImportFeatures = []string{
	"scene_associations",
	"scene_interactions",
	"scene_update_existing",
	"marker_dedup",
	"marker_tags",
	"marker_end",
	"scene_summary",
	"skip_index",
}

var importColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// ImportConflictError is returned when the imported entity already exists and
// update_existing was not requested. It unwraps to an *apperrors.ConflictError.
type ImportConflictError struct {
	ID      uint
	Title   string
	SceneID uint
	err     *apperrors.ConflictError
}

func (e *ImportConflictError) Error() string { return e.err.Error() }
func (e *ImportConflictError) Unwrap() error { return e.err }

// ImportWatchInput is a watch history entry to import.
type ImportWatchInput struct {
	WatchedAt     string
	WatchDuration int
	LastPosition  int
	Completed     bool
}

// ImportInteractionsInput holds the interactions applied for the current user.
// Nil fields mean "don't touch".
type ImportInteractionsInput struct {
	Rating      *float64
	Liked       *bool
	JizzedCount *int
	Watches     []ImportWatchInput
}

// ImportSceneInput is the input of ImportService.ImportScene.
// Nil slices / pointers mean "don't touch"; an empty non-nil slice clears the association.
type ImportSceneInput struct {
	Title            string
	StoredPath       string
	OriginalFilename string
	Size             int64
	Duration         int
	Width            int
	Height           int
	FrameRate        float64
	BitRate          int64
	VideoCodec       string
	AudioCodec       string
	Description      *string
	ReleaseDate      *string
	StudioID         *uint
	StoragePathID    *uint
	Origin           string
	Type             string
	SkipFileCheck    bool
	TagIDs           []uint
	ActorIDs         []uint
	ViewCount        *int64
	CreatedAt        *string
	FileCreatedAt    *string
	UpdateExisting   bool
	SkipIndex        bool
	Interactions     *ImportInteractionsInput
}

// ImportSceneResult is the outcome of a scene import.
type ImportSceneResult struct {
	ID      uint
	Title   string
	Created bool
}

// ImportMarkerInput is the input of ImportService.ImportMarker.
type ImportMarkerInput struct {
	SceneID        uint
	UserID         *uint
	Timestamp      int
	EndTimestamp   *int
	Label          string
	Color          string
	TagIDs         []uint
	UpdateExisting bool
}

// ImportMarkerResult is the outcome of a marker import.
type ImportMarkerResult struct {
	ID      uint
	SceneID uint
	Created bool
}

// ImportSceneSummary describes the imported state of a scene for the current user.
type ImportSceneSummary struct {
	ID          uint
	Title       string
	StoredPath  string
	StudioID    *uint
	TagIDs      []uint
	ActorIDs    []uint
	ViewCount   int64
	MarkerCount int64
	Rating      float64
	Liked       bool
	JizzedCount int
	WatchCount  int64
	Trashed     bool
}

// ImportSummaryResult is the outcome of ImportService.Summary.
type ImportSummaryResult struct {
	Scenes  []ImportSceneSummary
	Missing []uint
}

// ImportService implements the admin import API used by external importers.
type ImportService struct {
	importRepo      data.ImportRepository
	sceneRepo       data.SceneRepository
	tagRepo         data.TagRepository
	actorRepo       data.ActorRepository
	studioRepo      data.StudioRepository
	storagePathRepo data.StoragePathRepository
	userRepo        data.UserRepository
	interactionRepo data.InteractionRepository
	indexer         SceneIndexer
	logger          *zap.Logger
}

func NewImportService(
	importRepo data.ImportRepository,
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	storagePathRepo data.StoragePathRepository,
	userRepo data.UserRepository,
	interactionRepo data.InteractionRepository,
	indexer SceneIndexer,
	logger *zap.Logger,
) *ImportService {
	return &ImportService{
		importRepo:      importRepo,
		sceneRepo:       sceneRepo,
		tagRepo:         tagRepo,
		actorRepo:       actorRepo,
		studioRepo:      studioRepo,
		storagePathRepo: storagePathRepo,
		userRepo:        userRepo,
		interactionRepo: interactionRepo,
		indexer:         indexer,
		logger:          logger,
	}
}

// preparedWatch is a validated watch entry.
type preparedWatch struct {
	watchedAt     time.Time
	watchDuration int
	lastPosition  int
	completed     bool
}

// preparedInteractions is a validated interactions block.
type preparedInteractions struct {
	rating      *float64
	liked       *bool
	jizzedCount *int
	watches     []preparedWatch
}

// preparedScene holds the validated and resolved data of a scene import.
type preparedScene struct {
	releaseDate      *time.Time
	clearReleaseDate bool
	createdAt        *time.Time
	fileCreatedAt    *time.Time
	studio           *data.Studio
	tagIDs           []uint
	tagNames         []string
	actorIDs         []uint
	actorNames       []string
	interactions     *preparedInteractions
}

// ImportScene validates the input, then creates (or, with UpdateExisting, updates) the scene
// together with its associations and the current user's interactions in a single transaction.
// The scene is reindexed after commit unless SkipIndex is set.
func (s *ImportService) ImportScene(userID uint, in ImportSceneInput) (*ImportSceneResult, error) {
	prepared, err := s.prepareScene(userID, in)
	if err != nil {
		return nil, err
	}

	existing, err := s.sceneRepo.GetByStoredPath(in.StoredPath)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewInternalError("failed to check existing scene", err)
		}
		existing = nil
	}

	if existing != nil && !in.UpdateExisting {
		return nil, &ImportConflictError{
			ID:    existing.ID,
			Title: existing.Title,
			err:   apperrors.NewConflictError("scene", fmt.Sprintf("scene already exists at path: %s", in.StoredPath)),
		}
	}

	var sceneID uint
	created := existing == nil
	err = s.importRepo.RunInTransaction(func(tx data.ImportRepository) error {
		if created {
			scene := s.buildNewScene(in, prepared)
			if err := tx.CreateScene(scene); err != nil {
				return fmt.Errorf("failed to create scene record: %w", err)
			}
			sceneID = scene.ID
			if len(prepared.tagIDs) > 0 {
				if err := tx.SetSceneTags(sceneID, prepared.tagIDs); err != nil {
					return fmt.Errorf("failed to set scene tags: %w", err)
				}
			}
			if len(prepared.actorIDs) > 0 {
				if err := tx.SetSceneActors(sceneID, prepared.actorIDs); err != nil {
					return fmt.Errorf("failed to set scene actors: %w", err)
				}
			}
		} else {
			sceneID = existing.ID
			if err := tx.UpdateSceneFields(sceneID, s.buildUpdateFields(in, prepared)); err != nil {
				return fmt.Errorf("failed to update scene record: %w", err)
			}
			if prepared.tagIDs != nil {
				if err := tx.SetSceneTags(sceneID, prepared.tagIDs); err != nil {
					return fmt.Errorf("failed to set scene tags: %w", err)
				}
			}
			if prepared.actorIDs != nil {
				if err := tx.SetSceneActors(sceneID, prepared.actorIDs); err != nil {
					return fmt.Errorf("failed to set scene actors: %w", err)
				}
			}
		}
		return applyImportInteractions(tx, userID, sceneID, prepared.interactions)
	})
	if err != nil {
		s.logger.Error("failed to import scene",
			zap.String("stored_path", in.StoredPath),
			zap.Bool("update_existing", !created),
			zap.Error(err))
		return nil, apperrors.NewInternalError("failed to import scene", err)
	}

	if !in.SkipIndex {
		s.reindexScene(sceneID)
	}

	s.logger.Info("imported scene",
		zap.Uint("id", sceneID),
		zap.String("title", in.Title),
		zap.Bool("created", created))

	return &ImportSceneResult{ID: sceneID, Title: in.Title, Created: created}, nil
}

func (s *ImportService) prepareScene(userID uint, in ImportSceneInput) (*preparedScene, error) {
	p := &preparedScene{}

	if in.Title == "" {
		return nil, apperrors.NewValidationErrorWithField("title", "title is required")
	}
	if utf8.RuneCountInString(in.Title) > importMaxTitleLen {
		return nil, apperrors.NewValidationErrorWithField("title", fmt.Sprintf("title must be at most %d characters", importMaxTitleLen))
	}
	if in.StoredPath == "" {
		return nil, apperrors.NewValidationErrorWithField("stored_path", "stored_path is required")
	}
	if utf8.RuneCountInString(in.StoredPath) > importMaxStoredPathLen {
		return nil, apperrors.NewValidationErrorWithField("stored_path", fmt.Sprintf("stored_path must be at most %d characters", importMaxStoredPathLen))
	}
	if utf8.RuneCountInString(in.OriginalFilename) > importMaxFilenameLen {
		return nil, apperrors.NewValidationErrorWithField("original_filename", fmt.Sprintf("original_filename must be at most %d characters", importMaxFilenameLen))
	}

	if in.ReleaseDate != nil {
		if *in.ReleaseDate == "" {
			p.clearReleaseDate = true
		} else {
			parsed, err := time.Parse(importReleaseDateLayout, *in.ReleaseDate)
			if err != nil {
				return nil, apperrors.NewValidationErrorWithField("release_date",
					fmt.Sprintf("invalid release_date format, expected YYYY-MM-DD: %s", *in.ReleaseDate))
			}
			p.releaseDate = &parsed
		}
	}

	if in.Origin != "" && !data.IsValidSceneOrigin(in.Origin) {
		return nil, apperrors.NewValidationErrorWithField("origin", fmt.Sprintf("invalid origin: %s", in.Origin))
	}
	if in.Type != "" && !data.IsValidSceneType(in.Type) {
		return nil, apperrors.NewValidationErrorWithField("type", fmt.Sprintf("invalid type: %s", in.Type))
	}
	if in.ViewCount != nil && *in.ViewCount < 0 {
		return nil, apperrors.NewValidationErrorWithField("view_count", "view_count must be >= 0")
	}

	var err error
	if p.createdAt, err = parseImportTimestamp("created_at", in.CreatedAt); err != nil {
		return nil, err
	}
	if p.fileCreatedAt, err = parseImportTimestamp("file_created_at", in.FileCreatedAt); err != nil {
		return nil, err
	}

	if in.Interactions != nil {
		if userID == 0 {
			return nil, apperrors.NewValidationError("interactions require an authenticated user")
		}
		if p.interactions, err = prepareImportInteractions(in.Interactions); err != nil {
			return nil, err
		}
	}

	if !in.SkipFileCheck {
		if _, err := os.Stat(in.StoredPath); err != nil {
			if os.IsNotExist(err) {
				return nil, apperrors.NewValidationErrorWithField("stored_path",
					fmt.Sprintf("file not found at stored_path: %s", in.StoredPath))
			}
			return nil, apperrors.NewInternalError("failed to verify file", err)
		}
	}

	if in.TagIDs != nil {
		if p.tagIDs, p.tagNames, err = s.resolveTags(in.TagIDs); err != nil {
			return nil, err
		}
	}

	if in.ActorIDs != nil {
		if p.actorIDs, p.actorNames, err = s.resolveActors(in.ActorIDs); err != nil {
			return nil, err
		}
	}

	if in.StudioID != nil {
		studio, err := s.studioRepo.GetByID(*in.StudioID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperrors.NewValidationErrorWithField("studio_id", fmt.Sprintf("unknown studio_id: %d", *in.StudioID))
			}
			return nil, apperrors.NewInternalError("failed to look up studio", err)
		}
		p.studio = studio
	}

	if in.StoragePathID != nil {
		if _, err := s.storagePathRepo.GetByID(*in.StoragePathID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperrors.NewValidationErrorWithField("storage_path_id", fmt.Sprintf("unknown storage_path_id: %d", *in.StoragePathID))
			}
			return nil, apperrors.NewInternalError("failed to look up storage path", err)
		}
	}

	return p, nil
}

func parseImportTimestamp(field string, value *string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return nil, apperrors.NewValidationErrorWithField(field,
			fmt.Sprintf("invalid %s, expected RFC3339: %s", field, *value))
	}
	return &parsed, nil
}

func prepareImportInteractions(in *ImportInteractionsInput) (*preparedInteractions, error) {
	p := &preparedInteractions{liked: in.Liked}

	if in.Rating != nil && *in.Rating != 0 {
		rating := *in.Rating
		if math.IsNaN(rating) || rating < 0.5 || rating > 5.0 {
			return nil, apperrors.NewValidationErrorWithField("interactions.rating", "rating must be between 0.5 and 5.0 (0 = don't touch)")
		}
		rounded := math.Round(rating*2) / 2
		p.rating = &rounded
	}

	if in.JizzedCount != nil {
		if *in.JizzedCount < 0 {
			return nil, apperrors.NewValidationErrorWithField("interactions.jizzed_count", "jizzed_count must be >= 0")
		}
		count := *in.JizzedCount
		p.jizzedCount = &count
	}

	for i, w := range in.Watches {
		field := fmt.Sprintf("interactions.watches[%d]", i)
		if w.WatchedAt == "" {
			return nil, apperrors.NewValidationErrorWithField(field, fmt.Sprintf("%s.watched_at is required", field))
		}
		watchedAt, err := time.Parse(time.RFC3339, w.WatchedAt)
		if err != nil {
			return nil, apperrors.NewValidationErrorWithField(field,
				fmt.Sprintf("invalid %s.watched_at, expected RFC3339: %s", field, w.WatchedAt))
		}
		if w.WatchDuration < 0 || w.LastPosition < 0 {
			return nil, apperrors.NewValidationErrorWithField(field,
				fmt.Sprintf("%s.watch_duration and last_position must be >= 0", field))
		}
		p.watches = append(p.watches, preparedWatch{
			watchedAt:     watchedAt,
			watchDuration: w.WatchDuration,
			lastPosition:  w.LastPosition,
			completed:     w.Completed,
		})
	}

	return p, nil
}

// uniqueIDs deduplicates ids preserving order. A nil input yields nil; an empty one an empty slice.
func uniqueIDs(ids []uint) []uint {
	if ids == nil {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func missingIDs(requested []uint, found map[uint]struct{}) []uint {
	var missing []uint
	for _, id := range requested {
		if _, ok := found[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

func (s *ImportService) resolveTags(ids []uint) ([]uint, []string, error) {
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return ids, []string{}, nil
	}
	tags, err := s.tagRepo.GetByIDs(ids)
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to look up tags", err)
	}
	found := make(map[uint]struct{}, len(tags))
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		found[t.ID] = struct{}{}
		names = append(names, t.Name)
	}
	if missing := missingIDs(ids, found); len(missing) > 0 {
		return nil, nil, apperrors.NewValidationErrorWithField("tag_ids", fmt.Sprintf("unknown tag_ids: %v", missing))
	}
	sort.Strings(names)
	return ids, names, nil
}

func (s *ImportService) resolveActors(ids []uint) ([]uint, []string, error) {
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return ids, []string{}, nil
	}
	actors, err := s.actorRepo.GetByIDs(ids)
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to look up actors", err)
	}
	found := make(map[uint]struct{}, len(actors))
	names := make([]string, 0, len(actors))
	for _, a := range actors {
		found[a.ID] = struct{}{}
		names = append(names, a.Name)
	}
	if missing := missingIDs(ids, found); len(missing) > 0 {
		return nil, nil, apperrors.NewValidationErrorWithField("actor_ids", fmt.Sprintf("unknown actor_ids: %v", missing))
	}
	sort.Strings(names)
	return ids, names, nil
}

func (s *ImportService) buildNewScene(in ImportSceneInput, p *preparedScene) *data.Scene {
	originalFilename := in.OriginalFilename
	if originalFilename == "" {
		originalFilename = filepath.Base(in.StoredPath)
	}

	scene := &data.Scene{
		Title:            in.Title,
		StoredPath:       in.StoredPath,
		OriginalFilename: originalFilename,
		Size:             in.Size,
		Duration:         in.Duration,
		Width:            in.Width,
		Height:           in.Height,
		FrameRate:        in.FrameRate,
		BitRate:          in.BitRate,
		VideoCodec:       in.VideoCodec,
		AudioCodec:       in.AudioCodec,
		ReleaseDate:      p.releaseDate,
		StudioID:         in.StudioID,
		StoragePathID:    in.StoragePathID,
		ProcessingStatus: "completed",
		Origin:           in.Origin,
		Type:             in.Type,
		FileCreatedAt:    p.fileCreatedAt,
		Tags:             pq.StringArray{},
		Actors:           pq.StringArray{},
	}
	if in.Description != nil {
		scene.Description = *in.Description
	}
	if in.ViewCount != nil {
		scene.ViewCount = *in.ViewCount
	}
	if p.createdAt != nil {
		scene.CreatedAt = *p.createdAt
	}
	if p.tagNames != nil {
		scene.Tags = pq.StringArray(p.tagNames)
	}
	if p.actorNames != nil {
		scene.Actors = pq.StringArray(p.actorNames)
	}
	if p.studio != nil {
		scene.Studio = p.studio.Name
	}
	return scene
}

// buildUpdateFields returns the columns to update on an existing scene. Only provided
// fields are included; technical fields are only applied when non-zero.
func (s *ImportService) buildUpdateFields(in ImportSceneInput, p *preparedScene) map[string]any {
	fields := map[string]any{"title": in.Title}

	if in.Description != nil {
		fields["description"] = *in.Description
	}
	if p.releaseDate != nil {
		fields["release_date"] = *p.releaseDate
	} else if p.clearReleaseDate {
		fields["release_date"] = nil
	}
	if p.studio != nil {
		fields["studio_id"] = p.studio.ID
		fields["studio"] = p.studio.Name
	}
	if in.Origin != "" {
		fields["origin"] = in.Origin
	}
	if in.Type != "" {
		fields["type"] = in.Type
	}
	if in.ViewCount != nil {
		fields["view_count"] = *in.ViewCount
	}
	if p.fileCreatedAt != nil {
		fields["file_created_at"] = *p.fileCreatedAt
	}
	if p.tagNames != nil {
		fields["tags"] = pq.StringArray(p.tagNames)
	}
	if p.actorNames != nil {
		fields["actors"] = pq.StringArray(p.actorNames)
	}

	// Technical fields: applied only when provided (non-zero)
	if in.OriginalFilename != "" {
		fields["original_filename"] = in.OriginalFilename
	}
	if in.StoragePathID != nil {
		fields["storage_path_id"] = *in.StoragePathID
	}
	if in.Size != 0 {
		fields["size"] = in.Size
	}
	if in.Duration != 0 {
		fields["duration"] = in.Duration
	}
	if in.Width != 0 {
		fields["width"] = in.Width
	}
	if in.Height != 0 {
		fields["height"] = in.Height
	}
	if in.FrameRate != 0 {
		fields["frame_rate"] = in.FrameRate
	}
	if in.BitRate != 0 {
		fields["bit_rate"] = in.BitRate
	}
	if in.VideoCodec != "" {
		fields["video_codec"] = in.VideoCodec
	}
	if in.AudioCodec != "" {
		fields["audio_codec"] = in.AudioCodec
	}

	return fields
}

func applyImportInteractions(tx data.ImportRepository, userID, sceneID uint, p *preparedInteractions) error {
	if p == nil {
		return nil
	}
	if p.rating != nil {
		if err := tx.UpsertRating(userID, sceneID, *p.rating); err != nil {
			return fmt.Errorf("failed to set rating: %w", err)
		}
	}
	if p.liked != nil {
		if err := tx.SetLiked(userID, sceneID, *p.liked); err != nil {
			return fmt.Errorf("failed to set like: %w", err)
		}
	}
	if p.jizzedCount != nil {
		if err := tx.SetJizzedCount(userID, sceneID, *p.jizzedCount); err != nil {
			return fmt.Errorf("failed to set jizzed count: %w", err)
		}
	}
	for _, w := range p.watches {
		watch := &data.UserSceneWatch{
			UserID:        userID,
			SceneID:       sceneID,
			WatchedAt:     w.watchedAt,
			WatchDuration: w.watchDuration,
			LastPosition:  w.lastPosition,
			Completed:     w.completed,
		}
		if _, err := tx.InsertWatchIfAbsent(watch); err != nil {
			return fmt.Errorf("failed to insert watch history: %w", err)
		}
	}
	return nil
}

// reindexScene refreshes the scene in the search index (best effort).
func (s *ImportService) reindexScene(sceneID uint) {
	if s.indexer == nil {
		return
	}
	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		s.logger.Warn("failed to load imported scene for indexing", zap.Uint("scene_id", sceneID), zap.Error(err))
		return
	}
	if err := s.indexer.UpdateSceneIndex(scene); err != nil {
		s.logger.Warn("failed to index imported scene", zap.Uint("scene_id", sceneID), zap.Error(err))
	}
}

// ImportMarker creates a marker (without thumbnail generation), deduplicated on the natural key
// (user_id, scene_id, timestamp, label). With UpdateExisting, an existing marker gets its color,
// end timestamp (when provided) and individual tags updated instead of returning a conflict.
func (s *ImportService) ImportMarker(currentUserID uint, in ImportMarkerInput) (*ImportMarkerResult, error) {
	if in.SceneID == 0 {
		return nil, apperrors.NewValidationErrorWithField("scene_id", "scene_id is required")
	}
	if in.Timestamp < 0 {
		return nil, apperrors.NewValidationErrorWithField("timestamp", "timestamp must be >= 0")
	}
	if in.EndTimestamp != nil && *in.EndTimestamp <= in.Timestamp {
		return nil, apperrors.NewValidationErrorWithField("end_timestamp", "end_timestamp must be greater than timestamp")
	}
	if utf8.RuneCountInString(in.Label) > importMaxMarkerLabelLen {
		return nil, apperrors.NewValidationErrorWithField("label", fmt.Sprintf("label must be at most %d characters", importMaxMarkerLabelLen))
	}
	if in.Color != "" && !importColorPattern.MatchString(in.Color) {
		return nil, apperrors.NewValidationErrorWithField("color", "color must be a hex color like #FF4D4D")
	}

	userID := currentUserID
	if in.UserID != nil && *in.UserID != 0 && *in.UserID != currentUserID {
		if _, err := s.userRepo.GetByID(*in.UserID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperrors.NewValidationErrorWithField("user_id", fmt.Sprintf("unknown user_id: %d", *in.UserID))
			}
			return nil, apperrors.NewInternalError("failed to look up user", err)
		}
		userID = *in.UserID
	}
	if userID == 0 {
		return nil, apperrors.NewValidationErrorWithField("user_id", "user_id is required")
	}

	var tagIDs []uint
	if in.TagIDs != nil {
		var err error
		if tagIDs, _, err = s.resolveTags(in.TagIDs); err != nil {
			return nil, err
		}
	}

	if _, err := s.sceneRepo.GetByID(in.SceneID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(in.SceneID)
		}
		return nil, apperrors.NewInternalError("failed to verify scene", err)
	}

	result := &ImportMarkerResult{SceneID: in.SceneID}
	err := s.importRepo.RunInTransaction(func(tx data.ImportRepository) error {
		existing, err := tx.FindMarker(userID, in.SceneID, in.Timestamp, in.Label)
		if err != nil {
			return fmt.Errorf("failed to look up existing marker: %w", err)
		}

		if existing != nil {
			if !in.UpdateExisting {
				return &ImportConflictError{
					ID:      existing.ID,
					SceneID: existing.SceneID,
					err: apperrors.NewConflictError("marker", fmt.Sprintf(
						"marker already exists at %ds with label %q", in.Timestamp, in.Label)),
				}
			}
			result.ID = existing.ID
			if in.Color != "" && in.Color != existing.Color {
				if err := tx.UpdateMarkerColor(existing.ID, in.Color); err != nil {
					return fmt.Errorf("failed to update marker color: %w", err)
				}
			}
			if in.EndTimestamp != nil && (existing.EndTimestamp == nil || *existing.EndTimestamp != *in.EndTimestamp) {
				if err := tx.UpdateMarkerEndTimestamp(existing.ID, in.EndTimestamp); err != nil {
					return fmt.Errorf("failed to update marker end timestamp: %w", err)
				}
			}
			if err := tx.ApplyLabelTagsToMarker(userID, existing.ID, in.Label); err != nil {
				return fmt.Errorf("failed to apply label tags: %w", err)
			}
			if tagIDs != nil {
				if err := tx.SetMarkerTags(existing.ID, tagIDs); err != nil {
					return fmt.Errorf("failed to set marker tags: %w", err)
				}
			}
			return nil
		}

		color := in.Color
		if color == "" {
			color = importDefaultMarkerColor
		}
		marker := &data.UserSceneMarker{
			UserID:       userID,
			SceneID:      in.SceneID,
			Timestamp:    in.Timestamp,
			EndTimestamp: in.EndTimestamp,
			Label:        in.Label,
			Color:        color,
		}
		if err := tx.CreateMarker(marker); err != nil {
			return fmt.Errorf("failed to create marker record: %w", err)
		}
		result.ID = marker.ID
		result.Created = true
		if err := tx.ApplyLabelTagsToMarker(userID, marker.ID, in.Label); err != nil {
			return fmt.Errorf("failed to apply label tags: %w", err)
		}
		if len(tagIDs) > 0 {
			if err := tx.AddMarkerTags(marker.ID, tagIDs); err != nil {
				return fmt.Errorf("failed to add marker tags: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		var conflict *ImportConflictError
		if errors.As(err, &conflict) {
			return nil, conflict
		}
		s.logger.Error("failed to import marker", zap.Uint("scene_id", in.SceneID), zap.Error(err))
		return nil, apperrors.NewInternalError("failed to import marker", err)
	}

	s.logger.Info("imported marker",
		zap.Uint("id", result.ID),
		zap.Uint("scene_id", in.SceneID),
		zap.Bool("created", result.Created))
	return result, nil
}

// Summary returns the imported state of the given scenes for the current user.
// Missing lists the requested IDs that don't exist or are soft-deleted (trashed scenes are reported).
func (s *ImportService) Summary(userID uint, ids []uint) (*ImportSummaryResult, error) {
	if len(ids) > MaxImportSummaryIDs {
		return nil, apperrors.NewValidationErrorWithField("ids", fmt.Sprintf("at most %d ids per request", MaxImportSummaryIDs))
	}
	ids = uniqueIDs(ids)
	result := &ImportSummaryResult{Scenes: []ImportSceneSummary{}, Missing: []uint{}}
	if len(ids) == 0 {
		return result, nil
	}

	scenes, err := s.importRepo.GetScenesForSummary(ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load scenes", err)
	}
	byID := make(map[uint]*data.Scene, len(scenes))
	found := make(map[uint]struct{}, len(scenes))
	foundIDs := make([]uint, 0, len(scenes))
	for i := range scenes {
		byID[scenes[i].ID] = &scenes[i]
		found[scenes[i].ID] = struct{}{}
		foundIDs = append(foundIDs, scenes[i].ID)
	}
	if missing := missingIDs(ids, found); missing != nil {
		result.Missing = missing
	}
	if len(foundIDs) == 0 {
		return result, nil
	}

	tagsByScene, err := s.tagRepo.GetSceneTagsMultiple(foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load scene tags", err)
	}
	actorsByScene, err := s.actorRepo.GetSceneActorsMultiple(foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load scene actors", err)
	}
	ratings, err := s.interactionRepo.GetRatingsBySceneIDs(userID, foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load ratings", err)
	}
	likes, err := s.interactionRepo.GetLikesBySceneIDs(userID, foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load likes", err)
	}
	jizz, err := s.interactionRepo.GetJizzCountsBySceneIDs(userID, foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load jizzed counts", err)
	}
	markerCounts, err := s.importRepo.CountMarkersByScenes(userID, foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count markers", err)
	}
	watchCounts, err := s.importRepo.CountWatchesByScenes(userID, foundIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count watches", err)
	}

	for _, id := range ids {
		scene, ok := byID[id]
		if !ok {
			continue
		}
		tagIDs := make([]uint, 0, len(tagsByScene[id]))
		for _, t := range tagsByScene[id] {
			tagIDs = append(tagIDs, t.ID)
		}
		actorIDs := make([]uint, 0, len(actorsByScene[id]))
		for _, a := range actorsByScene[id] {
			actorIDs = append(actorIDs, a.ID)
		}
		result.Scenes = append(result.Scenes, ImportSceneSummary{
			ID:          scene.ID,
			Title:       scene.Title,
			StoredPath:  scene.StoredPath,
			StudioID:    scene.StudioID,
			TagIDs:      tagIDs,
			ActorIDs:    actorIDs,
			ViewCount:   scene.ViewCount,
			MarkerCount: markerCounts[id],
			Rating:      ratings[id],
			Liked:       likes[id],
			JizzedCount: jizz[id],
			WatchCount:  watchCounts[id],
			Trashed:     scene.TrashedAt != nil,
		})
	}
	return result, nil
}
