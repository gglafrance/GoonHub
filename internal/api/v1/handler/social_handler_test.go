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

func newSocialTestRouter(t *testing.T, access *core.UserAccess) (*gin.Engine, *mocks.MockSocialRepository, *mocks.MockPrivacyRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	socialRepo := mocks.NewMockSocialRepository(ctrl)
	privacyRepo := mocks.NewMockPrivacyRepository(ctrl)
	privacy := core.NewPrivacyService(privacyRepo, socialRepo, zap.NewNop())
	social := core.NewSocialService(socialRepo, privacy, core.NewEventBus(zap.NewNop()), config.SocialConfig{}, zap.NewNop())
	h := NewSocialHandler(social, privacy, 100)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: access.UserID, Role: access.Role})
		middleware.SetAccess(c, access)
		c.Next()
	})
	perm := middleware.RequirePermission
	// same shape as routes.go
	r.GET("/social/users/:username/friends", h.ListFriends)
	r.POST("/social/follows/:userId", perm(core.PermSocialUse), h.Follow)
	r.POST("/social/friend-requests", perm(core.PermSocialUse), h.SendFriendRequest)
	r.DELETE("/social/friend-requests/:userId", h.CancelFriendRequest)
	r.PUT("/social/privacy", h.UpdatePrivacy)
	return r, socialRepo, privacyRepo
}

func TestSocialEndpointsNeedSocialUse(t *testing.T) {
	r, _, _ := newSocialTestRouter(t, core.NewUserAccess(1, "viewer", false, core.PermScenesView))
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/social/follows/2", ""); w.Code != http.StatusForbidden {
		t.Fatalf("follow without social:use: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/social/friend-requests", `{"user_id":2}`); w.Code != http.StatusForbidden {
		t.Fatalf("request without social:use: %d", w.Code)
	}
}

func TestSocialEndpoints(t *testing.T) {
	r, socialRepo, privacyRepo := newSocialTestRouter(t, core.NewUserAccess(1, "user", false, core.PermSocialUse))

	if w, _ := doAliasRequest(t, r, http.MethodPost, "/social/follows/abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id: %d", w.Code)
	}
	if w, body := doAliasRequest(t, r, http.MethodPost, "/social/follows/1", ""); w.Code != http.StatusBadRequest || body["code"] != "SELF_RELATION" {
		t.Fatalf("self follow: %d %v", w.Code, body)
	}

	// a request to someone who already asked: accepted
	socialRepo.EXPECT().GetUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, id uint) (*data.SocialUser, error) {
		return &data.SocialUser{ID: id, Username: "u"}, nil
	}).AnyTimes()
	gomock.InOrder(
		socialRepo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, RequestReceived: true, RequestPolicy: "nobody"}}, nil),
		socialRepo.EXPECT().SendFriendRequest(gomock.Any(), uint(1), uint(2)).Return(data.FriendRequestAccepted, nil),
		socialRepo.EXPECT().CountIncomingRequests(gomock.Any(), uint(1)).Return(int64(0), nil),
		socialRepo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, Friend: true}}, nil),
	)
	w, body := doAliasRequest(t, r, http.MethodPost, "/social/friend-requests", `{"user_id":2}`)
	if w.Code != http.StatusCreated || body["status"] != "accepted" || !body["relationship"].(map[string]any)["friend"].(bool) {
		t.Fatalf("send: %d %v", w.Code, body)
	}

	socialRepo.EXPECT().DeleteFriendRequest(gomock.Any(), uint(1), uint(3)).Return(false, nil)
	if w, body := doAliasRequest(t, r, http.MethodDelete, "/social/friend-requests/3", ""); w.Code != http.StatusNotFound || body["code"] != "FRIEND_REQUEST_NOT_FOUND" {
		t.Fatalf("cancel unknown: %d %v", w.Code, body)
	}

	// a hidden list answers 403 PRIVACY_RESTRICTED
	socialRepo.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob"}, nil)
	privacyRepo.EXPECT().ListSettings(gomock.Any(), uint(5)).Return([]data.UserPrivacySetting{{UserID: 5, Item: core.PrivacyFriendsList, Audience: "private"}}, nil)
	socialRepo.EXPECT().Relations(gomock.Any(), uint(1), []uint{5}).Return([]data.UserRelation{{UserID: 5, Friend: true}}, nil)
	if w, body := doAliasRequest(t, r, http.MethodGet, "/social/users/bob/friends", ""); w.Code != http.StatusForbidden || body["code"] != "PRIVACY_RESTRICTED" {
		t.Fatalf("hidden list: %d %v", w.Code, body)
	}

	if w, _ := doAliasRequest(t, r, http.MethodPut, "/social/privacy", `{"items":{"stats":"everyone"}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid audience: %d", w.Code)
	}
}
