package handler

import (
	"strconv"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// SocialHandler serves the social graph and the privacy settings (docs/SOCIAL.md). Every
// route needs a signed-in user; the actions that create a relation need social:use (route
// middleware). Privacy is checked by the services, for every viewer (no admin bypass).
type SocialHandler struct {
	social          *core.SocialService
	privacy         *core.PrivacyService
	maxItemsPerPage int
}

func NewSocialHandler(social *core.SocialService, privacy *core.PrivacyService, maxItemsPerPage int) *SocialHandler {
	if maxItemsPerPage <= 0 {
		maxItemsPerPage = 100
	}
	return &SocialHandler{social: social, privacy: privacy, maxItemsPerPage: maxItemsPerPage}
}

const socialDefaultPageSize = 30

func socialUnauthorized(c *gin.Context) {
	response.Error(c, apperrors.NewUnauthorizedError("authentication required"))
}

func (h *SocialHandler) userID(c *gin.Context) (uint, bool) {
	user, exists := c.Get("user")
	if !exists {
		return 0, false
	}
	payload, ok := user.(*core.UserPayload)
	if !ok {
		return 0, false
	}
	return payload.UserID, true
}

func (h *SocialHandler) pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(socialDefaultPageSize)))
	return clampPagination(page, limit, socialDefaultPageSize, h.maxItemsPerPage)
}

// targetID parses the :userId path parameter.
func targetID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "invalid user ID")
		return 0, false
	}
	return uint(id), true
}

// relationshipAction runs an action on the user of the :userId path parameter and answers
// the new relation.
func (h *SocialHandler) relationshipAction(c *gin.Context, action func(viewerID, targetID uint) (core.Relationship, error)) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	target, ok := targetID(c)
	if !ok {
		return
	}
	rel, err := action(viewerID, target)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.RelationshipChangeResponse{UserID: target, Relationship: response.NewRelationshipResponse(rel)})
}

// Me is GET /social/me: pending request counts and relation counts of the signed-in user.
func (h *SocialHandler) Me(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	summary, err := h.social.Summary(c.Request.Context(), viewerID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewSocialSummaryResponse(summary))
}

// SearchUsers is GET /social/users?q=: the user directory with the viewer's relations.
func (h *SocialHandler) SearchUsers(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	page, limit := h.pagination(c)
	q := c.Query("q")
	if len(q) > 100 {
		q = q[:100]
	}
	result, err := h.social.SearchUsers(c.Request.Context(), viewerID, q, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewSocialUserPage(result, page, limit))
}

// GetUser is GET /social/users/:username: the social summary of a user as the viewer sees it.
func (h *SocialHandler) GetUser(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	profile, err := h.social.Profile(c.Request.Context(), viewerID, c.Param("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewSocialProfileResponse(profile))
}

func (h *SocialHandler) listRelations(c *gin.Context, list core.SocialList) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	page, limit := h.pagination(c)
	result, err := h.social.ListRelations(c.Request.Context(), viewerID, c.Param("username"), list, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewSocialUserPage(result, page, limit))
}

// ListFriends is GET /social/users/:username/friends (privacy: profile + friends_list).
func (h *SocialHandler) ListFriends(c *gin.Context) { h.listRelations(c, core.SocialListFriends) }

// ListFollowers is GET /social/users/:username/followers (privacy: profile + followers_list).
func (h *SocialHandler) ListFollowers(c *gin.Context) { h.listRelations(c, core.SocialListFollowers) }

// ListFollowing is GET /social/users/:username/following (privacy: profile + followers_list).
func (h *SocialHandler) ListFollowing(c *gin.Context) { h.listRelations(c, core.SocialListFollowing) }

// Follow is POST /social/follows/:userId (social:use).
func (h *SocialHandler) Follow(c *gin.Context) {
	h.relationshipAction(c, func(viewerID, target uint) (core.Relationship, error) {
		return h.social.Follow(c.Request.Context(), viewerID, target)
	})
}

// Unfollow is DELETE /social/follows/:userId.
func (h *SocialHandler) Unfollow(c *gin.Context) {
	h.relationshipAction(c, func(viewerID, target uint) (core.Relationship, error) {
		return h.social.Unfollow(c.Request.Context(), viewerID, target)
	})
}

// RemoveFriend is DELETE /social/friends/:userId.
func (h *SocialHandler) RemoveFriend(c *gin.Context) {
	h.relationshipAction(c, func(viewerID, target uint) (core.Relationship, error) {
		return h.social.RemoveFriend(c.Request.Context(), viewerID, target)
	})
}

// ListFriendRequests is GET /social/friend-requests?direction=incoming|outgoing.
func (h *SocialHandler) ListFriendRequests(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	page, limit := h.pagination(c)
	side := core.FriendRequestList(c.DefaultQuery("direction", string(core.FriendRequestsIncoming)))
	result, err := h.social.ListFriendRequests(c.Request.Context(), viewerID, side, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewSocialUserPage(result, page, limit))
}

// SendFriendRequest is POST /social/friend-requests {user_id} (social:use). When that user had
// already sent a request, it is accepted (status "accepted").
func (h *SocialHandler) SendFriendRequest(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	var req request.SendFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	accepted, rel, err := h.social.SendFriendRequest(c.Request.Context(), viewerID, req.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}
	status := "sent"
	if accepted {
		status = "accepted"
	}
	response.Created(c, response.FriendRequestSentResponse{
		Status:                     status,
		RelationshipChangeResponse: response.RelationshipChangeResponse{UserID: req.UserID, Relationship: response.NewRelationshipResponse(rel)},
	})
}

// AcceptFriendRequest is POST /social/friend-requests/:userId/accept (social:use).
func (h *SocialHandler) AcceptFriendRequest(c *gin.Context) {
	h.relationshipAction(c, func(viewerID, target uint) (core.Relationship, error) {
		return h.social.AcceptFriendRequest(c.Request.Context(), viewerID, target)
	})
}

// DeclineFriendRequest is POST /social/friend-requests/:userId/decline.
func (h *SocialHandler) DeclineFriendRequest(c *gin.Context) {
	h.relationshipAction(c, func(viewerID, target uint) (core.Relationship, error) {
		return h.social.DeclineFriendRequest(c.Request.Context(), viewerID, target)
	})
}

// CancelFriendRequest is DELETE /social/friend-requests/:userId (the viewer's own request).
func (h *SocialHandler) CancelFriendRequest(c *gin.Context) {
	h.relationshipAction(c, func(viewerID, target uint) (core.Relationship, error) {
		return h.social.CancelFriendRequest(c.Request.Context(), viewerID, target)
	})
}

// GetPrivacy is GET /social/privacy: the catalog with the viewer's choices.
func (h *SocialHandler) GetPrivacy(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	settings, err := h.privacy.GetSettings(c.Request.Context(), viewerID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewPrivacySettingsResponse(settings))
}

// UpdatePrivacy is PUT /social/privacy {items, friend_request_policy}.
func (h *SocialHandler) UpdatePrivacy(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	var req request.UpdatePrivacyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	settings, err := h.privacy.UpdateSettings(c.Request.Context(), viewerID, core.UpdatePrivacyInput{Items: req.Items, FriendRequestPolicy: req.FriendRequestPolicy})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewPrivacySettingsResponse(settings))
}
