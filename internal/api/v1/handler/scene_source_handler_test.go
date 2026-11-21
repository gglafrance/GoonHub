package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func putSceneDetails(t *testing.T, handler *SceneHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.PUT("/scenes/:id/details", handler.UpdateSceneDetails)
	req, _ := http.NewRequest("PUT", "/scenes/1/details", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestUpdateSceneDetails_InvalidSourceURL(t *testing.T) {
	// No repository expectation: an invalid source is rejected before any write.
	handler, _, _ := newTestSceneHandler(t)

	w := putSceneDetails(t, handler, `{"title":"T","description":"D","source_url":"not a url"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "source_url") {
		t.Fatalf("expected the error to name source_url, got %s", w.Body.String())
	}
}

func TestUpdateSceneDetails_SetsSourceURL(t *testing.T) {
	handler, sceneRepo, _ := newTestSceneHandler(t)

	sceneRepo.EXPECT().UpdateDetails(uint(1), "T", "D", gomock.Nil()).Return(nil)
	sceneRepo.EXPECT().UpdateSourceURL(uint(1), "https://example.com/v/1").Return(nil)
	first := sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, Title: "T"}, nil).Times(2)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, Title: "T", SourceURL: "https://example.com/v/1"}, nil).After(first)

	w := putSceneDetails(t, handler, `{"title":"T","description":"D","source_url":"https://example.com/v/1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if out["source_url"] != "https://example.com/v/1" {
		t.Fatalf("expected source_url in the answer, got %v", out["source_url"])
	}
}

func TestUpdateSceneDetails_WithoutSourceURLLeavesIt(t *testing.T) {
	// No UpdateSourceURL expectation: clients that don't send source_url keep it.
	handler, sceneRepo, _ := newTestSceneHandler(t)

	sceneRepo.EXPECT().UpdateDetails(uint(1), "T", "D", gomock.Nil()).Return(nil)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, Title: "T", SourceURL: "https://kept.example"}, nil)

	w := putSceneDetails(t, handler, `{"title":"T","description":"D"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
