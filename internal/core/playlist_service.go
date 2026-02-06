package core

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

// PlaylistService handles playlist business logic
type PlaylistService struct {
	repo      data.PlaylistRepository
	sceneRepo data.SceneRepository
	tagRepo   data.TagRepository
	logger    *zap.Logger
}

// NewPlaylistService creates a new PlaylistService
func NewPlaylistService(repo data.PlaylistRepository, sceneRepo data.SceneRepository, tagRepo data.TagRepository, logger *zap.Logger) *PlaylistService {
	return &PlaylistService{
		repo:      repo,
		sceneRepo: sceneRepo,
		tagRepo:   tagRepo,
		logger:    logger,
	}
}

// CreatePlaylistInput holds input for creating a playlist
type CreatePlaylistInput struct {
	Name        string
	Description *string
	Visibility  string
	TagIDs      []uint
	SceneIDs    []uint
}

// UpdatePlaylistInput holds input for updating a playlist
type UpdatePlaylistInput struct {
	Name        *string
	Description *string
	Visibility  *string
}

// PlaylistOwner contains owner info for responses
type PlaylistOwner struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// PlaylistThumbnailScene contains minimal scene info for thumbnail grid
type PlaylistThumbnailScene struct {
	ID            uint   `json:"id"`
	ThumbnailPath string `json:"thumbnail_path"`
}

