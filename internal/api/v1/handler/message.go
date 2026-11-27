package handler

import (
	"strconv"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// MessageHandler serves the messages between users and the shares (docs/MESSAGES.md). Every
// route needs a signed-in user; sending (messages, edits, shares, typing) needs messages:use
// (route middleware). The service only lets the participants of a conversation read it: there
// is no administrator access to other users' messages.
type MessageHandler struct {
	messages        *core.MessageService
	maxItemsPerPage int
}

func NewMessageHandler(messages *core.MessageService, maxItemsPerPage int) *MessageHandler {
	if maxItemsPerPage <= 0 {
		maxItemsPerPage = 100
	}
	return &MessageHandler{messages: messages, maxItemsPerPage: maxItemsPerPage}
}

const conversationsDefaultPageSize = 30

// actor returns the signed-in user with their live access.
func (h *MessageHandler) actor(c *gin.Context) (core.MessageActor, bool) {
	user, exists := c.Get("user")
	if !exists {
		socialUnauthorized(c)
		return core.MessageActor{}, false
	}
	payload, ok := user.(*core.UserPayload)
	if !ok || payload.UserID == 0 {
		socialUnauthorized(c)
		return core.MessageActor{}, false
	}
	access := middleware.GetAccess(c)
	if access == nil {
		access = core.NewUserAccess(payload.UserID, payload.Role, false)
	}
	return core.MessageActor{ID: payload.UserID, Username: payload.Username, Access: access}, true
}

func idParam(c *gin.Context, name, what string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "invalid "+what+" ID")
		return 0, false
	}
	return uint(id), true
}

func conversationID(c *gin.Context) (uint, bool) { return idParam(c, "id", "conversation") }
func messageID(c *gin.Context) (uint, bool)      { return idParam(c, "messageId", "message") }

// ListConversations is GET /messages/conversations?page&limit.
func (h *MessageHandler) ListConversations(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(conversationsDefaultPageSize)))
	page, limit = clampPagination(page, limit, conversationsDefaultPageSize, h.maxItemsPerPage)
	result, err := h.messages.ListConversations(c.Request.Context(), actor, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewPaginatedResponse(result.Conversations, page, limit, result.Total))
}

// Unread is GET /messages/unread: {messages, conversations}, muted conversations excluded.
func (h *MessageHandler) Unread(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	v, err := h.messages.Unread(c.Request.Context(), actor.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// WithUser is GET /messages/with/:username: the thread with a user (conversation or null).
func (h *MessageHandler) WithUser(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	thread, err := h.messages.WithUser(c.Request.Context(), actor, c.Param("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, thread)
}

// GetConversation is GET /messages/conversations/:id.
func (h *MessageHandler) GetConversation(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	v, err := h.messages.Conversation(c.Request.Context(), actor, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// History is GET /messages/conversations/:id/messages?before=<message id>&limit=.
func (h *MessageHandler) History(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	var before uint
	if raw := c.Query("before"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			response.BadRequest(c, "invalid before")
			return
		}
		before = uint(v)
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	v, err := h.messages.History(c.Request.Context(), actor, id, before, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// Send is POST /messages/conversations/:id/messages {body, attachment} (messages:use).
func (h *MessageHandler) Send(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	var req request.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	v, err := h.messages.Send(c.Request.Context(), actor, id, core.SendMessageInput{Body: req.Body, Attachment: req.Attachment})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, v)
}

// directSendResponse answers POST /messages/direct/:userId/messages.
type directSendResponse struct {
	Conversation *core.ConversationView `json:"conversation"`
	Message      *core.MessageView      `json:"message"`
}

// SendDirect is POST /messages/direct/:userId/messages {body, attachment} (messages:use): the
// conversation with that user is created on the first message.
func (h *MessageHandler) SendDirect(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	target, ok := targetID(c)
	if !ok {
		return
	}
	var req request.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	conv, msg, err := h.messages.SendDirect(c.Request.Context(), actor, target, core.SendMessageInput{Body: req.Body, Attachment: req.Attachment})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, directSendResponse{Conversation: conv, Message: msg})
}

// Share is POST /messages/share {user_ids, body, items} (messages:use): {results: [...]}.
func (h *MessageHandler) Share(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req request.ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	results, err := h.messages.Share(c.Request.Context(), actor, core.ShareInput{UserIDs: req.UserIDs, Body: req.Body, Items: req.Items})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"results": results})
}

// Edit is PATCH /messages/conversations/:id/messages/:messageId {body} (messages:use).
func (h *MessageHandler) Edit(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	mid, ok := messageID(c)
	if !ok {
		return
	}
	var req request.EditMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	v, err := h.messages.Edit(c.Request.Context(), actor, id, mid, req.Body)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// Delete is DELETE /messages/conversations/:id/messages/:messageId (always allowed to the
// author).
func (h *MessageHandler) Delete(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	mid, ok := messageID(c)
	if !ok {
		return
	}
	if err := h.messages.Delete(c.Request.Context(), actor, id, mid); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// MarkRead is POST /messages/conversations/:id/read {message_id}.
func (h *MessageHandler) MarkRead(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	var req request.MarkReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.messages.MarkRead(c.Request.Context(), actor.ID, id, req.MessageID); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// Typing is POST /messages/conversations/:id/typing (messages:use): relayed, never stored.
func (h *MessageHandler) Typing(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	if err := h.messages.Typing(c.Request.Context(), actor, id); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// Mute is PUT /messages/conversations/:id/mute {muted}.
func (h *MessageHandler) Mute(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	var req request.MuteConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	v, err := h.messages.SetMuted(c.Request.Context(), actor, id, req.Muted)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// JoinLobby is POST /messages/conversations/:id/messages/:messageId/join-lobby: joins the lobby
// the message invites to, {lobby_id}.
func (h *MessageHandler) JoinLobby(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	mid, ok := messageID(c)
	if !ok {
		return
	}
	lobbyID, err := h.messages.JoinLobby(c.Request.Context(), actor, id, mid)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"lobby_id": lobbyID})
}

// GetSettings is GET /messages/settings.
func (h *MessageHandler) GetSettings(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	v, err := h.messages.Settings(c.Request.Context(), actor.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// UpdateSettings is PUT /messages/settings {policy?, toasts?}.
func (h *MessageHandler) UpdateSettings(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req request.UpdateMessageSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	v, err := h.messages.UpdateSettings(c.Request.Context(), actor.ID, req.Policy, req.Toasts)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// AddReaction is POST /messages/conversations/:id/messages/:messageId/reactions {emoji}
// (messages:use): the reactions of the message after the change.
func (h *MessageHandler) AddReaction(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	mid, ok := messageID(c)
	if !ok {
		return
	}
	var req request.ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	v, err := h.messages.React(c.Request.Context(), actor, id, mid, req.Emoji)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// RemoveReaction is DELETE /messages/conversations/:id/messages/:messageId/reactions?emoji=
// (always allowed to a participant, like deleting one's own message).
func (h *MessageHandler) RemoveReaction(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := conversationID(c)
	if !ok {
		return
	}
	mid, ok := messageID(c)
	if !ok {
		return
	}
	emoji := c.Query("emoji")
	if emoji == "" {
		response.BadRequest(c, "emoji is required")
		return
	}
	v, err := h.messages.Unreact(c.Request.Context(), actor, id, mid, emoji)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}
