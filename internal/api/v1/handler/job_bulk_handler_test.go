package handler

import (
	"net/http"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestJobBulkRouter(t *testing.T, role string) (*gin.Engine, *mocks.MockJobHistoryRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockJobHistoryRepository(ctrl)

	dir := t.TempDir()
	cfg := config.ProcessingConfig{
		ThumbnailDir: dir, SpriteDir: dir, VttDir: dir, MarkerThumbnailDir: dir, ScenePreviewDir: dir,
		MetadataWorkers: 1, ThumbnailWorkers: 1, SpritesWorkers: 1,
	}
	poolManager := processing.NewPoolManager(cfg, zap.NewNop(), nil, nil, nil)
	feeder := core.NewJobQueueFeeder(repo, mocks.NewMockSceneRepository(ctrl), nil, nil, poolManager, false, nil, zap.NewNop())
	h := NewJobHandler(core.NewJobHistoryService(repo, cfg, zap.NewNop()), nil, feeder, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: role})
		middleware.SetAccess(c, testAccessForRole(role))
		c.Next()
	})
	admin := r.Group("/admin")
	admin.Use(middleware.RequirePermission(core.PermJobsManage))
	admin.POST("/jobs/cancel-bulk", h.CancelBulk)
	admin.GET("/jobs/history-purge", h.GetHistoryPurge)
	admin.POST("/jobs/history-purge", h.PurgeHistory)
	return r, repo
}

func TestJobBulkHandler_AdminOnly(t *testing.T) {
	r, _ := newTestJobBulkRouter(t, "user")
	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/admin/jobs/cancel-bulk"},
		{http.MethodGet, "/admin/jobs/history-purge"},
		{http.MethodPost, "/admin/jobs/history-purge"},
	} {
		w, _ := doImportRequest(t, r, req.method, req.path, `{"phase":"all"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s as a non-admin: got %d, want 403", req.method, req.path, w.Code)
		}
	}
}

func TestJobBulkHandler_CancelBulkValidation(t *testing.T) {
	r, _ := newTestJobBulkRouter(t, "admin")
	for _, body := range []string{`not json`, `{}`, `{"phase":"scan"}`, `{"phase":"everything"}`} {
		w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/cancel-bulk", body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: got %d, want 400", body, w.Code)
		}
	}
}

func TestJobBulkHandler_CancelBulk(t *testing.T) {
	r, repo := newTestJobBulkRouter(t, "admin")
	repo.EXPECT().CancelPendingJobsByPhases([]string{"fingerprint"}, gomock.Any()).Return(int64(30931), nil)

	w, out := doImportRequest(t, r, http.MethodPost, "/admin/jobs/cancel-bulk", `{"phase":"fingerprint","include_running":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if out["pending_cancelled"] != float64(30931) || out["queued_skipped"] != float64(0) ||
		out["running_cancelled"] != float64(0) || out["include_running"] != true {
		t.Fatalf("response = %v", out)
	}
}

func TestJobBulkHandler_HistoryPurge(t *testing.T) {
	r, repo := newTestJobBulkRouter(t, "admin")
	repo.EXPECT().CountPurgeable([]string{data.JobStatusCompleted, data.JobStatusCancelled}, gomock.Any()).
		Return(map[string]int64{data.JobStatusCompleted: 10, data.JobStatusCancelled: 5}, nil)
	repo.EXPECT().CountPurgeable([]string{data.JobStatusFailed, data.JobStatusTimedOut}, gomock.Any()).
		Return(map[string]int64{data.JobStatusFailed: 2}, nil)

	w, out := doImportRequest(t, r, http.MethodGet, "/admin/jobs/history-purge", "")
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	preview, _ := out["preview"].(map[string]any)
	settings, _ := out["settings"].(map[string]any)
	if preview["total"] != float64(17) || settings["completed_retention"] != "30d" || settings["failed_retention"] != "90d" {
		t.Fatalf("response = %v", out)
	}
	if out["last_run"] != nil {
		t.Fatalf("last_run = %v, want null before any purge", out["last_run"])
	}

	repo.EXPECT().PurgeFinishedBatch([]string{data.JobStatusCompleted, data.JobStatusCancelled}, gomock.Any(), 5000).Return(int64(15), nil)
	repo.EXPECT().PurgeFinishedBatch([]string{data.JobStatusFailed, data.JobStatusTimedOut}, gomock.Any(), 5000).Return(int64(2), nil)

	w, out = doImportRequest(t, r, http.MethodPost, "/admin/jobs/history-purge", "")
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	result, _ := out["result"].(map[string]any)
	if result["total"] != float64(17) || result["trigger"] != core.HistoryPurgeManual {
		t.Fatalf("response = %v", out)
	}
}