// PlaylistTagInfo contains tag info for responses
type PlaylistTagInfo struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// PlaylistListItem is an enriched playlist for list views
type PlaylistListItem struct {
	UUID            string                   `json:"uuid"`
	Name            string                   `json:"name"`
	Description     *string                  `json:"description"`
	Visibility      string                   `json:"visibility"`
	SceneCount      int64                    `json:"scene_count"`
	TotalDuration   int64                    `json:"total_duration"`
	Owner           PlaylistOwner            `json:"owner"`
	Tags            []PlaylistTagInfo        `json:"tags"`
	ThumbnailScenes []PlaylistThumbnailScene `json:"thumbnail_scenes"`
	IsLiked         bool                     `json:"is_liked"`
	LikeCount       int64                    `json:"like_count"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
}

// PlaylistSceneEntry is a scene entry within a playlist
type PlaylistSceneEntry struct {
	Position int        `json:"position"`
	Scene    data.Scene `json:"scene"`
	AddedAt  time.Time  `json:"added_at"`
}

// PlaylistResume holds resume position info
type PlaylistResume struct {
	SceneID   *uint   `json:"scene_id"`
	PositionS float64 `json:"position_s"`
}

// PlaylistDetail extends PlaylistListItem with scenes and resume info
type PlaylistDetail struct {
	PlaylistListItem
	Scenes []PlaylistSceneEntry `json:"scenes"`
	Resume *PlaylistResume      `json:"resume"`
}

func validateVisibility(v string) error {
	if v != "" && v != "private" && v != "public" {
		return apperrors.ErrPlaylistInvalidVisibility
	}
	return nil
}

// Create creates a new playlist
func (s *PlaylistService) Create(userID uint, input CreatePlaylistInput) (*data.Playlist, error) {
	if input.Name == "" {
		return nil, apperrors.ErrPlaylistNameRequired
	}
	if len(input.Name) > 255 {
		return nil, apperrors.ErrPlaylistNameTooLong
	}
	if err := validateVisibility(input.Visibility); err != nil {
		return nil, err
	}

	visibility := input.Visibility
	if visibility == "" {
		visibility = "private"
	}

	playlist := &data.Playlist{
		UserID:      userID,
		Name:        input.Name,
		Description: input.Description,
		Visibility:  visibility,
	}

	if err := s.repo.Create(playlist); err != nil {
		return nil, apperrors.NewInternalError("failed to create playlist", err)
	}

	// Set tags if provided
	if len(input.TagIDs) > 0 {
		if err := s.repo.SetPlaylistTags(playlist.ID, input.TagIDs); err != nil {
			s.logger.Warn("failed to set tags on new playlist", zap.Error(err))
		}
	}

	// Add scenes if provided
	if len(input.SceneIDs) > 0 {
		if err := s.repo.AddScenes(playlist.ID, input.SceneIDs); err != nil {
			s.logger.Warn("failed to add scenes to new playlist", zap.Error(err))
		}
	}

	s.logger.Info("Playlist created",
		zap.Uint("user_id", userID),
		zap.String("name", input.Name),
		zap.String("uuid", playlist.UUID.String()),
	)

	// Re-fetch to get User populated
	created, err := s.repo.GetByID(playlist.ID)
	if err != nil {
		return playlist, nil
	}
	return created, nil
}

// GetByUUID returns a playlist detail by UUID
func (s *PlaylistService) GetByUUID(userID uint, uuid string) (*PlaylistDetail, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrPlaylistNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find playlist", err)
	}

	// Access check: owner always, public to any auth user
	if playlist.UserID != userID && playlist.Visibility != "public" {
		return nil, apperrors.ErrPlaylistForbidden
	}

	// Build detail
	item, err := s.enrichPlaylistItem(userID, playlist)
	if err != nil {
		return nil, err
	}

	// Get scenes
	playlistScenes, err := s.repo.GetPlaylistScenes(playlist.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get playlist scenes", err)
	}

	entries := make([]PlaylistSceneEntry, len(playlistScenes))
	for i, ps := range playlistScenes {
		entries[i] = PlaylistSceneEntry{
			Position: ps.Position,
			Scene:    ps.Scene,
			AddedAt:  ps.AddedAt,
		}
	}

	// Get resume progress
	var resume *PlaylistResume
	progress, err := s.repo.GetProgress(userID, playlist.ID)
	if err != nil {
		s.logger.Warn("failed to get playlist progress", zap.Error(err))
	}
	if progress != nil {
		resume = &PlaylistResume{
			SceneID:   progress.LastSceneID,
			PositionS: progress.LastPositionS,
		}
	}

	return &PlaylistDetail{
		PlaylistListItem: *item,
		Scenes:           entries,
		Resume:           resume,
	}, nil
}

// List returns a paginated list of playlists
func (s *PlaylistService) List(userID uint, params data.PlaylistListParams) ([]PlaylistListItem, int64, error) {
	params.UserID = userID
	playlists, total, err := s.repo.List(params)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to list playlists", err)
	}

	items := make([]PlaylistListItem, len(playlists))
	for i, p := range playlists {
		item, err := s.enrichPlaylistItem(userID, &p)
		if err != nil {
			s.logger.Warn("failed to enrich playlist item", zap.Uint("playlist_id", p.ID), zap.Error(err))
			items[i] = PlaylistListItem{
				UUID:       p.UUID.String(),
				Name:       p.Name,
				Visibility: p.Visibility,
				CreatedAt:  p.CreatedAt,
				UpdatedAt:  p.UpdatedAt,
			}
			continue
		}
		items[i] = *item
	}

	return items, total, nil
}

// Update updates a playlist
func (s *PlaylistService) Update(userID uint, uuid string, input UpdatePlaylistInput) (*data.Playlist, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrPlaylistNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID {
		return nil, apperrors.ErrPlaylistForbidden
	}

	if input.Name != nil {
		if *input.Name == "" {
			return nil, apperrors.ErrPlaylistNameRequired
		}
		if len(*input.Name) > 255 {
			return nil, apperrors.ErrPlaylistNameTooLong
		}
		playlist.Name = *input.Name
	}

	if input.Description != nil {
		playlist.Description = input.Description
	}

	if input.Visibility != nil {
		if err := validateVisibility(*input.Visibility); err != nil {
			return nil, err
		}
		playlist.Visibility = *input.Visibility
	}

	if err := s.repo.Update(playlist); err != nil {
		return nil, apperrors.NewInternalError("failed to update playlist", err)
	}

	s.logger.Info("Playlist updated",
		zap.Uint("user_id", userID),
		zap.String("uuid", uuid),
	)

	return playlist, nil
}

// Delete deletes a playlist
func (s *PlaylistService) Delete(userID uint, uuid string) error {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrPlaylistNotFound(uuid)
		}
		return apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID {
		return apperrors.ErrPlaylistForbidden
	}

	if err := s.repo.Delete(playlist.ID); err != nil {
		return apperrors.NewInternalError("failed to delete playlist", err)
	}

	s.logger.Info("Playlist deleted",
		zap.Uint("user_id", userID),
		zap.String("uuid", uuid),
	)

	return nil
}

// AddScenes adds scenes to a playlist
func (s *PlaylistService) AddScenes(userID uint, uuid string, sceneIDs []uint) error {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrPlaylistNotFound(uuid)
		}
		return apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID {
		return apperrors.ErrPlaylistForbidden
	}

	if err := s.repo.AddScenes(playlist.ID, sceneIDs); err != nil {
		if data.IsDuplicateScene(err) {
			return apperrors.ErrPlaylistSceneAlreadyAdded
		}
		return apperrors.NewInternalError("failed to add scenes to playlist", err)
	}

	return nil
}

// RemoveScene removes a scene from a playlist
func (s *PlaylistService) RemoveScene(userID uint, uuid string, sceneID uint) error {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrPlaylistNotFound(uuid)
		}
		return apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID {
		return apperrors.ErrPlaylistForbidden
	}

	if err := s.repo.RemoveScene(playlist.ID, sceneID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrPlaylistSceneNotInPlaylist
		}
		return apperrors.NewInternalError("failed to remove scene from playlist", err)
	}

	return nil
}

// ReorderScenes reorders scenes in a playlist
func (s *PlaylistService) ReorderScenes(userID uint, uuid string, sceneIDs []uint) error {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrPlaylistNotFound(uuid)
		}
		return apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID {
		return apperrors.ErrPlaylistForbidden
	}

	if err := s.repo.ReorderScenes(playlist.ID, sceneIDs); err != nil {
		return apperrors.NewInternalError("failed to reorder scenes", err)
	}

	return nil
}

// SetTags sets tags on a playlist
func (s *PlaylistService) SetTags(userID uint, uuid string, tagIDs []uint) ([]data.Tag, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrPlaylistNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID {
		return nil, apperrors.ErrPlaylistForbidden
	}

	if err := s.repo.SetPlaylistTags(playlist.ID, tagIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to set playlist tags", err)
	}

	tags, err := s.repo.GetPlaylistTags(playlist.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get playlist tags", err)
	}

	return tags, nil
}

// GetTags returns tags for a playlist
func (s *PlaylistService) GetTags(userID uint, uuid string) ([]data.Tag, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrPlaylistNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find playlist", err)
	}

	if playlist.UserID != userID && playlist.Visibility != "public" {
		return nil, apperrors.ErrPlaylistForbidden
	}

	tags, err := s.repo.GetPlaylistTags(playlist.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get playlist tags", err)
	}

	return tags, nil
}

// ToggleLike toggles a like on a playlist
func (s *PlaylistService) ToggleLike(userID uint, uuid string) (bool, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, apperrors.ErrPlaylistNotFound(uuid)
		}
		return false, apperrors.NewInternalError("failed to find playlist", err)
	}

	// Must be public or own playlist
	if playlist.UserID != userID && playlist.Visibility != "public" {
		return false, apperrors.ErrPlaylistForbidden
	}

	liked, err := s.repo.ToggleLike(userID, playlist.ID)
	if err != nil {
		return false, apperrors.NewInternalError("failed to toggle like", err)
	}

	return liked, nil
}

// GetLikeStatus returns whether a user has liked a playlist
func (s *PlaylistService) GetLikeStatus(userID uint, uuid string) (bool, int64, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, 0, apperrors.ErrPlaylistNotFound(uuid)
		}
		return false, 0, apperrors.NewInternalError("failed to find playlist", err)
	}

	liked, err := s.repo.GetLikeStatus(userID, playlist.ID)
	if err != nil {
		return false, 0, apperrors.NewInternalError("failed to get like status", err)
	}

	count, err := s.repo.GetLikeCount(playlist.ID)
	if err != nil {
		return liked, 0, apperrors.NewInternalError("failed to get like count", err)
	}

	return liked, count, nil
}

// GetProgress returns resume progress for a playlist
func (s *PlaylistService) GetProgress(userID uint, uuid string) (*PlaylistResume, error) {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrPlaylistNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find playlist", err)
	}

	progress, err := s.repo.GetProgress(userID, playlist.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get progress", err)
	}

	if progress == nil {
		return &PlaylistResume{}, nil
	}

	return &PlaylistResume{
		SceneID:   progress.LastSceneID,
		PositionS: progress.LastPositionS,
	}, nil
}

// UpdateProgress updates resume progress for a playlist
func (s *PlaylistService) UpdateProgress(userID uint, uuid string, sceneID uint, positionS float64) error {
	playlist, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrPlaylistNotFound(uuid)
		}
		return apperrors.NewInternalError("failed to find playlist", err)
	}

	if err := s.repo.UpsertProgress(userID, playlist.ID, sceneID, positionS); err != nil {
		return apperrors.NewInternalError("failed to update progress", err)
	}

	return nil
}

// enrichPlaylistItem enriches a playlist with stats, tags, thumbnails, and like info
func (s *PlaylistService) enrichPlaylistItem(userID uint, p *data.Playlist) (*PlaylistListItem, error) {
	sceneCount, err := s.repo.GetSceneCount(p.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get scene count", err)
	}

	totalDuration, err := s.repo.GetTotalDuration(p.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get total duration", err)
	}

	tags, err := s.repo.GetPlaylistTags(p.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get playlist tags", err)
	}

	tagInfos := make([]PlaylistTagInfo, len(tags))
	for i, t := range tags {
		tagInfos[i] = PlaylistTagInfo{
			ID:    t.ID,
			Name:  t.Name,
			Color: t.Color,
		}
	}

	thumbScenes, err := s.repo.GetThumbnailScenes(p.ID, 4)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get thumbnail scenes", err)
	}

	thumbnails := make([]PlaylistThumbnailScene, len(thumbScenes))
	for i, sc := range thumbScenes {
		thumbnails[i] = PlaylistThumbnailScene{
			ID:            sc.ID,
			ThumbnailPath: sc.ThumbnailPath,
		}
	}

	isLiked, err := s.repo.GetLikeStatus(userID, p.ID)
	if err != nil {
		s.logger.Warn("failed to get like status", zap.Error(err))
	}

	likeCount, err := s.repo.GetLikeCount(p.ID)
	if err != nil {
		s.logger.Warn("failed to get like count", zap.Error(err))
	}

	owner := PlaylistOwner{
		ID:       p.UserID,
		Username: p.User.Username,
	}

	return &PlaylistListItem{
		UUID:            p.UUID.String(),
		Name:            p.Name,
		Description:     p.Description,
		Visibility:      p.Visibility,
		SceneCount:      sceneCount,
		TotalDuration:   totalDuration,
		Owner:           owner,
		Tags:            tagInfos,
		ThumbnailScenes: thumbnails,
		IsLiked:         isLiked,
		LikeCount:       likeCount,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}, nil
}
