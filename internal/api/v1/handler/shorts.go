package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// shortsSearcher runs the feed query. core.SearchService satisfies it.
type shortsSearcher interface {
	Search(params data.SceneSearchParams) (*core.SearchResult, error)
}

// shortsSettings reads and writes the shorts settings. core.ShortsSettingsService satisfies it.
type shortsSettings interface {
	GetMaxDuration() (int, error)
	GetSearchDefault() (string, error)
	CanCreate() bool
	GetSettings() (*core.ShortsSettings, error)
	UpdateSettings(maxDuration int, saveDir *string, searchDefault string) (*core.ShortsSettings, error)
}

// shortCreator queues shorts. core.ShortClipService satisfies it.
type shortCreator interface {
	CreateShort(sourceID uint, start, end float64, title string) (*core.ShortClipTask, error)
	ListTasks(sourceID uint) []core.ShortClipTask
}

// permissionChecker answers whether a role has a permission. core.RBACService satisfies it.
type permissionChecker interface {
	HasPermission(role, permission string) bool
}

type ShortsHandler struct {
	Search          shortsSearcher
	Settings        shortsSettings
	Creator         shortCreator
	SceneRepo       data.SceneRepository
	InteractionRepo data.InteractionRepository
	RBAC            permissionChecker
	MaxItemsPerPage int
}

func NewShortsHandler(search *core.SearchService, settings *core.ShortsSettingsService, creator *core.ShortClipService, sceneRepo data.SceneRepository, interactionRepo data.InteractionRepository, rbac *core.RBACService, maxItemsPerPage int) *ShortsHandler {
	return &ShortsHandler{
		Search:          search,
		Settings:        settings,
		Creator:         creator,
		SceneRepo:       sceneRepo,
		InteractionRepo: interactionRepo,
		RBAC:            rbac,
		MaxItemsPerPage: maxItemsPerPage,
	}
}

// parseClipsFilter maps the clips query value to SceneSearchParams.IsClip:
// "" or "all" is no filter, "only" keeps created shorts, "hide" drops them.
func parseClipsFilter(v string) (*bool, error) {
	switch v {
	case "", "all":
		return nil, nil
	case "only":
		b := true
		return &b, nil
	case "hide":
		b := false
		return &b, nil
	default:
		return nil, fmt.Errorf("clips must be all, only or hide")
	}
}

// parseShortsSearchFilter maps the search page's shorts value to filters:
// "" or "all" is no filter, "only"/"hide" keep or drop every short (which
// needs the Shorts length limit), "only_clips"/"hide_clips" keep or drop
// only shorts cut from another scene.
func (h *SceneHandler) parseShortsSearchFilter(v string) (*bool, *data.ShortsFilter, error) {
	switch v {
	case "", "all":
		return nil, nil, nil
	case "only_clips":
		b := true
		return &b, nil, nil
	case "hide_clips":
		b := false
		return &b, nil, nil
	case "only", "hide":
		maxDuration, err := h.ShortsSettings.GetMaxDuration()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get shorts length limit: %w", err)
		}
		return nil, &data.ShortsFilter{Only: v == "only", MaxDuration: maxDuration}, nil
	default:
		return nil, nil, apperrors.NewValidationError("shorts must be all, only, hide, only_clips or hide_clips")
	}
}

// parseShortsSort maps the feed sort to a search sort.
func parseShortsSort(v string) (string, error) {
	switch v {
	case "", "newest":
		return "created_at_desc", nil
	case "random":
		return "random", nil
	default:
		return "", fmt.Errorf("sort must be newest or random")
	}
}

// ListShorts returns the Shorts feed: every scene from 1 second up to the
// admin's length limit.
func (h *ShortsHandler) ListShorts(c *gin.Context) {
	var req request.ShortsFeedRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid query parameters")
		return
	}

	sort, err := parseShortsSort(req.Sort)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	isClip, err := parseClipsFilter(req.Clips)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	maxDuration, err := h.Settings.GetMaxDuration()
	if err != nil {
		response.Error(c, err)
		return
	}

	req.Page, req.Limit = clampPagination(req.Page, req.Limit, 10, h.MaxItemsPerPage)

	var userID uint
	if payload, err := middleware.GetUserFromContext(c); err == nil {
		userID = payload.UserID
	}

	result, err := h.Search.Search(data.SceneSearchParams{
		Page:        req.Page,
		Limit:       req.Limit,
		MinDuration: 1,
		MaxDuration: maxDuration,
		Sort:        sort,
		Seed:        req.Seed,
		IsClip:      isClip,
		UserID:      userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to load shorts")
		return
	}

	resp := gin.H{
		"data":         response.ToShortListItems(result.Scenes),
		"total":        result.Total,
		"page":         req.Page,
		"limit":        req.Limit,
		"max_duration": maxDuration,
	}
	if result.Seed != 0 {
		resp["seed"] = result.Seed
	}
	h.addInteractionMaps(resp, userID, result.Scenes)

	response.OK(c, resp)
}

