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

func newExcludeHandlerForTest(t *testing.T) (*gin.Engine, *mocks.MockDuplicateGroupRepository, *mocks.MockSceneRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	groups := mocks.NewMockDuplicateGroupRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	svc := core.NewDuplicateService(groups, scenes, nil, nil, nil, nil, nil, zap.NewNop())
	h := NewDuplicateHandler(svc, nil)
	r := gin.New()
	r.POST("/duplicates/:id/exclude", h.ExcludeMembers)
	return r, groups, scenes
}

func postExclude(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestDuplicateExcludeMembers_DismissesGroupLeftWithOneMember(t *testing.T) {
	r, groups, scenes := newExcludeHandlerForTest(t)
	best := uint(10)
	group := &data.DuplicateGroup{ID: 4, Status: "unresolved", SceneCount: 2, BestSceneID: &best, Members: []data.DuplicateGroupMember{
		{GroupID: 4, SceneID: 10, IsBest: true}, {GroupID: 4, SceneID: 20},
	}}
	after := &data.DuplicateGroup{ID: 4, Status: "dismissed", SceneCount: 1, BestSceneID: &best, Members: group.Members[:1]}
	gomock.InOrder(
		groups.EXPECT().GetByIDWithMembers(uint(4)).Return(group, nil),
		groups.EXPECT().GetByIDWithMembers(uint(4)).Return(after, nil),
	)
	groups.EXPECT().RunInTransaction(gomock.Any()).DoAndReturn(func(fn func(data.DuplicateGroupRepository) error) error {
		return fn(groups)
	})
	groups.EXPECT().AddExclusions([]uint{20}, []uint{10}).Return(nil)
	groups.EXPECT().RemoveMembers(uint(4), []uint{20}).Return(nil)
	groups.EXPECT().UpdateSceneCount(uint(4)).Return(nil)
	groups.EXPECT().UpdateStatus(uint(4), "dismissed", nil).Return(nil)
	scenes.EXPECT().GetByIDsIncludingTrashed([]uint{10}).Return([]data.Scene{{ID: 10, Title: "kept"}}, nil)

	w := postExclude(r, "/duplicates/4/exclude", `{"scene_ids": [20]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Excluded  []uint `json:"excluded"`
		Dismissed bool   `json:"dismissed"`
		Group     struct {
			ID         uint   `json:"id"`
			Status     string `json:"status"`
			SceneCount int    `json:"scene_count"`
			Members    []struct {
				SceneID uint `json:"scene_id"`
			} `json:"members"`
		} `json:"group"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(body.Excluded) != 1 || body.Excluded[0] != 20 || !body.Dismissed {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if body.Group.ID != 4 || body.Group.Status != "dismissed" || body.Group.SceneCount != 1 || len(body.Group.Members) != 1 {
		t.Fatalf("unexpected group: %+v", body.Group)
	}
}

func TestDuplicateExcludeMembers_Errors(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   string
		setup  func(groups *mocks.MockDuplicateGroupRepository)
		status int
	}{
		{"bad id", "/duplicates/x/exclude", `{"scene_ids": [1]}`, nil, http.StatusBadRequest},
		{"no body", "/duplicates/4/exclude", `{}`, nil, http.StatusBadRequest},
		{"empty list", "/duplicates/4/exclude", `{"scene_ids": []}`, nil, http.StatusBadRequest},
		{"not found", "/duplicates/4/exclude", `{"scene_ids": [1]}`, func(groups *mocks.MockDuplicateGroupRepository) {
			groups.EXPECT().GetByIDWithMembers(uint(4)).Return(nil, gorm.ErrRecordNotFound)
		}, http.StatusNotFound},
		{"not a member", "/duplicates/4/exclude", `{"scene_ids": [99]}`, func(groups *mocks.MockDuplicateGroupRepository) {
			groups.EXPECT().GetByIDWithMembers(uint(4)).Return(&data.DuplicateGroup{ID: 4, Status: "unresolved", Members: []data.DuplicateGroupMember{
				{GroupID: 4, SceneID: 10}, {GroupID: 4, SceneID: 20},
			}}, nil)
		}, http.StatusBadRequest},
		{"resolved group", "/duplicates/4/exclude", `{"scene_ids": [20]}`, func(groups *mocks.MockDuplicateGroupRepository) {
			groups.EXPECT().GetByIDWithMembers(uint(4)).Return(&data.DuplicateGroup{ID: 4, Status: "resolved", Members: []data.DuplicateGroupMember{
				{GroupID: 4, SceneID: 10}, {GroupID: 4, SceneID: 20},
			}}, nil)
		}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, groups, _ := newExcludeHandlerForTest(t)
			if tt.setup != nil {
				tt.setup(groups)
			}
			if w := postExclude(r, tt.path, tt.body); w.Code != tt.status {
				t.Fatalf("expected %d, got %d: %s", tt.status, w.Code, w.Body.String())
			}
		})
	}
}
