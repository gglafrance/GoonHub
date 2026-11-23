package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
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

func newTrashAllHandlerForTest(t *testing.T) (*gin.Engine, *mocks.MockDuplicateGroupRepository, *mocks.MockSceneRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	groups := mocks.NewMockDuplicateGroupRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	svc := core.NewDuplicateService(groups, scenes, nil, nil, nil, nil, nil, zap.NewNop())
	h := NewDuplicateHandler(svc, nil)
	r := gin.New()
	r.POST("/duplicates/trash-all-bulk", h.TrashAllBulk)
	r.POST("/duplicates/:id/trash-all", h.TrashAll)
	return r, groups, scenes
}

func TestDuplicateTrashAll_ResolvesGroupWithoutKeptScene(t *testing.T) {
	r, groups, scenes := newTrashAllHandlerForTest(t)
	trashedAt := time.Now()
	// Every scene is already in the trash: skipped, the group is still resolved
	groups.EXPECT().GetByIDWithMembers(uint(5)).Return(&data.DuplicateGroup{ID: 5, Status: "unresolved", Members: []data.DuplicateGroupMember{
		{GroupID: 5, SceneID: 10, IsBest: true},
	}}, nil)
	scenes.EXPECT().GetByIDsIncludingTrashed([]uint{10}).Return([]data.Scene{{ID: 10, TrashedAt: &trashedAt}}, nil)
	groups.EXPECT().UpdateMemberBest(uint(5), uint(10), false).Return(nil)
	groups.EXPECT().ClearBestScene(uint(5)).Return(nil)
	groups.EXPECT().UpdateStatus(uint(5), "resolved", gomock.Any()).Return(nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/duplicates/5/trash-all", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"trashed":[],"skipped":[10]}` {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDuplicateTrashAll_NotFound(t *testing.T) {
	r, groups, _ := newTrashAllHandlerForTest(t)
	groups.EXPECT().GetByIDWithMembers(uint(9)).Return(nil, gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/duplicates/9/trash-all", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDuplicateTrashAllBulk_Validation(t *testing.T) {
	r, _, _ := newTrashAllHandlerForTest(t)
	tooMany := make([]string, maxBulkDuplicateGroups+1)
	for i := range tooMany {
		tooMany[i] = "1"
	}
	for _, body := range []string{`{}`, `{"group_ids": []}`, `{"group_ids": [` + strings.Join(tooMany, ",") + `]}`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/duplicates/trash-all-bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %.40s: expected 400, got %d", body, w.Code)
		}
	}
}

func TestDuplicateTrashAllBulk_ReportsFailedGroups(t *testing.T) {
	r, groups, _ := newTrashAllHandlerForTest(t)
	groups.EXPECT().GetByIDWithMembers(uint(1)).Return(&data.DuplicateGroup{ID: 1, Status: "dismissed"}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/duplicates/trash-all-bulk", strings.NewReader(`{"group_ids": [1]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	want := `{"resolved":[],"failed":[{"group_id":1,"error":"group is dismissed"}],"trashed_scenes":0,"skipped_scenes":0}`
	if w.Body.String() != want {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func newSelectableIDsRouter(t *testing.T) (*gin.Engine, *mocks.MockDuplicateGroupRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	groups := mocks.NewMockDuplicateGroupRepository(ctrl)
	h := NewDuplicateHandler(core.NewDuplicateService(groups, nil, nil, nil, nil, nil, nil, zap.NewNop()), nil)
	r := gin.New()
	r.GET("/duplicates/selectable-ids", h.SelectableIDs)
	return r, groups
}

func TestDuplicateSelectableIDs_ReturnsIDsTotalAndLimit(t *testing.T) {
	r, groups := newSelectableIDsRouter(t)
	groups.EXPECT().ListUnresolvedIDs(data.DuplicateKindIdentical, maxBulkDuplicateGroups).Return([]uint{9, 7, 3}, int64(612), nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/duplicates/selectable-ids?kind=identical", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		IDs   []uint `json:"ids"`
		Total int64  `json:"total"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(body.IDs) != 3 || body.IDs[0] != 9 || body.Total != 612 || body.Limit != maxBulkDuplicateGroups {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDuplicateSelectableIDs_RejectsUnknownKindAndReturnsEmptyList(t *testing.T) {
	r, groups := newSelectableIDsRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/duplicates/selectable-ids?kind=bogus", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	groups.EXPECT().ListUnresolvedIDs("", maxBulkDuplicateGroups).Return(nil, int64(0), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/duplicates/selectable-ids", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ids":[]`) {
		t.Fatalf("expected 200 with an empty list, got %d: %s", w.Code, w.Body.String())
	}
}
