package handler

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// failingTestSubmitter records the jobs queued by the Retry action.
type failingTestSubmitter struct {
	queued []uint
	err    error
}

func (s *failingTestSubmitter) SubmitPhaseWithPriority(sceneID uint, _ string, _ int) error {
	if s.err != nil {
		return s.err
	}
	s.queued = append(s.queued, sceneID)
	return nil
}

func newTestFailingRouter(t *testing.T, role string) (*gin.Engine, *mocks.MockScenePhaseFailureRepository, *failingTestSubmitter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockScenePhaseFailureRepository(ctrl)
	svc := core.NewSceneFailureService(repo, mocks.NewMockSceneRepository(ctrl), config.ProcessingConfig{FailingThreshold: 3}, zap.NewNop())
	sub := &failingTestSubmitter{}
	svc.SetSubmitter(sub)
	h := NewJobHandler(nil, nil, nil, svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: role})
		c.Next()
	})
	admin := r.Group("/admin")
	admin.Use(middleware.RequireRole("admin"))
	admin.GET("/jobs/failing", h.ListFailing)
	admin.POST("/jobs/failing/bulk", h.BulkFailing)
	admin.POST("/jobs/failing/:id/retry", h.RetryFailing)
	admin.POST("/jobs/failing/:id/clear", h.ClearFailing)
	return r, repo, sub
}

// failingHandlerRow is a counter without file state and path: its file counts as unchanged.
func failingHandlerRow(id, sceneID uint, phase string, failures int) data.ScenePhaseFailureRow {
	return data.ScenePhaseFailureRow{
		ScenePhaseFailure: data.ScenePhaseFailure{
			ID: id, SceneID: sceneID, Phase: phase, Failures: failures,
			LastError: "failed to parse duration", LastFailedAt: time.Now(),
		},
		SceneTitle: "Broken",
	}
}

