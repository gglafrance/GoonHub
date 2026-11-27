package handler

import (
	"net/http"
	"testing"
	"time"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newMessageTestRouter(t *testing.T, access *core.UserAccess) (*gin.Engine, *mocks.MockMessageRepository, *mocks.MockSocialRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockMessageRepository(ctrl)
	socialRepo := mocks.NewMockSocialRepository(ctrl)
	svc := core.NewMessageService(repo, socialRepo, nil, nil, nil, core.NewEventBus(zap.NewNop()), config.MessagesConfig{}, zap.NewNop())
	h := NewMessageHandler(svc, 100)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: access.UserID, Username: "alice", Role: access.Role})
		middleware.SetAccess(c, access)
		c.Next()
	})
	perm := middleware.RequirePermission
	// same shape as routes.go
	r.GET("/messages/conversations", h.ListConversations)
	r.GET("/messages/unread", h.Unread)
	r.GET("/messages/with/:username", h.WithUser)
	r.PUT("/messages/settings", h.UpdateSettings)
	r.POST("/messages/share", perm(core.PermMessagesUse), h.Share)
	r.POST("/messages/direct/:userId/messages", perm(core.PermMessagesUse), h.SendDirect)
	r.GET("/messages/conversations/:id/messages", h.History)
	r.POST("/messages/conversations/:id/messages", perm(core.PermMessagesUse), h.Send)
	r.PATCH("/messages/conversations/:id/messages/:messageId", perm(core.PermMessagesUse), h.Edit)
	r.DELETE("/messages/conversations/:id/messages/:messageId", h.Delete)
	r.POST("/messages/conversations/:id/read", h.MarkRead)
	r.POST("/messages/conversations/:id/typing", perm(core.PermMessagesUse), h.Typing)
	return r, repo, socialRepo
}

func TestMessageEndpointsNeedMessagesUse(t *testing.T) {
	r, _, _ := newMessageTestRouter(t, core.NewUserAccess(1, "viewer", false, core.PermScenesView))
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/messages/direct/2/messages", `{"body":"hi"}`},
		{http.MethodPost, "/messages/conversations/7/messages", `{"body":"hi"}`},
		{http.MethodPatch, "/messages/conversations/7/messages/3", `{"body":"hi"}`},
		{http.MethodPost, "/messages/conversations/7/typing", ``},
		{http.MethodPost, "/messages/share", `{"user_ids":[2],"items":[{"kind":"scene","scene_id":1}]}`},
	} {
		if w, _ := doAliasRequest(t, r, tc.method, tc.path, tc.body); w.Code != http.StatusForbidden {
			t.Fatalf("%s %s without messages:use: %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestMessageEndpoints(t *testing.T) {
	r, repo, socialRepo := newMessageTestRouter(t, core.NewUserAccess(1, "user", false, core.PermMessagesUse))

	if w, _ := doAliasRequest(t, r, http.MethodPost, "/messages/conversations/abc/messages", `{"body":"x"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid conversation id: %d", w.Code)
	}
	if w, _ := doAliasRequest(t, r, http.MethodGet, "/messages/conversations/7/messages?before=x", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid before: %d", w.Code)
	}

	// a conversation the user is not in: 404, whatever the action
	repo.EXPECT().ParticipantIDs(gomock.Any(), uint(8)).Return([]uint{2, 3}, nil).AnyTimes()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/messages/conversations/8/messages", ""},
		{http.MethodPost, "/messages/conversations/8/messages", `{"body":"x"}`},
		{http.MethodDelete, "/messages/conversations/8/messages/3", ""},
		{http.MethodPost, "/messages/conversations/8/read", `{"message_id":3}`},
	} {
		if w, body := doAliasRequest(t, r, tc.method, tc.path, tc.body); w.Code != http.StatusNotFound || body["code"] != "CONVERSATION_NOT_FOUND" {
			t.Fatalf("%s %s on another conversation: %d %v", tc.method, tc.path, w.Code, body)
		}
	}

	// sending to a user whose policy refuses: 403 MESSAGES_CLOSED
	socialRepo.EXPECT().GetUser(gomock.Any(), uint(2)).Return(&data.SocialUser{ID: 2, Username: "bob"}, nil).AnyTimes()
	socialRepo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, MessagePolicy: data.MessagePolicyNobody, Friend: true}}, nil)
	if w, body := doAliasRequest(t, r, http.MethodPost, "/messages/direct/2/messages", `{"body":"hi"}`); w.Code != http.StatusForbidden || body["code"] != "MESSAGES_CLOSED" {
		t.Fatalf("closed: %d %v", w.Code, body)
	}

	// allowed: 201 with the conversation and the message
	socialRepo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, Friend: true}}, nil).AnyTimes()
	repo.EXPECT().FindOrCreateDirect(gomock.Any(), uint(1), uint(2)).Return(&data.Conversation{ID: 7, Kind: "direct"}, true, nil)
	repo.EXPECT().InsertMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, m *data.Message) error {
		m.ID = 50
		return nil
	})
	other := uint(2)
	at := time.Now()
	last := uint(50)
	repo.EXPECT().GetConversationRow(gomock.Any(), uint(1), uint(7)).Return(&data.ConversationRow{ID: 7, Kind: "direct", OtherUserID: &other, OtherUsername: "bob", LastID: &last, LastCreatedAt: &at, LastBody: "hi"}, nil)
	w, body := doAliasRequest(t, r, http.MethodPost, "/messages/direct/2/messages", `{"body":"hi"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("send: %d %v", w.Code, body)
	}
	conv, _ := body["conversation"].(map[string]any)
	msg, _ := body["message"].(map[string]any)
	if conv["id"] != float64(7) || conv["can_send"] != true || msg["id"] != float64(50) || msg["body"] != "hi" {
		t.Fatalf("send answer: %v", body)
	}

	// unread counts
	repo.EXPECT().UnreadSummary(gomock.Any(), uint(1)).Return(data.UnreadSummary{Messages: 3, Conversations: 2}, nil)
	if w, body := doAliasRequest(t, r, http.MethodGet, "/messages/unread", ""); w.Code != http.StatusOK || body["messages"] != float64(3) || body["conversations"] != float64(2) {
		t.Fatalf("unread: %d %v", w.Code, body)
	}

	// settings validation
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/messages/settings", `{"policy":"strangers"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown policy: %d", w.Code)
	}
}
