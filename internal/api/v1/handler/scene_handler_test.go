package handler

import (
	"fmt"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"goonhub/internal/streaming"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestSceneHandler(t *testing.T) (*SceneHandler, *mocks.MockSceneRepository, string) {
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	dataPath := t.TempDir()

	svc := &core.SceneService{
		Repo:         sceneRepo,
		ScenePath:    dataPath,
		MetadataPath: dataPath,
	}

	// Create a streaming manager for tests
	streamManager := streaming.NewManager(streaming.DefaultConfig(), sceneRepo, zap.NewNop())
	t.Cleanup(func() { streamManager.Stop() })

	handler := &SceneHandler{
		Service:       svc,
		StreamManager: streamManager,
	}
	return handler, sceneRepo, dataPath
}

func createTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

func TestStreamScene_FullFile(t *testing.T) {
	handler, sceneRepo, dataPath := newTestSceneHandler(t)

	fileContent := []byte("fake video content here - 32 bytes!!")
	filePath := createTestFile(t, dataPath, "test.mp4", fileContent)

	scene := &data.Scene{ID: 1, StoredPath: filePath}
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatal("expected Accept-Ranges: bytes header")
	}
	if w.Header().Get("Content-Length") != fmt.Sprintf("%d", len(fileContent)) {
		t.Fatalf("expected Content-Length %d, got %s", len(fileContent), w.Header().Get("Content-Length"))
	}
	if w.Body.Len() != len(fileContent) {
		t.Fatalf("expected body length %d, got %d", len(fileContent), w.Body.Len())
	}
}

func TestStreamScene_PartialContent(t *testing.T) {
	handler, sceneRepo, dataPath := newTestSceneHandler(t)

	fileContent := make([]byte, 2048)
	for i := range fileContent {
		fileContent[i] = byte(i % 256)
	}
	filePath := createTestFile(t, dataPath, "test.mp4", fileContent)

	scene := &data.Scene{ID: 1, StoredPath: filePath}
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/1", nil)
	req.Header.Set("Range", "bytes=0-1023")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 206 {
		t.Fatalf("expected 206, got %d", w.Code)
	}
	if w.Header().Get("Content-Range") != "bytes 0-1023/2048" {
		t.Fatalf("expected Content-Range 'bytes 0-1023/2048', got %q", w.Header().Get("Content-Range"))
	}
	if w.Header().Get("Content-Length") != "1024" {
		t.Fatalf("expected Content-Length 1024, got %s", w.Header().Get("Content-Length"))
	}
	if w.Body.Len() != 1024 {
		t.Fatalf("expected 1024 bytes in body, got %d", w.Body.Len())
	}
}

func TestStreamScene_OpenEndedRange(t *testing.T) {
	handler, sceneRepo, dataPath := newTestSceneHandler(t)

	fileContent := make([]byte, 4096)
	for i := range fileContent {
		fileContent[i] = byte(i % 256)
	}
	filePath := createTestFile(t, dataPath, "test.mp4", fileContent)

	scene := &data.Scene{ID: 1, StoredPath: filePath}
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/1", nil)
	req.Header.Set("Range", "bytes=1024-")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 206 {
		t.Fatalf("expected 206, got %d", w.Code)
	}
	expectedLen := 4096 - 1024
	if w.Body.Len() != expectedLen {
		t.Fatalf("expected %d bytes, got %d", expectedLen, w.Body.Len())
	}
}

func TestStreamScene_InvalidRange(t *testing.T) {
	handler, sceneRepo, dataPath := newTestSceneHandler(t)

	fileContent := make([]byte, 1024)
	filePath := createTestFile(t, dataPath, "test.mp4", fileContent)

	scene := &data.Scene{ID: 1, StoredPath: filePath}
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/1", nil)
	req.Header.Set("Range", "bytes=500-100") // start > end
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 416 {
		t.Fatalf("expected 416 for start > end, got %d", w.Code)
	}
}

func TestStreamScene_RangeExceedsFile(t *testing.T) {
	handler, sceneRepo, dataPath := newTestSceneHandler(t)

	fileContent := make([]byte, 512)
	filePath := createTestFile(t, dataPath, "test.mp4", fileContent)

	scene := &data.Scene{ID: 1, StoredPath: filePath}
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/1", nil)
	req.Header.Set("Range", "bytes=0-1023") // end > file size
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 416 {
		t.Fatalf("expected 416 for range exceeding file, got %d", w.Code)
	}
}

func TestStreamScene_SceneNotFound(t *testing.T) {
	handler, sceneRepo, _ := newTestSceneHandler(t)

	// Use gorm.ErrRecordNotFound which is what the service now checks for
	sceneRepo.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStreamScene_FileNotOnDisk(t *testing.T) {
	handler, sceneRepo, _ := newTestSceneHandler(t)

	scene := &data.Scene{ID: 1, StoredPath: "/nonexistent/path/video.mp4"}
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

	router := gin.New()
	router.GET("/stream/:id", handler.StreamScene)

	req, _ := http.NewRequest("GET", "/stream/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for missing file, got %d", w.Code)
	}
}

func TestStreamScene_CorrectMimeType(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".mp4", "video/mp4"},
		{".webm", "video/webm"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			handler, sceneRepo, dataPath := newTestSceneHandler(t)

			fileContent := []byte("fake video data")
			filePath := createTestFile(t, dataPath, "video"+tt.ext, fileContent)

			scene := &data.Scene{ID: 1, StoredPath: filePath}
			sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)

			router := gin.New()
			router.GET("/stream/:id", handler.StreamScene)

			req, _ := http.NewRequest("GET", "/stream/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expected {
				t.Fatalf("expected Content-Type %q, got %q", tt.expected, contentType)
			}
		})
	}
}
