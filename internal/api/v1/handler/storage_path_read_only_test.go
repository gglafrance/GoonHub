package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestStoragePathHandler(t *testing.T) (*StoragePathHandler, *mocks.MockStoragePathRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStoragePathRepository(ctrl)
	return NewStoragePathHandler(core.NewStoragePathService(repo, zap.NewNop())), repo
}

func TestStoragePathCreate_ReadOnly(t *testing.T) {
	h, repo := newTestStoragePathHandler(t)
	dir := t.TempDir()
	repo.EXPECT().GetByPath(dir).Return(nil, nil)
	repo.EXPECT().Create(gomock.Any()).DoAndReturn(func(p *data.StoragePath) error {
		if !p.ReadOnly {
			t.Fatal("expected the read-only flag to be stored")
		}
		p.ID = 7
		return nil
	})

	router := gin.New()
	router.POST("/storage-paths", h.Create)
	body, _ := json.Marshal(map[string]any{"name": "Stash", "path": dir, "read_only": true})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/storage-paths", bytes.NewReader(body)))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["read_only"] != true {
		t.Fatalf("expected read_only=true in the response, got %v", got["read_only"])
	}
}

func TestStoragePathUpdate_ReadOnlyOmittedIsKept(t *testing.T) {
	h, repo := newTestStoragePathHandler(t)
	repo.EXPECT().GetByID(uint(1)).Return(&data.StoragePath{ID: 1, Name: "Stash", Path: "/media/stash", ReadOnly: true}, nil)
	repo.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *data.StoragePath) error {
		if !p.ReadOnly {
			t.Fatal("an update without read_only must keep the flag")
		}
		return nil
	})

	router := gin.New()
	router.PUT("/storage-paths/:id", h.Update)
	body := `{"name":"Stash renamed","path":"/media/stash"}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("PUT", "/storage-paths/1", strings.NewReader(body)))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStoragePathList_ReadOnly(t *testing.T) {
	h, repo := newTestStoragePathHandler(t)
	repo.EXPECT().List().Return([]data.StoragePath{
		{ID: 1, Name: "Library", Path: t.TempDir()},
		{ID: 2, Name: "Stash", Path: t.TempDir(), ReadOnly: true},
	}, nil)

	router := gin.New()
	router.GET("/storage-paths", h.List)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/storage-paths", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		StoragePaths []struct {
			ID       uint `json:"id"`
			ReadOnly bool `json:"read_only"`
		} `json:"storage_paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	paths := got.StoragePaths
	if len(paths) != 2 || paths[0].ReadOnly || !paths[1].ReadOnly {
		t.Fatalf("unexpected storage paths: %s", w.Body.String())
	}
}

func TestUploadScene_ReadOnlyPathIsBadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	pathRepo := mocks.NewMockStoragePathRepository(ctrl)
	uploadDir := t.TempDir()
	pathRepo.EXPECT().List().Return([]data.StoragePath{{ID: 1, Name: "Stash", Path: uploadDir, ReadOnly: true}}, nil)

	svc := core.NewSceneService(sceneRepo, uploadDir, t.TempDir(), nil, nil, zap.NewNop(), nil, nil, nil, nil,
		core.NewMediaFileGuard(pathRepo, zap.NewNop()))
	h := &SceneHandler{Service: svc}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("scene", "clip.mp4")
	_, _ = part.Write([]byte("video"))
	_ = mw.Close()

	router := gin.New()
	router.POST("/scenes", h.UploadScene)
	req := httptest.NewRequest("POST", "/scenes", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "read-only") {
		t.Fatalf("expected a read-only error message, got %s", w.Body.String())
	}
	entries, _ := os.ReadDir(uploadDir)
	if len(entries) != 0 {
		t.Fatalf("nothing must be written, found %v", filepath.Join(uploadDir, entries[0].Name()))
	}
}
