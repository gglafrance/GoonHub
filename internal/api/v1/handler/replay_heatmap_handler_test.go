package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type heatmapTestDeps struct {
	repo         *mocks.MockReplayHeatmapRepository
	sceneRepo    *mocks.MockSceneRepository
	settingsRepo *mocks.MockUserSettingsRepository
}

func newHeatmapTestRouter(t *testing.T) (*gin.Engine, heatmapTestDeps) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	deps := heatmapTestDeps{
		repo:         mocks.NewMockReplayHeatmapRepository(ctrl),
		sceneRepo:    mocks.NewMockSceneRepository(ctrl),
		settingsRepo: mocks.NewMockUserSettingsRepository(ctrl),
	}
	h := NewWatchHistoryHandler(nil, core.NewReplayHeatmapService(deps.repo, deps.sceneRepo, deps.settingsRepo, zap.NewNop()))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "user"})
	})
	r.GET("/scenes/:id/heatmap", h.GetReplayHeatmap)
	r.POST("/scenes/:id/heatmap", h.RecordReplayHeatmap)
	return r, deps
}

type heatmapBody struct {
	Recorded    bool    `json:"recorded"`
	BucketCount int     `json:"bucket_count"`
	Buckets     []int32 `json:"buckets"`
}

func decodeHeatmap(t *testing.T, w *httptest.ResponseRecorder) heatmapBody {
	t.Helper()
	var body heatmapBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return body
}

func TestRecordReplayHeatmap(t *testing.T) {
	r, deps := newHeatmapTestRouter(t)
	deps.sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, Duration: 100}, nil)
	deps.settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	stored := make([]int32, 100)
	stored[10], stored[11] = 1000, 500
	deps.repo.EXPECT().AddBuckets(uint(1), uint(7), gomock.Any()).DoAndReturn(
		func(_, _ uint, inc []int32) ([]int32, error) {
			if len(inc) != 100 || inc[10] != 1000 || inc[11] != 500 || inc[50] != 1000 {
				t.Fatalf("unexpected increments: %v", inc)
			}
			return stored, nil
		})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scenes/7/heatmap",
		strings.NewReader(`{"ranges":[[10,11.5],[50,51]],"duration":100.04}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decodeHeatmap(t, w)
	if !body.Recorded || body.BucketCount != 100 || len(body.Buckets) != 100 || body.Buckets[10] != 1000 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestRecordReplayHeatmap_BadRequests(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{"invalid scene id", "/scenes/abc/heatmap", `{"ranges":[[0,1]]}`},
		{"missing ranges", "/scenes/7/heatmap", `{"duration":10}`},
		{"malformed range", "/scenes/7/heatmap", `{"ranges":[["a",1]]}`},
		{"negative duration", "/scenes/7/heatmap", `{"ranges":[[0,1]],"duration":-1}`},
		{"too many ranges", "/scenes/7/heatmap", `{"ranges":[` + strings.Repeat(`[0,1],`, 500) + `[0,1]]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newHeatmapTestRouter(t)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRecordReplayHeatmap_SceneNotFound(t *testing.T) {
	r, deps := newHeatmapTestRouter(t)
	deps.sceneRepo.EXPECT().GetByID(uint(7)).Return(nil, gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/scenes/7/heatmap", strings.NewReader(`{"ranges":[[0,1]]}`)))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", w.Code, w.Body.String())
	}
}

func TestGetReplayHeatmap(t *testing.T) {
	r, deps := newHeatmapTestRouter(t)

	// Nothing recorded: an empty list, not null
	deps.repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(nil, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/7/heatmap", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"buckets":[]`) {
		t.Fatalf("expected an empty bucket list: %s", w.Body.String())
	}

	stored := make([]int32, 100)
	stored[42] = 3000
	deps.repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(stored, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/7/heatmap", nil))
	body := decodeHeatmap(t, w)
	if body.BucketCount != 100 || len(body.Buckets) != 100 || body.Buckets[42] != 3000 {
		t.Fatalf("unexpected body: %+v", body)
	}
}
