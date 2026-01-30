package core

import (
	"errors"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type StudioService struct {
	studioRepo data.StudioRepository
	videoRepo  data.VideoRepository
	logger     *zap.Logger
	indexer    VideoIndexer
}

func NewStudioService(studioRepo data.StudioRepository, videoRepo data.VideoRepository, logger *zap.Logger) *StudioService {
	return &StudioService{
		studioRepo: studioRepo,
		videoRepo:  videoRepo,
		logger:     logger,
	}
}

// SetIndexer sets the video indexer for search index updates.
func (s *StudioService) SetIndexer(indexer VideoIndexer) {
	s.indexer = indexer
}

type CreateStudioInput struct {
	Name        string
	ShortName   string
	URL         string
	Description string
	Rating      *float64
	Logo        string
	Favicon     string
	Poster      string
	PornDBID    string
	ParentID    *uint
	NetworkID   *uint
}

type UpdateStudioInput struct {
	Name        *string
	ShortName   *string
	URL         *string
	Description *string
	Rating      *float64
	Logo        *string
	Favicon     *string
	Poster      *string
	PornDBID    *string
	ParentID    *uint
	NetworkID   *uint
}

func (s *StudioService) Create(input CreateStudioInput) (*data.Studio, error) {
	if input.Name == "" {
		return nil, apperrors.NewValidationErrorWithField("name", "studio name is required")
	}
	if len(input.Name) > 255 {
		return nil, apperrors.NewValidationErrorWithField("name", "studio name must be 255 characters or less")
	}

	studio := &data.Studio{
		UUID:        uuid.New(),
		Name:        input.Name,
		ShortName:   input.ShortName,
		URL:         input.URL,
		Description: input.Description,
		Rating:      input.Rating,
		Logo:        input.Logo,
		Favicon:     input.Favicon,
		Poster:      input.Poster,
		PornDBID:    input.PornDBID,
		ParentID:    input.ParentID,
		NetworkID:   input.NetworkID,
	}

	if err := s.studioRepo.Create(studio); err != nil {
		return nil, apperrors.NewInternalError("failed to create studio", err)
	}

	s.logger.Info("Studio created", zap.String("name", input.Name), zap.String("uuid", studio.UUID.String()))
	return studio, nil
}

func (s *StudioService) GetByID(id uint) (*data.Studio, error) {
	studio, err := s.studioRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrStudioNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find studio", err)
	}
	return studio, nil
}

func (s *StudioService) GetByUUID(uuid string) (*data.StudioWithCount, error) {
	studio, err := s.studioRepo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrStudioNotFoundByName(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find studio", err)
	}

	videoCount, err := s.studioRepo.GetVideoCount(studio.ID)
	if err != nil {
		s.logger.Warn("Failed to get video count for studio", zap.Uint("studio_id", studio.ID), zap.Error(err))
		videoCount = 0
	}

	return &data.StudioWithCount{
		Studio:     *studio,
		VideoCount: videoCount,
	}, nil
}

func (s *StudioService) GetByName(name string) (*data.Studio, error) {
	studio, err := s.studioRepo.GetByName(name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrStudioNotFoundByName(name)
		}
		return nil, apperrors.NewInternalError("failed to find studio", err)
	}
	return studio, nil
}

func (s *StudioService) Update(id uint, input UpdateStudioInput) (*data.Studio, error) {
	studio, err := s.studioRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrStudioNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find studio", err)
	}

	if input.Name != nil {
		if *input.Name == "" {
			return nil, apperrors.NewValidationErrorWithField("name", "studio name is required")
		}
		if len(*input.Name) > 255 {
			return nil, apperrors.NewValidationErrorWithField("name", "studio name must be 255 characters or less")
		}
		studio.Name = *input.Name
	}
	if input.ShortName != nil {
		studio.ShortName = *input.ShortName
	}
	if input.URL != nil {
		studio.URL = *input.URL
	}
	if input.Description != nil {
		studio.Description = *input.Description
	}
	if input.Rating != nil {
		studio.Rating = input.Rating
	}
	if input.Logo != nil {
		studio.Logo = *input.Logo
	}
	if input.Favicon != nil {
		studio.Favicon = *input.Favicon
	}
	if input.Poster != nil {
		studio.Poster = *input.Poster
	}
	if input.PornDBID != nil {
		studio.PornDBID = *input.PornDBID
	}
	if input.ParentID != nil {
		studio.ParentID = input.ParentID
	}
	if input.NetworkID != nil {
		studio.NetworkID = input.NetworkID
	}

	if err := s.studioRepo.Update(studio); err != nil {
		return nil, apperrors.NewInternalError("failed to update studio", err)
	}

	s.logger.Info("Studio updated", zap.Uint("id", id), zap.String("name", studio.Name))
	return studio, nil
}

