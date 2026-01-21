package core

import (
	"fmt"
	"goonhub/internal/data"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type VideoService struct {
	Repo     data.VideoRepository
	DataPath string
}

func NewVideoService(repo data.VideoRepository, dataPath string) *VideoService {
	// Ensure data directory exists
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		// Log but don't panic here, though usually this would be fatal in main
		fmt.Printf("Warning: failed to create data directory: %v\n", err)
	}
	return &VideoService{
		Repo:     repo,
		DataPath: dataPath,
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
	}

	if err := s.Repo.Create(video); err != nil {
		// Cleanup file if DB insert fails
		os.Remove(storedPath)
		return nil, err
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