func TestFailingHandler_AdminOnly(t *testing.T) {
	r, _, _ := newTestFailingRouter(t, "user")
	for _, req := range []struct{ method, path string }{
		{http.MethodGet, "/admin/jobs/failing"},
		{http.MethodPost, "/admin/jobs/failing/bulk"},
		{http.MethodPost, "/admin/jobs/failing/1/retry"},
		{http.MethodPost, "/admin/jobs/failing/1/clear"},
	} {
		w, _ := doImportRequest(t, r, req.method, req.path, `{}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s as a non-admin: got %d, want 403", req.method, req.path, w.Code)
		}
	}
}

func TestFailingHandler_List(t *testing.T) {
	r, repo, _ := newTestFailingRouter(t, "admin")
	repo.EXPECT().List(data.ScenePhaseFailureFilter{MinFailures: 3, ErrorQuery: "duration"}).Return([]data.ScenePhaseFailureRow{
		failingHandlerRow(1, 10, "metadata", 17),
		failingHandlerRow(2, 11, "sprites", 14),
	}, nil)

	w, out := doImportRequest(t, r, http.MethodGet, "/admin/jobs/failing?phase=metadata&q=duration&page=1&limit=500", "")
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	items, _ := out["data"].([]any)
	if out["total"] != float64(1) || len(items) != 1 || out["limit"] != float64(100) || out["threshold"] != float64(3) {
		t.Fatalf("unexpected response %v", out)
	}
	totals, _ := out["totals"].(map[string]any)
	if totals["metadata"] != float64(1) || totals["sprites"] != float64(1) {
		t.Fatalf("totals %v", totals)
	}
	item, _ := items[0].(map[string]any)
	if item["scene_id"] != float64(10) || item["last_error"] != "failed to parse duration" || item["scene_title"] != "Broken" {
		t.Fatalf("item %v", item)
	}

	w, _ = doImportRequest(t, r, http.MethodGet, "/admin/jobs/failing?phase=everything", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid phase: got %d, want 400", w.Code)
	}
}

func TestFailingHandler_RetryAndClearOne(t *testing.T) {
	r, repo, sub := newTestFailingRouter(t, "admin")
	repo.EXPECT().GetByIDs([]uint{5}).Return([]data.ScenePhaseFailureRow{failingHandlerRow(5, 10, "sprites", 3)}, nil)
	repo.EXPECT().Clear(uint(10), "sprites").Return(nil)
	w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/5/retry", "")
	if w.Code != http.StatusOK || len(sub.queued) != 1 || sub.queued[0] != 10 {
		t.Fatalf("retry: got %d %s, queued %v", w.Code, w.Body.String(), sub.queued)
	}

	repo.EXPECT().GetByIDs([]uint{6}).Return(nil, nil)
	if w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/6/retry", ""); w.Code != http.StatusNotFound {
		t.Fatalf("retry of an unknown ID: got %d, want 404", w.Code)
	}

	repo.EXPECT().DeleteByIDs([]uint{5}).Return(int64(1), nil)
	if w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/5/clear", ""); w.Code != http.StatusOK {
		t.Fatalf("clear: got %d", w.Code)
	}
	repo.EXPECT().DeleteByIDs([]uint{6}).Return(int64(0), nil)
	if w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/6/clear", ""); w.Code != http.StatusNotFound {
		t.Fatalf("clear of an unknown ID: got %d, want 404", w.Code)
	}
	if w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/abc/clear", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID: got %d, want 400", w.Code)
	}
}

func TestFailingHandler_RetryQueueError(t *testing.T) {
	r, repo, sub := newTestFailingRouter(t, "admin")
	sub.err = errors.New("metadata must be extracted before sprites generation")
	repo.EXPECT().GetByIDs([]uint{5}).Return([]data.ScenePhaseFailureRow{failingHandlerRow(5, 10, "sprites", 3)}, nil)
	repo.EXPECT().Clear(uint(10), "sprites").Return(nil)
	if w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/5/retry", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("queue error: got %d, want 400", w.Code)
	}
}

func TestFailingHandler_BulkValidation(t *testing.T) {
	r, _, _ := newTestFailingRouter(t, "admin")
	for _, body := range []string{
		`not json`,
		`{"action":"delete","ids":[1]}`,
		`{"action":"retry"}`,
		`{"action":"clear","ids":[1],"all":true}`,
		`{"action":"clear","all":true,"phase":"everything"}`,
	} {
		if w, _ := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/bulk", body); w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: got %d, want 400", body, w.Code)
		}
	}
}

func TestFailingHandler_BulkByIDsAndAll(t *testing.T) {
	r, repo, sub := newTestFailingRouter(t, "admin")
	repo.EXPECT().DeleteByIDs([]uint{1, 2}).Return(int64(2), nil)
	w, out := doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/bulk", `{"action":"clear","ids":[1,2]}`)
	result, _ := out["result"].(map[string]any)
	if w.Code != http.StatusOK || result["cleared"] != float64(2) {
		t.Fatalf("bulk clear: %d %v", w.Code, out)
	}

	// "all" retries every failing (scene, phase) matching the filters
	repo.EXPECT().List(data.ScenePhaseFailureFilter{MinFailures: 3, ErrorQuery: "duration"}).Return([]data.ScenePhaseFailureRow{
		failingHandlerRow(1, 10, "metadata", 17),
		failingHandlerRow(2, 11, "sprites", 14),
		failingHandlerRow(3, 12, "metadata", 4),
	}, nil)
	repo.EXPECT().GetByIDs([]uint{1, 3}).Return([]data.ScenePhaseFailureRow{
		failingHandlerRow(1, 10, "metadata", 17),
		failingHandlerRow(3, 12, "metadata", 4),
	}, nil)
	repo.EXPECT().Clear(gomock.Any(), "metadata").Return(nil).Times(2)
	w, out = doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/bulk", `{"action":"retry","all":true,"phase":"metadata","q":"duration"}`)
	result, _ = out["result"].(map[string]any)
	if w.Code != http.StatusOK || result["retried"] != float64(2) || len(sub.queued) != 2 {
		t.Fatalf("bulk retry all: %d %v (queued %v)", w.Code, out, sub.queued)
	}

	// "all" with nothing failing is a no-op
	repo.EXPECT().List(gomock.Any()).Return(nil, nil)
	w, out = doImportRequest(t, r, http.MethodPost, "/admin/jobs/failing/bulk", `{"action":"clear","all":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("empty bulk: %d %v", w.Code, out)
	}
}
