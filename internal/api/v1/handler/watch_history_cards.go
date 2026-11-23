package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

// WithCardData gives the history handler what the scene cards of the user's template need
// (tags, actors, markers, ratings, likes, jizz counts), like the scene lists. Without it the
// history scenes carry the default card fields only.
func (h *WatchHistoryHandler) WithCardData(markerService *core.MarkerService, interactionRepo data.InteractionRepository, tagRepo data.TagRepository, actorRepo data.ActorRepository) *WatchHistoryHandler {
	h.MarkerService = markerService
	h.InteractionRepo = interactionRepo
	h.TagRepo = tagRepo
	h.ActorRepo = actorRepo
	return h
}

// historyEntriesResponse builds the history response. With ?card_fields= (the fields of the
// user's card template, as on GET /scenes) every scene is enriched like a scene list item and
// the user's ratings / likes / jizz counts / watched maps are added next to the entries.
func (h *WatchHistoryHandler) historyEntriesResponse(c *gin.Context, entries []core.WatchHistoryEntry, userID uint) gin.H {
	fields := response.ParseCardFields(c.Query("card_fields"))
	if !fields.HasAny() {
		return gin.H{"entries": response.ToWatchHistoryEntriesResponse(entries)}
	}

	// one card item per scene, even when the scene was watched several times
	var scenes []data.Scene
	index := map[uint]int{}
	for _, e := range entries {
		if e.Scene == nil {
			continue
		}
		if _, seen := index[e.Scene.ID]; seen {
			continue
		}
		index[e.Scene.ID] = len(scenes)
		scenes = append(scenes, *e.Scene)
	}
	items := response.ToSceneListItemsWithFields(scenes, fields)
	sceneIDs := make([]uint, len(scenes))
	for i, s := range scenes {
		sceneIDs[i] = s.ID
	}
	if fields.Tags {
		fillCardMarkers(h.MarkerService, items, sceneIDs, userID)
	}
	fillCardTagsActors(h.TagRepo, h.ActorRepo, items, sceneIDs, fields)

	out := make([]response.WatchHistoryEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = response.WatchHistoryEntryResponse{Watch: e.Watch}
		if e.Scene != nil {
			item := items[index[e.Scene.ID]]
			out[i].Scene = &item
		}
	}
	resp := gin.H{"entries": out}
	addCardSidecars(resp, loadCardSidecars(h.InteractionRepo, userID, sceneIDs, fields))
	return resp
}
