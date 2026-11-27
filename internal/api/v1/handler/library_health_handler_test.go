package handler

import (
	"net/http"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestLibraryHealthRouter(t *testing.T, role string) (*gin.Engine, *mocks.MockLibraryHealthRepository, *mocks.MockStoragePathRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockLibraryHealthRepository(ctrl)
	storageRepo := mocks.NewMockStoragePathRepository(ctrl)
	svc := core.NewLibraryHealthService(repo, nil, core.NewStoragePathService(storageRepo, zap.NewNop()), nil, 2, zap.NewNop())
	h := NewLibraryHealthHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: role})
		middleware.SetAccess(c, testAccessForRole(role))
		c.Next()
	})
	admin := r.Group("/admin")
	admin.Use(middleware.RequirePermission(core.PermLibraryHealth))
	admin.GET("/library-health", h.GetSummary)
	admin.GET("/library-health/scenes", h.ListScenes)
	admin.GET("/library-health/preview", h.PreviewAction)
	admin.POST("/library-health/tasks", h.StartTask)
	admin.POST("/library-health/tasks/cancel", h.CancelTask)
	admin.GET("/library-health/tasks/status", h.GetTaskStatus)
	return r, repo, storageRepo
}

func TestLibraryHealthHandler_AdminOnly(t *testing.T) {
	r, _, _ := newTestLibraryHealthRouter(t, "user")
	for _, req := range []struct{ method, path string }{
		{http.MethodGet, "/admin/library-health"},
		{http.MethodGet, "/admin/library-health/scenes?category=size_zero"},
		{http.MethodGet, "/admin/library-health/preview?action=thumbnail"},
		{http.MethodPost, "/admin/library-health/tasks"},
		{http.MethodPost, "/admin/library-health/tasks/cancel"},
		{http.MethodGet, "/admin/library-health/tasks/status"},
	} {
		w, _ := doImportRequest(t, r, req.method, req.path, `{"kind":"missing_files"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s as a non-admin: got %d, want 403", req.method, req.path, w.Code)
		}
	}
}

func TestLibraryHealthHandler_GetSummary(t *testing.T) {
	r, repo, storageRepo := newTestLibraryHealthRouter(t, "admin")
	dir := t.TempDir()
	id := uint(1)
	storageRepo.EXPECT().List().Return([]data.StoragePath{{ID: 1, Name: "Main", Path: dir}}, nil)
	repo.EXPECT().CountLiveCategories().Return(map[string]int64{data.HealthCategoryNoFingerprint: 30900}, int64(41000), nil)
	repo.EXPECT().GetLatestCheck(data.HealthCheckKindMissingFiles, data.HealthCheckStatusCompleted).Return(nil, nil)
	repo.EXPECT().GetLatestCheck(data.HealthCheckKindFixSizes, "").Return(nil, nil)
	repo.EXPECT().StoragePathSceneStats().Return([]data.StoragePathSceneStats{{StoragePathID: &id, SceneCount: 41000, TotalSize: 1 << 40}}, nil)

	w, out := doImportRequest(t, r, http.MethodGet, "/admin/library-health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if out["total_scenes"] != float64(41000) || out["last_missing_check"] != nil {
		t.Fatalf("summary: %v", out)
	}
	counts := out["counts"].(map[string]any)
	if counts["no_fingerprint"] != float64(30900) {
		t.Fatalf("counts: %v", counts)
	}
	if _, ok := counts["missing_file"]; ok {
		t.Fatal("missing_file must be absent before any check")
	}
	paths := out["storage_paths"].([]any)
	first := paths[0].(map[string]any)
	if len(paths) != 1 || first["scene_count"] != float64(41000) || first["disk_usage"] == nil {
		t.Fatalf("storage paths: %v", paths)
	}
}

func TestLibraryHealthHandler_ListScenes(t *testing.T) {
	r, repo, _ := newTestLibraryHealthRouter(t, "admin")
	repo.EXPECT().ListCategoryScenes(data.HealthCategoryUnplayable, uint(0), 2, 100).
		Return([]data.HealthSceneRow{{ID: 4, Title: "x", VideoCodec: "hevc"}}, int64(150), nil)

	w, out := doImportRequest(t, r, http.MethodGet, "/admin/library-health/scenes?category=unplayable&page=2&limit=1000", "")
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	rows := out["data"].([]any)
	pagination := out["pagination"].(map[string]any)
	if len(rows) != 1 || rows[0].(map[string]any)["video_codec"] != "hevc" {
		t.Fatalf("rows: %v", rows)
	}
	if pagination["total_items"] != float64(150) || pagination["limit"] != float64(100) || pagination["total_pages"] != float64(2) {
		t.Fatalf("pagination: %v", pagination)
	}

	w, _ = doImportRequest(t, r, http.MethodGet, "/admin/library-health/scenes?category=everything", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown category: got %d, want 400", w.Code)
	}
}

func TestLibraryHealthHandler_PreviewAction(t *testing.T) {
	r, repo, _ := newTestLibraryHealthRouter(t, "admin")
	repo.EXPECT().CountScenesNeedingPhase("thumbnail").Return(int64(321), nil)

	w, out := doImportRequest(t, r, http.MethodGet, "/admin/library-health/preview?action=thumbnail", "")
	if w.Code != http.StatusOK || out["count"] != float64(321) || out["enabled"] != true || out["action"] != "thumbnail" {
		t.Fatalf("preview: %d %v", w.Code, out)
	}
	w, _ = doImportRequest(t, r, http.MethodGet, "/admin/library-health/preview?action=rm", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown action: got %d, want 400", w.Code)
	}
}

func TestLibraryHealthHandler_Tasks(t *testing.T) {
	r, repo, _ := newTestLibraryHealthRouter(t, "admin")

	w, _ := doImportRequest(t, r, http.MethodPost, "/admin/library-health/tasks", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing kind: got %d, want 400", w.Code)
	}
	w, _ = doImportRequest(t, r, http.MethodPost, "/admin/library-health/tasks", `{"kind":"move_files"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown kind: got %d, want 400", w.Code)
	}
	w, _ = doImportRequest(t, r, http.MethodPost, "/admin/library-health/tasks/cancel", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cancel with no task: got %d, want 400", w.Code)
	}

	// A task over no scenes finishes right away
	done := make(chan struct{})
	repo.EXPECT().CountScenePaths(data.HealthScenesSizeZero).Return(int64(0), nil)
	repo.EXPECT().CreateCheck(gomock.Any()).DoAndReturn(func(c *data.LibraryHealthCheck) error { c.ID = 3; return nil })
	repo.EXPECT().ListScenePaths(data.HealthScenesSizeZero, uint(0), gomock.Any()).Return(nil, nil)
	repo.EXPECT().FinishCheck(gomock.Any()).DoAndReturn(func(c *data.LibraryHealthCheck) error {
		defer close(done)
		if c.Status != data.HealthCheckStatusCompleted {
			t.Errorf("finished with status %s", c.Status)
		}
		return nil
	})
	w, out := doImportRequest(t, r, http.MethodPost, "/admin/library-health/tasks", `{"kind":"fix_sizes"}`)
	if w.Code != http.StatusOK || out["id"] != float64(3) || out["kind"] != "fix_sizes" || out["status"] != "running" {
		t.Fatalf("start: %d %v", w.Code, out)
	}
	<-done

	w, out = doImportRequest(t, r, http.MethodGet, "/admin/library-health/tasks/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if _, ok := out["running"]; !ok {
		t.Fatalf("status: %v", out)
	}
}
