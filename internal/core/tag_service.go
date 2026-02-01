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
	sceneRepo data.SceneRepository
	logger    *zap.Logger
	indexer   SceneIndexer
}

func NewTagService(tagRepo data.TagRepository, sceneRepo data.SceneRepository, logger *zap.Logger) *TagService {
	return &TagService{
		tagRepo:   tagRepo,
		sceneRepo: sceneRepo,
		logger:    logger,
	}
}

// SetIndexer sets the scene indexer for search index updates.
func (s *TagService) SetIndexer(indexer SceneIndexer) {
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

func (s *TagService) GetSceneTags(sceneID uint) ([]data.Tag, error) {
	if _, err := s.sceneRepo.GetByID(sceneID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(sceneID)
		}
		return nil, apperrors.NewInternalError("failed to find scene", err)
	}

	return s.tagRepo.GetSceneTags(sceneID)
}

func (s *TagService) GetTagsByNames(names []string) ([]data.Tag, error) {
	return s.tagRepo.GetByNames(names)
}

func (s *TagService) SetSceneTags(sceneID uint, tagIDs []uint) ([]data.Tag, error) {
	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(sceneID)
		}
		return nil, apperrors.NewInternalError("failed to find scene", err)
	}

	if err := s.tagRepo.SetSceneTags(sceneID, tagIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to set scene tags", err)
	}

	// Re-index scene in search engine after tag changes
	if s.indexer != nil {
		if err := s.indexer.UpdateSceneIndex(scene); err != nil {
			s.logger.Warn("Failed to update scene in search index after tag change",
				zap.Uint("scene_id", sceneID),
				zap.Error(err),
			)
		}
	}

	return s.tagRepo.GetSceneTags(sceneID)
}
