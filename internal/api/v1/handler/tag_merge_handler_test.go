package handler

import (
	"net/http"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTagMergeTestRouter(t *testing.T) (*gin.Engine, *mocks.MockTagRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	h := NewTagHandler(core.NewTagService(tagRepo, sceneRepo, zap.NewNop()))

	r := gin.New()
	// same shape as routes.go: the static paths next to /:id
	r.GET("/tags/duplicates", h.ListDuplicateTags)
	r.POST("/tags/merge", h.MergeTags)
	r.PUT("/tags/:id", h.UpdateTag)
	r.DELETE("/tags/:id", h.DeleteTag)
	return r, tagRepo
}

func TestListDuplicateTagsEndpoint(t *testing.T) {
	r, tagRepo := newTagMergeTestRouter(t)
	tagRepo.EXPECT().ListWithCounts().Return([]data.TagWithCount{
		{Tag: data.Tag{ID: 1, Name: "Cumshot"}, SceneCount: 3},
		{Tag: data.Tag{ID: 2, Name: "cumshot"}, SceneCount: 7},
		{Tag: data.Tag{ID: 3, Name: "Anal"}, SceneCount: 1},
	}, nil)
	w, body := doAliasRequest(t, r, http.MethodGet, "/tags/duplicates", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	groups := body["data"].([]any)
	if len(groups) != 1 {
		t.Fatalf("groups = %v", groups)
	}
	g := groups[0].(map[string]any)
	tags := g["tags"].([]any)
	first := tags[0].(map[string]any)
	if g["key"] != "cumshot" || g["suggested_target_id"] != float64(2) || len(tags) != 2 || first["scene_count"] != float64(7) {
		t.Fatalf("group = %v", g)
	}
}

func TestMergeTagsEndpoint(t *testing.T) {
	r, tagRepo := newTagMergeTestRouter(t)

	for _, body := range []string{`{}`, `{"target_id":1}`, `not json`} {
		if w, _ := doAliasRequest(t, r, http.MethodPost, "/tags/merge", body); w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status %d, want 400", body, w.Code)
		}
	}
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/tags/merge", `{"target_id":1,"source_ids":[1]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("target among sources: status %d, want 400", w.Code)
	}

	tagRepo.EXPECT().GetByID(uint(5)).Return(nil, gorm.ErrRecordNotFound)
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/tags/merge", `{"target_id":5,"source_ids":[2]}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown target: status %d, want 404", w.Code)
	}

	target := data.Tag{ID: 1, Name: "Cumshot"}
	tagRepo.EXPECT().GetByID(uint(1)).Return(&target, nil)
	tagRepo.EXPECT().GetByIDs([]uint{2}).Return([]data.Tag{{ID: 2, Name: "cumshot"}}, nil)
	tagRepo.EXPECT().MergeTags(target, gomock.Any(), true).Return(&data.TagMergeStats{
		ScenesRetagged: 4, MarkersUpdated: 2, ReferencesUpdated: 1, SourcesDeleted: 1,
	}, nil, nil)
	w, body := doAliasRequest(t, r, http.MethodPost, "/tags/merge", `{"target_id":1,"source_ids":[2],"dry_run":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if body["dry_run"] != true || body["scenes_retagged"] != float64(4) || body["markers_updated"] != float64(2) ||
		body["references_updated"] != float64(1) || body["sources_deleted"] != float64(1) {
		t.Fatalf("body = %v", body)
	}
	if tgt := body["target"].(map[string]any); tgt["name"] != "Cumshot" {
		t.Fatalf("target = %v", tgt)
	}
	if srcs := body["sources"].([]any); len(srcs) != 1 {
		t.Fatalf("sources = %v", srcs)
	}
}

func TestUpdateTagEndpoint(t *testing.T) {
	r, tagRepo := newTagMergeTestRouter(t)

	if w, _ := doAliasRequest(t, r, http.MethodPut, "/tags/abc", `{"color":"#FF4D4D"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id: status %d, want 400", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/tags/1", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing color: status %d, want 400", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/tags/1", `{"color":"red"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid color: status %d, want 400", w.Code)
	}

	tagRepo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/tags/9", `{"color":"#FF4D4D"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown tag: status %d, want 404", w.Code)
	}

	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "Anal", Color: "#6B7280"}, nil)
	tagRepo.EXPECT().Update(gomock.Any()).Return(nil)
	w, body := doAliasRequest(t, r, http.MethodPut, "/tags/1", `{"color":"#22c55e"}`)
	if w.Code != http.StatusOK || body["color"] != "#22C55E" || body["name"] != "Anal" {
		t.Fatalf("status %d body %v", w.Code, body)
	}
}
