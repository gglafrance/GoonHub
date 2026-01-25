package core

import (
	"fmt"
	"goonhub/internal/data"
	"regexp"

	"go.uber.org/zap"
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
		return nil, fmt.Errorf("tag name is required")
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("tag name must be 100 characters or less")
	}

	if color == "" {
		color = "#6B7280"
	}
	if !colorRegex.MatchString(color) {
		return nil, fmt.Errorf("invalid color format, must be a hex color like #6B7280")
	}

	tag := &data.Tag{
		Name:  name,
		Color: color,
	}

	if err := s.tagRepo.Create(tag); err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	s.logger.Info("Tag created", zap.String("name", name), zap.String("color", color))
	return tag, nil
}

func (s *TagService) DeleteTag(id uint) error {
	if _, err := s.tagRepo.GetByID(id); err != nil {
		return fmt.Errorf("tag not found")
	}

	if err := s.tagRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	s.logger.Info("Tag deleted", zap.Uint("id", id))
	return nil
}

func (s *TagService) GetVideoTags(videoID uint) ([]data.Tag, error) {
	if _, err := s.videoRepo.GetByID(videoID); err != nil {
		return nil, fmt.Errorf("video not found")
	}

	return s.tagRepo.GetVideoTags(videoID)
}

func (s *TagService) GetTagsByNames(names []string) ([]data.Tag, error) {
	return s.tagRepo.GetByNames(names)
}

func (s *TagService) SetVideoTags(videoID uint, tagIDs []uint) ([]data.Tag, error) {
	video, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		return nil, fmt.Errorf("video not found")
	}

	if err := s.tagRepo.SetVideoTags(videoID, tagIDs); err != nil {
		return nil, fmt.Errorf("failed to set video tags: %w", err)
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