func (s *StudioService) Delete(id uint) error {
	if _, err := s.studioRepo.GetByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrStudioNotFound(id)
		}
		return apperrors.NewInternalError("failed to find studio", err)
	}

	if err := s.studioRepo.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete studio", err)
	}

	s.logger.Info("Studio deleted", zap.Uint("id", id))
	return nil
}

func (s *StudioService) List(page, limit int, query string) ([]data.StudioWithCount, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if query != "" {
		return s.studioRepo.Search(query, page, limit)
	}
	return s.studioRepo.List(page, limit)
}

func (s *StudioService) GetVideoStudio(videoID uint) (*data.Studio, error) {
	if _, err := s.videoRepo.GetByID(videoID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrVideoNotFound(videoID)
		}
		return nil, apperrors.NewInternalError("failed to find video", err)
	}

	return s.studioRepo.GetVideoStudio(videoID)
}

func (s *StudioService) SetVideoStudio(videoID uint, studioID *uint) (*data.Studio, error) {
	video, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrVideoNotFound(videoID)
		}
		return nil, apperrors.NewInternalError("failed to find video", err)
	}

	// Validate studio exists if studioID is not nil
	var studio *data.Studio
	if studioID != nil {
		studio, err = s.studioRepo.GetByID(*studioID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperrors.ErrStudioNotFound(*studioID)
			}
			return nil, apperrors.NewInternalError("failed to find studio", err)
		}
	}

	if err := s.studioRepo.SetVideoStudio(videoID, studioID); err != nil {
		return nil, apperrors.NewInternalError("failed to set video studio", err)
	}

	// Re-index video in search engine after studio change
	if s.indexer != nil {
		if err := s.indexer.UpdateVideoIndex(video); err != nil {
			s.logger.Warn("Failed to update video in search index after studio change",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
		}
	}

	return studio, nil
}

func (s *StudioService) GetStudioVideos(studioID uint, page, limit int) ([]data.Video, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if _, err := s.studioRepo.GetByID(studioID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, apperrors.ErrStudioNotFound(studioID)
		}
		return nil, 0, apperrors.NewInternalError("failed to find studio", err)
	}

	return s.studioRepo.GetStudioVideos(studioID, page, limit)
}

func (s *StudioService) UpdateLogoURL(id uint, logoURL string) (*data.Studio, error) {
	studio, err := s.studioRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrStudioNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find studio", err)
	}

	studio.Logo = logoURL
	if err := s.studioRepo.Update(studio); err != nil {
		return nil, apperrors.NewInternalError("failed to update studio logo", err)
	}

	s.logger.Info("Studio logo updated", zap.Uint("id", id), zap.String("logo", logoURL))
	return studio, nil
}

// GetOrCreateByName returns an existing studio by name or creates a new one
func (s *StudioService) GetOrCreateByName(name string) (*data.Studio, error) {
	if name == "" {
		return nil, apperrors.NewValidationErrorWithField("name", "studio name is required")
	}

	// Try to find existing studio
	studio, err := s.studioRepo.GetByName(name)
	if err == nil {
		return studio, nil
	}

	// If not found, create new studio
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.Create(CreateStudioInput{Name: name})
	}

	return nil, apperrors.NewInternalError("failed to get or create studio", err)
}
