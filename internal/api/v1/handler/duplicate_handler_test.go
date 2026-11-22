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
)

func newDuplicateHandlerForTest(t *testing.T) (*gin.Engine, *mocks.MockDuplicateGroupRepository, *mocks.MockSceneRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	groups := mocks.NewMockDuplicateGroupRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	svc := core.NewDuplicateService(groups, scenes, nil, nil, nil, nil, nil, zap.NewNop())
	h := NewDuplicateHandler(svc, nil)
	r := gin.New()
	r.GET("/duplicates", h.ListGroups)
	return r, groups, scenes
}

func TestDuplicateListGroups_RejectsUnknownKind(t *testing.T) {
	r, _, _ := newDuplicateHandlerForTest(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/duplicates?kind=bogus", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDuplicateListGroups_FiltersByKindAndExposesRelations(t *testing.T) {
	r, groups, scenes := newDuplicateHandlerForTest(t)
	ref4446, ref4448 := uint(4446), uint(4448)
	groups.EXPECT().ListWithMembers(1, 20, "unresolved", data.DuplicateKindContainment).Return([]data.DuplicateGroup{{
		ID: 64, Status: "unresolved", SceneCount: 2, Members: []data.DuplicateGroupMember{
			{GroupID: 64, SceneID: 4446, RefSceneID: &ref4448, Relation: data.DuplicateRelationContains},
			{GroupID: 64, SceneID: 4448, RefSceneID: &ref4446, Relation: data.DuplicateRelationContained},
		},
	}}, int64(1), nil)
	scenes.EXPECT().GetByIDsIncludingTrashed([]uint{4446, 4448}).Return([]data.Scene{
		{ID: 4446, Duration: 5627}, {ID: 4448, Duration: 1642},
	}, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/duplicates?status=unresolved&kind=containment", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []struct {
			ID      uint   `json:"id"`
			Kind    string `json:"kind"`
			Members []struct {
				SceneID  uint   `json:"scene_id"`
				Relation string `json:"relation"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(body.Data) != 1 {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	g := body.Data[0]
	if g.Kind != "containment" || len(g.Members) != 2 || g.Members[0].Relation != "contains" || g.Members[1].Relation != "contained" {
		t.Fatalf("unexpected group: %+v", g)
	}
}

func TestDuplicateResolveGroup_ReportsMarkers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	groups := mocks.NewMockDuplicateGroupRepository(ctrl)
	svc := core.NewDuplicateService(groups, nil, nil, nil, nil, nil, nil, zap.NewNop())
	h := NewDuplicateHandler(svc, nil)
	r := gin.New()
	r.POST("/duplicates/:id/resolve", h.ResolveGroup)

	groups.EXPECT().GetByIDWithMembers(uint(3)).Return(&data.DuplicateGroup{ID: 3, Status: "unresolved", Members: []data.DuplicateGroupMember{{GroupID: 3, SceneID: 10}}}, nil)
	groups.EXPECT().UpdateMemberBest(uint(3), uint(10), true).Return(nil)
	groups.EXPECT().SetBestScene(uint(3), uint(10)).Return(nil)
	groups.EXPECT().UpdateStatus(uint(3), "resolved", gomock.Any()).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/duplicates/3/resolve", strings.NewReader(`{"best_scene_id": 10, "merge_metadata": true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Message string                  `json:"message"`
		Markers core.MarkerMergeSummary `json:"markers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Message != "Group resolved" {
		t.Fatalf("unexpected body %s (%v)", w.Body.String(), err)
	}
	if !strings.Contains(w.Body.String(), `"markers":{"moved":0,"shifted":0,"deduplicated":0,"left_behind":0}`) {
		t.Fatalf("markers summary missing: %s", w.Body.String())
	}
}
