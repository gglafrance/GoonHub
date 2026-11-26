package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/together"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TogetherHandler serves the Goon2gether REST routes and WebSocket (docs/GOON2GETHER.md).
type TogetherHandler struct {
	hub            *together.Hub
	emotes         *together.EmoteService
	originPatterns []string
	logger         *zap.Logger
}

// NewTogetherHandler creates the handler. originPatterns are the extra hosts accepted as the
// Origin of a WebSocket upgrade (the request host is always accepted).
func NewTogetherHandler(hub *together.Hub, emotes *together.EmoteService, originPatterns []string, logger *zap.Logger) *TogetherHandler {
	return &TogetherHandler{hub: hub, emotes: emotes, originPatterns: originPatterns, logger: logger.With(zap.String("handler", "together"))}
}

// EmoteDir is the directory the emote images are served from.
func (h *TogetherHandler) EmoteDir() string { return h.emotes.Dir() }

// actor is the authenticated user with their live access.
func (h *TogetherHandler) actor(c *gin.Context) (together.Actor, bool) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, apperrors.NewUnauthorizedError("authentication required"))
		return together.Actor{}, false
	}
	return together.Actor{ID: payload.UserID, Username: payload.Username, Access: middleware.GetAccess(c)}, true
}

func lobbyID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperrors.NewNotFoundError("lobby", c.Param("id")))
		return uuid.Nil, false
	}
	return id, true
}

// ListLobbies: GET /together/lobbies[?all=true]
func (h *TogetherHandler) ListLobbies(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	lobbies, err := h.hub.ListLobbies(a, c.Query("all") == "true")
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"lobbies": lobbies})
}

// CreateLobby: POST /together/lobbies
func (h *TogetherHandler) CreateLobby(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	var in together.CreateInput
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Error(c, apperrors.NewValidationError("invalid request body"))
			return
		}
	}
	s, err := h.hub.CreateLobby(a, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, s)
}

// GetLobby: GET /together/lobbies/:id
func (h *TogetherHandler) GetLobby(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	s, err := h.hub.GetLobby(a, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, s)
}

// JoinLobby: POST /together/lobbies/:id/join [{anonymous}]
func (h *TogetherHandler) JoinLobby(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	var in struct {
		Anonymous bool `json:"anonymous"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Error(c, apperrors.NewValidationError("invalid request body"))
			return
		}
	}
	s, err := h.hub.JoinLobby(a, id, in.Anonymous)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, s)
}

// JoinByCode: POST /together/join {code}
func (h *TogetherHandler) JoinByCode(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request body"))
		return
	}
	s, err := h.hub.JoinByCode(a, in.Code)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, s)
}

// LeaveLobby: POST /together/lobbies/:id/leave
func (h *TogetherHandler) LeaveLobby(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	if err := h.hub.LeaveLobby(a, id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CloseLobby: DELETE /together/lobbies/:id
func (h *TogetherHandler) CloseLobby(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	if err := h.hub.CloseLobby(a, id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RotateInviteCode: POST /together/lobbies/:id/invite-code
func (h *TogetherHandler) RotateInviteCode(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	code, err := h.hub.RotateInviteCode(a, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"invite_code": code})
}

// AddToQueue: POST /together/lobbies/:id/queue {items}
func (h *TogetherHandler) AddToQueue(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	var in struct {
		Items []together.ItemInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request body"))
		return
	}
	queued, suggested, err := h.hub.AddToQueue(a, id, in.Items)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"queued": queued, "suggested": suggested})
}

// ListEmotes: GET /together/emotes
func (h *TogetherHandler) ListEmotes(c *gin.Context) {
	emotes, err := h.emotes.List()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"emotes": emotes})
}

// UploadEmote: POST /together/emotes (multipart name, file)
func (h *TogetherHandler) UploadEmote(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperrors.NewValidationErrorWithField("file", "an image file is required"))
		return
	}
	f, err := file.Open()
	if err != nil {
		response.Error(c, apperrors.NewValidationErrorWithField("file", "failed to read the file"))
		return
	}
	defer f.Close()
	emote, err := h.emotes.Upload(c.PostForm("name"), f, a.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, emote)
}

func emoteID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, apperrors.NewNotFoundError("emote", c.Param("id")))
		return 0, false
	}
	return uint(id), true
}

// RenameEmote: PUT /together/emotes/:id {name}
func (h *TogetherHandler) RenameEmote(c *gin.Context) {
	id, ok := emoteID(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request body"))
		return
	}
	emote, err := h.emotes.Rename(id, in.Name)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, emote)
}

// DeleteEmote: DELETE /together/emotes/:id
func (h *TogetherHandler) DeleteEmote(c *gin.Context) {
	id, ok := emoteID(c)
	if !ok {
		return
	}
	if err := h.emotes.Delete(id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// OriginPatterns turns the allowed CORS origins (server.allowed_origins, full URLs) into the
// host patterns accepted as the Origin of a WebSocket upgrade, besides the request host.
func OriginPatterns(allowedOrigins []string) []string {
	out := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" || o == "*" {
			continue
		}
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		} else {
			out = append(out, o)
		}
	}
	return out
}
