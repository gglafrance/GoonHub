package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/pkg/ffmpeg"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

type VideoService struct {
	Repo              data.VideoRepository
	DataPath          string
	ProcessingService *VideoProcessingService
	EventBus          *EventBus
	logger            *zap.Logger
}

func NewVideoService(repo data.VideoRepository, dataPath string, processingService *VideoProcessingService, eventBus *EventBus, logger *zap.Logger) *VideoService {
	// Ensure data directory exists
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		logger.Warn("Failed to create data directory",
			zap.String("directory", dataPath),
			zap.Error(err),
		)
	}
	return &VideoService{
		Repo:              repo,
		DataPath:          dataPath,
		ProcessingService: processingService,
		EventBus:          eventBus,
		logger:            logger,
	}
}

var AllowedExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".wmv":  true,
	".m4v":  true,
}

func (s *VideoService) ValidateExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return AllowedExtensions[ext]
}

func (s *VideoService) UploadVideo(file *multipart.FileHeader, title string) (*data.Video, error) {
	if !s.ValidateExtension(file.Filename) {
		return nil, fmt.Errorf("invalid file extension")
	}

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// Generate unique filename
	uniqueName := fmt.Sprintf("%s_%s", uuid.New().String(), file.Filename)
	storedPath := filepath.Join(s.DataPath, uniqueName)

	// Save file
	dst, err := os.Create(storedPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return nil, err
	}

	if title == "" {
		title = file.Filename
	}

	video := &data.Video{
		Title:            title,
		OriginalFilename: file.Filename,
		StoredPath:       storedPath,
		Size:             file.Size,
		ProcessingStatus: "pending",
		Tags:             pq.StringArray{},
		Actors:           pq.StringArray{},
	}

	if stat, err := os.Stat(storedPath); err == nil {
		modTime := stat.ModTime()
		video.FileCreatedAt = &modTime
	}

	if err := s.Repo.Create(video); err != nil {
		// Cleanup file if DB insert fails
		os.Remove(storedPath)
		return nil, err
	}

	if s.ProcessingService != nil {
		go func() {
			if err := s.ProcessingService.SubmitVideo(video.ID, storedPath); err != nil {
				s.logger.Error("Failed to submit video for processing",
					zap.Uint("video_id", video.ID),
					zap.String("video_path", storedPath),
					zap.Error(err),
				)
			}
		}()
	}

	return video, nil
}

func (s *VideoService) ListVideos(page, limit int) ([]data.Video, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return s.Repo.List(page, limit)
}

func (s *VideoService) GetVideo(id uint) (*data.Video, error) {
	return s.Repo.GetByID(id)
}

func (s *VideoService) DeleteVideo(id uint) error {
	video, err := s.Repo.GetByID(id)
	if err != nil {
		return err
	}

	if err := s.Repo.Delete(id); err != nil {
		return err
	}

	os.Remove(video.StoredPath)

	if video.ThumbnailPath != "" {
		os.Remove(video.ThumbnailPath)
	}

	if video.SpriteSheetPath != "" {
		spriteDir := filepath.Join(s.DataPath, "sprites")
		spritePattern := filepath.Join(spriteDir, fmt.Sprintf("%d_sheet_*.jpg", id))
		files, _ := filepath.Glob(spritePattern)
		for _, file := range files {
			os.Remove(file)
		}
	}

	if video.VttPath != "" {
		os.Remove(video.VttPath)
	}

	return nil
}

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

func (s *VideoService) SetThumbnailFromTimecode(videoID uint, timecode float64) error {
	video, err := s.Repo.GetByID(videoID)
	if err != nil {
		return fmt.Errorf("failed to get video: %w", err)
	}

	if video.Width == 0 || video.Height == 0 {
		return fmt.Errorf("video dimensions not available, metadata must be extracted first")
	}

	qualityConfig := s.ProcessingService.GetProcessingQualityConfig()

	tileWidthSm, tileHeightSm := ffmpeg.CalculateTileDimensions(video.Width, video.Height, qualityConfig.MaxFrameDimensionSm)
	tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(video.Width, video.Height, qualityConfig.MaxFrameDimensionLg)

	thumbnailDir := filepath.Join(s.DataPath, "thumbnails")
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	seekPos := strconv.FormatFloat(timecode, 'f', 3, 64)
	smPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", videoID))
	lgPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", videoID))

	if err := ffmpeg.ExtractThumbnail(video.StoredPath, smPath, seekPos, tileWidthSm, tileHeightSm, qualityConfig.FrameQualitySm); err != nil {
		return fmt.Errorf("failed to extract small thumbnail: %w", err)
	}

	if err := ffmpeg.ExtractThumbnail(video.StoredPath, lgPath, seekPos, tileWidthLg, tileHeightLg, qualityConfig.FrameQualityLg); err != nil {
		return fmt.Errorf("failed to extract large thumbnail: %w", err)
	}

	if err := s.Repo.UpdateThumbnail(videoID, smPath, tileWidthSm, tileHeightSm); err != nil {
		return fmt.Errorf("failed to update thumbnail in database: %w", err)
	}

	if s.EventBus != nil {
		s.EventBus.Publish(VideoEvent{
			Type:    "video:thumbnail_complete",
			VideoID: videoID,
			Data: map[string]any{
				"thumbnail_path": smPath,
			},
		})
	}

	return nil
}

func (s *VideoService) SetThumbnailFromUpload(videoID uint, file *multipart.FileHeader) error {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExtensions[ext] {
		return fmt.Errorf("invalid image extension, allowed: .jpg, .jpeg, .png, .webp")
	}

	video, err := s.Repo.GetByID(videoID)
	if err != nil {
		return fmt.Errorf("failed to get video: %w", err)
	}

	if video.Width == 0 || video.Height == 0 {
		return fmt.Errorf("video dimensions not available, metadata must be extracted first")
	}

	// Save uploaded file to temp location
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "goonhub-thumb-*"+ext)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to save uploaded file: %w", err)
	}
	tmpFile.Close()

	qualityConfig := s.ProcessingService.GetProcessingQualityConfig()

	tileWidthSm, tileHeightSm := ffmpeg.CalculateTileDimensions(video.Width, video.Height, qualityConfig.MaxFrameDimensionSm)
	tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(video.Width, video.Height, qualityConfig.MaxFrameDimensionLg)

	thumbnailDir := filepath.Join(s.DataPath, "thumbnails")
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	smPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", videoID))
	lgPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", videoID))

	if err := ffmpeg.ResizeImageToWebp(tmpPath, smPath, tileWidthSm, tileHeightSm, qualityConfig.FrameQualitySm); err != nil {
		return fmt.Errorf("failed to resize to small thumbnail: %w", err)
	}

	if err := ffmpeg.ResizeImageToWebp(tmpPath, lgPath, tileWidthLg, tileHeightLg, qualityConfig.FrameQualityLg); err != nil {
		return fmt.Errorf("failed to resize to large thumbnail: %w", err)
	}

	if err := s.Repo.UpdateThumbnail(videoID, smPath, tileWidthSm, tileHeightSm); err != nil {
		return fmt.Errorf("failed to update thumbnail in database: %w", err)
	}

	if s.EventBus != nil {
		s.EventBus.Publish(VideoEvent{
			Type:    "video:thumbnail_complete",
			VideoID: videoID,
			Data: map[string]any{
				"thumbnail_path": smPath,
			},
		})
	}

	return nil
}