// addInteractionMaps adds the viewer's likes, ratings and jizz counts, keyed by
// scene ID, the same side maps ListScenes returns.
func (h *ShortsHandler) addInteractionMaps(resp gin.H, userID uint, scenes []data.Scene) {
	if userID == 0 || h.InteractionRepo == nil || len(scenes) == 0 {
		return
	}
	ids := make([]uint, len(scenes))
	for i, s := range scenes {
		ids[i] = s.ID
	}
	if ratings, err := h.InteractionRepo.GetRatingsBySceneIDs(userID, ids); err == nil {
		resp["ratings"] = ratings
	}
	if likes, err := h.InteractionRepo.GetLikesBySceneIDs(userID, ids); err == nil {
		resp["likes"] = likes
	}
	if jizzCounts, err := h.InteractionRepo.GetJizzCountsBySceneIDs(userID, ids); err == nil {
		resp["jizz_counts"] = jizzCounts
	}
}

// GetConfig returns what the player needs to offer "Save short": the feed
// limit, whether this user may create shorts (can_upload), and whether the
// shorts folder can be written to (can_create); plus what the search page's
// Shorts filter starts on (search_default).
func (h *ShortsHandler) GetConfig(c *gin.Context) {
	maxDuration, err := h.Settings.GetMaxDuration()
	if err != nil {
		response.Error(c, err)
		return
	}
	searchDefault, err := h.Settings.GetSearchDefault()
	if err != nil {
		response.Error(c, err)
		return
	}

	canUpload := false
	if payload, err := middleware.GetUserFromContext(c); err == nil && h.RBAC != nil {
		canUpload = h.RBAC.HasPermission(payload.Role, "scenes:upload")
	}

	response.OK(c, gin.H{
		"max_duration":   maxDuration,
		"can_upload":     canUpload,
		"can_create":     h.Settings.CanCreate(),
		"search_default": searchDefault,
	})
}

// ListSceneShorts lists the shorts cut from a scene, by start time, plus any
// still being encoded.
func (h *ShortsHandler) ListSceneShorts(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	page, limit = clampPagination(page, limit, 50, h.MaxItemsPerPage)

	scenes, total, err := h.SceneRepo.ListBySourceScene(uint(id), page, limit)
	if err != nil {
		response.InternalError(c, "Failed to load shorts")
		return
	}

	var pending []core.ShortClipTask
	if h.Creator != nil {
		pending = h.Creator.ListTasks(uint(id))
	}
	if pending == nil {
		pending = []core.ShortClipTask{}
	}

	response.OK(c, gin.H{
		"data":    response.ToShortListItems(scenes),
		"total":   total,
		"page":    page,
		"limit":   limit,
		"pending": pending,
	})
}

// CreateShort queues an encode of the range [start, end] of a scene as a new
// short. It answers 202 with the task ID; progress arrives as short:* events.
func (h *ShortsHandler) CreateShort(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	var req request.CreateShortRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: start and end are required")
		return
	}

	task, err := h.Creator.CreateShort(uint(id), *req.Start, *req.End, req.Title)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"task_id": task.TaskID, "task": task})
}

// GetShortsSettings returns the admin shorts settings.
func (h *ShortsHandler) GetShortsSettings(c *gin.Context) {
	settings, err := h.Settings.GetSettings()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, settings)
}

// UpdateShortsSettings saves the feed limit, the save folder (null or "" for
// the default) and the search default (omitted keeps the stored one).
func (h *ShortsHandler) UpdateShortsSettings(c *gin.Context) {
	var req request.UpdateShortsSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings, err := h.Settings.UpdateSettings(req.MaxDuration, req.SaveDir, req.SearchDefault)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, settings)
}
