package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newJizzTestRouter(t *testing.T) (*gin.Engine, *mocks.MockInteractionRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInteractionRepository(ctrl)
	h := NewInteractionHandler(core.NewInteractionService(repo, zap.NewNop()))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Role: "user"})
	})
	r.GET("/scenes/:id/jizzed", h.GetJizzed)
	r.POST("/scenes/:id/jizzed", h.ToggleJizzed)
	return r, repo
}

func TestJizzedEndpoints_IncludeHistory(t *testing.T) {
	r, repo := newJizzTestRouter(t)
	recent := time.Date(2024, 5, 2, 21, 0, 0, 0, time.UTC)
	older := time.Date(2024, 4, 1, 20, 0, 0, 0, time.UTC)
	events := []data.UserSceneJizzEvent{{JizzedAt: recent}, {JizzedAt: older}}

	repo.EXPECT().GetJizzedCount(uint(1), uint(7)).Return(2, nil)
	repo.EXPECT().GetJizzEvents(uint(1), uint(7)).Return(events, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/7/jizzed", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Count        int    `json:"count"`
		HistoryTotal int    `json:"history_total"`
		LastJizzedAt string `json:"last_jizzed_at"`
		History      []struct {
			JizzedAt string `json:"jizzed_at"`
		} `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 2 || body.HistoryTotal != 2 || len(body.History) != 2 {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.LastJizzedAt != "2024-05-02T21:00:00Z" || body.History[0].JizzedAt != body.LastJizzedAt {
		t.Fatalf("last o not the most recent: %+v", body)
	}

	// Increment returns the same shape
	repo.EXPECT().IncrementJizzed(uint(1), uint(7)).Return(3, nil)
	repo.EXPECT().GetJizzEvents(uint(1), uint(7)).Return(events, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/scenes/7/jizzed", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 3 || len(body.History) != 2 {
		t.Fatalf("unexpected increment body: %+v", body)
	}
}

func TestJizzedEndpoint_NoHistory(t *testing.T) {
	r, repo := newJizzTestRouter(t)
	repo.EXPECT().GetJizzedCount(uint(1), uint(7)).Return(0, nil)
	repo.EXPECT().GetJizzEvents(uint(1), uint(7)).Return(nil, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/7/jizzed", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["last_jizzed_at"] != nil {
		t.Fatalf("last_jizzed_at must be null without history: %v", body)
	}
	if history, ok := body["history"].([]any); !ok || len(history) != 0 {
		t.Fatalf("history must be an empty list: %v", body)
	}
}
