package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

// ProfileHandler serves the profile pages (docs/PROFILES.md). Every route needs a signed-in
// user; editing one's own profile needs nothing more, the routes that pick scenes or markers
// need scenes:view (route middleware). Privacy is checked by the service for every viewer.
type ProfileHandler struct {
	profiles        *core.ProfileService
	maxItemsPerPage int
	// cards: data of the scene cards of the lists (WithCardData)
	cards profileCardData
}

func NewProfileHandler(profiles *core.ProfileService, maxItemsPerPage int) *ProfileHandler {
	if maxItemsPerPage <= 0 {
		maxItemsPerPage = 100
	}
	return &ProfileHandler{profiles: profiles, maxItemsPerPage: maxItemsPerPage}
}

func (h *ProfileHandler) userID(c *gin.Context) (uint, bool) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil || payload == nil {
		response.Error(c, apperrors.NewUnauthorizedError("authentication required"))
		return 0, false
	}
	return payload.UserID, true
}

func (h *ProfileHandler) viewer(c *gin.Context) (core.ProfileViewer, bool) {
	id, ok := h.userID(c)
	if !ok {
		return core.ProfileViewer{}, false
	}
	return core.ProfileViewer{UserID: id, Access: middleware.GetAccess(c)}, true
}

// GetProfile is GET /profiles/users/:username: the profile as the viewer sees it.
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	viewer, ok := h.viewer(c)
	if !ok {
		return
	}
	view, err := h.profiles.Get(c.Request.Context(), viewer, c.Param("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewProfileResponse(view, fileVisibility(c)))
}

// GetMyProfile is GET /profiles/me: the edit state of the signed-in user's profile.
func (h *ProfileHandler) GetMyProfile(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	state, err := h.profiles.EditState(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewProfileEditResponse(state, h.profiles.Config(), fileVisibility(c)))
}

// UpdateMyProfile is PATCH /profiles/me {display_name?, bio?}; answers the edit state.
func (h *ProfileHandler) UpdateMyProfile(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	var req request.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if _, err := h.profiles.UpdateProfile(c.Request.Context(), userID, core.UpdateProfileInput{DisplayName: req.DisplayName, Bio: req.Bio}); err != nil {
		response.Error(c, err)
		return
	}
	h.GetMyProfile(c)
}

// SetMyLayout is PUT /profiles/me/layout {blocks} or {reset: true}.
func (h *ProfileHandler) SetMyLayout(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	var req request.SetProfileLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	var layout data.ProfileLayout
	if !req.Reset {
		if req.Blocks == nil {
			response.BadRequest(c, "blocks is required (or reset)")
			return
		}
		layout = data.ProfileLayout(req.Blocks)
	}
	out, err := h.profiles.SetLayout(c.Request.Context(), userID, layout)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.ProfileLayoutResponse{Blocks: out})
}

// SetMyFavoriteScenes is PUT /profiles/me/showcases/scenes {scene_ids} (scenes:view).
func (h *ProfileHandler) SetMyFavoriteScenes(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	var req request.SetFavoriteScenesRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SceneIDs == nil {
		response.BadRequest(c, "scene_ids is required")
		return
	}
	scenes, err := h.profiles.SetFavoriteScenes(c.Request.Context(), userID, req.SceneIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"scenes": response.ToSceneListItems(scenes, fileVisibility(c))})
}

// SetMyFavoriteMarkers is PUT /profiles/me/showcases/markers {marker_ids} (scenes:view).
func (h *ProfileHandler) SetMyFavoriteMarkers(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	var req request.SetFavoriteMarkersRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MarkerIDs == nil {
		response.BadRequest(c, "marker_ids is required")
		return
	}
	markers, err := h.profiles.SetFavoriteMarkers(c.Request.Context(), userID, req.MarkerIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"markers": markers})
}

// ListMarkerCandidates is GET /profiles/me/marker-candidates?q=&page=&limit= (scenes:view):
// the markers the user may showcase.
func (h *ProfileHandler) ListMarkerCandidates(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	page, limit = clampPagination(page, limit, 24, h.maxItemsPerPage)
	markers, total, err := h.profiles.MarkerCandidates(c.Request.Context(), userID, c.Query("q"), page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewPaginatedResponse(markers, page, limit, total))
}

// readCrop reads the optional crop rectangle of an image upload (crop_x, crop_y, crop_w,
// crop_h form fields, source pixels).
func readCrop(c *gin.Context) core.ProfileImageCrop {
	n := func(key string) int {
		v, err := strconv.Atoi(c.PostForm(key))
		if err != nil || v < 0 {
			return 0
		}
		return v
	}
	return core.ProfileImageCrop{X: n("crop_x"), Y: n("crop_y"), W: n("crop_w"), H: n("crop_h")}
}

