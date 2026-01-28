package core

import (
	"errors"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"regexp"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var colorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type TagService struct {
	tagRepo   data.TagRepository
	videoRepo data.VideoRepository
	logger    *zap.Logger
	indexer   VideoIndexer
}

func NewTagService(tagRepo data.TagRepository, videoRepo data.VideoRepository, logger *zap.Logger) *TagService {
	return &TagService{
		tagRepo:   tagRepo,
		videoRepo: videoRepo,
		logger:    logger,
	}
}

// SetIndexer sets the video indexer for search index updates.
func (s *TagService) SetIndexer(indexer VideoIndexer) {
	s.indexer = indexer
}

func (s *TagService) ListTags() ([]data.TagWithCount, error) {
	return s.tagRepo.ListWithCounts()
}

func (s *TagService) CreateTag(name, color string) (*data.Tag, error) {
	if name == "" {
		return nil, apperrors.NewValidationErrorWithField("name", "tag name is required")
	}
	if len(name) > 100 {
		return nil, apperrors.NewValidationErrorWithField("name", "tag name must be 100 characters or less")
	}

	if color == "" {
		color = "#6B7280"
	}
	if !colorRegex.MatchString(color) {
		return nil, apperrors.NewValidationErrorWithField("color", "invalid color format, must be a hex color like #6B7280")
	}

	tag := &data.Tag{
		Name:  name,
		Color: color,
	}

	if err := s.tagRepo.Create(tag); err != nil {
		// Check for unique constraint violation (tag already exists)
		return nil, apperrors.ErrTagAlreadyExists(name)
	}

	s.logger.Info("Tag created", zap.String("name", name), zap.String("color", color))
	return tag, nil
}

func (s *TagService) DeleteTag(id uint) error {
	if _, err := s.tagRepo.GetByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrTagNotFound(id)
		}
		return apperrors.NewInternalError("failed to find tag", err)
	}

	if err := s.tagRepo.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete tag", err)
	}

	s.logger.Info("Tag deleted", zap.Uint("id", id))
	return nil
}

func (s *TagService) GetVideoTags(videoID uint) ([]data.Tag, error) {
	if _, err := s.videoRepo.GetByID(videoID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrVideoNotFound(videoID)
		}
		return nil, apperrors.NewInternalError("failed to find video", err)
	}

	return s.tagRepo.GetVideoTags(videoID)
}

func (s *TagService) GetTagsByNames(names []string) ([]data.Tag, error) {
	return s.tagRepo.GetByNames(names)
}

func (s *TagService) SetVideoTags(videoID uint, tagIDs []uint) ([]data.Tag, error) {
	video, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrVideoNotFound(videoID)
		}
		return nil, apperrors.NewInternalError("failed to find video", err)
	}

	if err := s.tagRepo.SetVideoTags(videoID, tagIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to set video tags", err)
	}

	// Re-index video in search engine after tag changes
	if s.indexer != nil {
		if err := s.indexer.UpdateVideoIndex(video); err != nil {
			s.logger.Warn("Failed to update video in search index after tag change",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
		}
	}

	return s.tagRepo.GetVideoTags(videoID)
}
