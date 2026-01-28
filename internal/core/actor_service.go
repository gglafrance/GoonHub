package core

import (
	"errors"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ActorService struct {
	actorRepo data.ActorRepository
	videoRepo data.VideoRepository
	logger    *zap.Logger
	indexer   VideoIndexer
}

func NewActorService(actorRepo data.ActorRepository, videoRepo data.VideoRepository, logger *zap.Logger) *ActorService {
	return &ActorService{
		actorRepo: actorRepo,
		videoRepo: videoRepo,
		logger:    logger,
	}
}

// SetIndexer sets the video indexer for search index updates.
func (s *ActorService) SetIndexer(indexer VideoIndexer) {
	s.indexer = indexer
}

type CreateActorInput struct {
	Name            string
	ImageURL        string
	Gender          string
	Birthday        *time.Time
	DateOfDeath     *time.Time
	Astrology       string
	Birthplace      string
	Ethnicity       string
	Nationality     string
	CareerStartYear *int
	CareerEndYear   *int
	HeightCm        *int
	WeightKg        *int
	Measurements    string
	Cupsize         string
	HairColor       string
	EyeColor        string
	Tattoos         string
	Piercings       string
	FakeBoobs       bool
	SameSexOnly     bool
}

type UpdateActorInput struct {
	Name            *string
	ImageURL        *string
	Gender          *string
	Birthday        *time.Time
	DateOfDeath     *time.Time
	Astrology       *string
	Birthplace      *string
	Ethnicity       *string
	Nationality     *string
	CareerStartYear *int
	CareerEndYear   *int
	HeightCm        *int
	WeightKg        *int
	Measurements    *string
	Cupsize         *string
	HairColor       *string
	EyeColor        *string
	Tattoos         *string
	Piercings       *string
	FakeBoobs       *bool
	SameSexOnly     *bool
}

func (s *ActorService) Create(input CreateActorInput) (*data.Actor, error) {
	if input.Name == "" {
		return nil, apperrors.NewValidationErrorWithField("name", "actor name is required")
	}
	if len(input.Name) > 255 {
		return nil, apperrors.NewValidationErrorWithField("name", "actor name must be 255 characters or less")
	}

	actor := &data.Actor{
		UUID:            uuid.New(),
		Name:            input.Name,
		ImageURL:        input.ImageURL,
		Gender:          input.Gender,
		Birthday:        input.Birthday,
		DateOfDeath:     input.DateOfDeath,
		Astrology:       input.Astrology,
		Birthplace:      input.Birthplace,
		Ethnicity:       input.Ethnicity,
		Nationality:     input.Nationality,
		CareerStartYear: input.CareerStartYear,
		CareerEndYear:   input.CareerEndYear,
		HeightCm:        input.HeightCm,
		WeightKg:        input.WeightKg,
		Measurements:    input.Measurements,
		Cupsize:         input.Cupsize,
		HairColor:       input.HairColor,
		EyeColor:        input.EyeColor,
		Tattoos:         input.Tattoos,
		Piercings:       input.Piercings,
		FakeBoobs:       input.FakeBoobs,
		SameSexOnly:     input.SameSexOnly,
	}

	if err := s.actorRepo.Create(actor); err != nil {
		return nil, apperrors.NewInternalError("failed to create actor", err)
	}

	s.logger.Info("Actor created", zap.String("name", input.Name), zap.String("uuid", actor.UUID.String()))
	return actor, nil
}

func (s *ActorService) GetByID(id uint) (*data.Actor, error) {
	actor, err := s.actorRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrActorNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find actor", err)
	}
	return actor, nil
}

func (s *ActorService) GetByUUID(uuid string) (*data.ActorWithCount, error) {
	actor, err := s.actorRepo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrActorNotFoundByName(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find actor", err)
	}

	videoCount, err := s.actorRepo.GetVideoCount(actor.ID)
	if err != nil {
		s.logger.Warn("Failed to get video count for actor", zap.Uint("actor_id", actor.ID), zap.Error(err))
		videoCount = 0
	}

	return &data.ActorWithCount{
		Actor:      *actor,
		VideoCount: videoCount,
	}, nil
}