// upload reads the "image" file of a multipart upload, limited to the configured size.
func (h *ProfileHandler) upload(c *gin.Context, store func(userID uint, file io.Reader, crop core.ProfileImageCrop) (int64, error)) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	limit := h.profiles.Config().MaxUploadBytes
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit+1<<20)
	header, err := c.FormFile("image")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(c, core.ErrProfileImageTooLarge("the image is too large"))
			return
		}
		response.BadRequest(c, "an image file is required (field image)")
		return
	}
	if header.Size > limit {
		response.Error(c, core.ErrProfileImageTooLarge("the image is too large"))
		return
	}
	file, err := header.Open()
	if err != nil {
		response.BadRequest(c, "failed to read the image")
		return
	}
	defer file.Close()
	version, err := store(userID, file, readCrop(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"version": version})
}

// UploadMyAvatar is POST /profiles/me/avatar (multipart image + optional crop): {version}.
func (h *ProfileHandler) UploadMyAvatar(c *gin.Context) {
	h.upload(c, func(userID uint, f io.Reader, crop core.ProfileImageCrop) (int64, error) {
		return h.profiles.UploadAvatar(c.Request.Context(), userID, f, crop)
	})
}

// DeleteMyAvatar is DELETE /profiles/me/avatar.
func (h *ProfileHandler) DeleteMyAvatar(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	if err := h.profiles.DeleteAvatar(c.Request.Context(), userID); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// UploadMyBanner is POST /profiles/me/banner (multipart image + optional crop): {version}.
func (h *ProfileHandler) UploadMyBanner(c *gin.Context) {
	h.upload(c, func(userID uint, f io.Reader, crop core.ProfileImageCrop) (int64, error) {
		return h.profiles.UploadBanner(c.Request.Context(), userID, f, crop)
	})
}

// SetMyBanner is PUT /profiles/me/banner {type: "scene", scene_id} or {type: "none"}. A scene
// banner needs scenes:view (checked here, the route serves both).
func (h *ProfileHandler) SetMyBanner(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	var req request.SetProfileBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	switch req.Type {
	case data.ProfileBannerScene:
		if !middleware.HasPermission(c, core.PermScenesView) {
			response.Error(c, apperrors.NewForbiddenError("a scene banner needs the scenes:view permission"))
			return
		}
		if req.SceneID == 0 {
			response.BadRequest(c, "scene_id is required")
			return
		}
		err := h.profiles.SetBannerScene(c.Request.Context(), userID, req.SceneID)
		if err != nil {
			response.Error(c, err)
			return
		}
	case data.ProfileBannerNone:
		if err := h.profiles.ClearBanner(c.Request.Context(), userID); err != nil {
			response.Error(c, err)
			return
		}
	default:
		response.BadRequest(c, `type must be "scene" or "none" (uploads use POST)`)
		return
	}
	response.NoContent(c)
}

// DeleteMyBanner is DELETE /profiles/me/banner.
func (h *ProfileHandler) DeleteMyBanner(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	if err := h.profiles.ClearBanner(c.Request.Context(), userID); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func imageOwnerID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "invalid user ID")
		return 0, false
	}
	return uint(id), true
}

// serveProfileImage sends a profile image file. The URLs are versioned (?v=), so the files
// are cached for a long time, privately (they are served to signed-in users only).
func serveProfileImage(c *gin.Context, path string) {
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(path)
}

// GetAvatar is GET /profiles/avatar/:userId/:size (sm, md, lg). Avatars are shown wherever a
// user is (lists, header), like usernames: no privacy item.
func (h *ProfileHandler) GetAvatar(c *gin.Context) {
	if _, ok := h.userID(c); !ok {
		return
	}
	ownerID, ok := imageOwnerID(c)
	if !ok {
		return
	}
	path, err := h.profiles.AvatarPath(ownerID, c.Param("size"))
	if err != nil {
		response.Error(c, err)
		return
	}
	serveProfileImage(c, path)
}

// GetBanner is GET /profiles/banner/:userId/:size (md, lg): an uploaded banner, for the
// viewers who may see the profile.
func (h *ProfileHandler) GetBanner(c *gin.Context) {
	viewerID, ok := h.userID(c)
	if !ok {
		return
	}
	ownerID, ok := imageOwnerID(c)
	if !ok {
		return
	}
	path, err := h.profiles.BannerPath(c.Request.Context(), viewerID, ownerID, c.Param("size"))
	if err != nil {
		response.Error(c, err)
		return
	}
	serveProfileImage(c, path)
}
