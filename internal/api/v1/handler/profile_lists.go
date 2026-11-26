package handler

import (
	"strconv"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

// Full lists of a profile (docs/PROFILES.md, "Lists"): liked, rated and jizz videos, and the
// ratings histogram. The service checks the owner's privacy for every viewer; the scenes are
// redacted with the viewer's file visibility and, with ?card_fields=, enriched like the scene
// lists (the sidecar maps are the viewer's own interactions, the owner's figures are on the
// items).

const profileListDefaultLimit = 24

// profileCardData is what the scene cards of the user's template need.
type profileCardData struct {
	markers     *core.MarkerService
	interaction data.InteractionRepository
	tags        data.TagRepository
	actors      data.ActorRepository
}

// WithCardData gives the list routes what the cards of the user's template need (tags,
// actors, markers, the viewer's ratings / likes / jizz counts).
func (h *ProfileHandler) WithCardData(markerService *core.MarkerService, interactionRepo data.InteractionRepository, tagRepo data.TagRepository, actorRepo data.ActorRepository) *ProfileHandler {
	h.cards = profileCardData{markers: markerService, interaction: interactionRepo, tags: tagRepo, actors: actorRepo}
	return h
}

// GetRatingHistogram is GET /profiles/users/:username/ratings/histogram (scenes:view):
// {buckets: [{rating, count}] (0.5 to 5, empty ones included), total, average}.
func (h *ProfileHandler) GetRatingHistogram(c *gin.Context) {
	viewer, ok := h.viewer(c)
	if !ok {
		return
	}
	hist, err := h.profiles.RatingHistogram(c.Request.Context(), viewer, c.Param("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, hist)
}

// ListRatedScenes is GET /profiles/users/:username/ratings?rating=&sort=&page=&limit=&card_fields=
func (h *ProfileHandler) ListRatedScenes(c *gin.Context) {
	h.list(c, core.ProfileListRated)
}

// ListLikedScenes is GET /profiles/users/:username/likes?sort=&page=&limit=&card_fields=
func (h *ProfileHandler) ListLikedScenes(c *gin.Context) {
	h.list(c, core.ProfileListLiked)
}

// ListJizzScenes is GET /profiles/users/:username/jizz?sort=&page=&limit=&card_fields=
func (h *ProfileHandler) ListJizzScenes(c *gin.Context) {
	h.list(c, core.ProfileListJizz)
}

func (h *ProfileHandler) list(c *gin.Context, list core.ProfileList) {
	viewer, ok := h.viewer(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(profileListDefaultLimit)))
	page, limit = clampPagination(page, limit, profileListDefaultLimit, h.maxItemsPerPage)
	in := core.ProfileListInput{Sort: c.Query("sort"), Page: page, Limit: limit}
	if raw := c.Query("rating"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			response.BadRequest(c, "rating must be a number")
			return
		}
		in.Rating = &v
	}
	out, err := h.profiles.List(c.Request.Context(), viewer, c.Param("username"), list, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, h.listResponse(c, viewer.UserID, out))
}

// listResponse converts a page, with the card fields asked for.
func (h *ProfileHandler) listResponse(c *gin.Context, viewerID uint, page *core.ProfileListPage) response.ProfileListResponse {
	fields := response.ParseCardFields(c.Query("card_fields"))
	vis := fileVisibility(c)
	scenes := make([]data.Scene, len(page.Items))
	sceneIDs := make([]uint, len(page.Items))
	for i, it := range page.Items {
		scenes[i] = it.Scene
		sceneIDs[i] = it.Scene.ID
	}
	items := response.ToSceneListItemsWithFields(scenes, fields, vis)
	out := response.ProfileListResponse{
		Data:       make([]response.ProfileSceneResponse, len(page.Items)),
		Pagination: response.NewPagination(page.Page, page.Limit, page.Total),
		Sort:       page.Sort,
	}
	if fields.HasAny() || fields.Watched {
		if fields.Tags {
			fillCardMarkers(h.cards.markers, items, sceneIDs, viewerID)
		}
		fillCardTagsActors(h.cards.tags, h.cards.actors, items, sceneIDs, fields)
		out.CardSidecars = loadCardSidecars(h.cards.interaction, viewerID, sceneIDs, fields)
	}
	for i, it := range page.Items {
		out.Data[i] = response.NewProfileSceneResponse(it, items[i])
	}
	return out
}
