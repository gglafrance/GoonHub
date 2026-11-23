package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// newSceneCopiesHandlerForTest serves GET /scenes/:id/copies.
func newSceneCopiesHandlerForTest(t *testing.T) (*gin.Engine, *mocks.MockSceneCopyRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	copies := mocks.NewMockSceneCopyRepository(ctrl)
	svc := core.NewScanService(nil, nil, copies, nil, nil, nil, zap.NewNop())
	h := NewScanHandler(svc)
	r := gin.New()
	r.GET("/scenes/:id/copies", h.ListSceneCopies)
	return r, copies
}

func TestListSceneCopies_InvalidID(t *testing.T) {
	r, _ := newSceneCopiesHandlerForTest(t)
	for _, id := range []string{"abc", "0", "-1"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/"+id+"/copies", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("id %s: expected 400, got %d", id, w.Code)
		}
	}
}

func TestListSceneCopies_ReturnsCopies(t *testing.T) {
	r, copies := newSceneCopiesHandlerForTest(t)
	copies.EXPECT().ListBySceneID(uint(5)).Return([]data.SceneCopy{
		{ID: 1, SceneID: 5, StoredPath: "/media/b/video.mp4", Size: 1234, Oshash: "00000000000004d2"},
		{ID: 2, SceneID: 5, StoredPath: "/media/c/video.mkv", Size: 1234, Oshash: "00000000000004d2"},
	}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/5/copies", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Copies []struct {
			StoredPath string `json:"stored_path"`
			Size       int64  `json:"size"`
		} `json:"copies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Copies) != 2 || body.Copies[0].StoredPath != "/media/b/video.mp4" || body.Copies[1].Size != 1234 {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestListSceneCopies_EmptyAndError(t *testing.T) {
	r, copies := newSceneCopiesHandlerForTest(t)
	copies.EXPECT().ListBySceneID(uint(6)).Return([]data.SceneCopy{}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/6/copies", nil))
	if w.Code != http.StatusOK || w.Body.String() != `{"copies":[]}` {
		t.Fatalf("expected an empty list, got %d: %s", w.Code, w.Body.String())
	}

	copies.EXPECT().ListBySceneID(uint(7)).Return(nil, errors.New("db down"))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/7/copies", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
