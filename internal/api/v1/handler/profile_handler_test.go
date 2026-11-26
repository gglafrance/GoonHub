package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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

type profileHandlerEnv struct {
	r       *gin.Engine
	repo    *mocks.MockProfileRepository
	social  *mocks.MockSocialRepository
	privacy *mocks.MockPrivacyRepository
}

func newProfileTestRouter(t *testing.T, access *core.UserAccess) *profileHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockProfileRepository(ctrl)
	socialRepo := mocks.NewMockSocialRepository(ctrl)
	privacyRepo := mocks.NewMockPrivacyRepository(ctrl)
	privacy := core.NewPrivacyService(privacyRepo, socialRepo, zap.NewNop())
	social := core.NewSocialService(socialRepo, privacy, core.NewEventBus(zap.NewNop()), config.SocialConfig{}, zap.NewNop())
	profiles := core.NewProfileService(repo, socialRepo, nil, social, privacy, nil, config.ProfilesConfig{ImageDir: t.TempDir()}, zap.NewNop())
	h := NewProfileHandler(profiles, 100)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
		middleware.SetAccess(c, access)
		c.Next()
	})
	perm := middleware.RequirePermission
	// same shape as routes.go
	r.GET("/profiles/users/:username", h.GetProfile)
	r.GET("/profiles/avatar/:userId/:size", h.GetAvatar)
	r.PUT("/profiles/me/layout", h.SetMyLayout)
	r.PUT("/profiles/me/showcases/scenes", perm(core.PermScenesView), h.SetMyFavoriteScenes)
	r.POST("/profiles/me/avatar", h.UploadMyAvatar)
	return &profileHandlerEnv{r: r, repo: repo, social: socialRepo, privacy: privacyRepo}
}

// bob (5) shows his favorite videos to everyone.
func (e *profileHandlerEnv) expectBobFavorites() {
	e.social.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob"}, nil)
	e.social.EXPECT().Relations(gomock.Any(), uint(1), []uint{5}).Return([]data.UserRelation{{UserID: 5}}, nil).AnyTimes()
	e.social.EXPECT().Counts(gomock.Any(), uint(5)).Return(data.SocialCounts{}, nil)
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(5)).Return([]data.UserPrivacySetting{{UserID: 5, Item: core.PrivacyFavorites, Audience: "public"}}, nil).AnyTimes()
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Layout: data.ProfileLayout{{Type: "favorite_scenes", Visible: true}}}, nil)
	e.repo.EXPECT().FavoriteSceneIDs(gomock.Any(), uint(5)).Return([]uint{10}, nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{10}).Return([]data.Scene{{ID: 10, Title: "fav", Size: 12345, StoredPath: "/media/fav.mp4"}}, nil)
}

func profileFirstScene(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	blocks, _ := body["blocks"].([]any)
	if len(blocks) != 1 {
		t.Fatalf("blocks: %v", body["blocks"])
	}
	scenes := blocks[0].(map[string]any)["scenes"].([]any)
	return scenes[0].(map[string]any)["scene"].(map[string]any)
}

func TestProfileHandlerRedactsScenes(t *testing.T) {
	e := newProfileTestRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))
	e.expectBobFavorites()
	w, body := doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	scene := profileFirstScene(t, body)
	if _, ok := scene["size"]; ok {
		t.Fatalf("size sent without files:view_size: %v", scene)
	}
	if _, ok := scene["stored_path"]; ok {
		t.Fatalf("path sent without files:view_paths: %v", scene)
	}

	e = newProfileTestRouter(t, core.NewUserAccess(1, "moderator", false, core.PermScenesView, core.PermFilesViewSize, core.PermFilesViewPaths))
	e.expectBobFavorites()
	_, body = doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob", "")
	scene = profileFirstScene(t, body)
	if scene["size"] != float64(12345) || scene["stored_path"] != "/media/fav.mp4" {
		t.Fatalf("file info hidden from an allowed viewer: %v", scene)
	}
}

func TestProfileHandlerPermissionsAndValidation(t *testing.T) {
	e := newProfileTestRouter(t, core.NewUserAccess(1, "guest", false))
	if w, _ := doAliasRequest(t, e.r, http.MethodPut, "/profiles/me/showcases/scenes", `{"scene_ids":[1]}`); w.Code != http.StatusForbidden {
		t.Fatalf("showcase without scenes:view: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodPut, "/profiles/me/layout", `{"blocks":[{"type":"nope"}]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown block: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodPut, "/profiles/me/layout", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("no blocks: %d", w.Code)
	}
	e.repo.EXPECT().SaveLayout(gomock.Any(), uint(1), data.ProfileLayout(nil)).Return(nil)
	if w, body := doAliasRequest(t, e.r, http.MethodPut, "/profiles/me/layout", `{"reset":true}`); w.Code != http.StatusOK || len(body["blocks"].([]any)) != len(core.ProfileBlockSpecs()) {
		t.Fatalf("reset: %d %v", w.Code, body)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodGet, "/profiles/avatar/3/xl", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown size: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodGet, "/profiles/avatar/3/sm", ""); w.Code != http.StatusNotFound {
		t.Fatalf("no avatar: %d", w.Code)
	}

	// Uploads: not an image, no file
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "a.png")
	_, _ = part.Write([]byte("#!/bin/sh\necho not an image"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/profiles/me/avatar", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non image upload: %d %v", w.Code, out)
	}
	if w, _ := doAliasRequest(t, e.r, http.MethodPost, "/profiles/me/avatar", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("upload without file: %d", w.Code)
	}
}
