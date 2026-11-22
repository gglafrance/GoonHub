package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newSimilarHandlerForTest serves GET /scenes/:id/similar as the given role (none if empty).
func newSimilarHandlerForTest(t *testing.T, role string) (*gin.Engine, *mocks.MockDuplicateGroupRepository, *mocks.MockSceneRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	groups := mocks.NewMockDuplicateGroupRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	svc := core.NewDuplicateService(groups, scenes, nil, nil, nil, nil, nil, zap.NewNop())
	h := NewDuplicateHandler(svc, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("user", &core.UserPayload{UserID: 1, Username: "u", Role: role})
		}
		c.Next()
	})
	r.GET("/scenes/:id/similar", h.GetSimilarScenes)
	return r, groups, scenes
}

func TestGetSimilarScenes_InvalidID(t *testing.T) {
	r, _, _ := newSimilarHandlerForTest(t, "user")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/abc/similar", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSimilarScenes_SceneNotFound(t *testing.T) {
	r, _, scenes := newSimilarHandlerForTest(t, "user")
	scenes.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/9/similar", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSimilarScenes_TrashedOnlyForAdmins(t *testing.T) {
	for _, tc := range []struct {
		role           string
		includeTrashed bool
	}{
		{"user", false},
		{"moderator", false},
		{"admin", true},
	} {
		t.Run(tc.role, func(t *testing.T) {
			r, groups, scenes := newSimilarHandlerForTest(t, tc.role)
			scenes.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
			groups.EXPECT().ListSceneGroupMembers(uint(1), tc.includeTrashed).Return(nil, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/1/similar", nil))
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGetSimilarScenes_ResponseShape(t *testing.T) {
	r, groups, scenes := newSimilarHandlerForTest(t, "user")
	f := func(v float64) *float64 { return &v }
	one := uint(1)
	scenes.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
	groups.EXPECT().ListSceneGroupMembers(uint(1), false).Return([]data.DuplicateMemberScene{
		{GroupID: 4, GroupStatus: "dismissed", SceneID: 1, Duration: 600},
		{GroupID: 4, GroupStatus: "dismissed", SceneID: 2, Title: "Clip", Duration: 90, Width: 1920, Height: 1080,
			Size: 1234, ThumbnailPath: "t.jpg", ConfidenceScore: 0.8, MatchType: "visual",
			RefSceneID: &one, MatchStart: f(0), MatchEnd: f(90), RefStart: f(70), RefEnd: f(160),
			Relation: data.DuplicateRelationContained},
	}, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/scenes/1/similar", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		SceneID     uint   `json:"scene_id"`
		GroupID     *uint  `json:"group_id"`
		GroupStatus string `json:"group_status"`
		Scenes      []struct {
			ID                 uint    `json:"id"`
			Title              string  `json:"title"`
			Width              int     `json:"width"`
			Size               int64   `json:"size"`
			IsTrashed          bool    `json:"is_trashed"`
			ConfidenceScore    float64 `json:"confidence_score"`
			MatchType          string  `json:"match_type"`
			Relation           string  `json:"relation"`
			RelationRefSceneID *uint   `json:"relation_ref_scene_id"`
			Segment            *struct {
				ThisStart  float64 `json:"this_start"`
				ThisEnd    float64 `json:"this_end"`
				OtherStart float64 `json:"other_start"`
				OtherEnd   float64 `json:"other_end"`
			} `json:"segment"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.SceneID != 1 || body.GroupID == nil || *body.GroupID != 4 || body.GroupStatus != "dismissed" {
		t.Fatalf("unexpected group fields: %s", w.Body.String())
	}
	if len(body.Scenes) != 1 {
		t.Fatalf("expected 1 scene, got %s", w.Body.String())
	}
	s := body.Scenes[0]
	if s.ID != 2 || s.Title != "Clip" || s.Width != 1920 || s.Size != 1234 || s.IsTrashed ||
		s.ConfidenceScore != 0.8 || s.MatchType != "visual" || s.Relation != "contained" ||
		s.RelationRefSceneID == nil || *s.RelationRefSceneID != 1 {
		t.Fatalf("unexpected scene: %s", w.Body.String())
	}
	if s.Segment == nil || s.Segment.ThisStart != 70 || s.Segment.ThisEnd != 160 || s.Segment.OtherStart != 0 || s.Segment.OtherEnd != 90 {
		t.Fatalf("unexpected segment: %s", w.Body.String())
	}
}
