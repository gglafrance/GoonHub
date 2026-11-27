package handler

import (
	"context"
	"net/http"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type stubSearcher struct{ scenes []data.Scene }

func (s stubSearcher) Search(data.SceneSearchParams) (*core.SearchResult, error) {
	return &core.SearchResult{Scenes: s.scenes, Total: int64(len(s.scenes))}, nil
}

func newProfileTabsRouter(t *testing.T, access *core.UserAccess) *profileListsEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	e := &profileListsEnv{
		repo:        mocks.NewMockProfileRepository(ctrl),
		social:      mocks.NewMockSocialRepository(ctrl),
		privacy:     mocks.NewMockPrivacyRepository(ctrl),
		interaction: mocks.NewMockInteractionRepository(ctrl),
	}
	privacy := core.NewPrivacyService(e.privacy, e.social, zap.NewNop())
	social := core.NewSocialService(e.social, privacy, core.NewEventBus(zap.NewNop()), config.SocialConfig{}, zap.NewNop())
	profiles := core.NewProfileService(e.repo, e.social, nil, social, privacy, nil, config.ProfilesConfig{ImageDir: t.TempDir()}, zap.NewNop()).
		WithCustomLists(stubSearcher{scenes: []data.Scene{{ID: 10, Size: 999, StoredPath: "/media/x.mp4"}}}, nil, nil)
	h := NewProfileHandler(profiles, 50)

	e.r = gin.New()
	e.r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
		middleware.SetAccess(c, access)
		c.Next()
	})
	perm := middleware.RequirePermission
	e.r.GET("/profiles/users/:username", h.GetProfile)
	e.r.GET("/profiles/users/:username/lists/:id", perm(core.PermScenesView), h.BrowseList)
	e.r.PUT("/profiles/me/tabs", h.SetMyTabs)
	e.r.POST("/profiles/me/lists", perm(core.PermScenesView), h.CreateMyList)
	e.r.DELETE("/profiles/me/lists/:id", h.DeleteMyList)
	return e
}

func TestProfileTabsHandler(t *testing.T) {
	e := newProfileTabsRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))
	for name, body := range map[string]string{
		"no tabs":      `{}`,
		"unknown kind": `{"tabs":[{"id":"x","kind":"feed"}]}`,
		"bad section":  `{"tabs":[{"id":"profile","kind":"profile","sections":[{"id":"s","template":"9","columns":[[]]}]}]}`,
	} {
		if w, _ := doAliasRequest(t, e.r, http.MethodPut, "/profiles/me/tabs", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", name, w.Code)
		}
	}
	e.repo.EXPECT().SaveTabs(gomock.Any(), uint(1), gomock.Any()).Return(nil)
	w, body := doAliasRequest(t, e.r, http.MethodPut, "/profiles/me/tabs",
		`{"tabs":[{"id":"profile","kind":"profile","visible":true,"sections":[{"id":"s","template":"1-1","columns":[[{"id":"st","type":"stats","visible":true}],[]]}]},{"id":"d","kind":"custom","title":"Discover","visible":true}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	tabs := body["tabs"].([]any)
	if len(tabs) != 5 || tabs[1].(map[string]any)["slug"] != "discover" {
		t.Fatalf("tabs: %v", tabs)
	}
}

func TestCustomListHandlers(t *testing.T) {
	e := newProfileTabsRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))
	e.repo.EXPECT().CountLists(gomock.Any(), uint(1)).Return(int64(0), nil).AnyTimes()
	if w, _ := doAliasRequest(t, e.r, http.MethodPost, "/profiles/me/lists", `{"title":"x","filters":{"sort":"nope"}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad sort: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodPost, "/profiles/me/lists", `{"title":"x","filters":{"min_size":10}}`); w.Code != http.StatusForbidden {
		t.Fatalf("size without files:view_size: %d", w.Code)
	}
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(1)).Return(nil, nil).AnyTimes()
	e.social.EXPECT().GetFriendRequestPolicy(gomock.Any(), uint(1)).Return("everyone", nil).AnyTimes()
	e.repo.EXPECT().CreateList(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *data.ProfileList) error {
		l.ID = 4
		return nil
	})
	w, body := doAliasRequest(t, e.r, http.MethodPost, "/profiles/me/lists", `{"title":"Top O","filters":{"sort":"jizz_count_desc"}}`)
	if w.Code != http.StatusCreated || body["id"] != float64(4) || body["audience"] != "public" {
		t.Fatalf("create: %d %v", w.Code, body)
	}
	deps := body["dependencies"].([]any)
	if len(deps) != 1 || deps[0].(map[string]any)["key"] != "jizz" || deps[0].(map[string]any)["audience"] != "friends" {
		t.Fatalf("dependencies: %v", deps)
	}
	e.repo.EXPECT().DeleteList(gomock.Any(), uint(1), uint(4)).Return(true, nil)
	if w, _ := doAliasRequest(t, e.r, http.MethodDelete, "/profiles/me/lists/4", ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodDelete, "/profiles/me/lists/abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", w.Code)
	}
}

func TestBrowseListHandlerRedacts(t *testing.T) {
	e := newProfileTabsRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))
	e.expectBob()
	list := data.ProfileList{ID: 7, UserID: 5, Title: "New", Audience: "public", Filters: data.Filters{Sort: "created_at_desc"}}
	e.repo.EXPECT().GetList(gomock.Any(), uint(5), uint(7)).Return(&list, nil)
	w, body := doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob/lists/7", "")
	if w.Code != http.StatusOK {
		t.Fatalf("browse: %d %s", w.Code, w.Body.String())
	}
	if body["list"].(map[string]any)["title"] != "New" {
		t.Fatalf("list: %v", body["list"])
	}
	if _, ok := body["list"].(map[string]any)["filters"]; ok {
		t.Fatal("the criteria are sent to a viewer")
	}
	scene := body["data"].([]any)[0].(map[string]any)["scene"].(map[string]any)
	if _, ok := scene["size"]; ok {
		t.Fatalf("size sent without files:view_size: %v", scene)
	}
	// A list on jizz (friends by default) is hidden from a stranger
	list.Filters = data.Filters{Sort: "jizz_count_desc"}
	e.repo.EXPECT().GetList(gomock.Any(), uint(5), uint(7)).Return(&list, nil)
	if w, _ := doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob/lists/7", ""); w.Code != http.StatusForbidden {
		t.Fatalf("hidden list: %d", w.Code)
	}
}
