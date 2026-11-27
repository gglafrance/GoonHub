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

type profileListsEnv struct {
	r           *gin.Engine
	repo        *mocks.MockProfileRepository
	social      *mocks.MockSocialRepository
	privacy     *mocks.MockPrivacyRepository
	interaction *mocks.MockInteractionRepository
}

func newProfileListsRouter(t *testing.T, access *core.UserAccess) *profileListsEnv {
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
	profiles := core.NewProfileService(e.repo, e.social, nil, social, privacy, nil, config.ProfilesConfig{ImageDir: t.TempDir()}, zap.NewNop())
	h := NewProfileHandler(profiles, 50).WithCardData(nil, e.interaction, nil, nil)

	e.r = gin.New()
	e.r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
		middleware.SetAccess(c, access)
		c.Next()
	})
	perm := middleware.RequirePermission
	// same shape as routes.go
	e.r.GET("/profiles/users/:username/ratings/histogram", perm(core.PermScenesView), h.GetRatingHistogram)
	e.r.GET("/profiles/users/:username/ratings", perm(core.PermScenesView), h.ListRatedScenes)
	e.r.GET("/profiles/users/:username/likes", perm(core.PermScenesView), h.ListLikedScenes)
	e.r.GET("/profiles/users/:username/jizz", perm(core.PermScenesView), h.ListJizzScenes)
	return e
}

// bob (5) shows his ratings to everyone; his jizz list keeps its default (friends).
func (e *profileListsEnv) expectBob() {
	e.social.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob"}, nil).AnyTimes()
	e.social.EXPECT().Relations(gomock.Any(), uint(1), []uint{5}).Return([]data.UserRelation{{UserID: 5}}, nil).AnyTimes()
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(5)).Return([]data.UserPrivacySetting{{UserID: 5, Item: core.PrivacyRatings, Audience: "public"}}, nil).AnyTimes()
}

func TestProfileRatedListRedactsAndCarriesFigures(t *testing.T) {
	e := newProfileListsRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))
	e.expectBob()
	e.repo.EXPECT().RatedScenes(gomock.Any(), uint(5), data.ProfileListQuery{Sort: data.ProfileSortRatedDesc, Rating: ptrFloat64(4.5), Page: 2, Limit: 50}).
		Return([]data.ProfileSceneRef{{SceneID: 10, Rating: ptrFloat64(4.5)}}, int64(51), nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{10}).Return([]data.Scene{{ID: 10, Title: "x", Size: 999, StoredPath: "/media/x.mp4"}}, nil)
	// the card sidecars are the viewer's own ratings
	e.interaction.EXPECT().GetRatingsBySceneIDs(uint(1), []uint{10}).Return(map[uint]float64{10: 2}, nil)

	w, body := doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob/ratings?rating=4.5&sort=rated_desc&page=2&limit=500&card_fields=rating", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	items := body["data"].([]any)
	item := items[0].(map[string]any)
	if item["rating"] != 4.5 {
		t.Fatalf("owner rating: %v", item)
	}
	scene := item["scene"].(map[string]any)
	if _, ok := scene["size"]; ok {
		t.Fatalf("size sent without files:view_size: %v", scene)
	}
	if _, ok := scene["stored_path"]; ok {
		t.Fatalf("path sent without files:view_paths: %v", scene)
	}
	pg := body["pagination"].(map[string]any)
	if pg["total_items"] != float64(51) || pg["limit"] != float64(50) || body["sort"] != "rated_desc" {
		t.Fatalf("pagination: %v, sort %v", pg, body["sort"])
	}
	if ratings := body["ratings"].(map[string]any); ratings["10"] != float64(2) {
		t.Fatalf("viewer sidecar: %v", body["ratings"])
	}
}

func TestProfileListsHandlerPrivacyAndValidation(t *testing.T) {
	e := newProfileListsRouter(t, core.NewUserAccess(1, "user", false, core.PermScenesView))
	e.expectBob()
	w, body := doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob/jizz", "")
	if w.Code != http.StatusForbidden || body["code"] != "PRIVACY_RESTRICTED" {
		t.Fatalf("jizz for a stranger: %d %v", w.Code, body)
	}
	for _, path := range []string{
		"/profiles/users/bob/ratings?rating=abc",
		"/profiles/users/bob/ratings?rating=3.3",
		"/profiles/users/bob/ratings?sort=size_desc",
		"/profiles/users/bob/likes?rating=4",
	} {
		if w, _ := doAliasRequest(t, e.r, http.MethodGet, path, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}

	e.repo.EXPECT().RatingSummary(gomock.Any(), uint(5)).Return(&data.ProfileRatingSummary{Buckets: []data.ProfileRatingBucket{{Rating: 3, Count: 1}}, Average: ptrFloat64(3)}, nil)
	w, body = doAliasRequest(t, e.r, http.MethodGet, "/profiles/users/bob/ratings/histogram", "")
	if w.Code != http.StatusOK || len(body["buckets"].([]any)) != 10 || body["total"] != float64(1) || body["average"] != float64(3) {
		t.Fatalf("histogram: %d %v", w.Code, body)
	}

	guest := newProfileListsRouter(t, core.NewUserAccess(1, "guest", false))
	if w, _ := doAliasRequest(t, guest.r, http.MethodGet, "/profiles/users/bob/likes", ""); w.Code != http.StatusForbidden {
		t.Fatalf("likes without scenes:view: %d", w.Code)
	}
}

func ptrFloat64(v float64) *float64 { return &v }
