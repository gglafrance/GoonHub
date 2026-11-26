package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newMarkerDedupHandlerForTest(t *testing.T) (*gin.Engine, *mocks.MockMarkerRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	markers := mocks.NewMockMarkerRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	svc := core.NewMarkerService(markers, nil, scenes, nil, &config.Config{}, zap.NewNop())
	h := NewMarkerHandler(svc, 100)
	r := gin.New()
	r.POST("/admin/markers/deduplicate", h.DeduplicateMarkers)
	return r, markers
}

func postMarkerDedup(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/markers/deduplicate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func duplicateMarkers() []data.UserSceneMarker {
	return []data.UserSceneMarker{
		{ID: 1, UserID: 1, SceneID: 7, Timestamp: 10, Label: "Kiss"},
		{ID: 2, UserID: 1, SceneID: 7, Timestamp: 11, Label: "Kiss"},
	}
}

func TestDeduplicateMarkers_RequiresScope(t *testing.T) {
	r, _ := newMarkerDedupHandlerForTest(t)
	for _, body := range []string{`{}`, `{"scene_ids": []}`, `not json`} {
		if w := postMarkerDedup(r, body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", body, w.Code, w.Body.String())
		}
	}
}

func TestDeduplicateMarkers_RejectsTooManyScenes(t *testing.T) {
	r, _ := newMarkerDedupHandlerForTest(t)
	ids := make([]string, maxMarkerDedupScenes+1)
	for i := range ids {
		ids[i] = "1"
	}
	if w := postMarkerDedup(r, `{"scene_ids": [`+strings.Join(ids, ",")+`]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeduplicateMarkers_DryRunByDefault(t *testing.T) {
	r, markers := newMarkerDedupHandlerForTest(t)
	markers.EXPECT().GetAllByScene(uint(7)).Return(duplicateMarkers(), nil)
	markers.EXPECT().GetMarkerTagsMultiple([]uint{1, 2}).Return(map[uint][]data.MarkerTagInfo{}, nil)
	// no ApplyMarkerDedup expectation: a dry run changes nothing

	w := postMarkerDedup(r, `{"scene_ids": [7]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body core.MarkerDedupResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.DryRun || body.Removed != 1 || len(body.Merges) != 1 || body.Merges[0].Kept.ID != 1 || body.Merges[0].Removed[0].ID != 2 {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDeduplicateMarkers_AppliesAllScenes(t *testing.T) {
	r, markers := newMarkerDedupHandlerForTest(t)
	markers.EXPECT().ListSceneIDsWithNearbyMarkers(gomock.Any()).Return([]uint{7}, nil)
	markers.EXPECT().GetAllByScene(uint(7)).Return(duplicateMarkers(), nil)
	markers.EXPECT().GetMarkerTagsMultiple([]uint{1, 2}).Return(map[uint][]data.MarkerTagInfo{}, nil)
	markers.EXPECT().ApplyMarkerDedup([]data.MarkerDedupMerge{{KeepID: 1, RemoveIDs: []uint{2}}}).Return(nil)

	// scene_ids are ignored with all
	w := postMarkerDedup(r, `{"all": true, "scene_ids": [99], "dry_run": false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"dry_run":false`) || !strings.Contains(w.Body.String(), `"removed":1`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDeduplicateMarkers_Failure(t *testing.T) {
	r, markers := newMarkerDedupHandlerForTest(t)
	markers.EXPECT().ListSceneIDsWithNearbyMarkers(gomock.Any()).Return(nil, errors.New("db down"))
	if w := postMarkerDedup(r, `{"all": true}`); w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