func (s *ActorService) Update(id uint, input UpdateActorInput) (*data.Actor, error) {
	actor, err := s.actorRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrActorNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find actor", err)
	}

	if input.Name != nil {
		if *input.Name == "" {
			return nil, apperrors.NewValidationErrorWithField("name", "actor name is required")
		}
		if len(*input.Name) > 255 {
			return nil, apperrors.NewValidationErrorWithField("name", "actor name must be 255 characters or less")
		}
		actor.Name = *input.Name
	}
	if input.ImageURL != nil {
		actor.ImageURL = *input.ImageURL
	}
	if input.Gender != nil {
		actor.Gender = *input.Gender
	}
	if input.Birthday != nil {
		actor.Birthday = input.Birthday
	}
	if input.DateOfDeath != nil {
		actor.DateOfDeath = input.DateOfDeath
	}
	if input.Astrology != nil {
		actor.Astrology = *input.Astrology
	}
	if input.Birthplace != nil {
		actor.Birthplace = *input.Birthplace
	}
	if input.Ethnicity != nil {
		actor.Ethnicity = *input.Ethnicity
	}
	if input.Nationality != nil {
		actor.Nationality = *input.Nationality
	}
	if input.CareerStartYear != nil {
		actor.CareerStartYear = input.CareerStartYear
	}
	if input.CareerEndYear != nil {
		actor.CareerEndYear = input.CareerEndYear
	}
	if input.HeightCm != nil {
		actor.HeightCm = input.HeightCm
	}
	if input.WeightKg != nil {
		actor.WeightKg = input.WeightKg
	}
	if input.Measurements != nil {
		actor.Measurements = *input.Measurements
	}
	if input.Cupsize != nil {
		actor.Cupsize = *input.Cupsize
	}
	if input.HairColor != nil {
		actor.HairColor = *input.HairColor
	}
	if input.EyeColor != nil {
		actor.EyeColor = *input.EyeColor
	}
	if input.Tattoos != nil {
		actor.Tattoos = *input.Tattoos
	}
	if input.Piercings != nil {
		actor.Piercings = *input.Piercings
	}
	if input.FakeBoobs != nil {
		actor.FakeBoobs = *input.FakeBoobs
	}
	if input.SameSexOnly != nil {
		actor.SameSexOnly = *input.SameSexOnly
	}

	if err := s.actorRepo.Update(actor); err != nil {
		return nil, apperrors.NewInternalError("failed to update actor", err)
	}

	s.logger.Info("Actor updated", zap.Uint("id", id), zap.String("name", actor.Name))
	return actor, nil
}

func (s *ActorService) Delete(id uint) error {
	if _, err := s.actorRepo.GetByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrActorNotFound(id)
		}
		return apperrors.NewInternalError("failed to find actor", err)
	}

	if err := s.actorRepo.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete actor", err)
	}

	s.logger.Info("Actor deleted", zap.Uint("id", id))
	return nil
}

func (s *ActorService) List(page, limit int, query string) ([]data.ActorWithCount, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if query != "" {
		return s.actorRepo.Search(query, page, limit)
	}
	return s.actorRepo.List(page, limit)
}

func (s *ActorService) GetVideoActors(videoID uint) ([]data.Actor, error) {
	if _, err := s.videoRepo.GetByID(videoID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrVideoNotFound(videoID)
		}
		return nil, apperrors.NewInternalError("failed to find video", err)
	}

	return s.actorRepo.GetVideoActors(videoID)
}

func (s *ActorService) SetVideoActors(videoID uint, actorIDs []uint) ([]data.Actor, error) {
	video, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrVideoNotFound(videoID)
		}
		return nil, apperrors.NewInternalError("failed to find video", err)
	}

	if err := s.actorRepo.SetVideoActors(videoID, actorIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to set video actors", err)
	}

	// Re-index video in search engine after actor changes
	if s.indexer != nil {
		if err := s.indexer.UpdateVideoIndex(video); err != nil {
			s.logger.Warn("Failed to update video in search index after actor change",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
		}
	}

	return s.actorRepo.GetVideoActors(videoID)
}

func (s *ActorService) GetActorVideos(actorID uint, page, limit int) ([]data.Video, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if _, err := s.actorRepo.GetByID(actorID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, apperrors.ErrActorNotFound(actorID)
		}
		return nil, 0, apperrors.NewInternalError("failed to find actor", err)
	}

	return s.actorRepo.GetActorVideos(actorID, page, limit)
}

func (s *ActorService) UpdateImageURL(id uint, imageURL string) (*data.Actor, error) {
	actor, err := s.actorRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrActorNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find actor", err)
	}

	actor.ImageURL = imageURL
	if err := s.actorRepo.Update(actor); err != nil {
		return nil, apperrors.NewInternalError("failed to update actor image", err)
	}

	s.logger.Info("Actor image updated", zap.Uint("id", id), zap.String("image_url", imageURL))
	return actor, nil
}
