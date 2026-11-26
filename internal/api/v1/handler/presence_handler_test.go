package handler

import (
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

func newPresenceTestRouter(t *testing.T, access *core.UserAccess) (*gin.Engine, *mocks.MockPresenceRepository, *mocks.MockSceneRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	presenceRepo := mocks.NewMockPresenceRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	privacy := core.NewPrivacyService(mocks.NewMockPrivacyRepository(ctrl), mocks.NewMockSocialRepository(ctrl), zap.NewNop())
	svc := core.NewPresenceService(presenceRepo, privacy, nil, sceneRepo, nil, core.NewEventBus(zap.NewNop()), config.PresenceConfig{}, zap.NewNop())
	h := NewPresenceHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
		middleware.SetAccess(c, access)
		c.Next()
	})
	// same shape as routes.go
	r.GET("/social/presence", h.Snapshot)
	r.GET("/social/presence/users/:userId", h.GetUser)
	r.POST("/social/presence/heartbeat", h.Heartbeat)
	r.PUT("/social/presence/activity", h.UpdateActivity)
	r.DELETE("/social/presence/activity", h.ClearActivity)
	r.GET("/social/presence/settings", h.GetSettings)
	r.PUT("/social/presence/settings", h.UpdateSettings)
	return r, presenceRepo, sceneRepo
}

func TestPresenceActivityNeedsScenesView(t *testing.T) {
	r, _, _ := newPresenceTestRouter(t, core.NewUserAccess(1, "user", false))
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/presence/activity", `{"type":"watching_scene","scene_id":10}`); w.Code != http.StatusForbidden {
		t.Fatalf("watching without scenes:view: %d", w.Code)
	}
}

func TestPresenceActivityEndpoints(t *testing.T) {
	r, _, sceneRepo := newPresenceTestRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))

	for _, body := range []string{`{}`, `{"type":"dancing"}`, `{"type":"watching_scene"}`, `{"type":"watching_scene","scene_id":10,"tab_id":"no spaces"}`} {
		if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/presence/activity", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
	sceneRepo.EXPECT().GetByIDs([]uint{99}).Return(nil, nil)
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/presence/activity", `{"type":"watching_scene","scene_id":99}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown scene: %d", w.Code)
	}
	sceneRepo.EXPECT().GetByIDs([]uint{10}).Return([]data.Scene{{ID: 10, Title: "Ten"}}, nil)
	body := `{"tab_id":"tab-1","type":"watching_scene","scene_id":10,"position":12.5,"paused":false}`
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/presence/activity", body); w.Code != http.StatusNoContent {
		t.Fatalf("activity: %d", w.Code)
	}
	// same scene again: no scene read
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/presence/activity", body); w.Code != http.StatusNoContent {
		t.Fatalf("activity progress: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodDelete, "/social/presence/activity?tab_id=bad!", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad tab id: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodDelete, "/social/presence/activity?tab_id=tab-1", ""); w.Code != http.StatusNoContent {
		t.Fatalf("clear: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/social/presence/heartbeat", `{"active":true}`); w.Code != http.StatusNoContent {
		t.Fatalf("heartbeat: %d", w.Code)
	}
}

func TestPresenceSnapshotAndSettingsEndpoints(t *testing.T) {
	r, repo, _ := newPresenceTestRouter(t, core.NewUserAccess(1, "user", false))
	repo.EXPECT().Contacts(gomock.Any(), uint(1)).Return(nil, nil)
	repo.EXPECT().GetUser(gomock.Any(), uint(1)).Return(&data.PresenceContact{ID: 1, Username: "me"}, nil).AnyTimes()

	w, body := doAliasRequest(t, r, http.MethodGet, "/social/presence", "")
	if w.Code != http.StatusOK {
		t.Fatalf("snapshot: %d", w.Code)
	}
	if users, ok := body["users"].([]any); !ok || len(users) != 0 || body["invisible"] != false ||
		body["heartbeat_interval_seconds"] != float64(60) || body["activity_interval_seconds"] != float64(15) {
		t.Fatalf("snapshot body: %v", body)
	}

	if w, _ := doAliasRequest(t, r, http.MethodGet, "/social/presence/users/abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad user id: %d", w.Code)
	}

	if w, body := doAliasRequest(t, r, http.MethodGet, "/social/presence/settings", ""); w.Code != http.StatusOK || body["invisible"] != false {
		t.Fatalf("settings: %d %v", w.Code, body)
	}
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/presence/settings", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("settings without invisible: %d", w.Code)
	}
	repo.EXPECT().SetInvisible(gomock.Any(), uint(1), true, nil).Return(nil)
	if w, body := doAliasRequest(t, r, http.MethodPut, "/social/presence/settings", `{"invisible":true}`); w.Code != http.StatusOK || body["invisible"] != true {
		t.Fatalf("set invisible: %d %v", w.Code, body)
	}
}
