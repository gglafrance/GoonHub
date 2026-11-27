package handler

import (
	"net/http"
	"strconv"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// strictQuery tells whether a create/update request asks for the duplicate checks
// (?strict=true): a name or alias already used by another entity answers 409.
func strictQuery(c *gin.Context) bool {
	strict, _ := strconv.ParseBool(c.Query("strict"))
	return strict
}

// writeEntityConflict answers 409 with the entity a name or alias collides with, and tells
// whether err was such a conflict.
func writeEntityConflict(c *gin.Context, err error) bool {
	conflict, ok := apperrors.AsEntityConflict(err)
	if !ok {
		return false
	}
	c.JSON(http.StatusConflict, gin.H{
		"error":    conflict.Error(),
		"code":     conflict.Code(),
		"field":    conflict.Field,
		"value":    conflict.Value,
		"existing": conflict.Existing,
	})
	return true
}

// resolveResponse is the answer of GET /actors/resolve and GET /studios/resolve.
func resolveResponse(entity any, matchedAlias string, candidates int) gin.H {
	matched := "name"
	if matchedAlias != "" {
		matched = "alias"
	}
	return gin.H{
		"data":          entity,
		"matched":       matched,
		"matched_alias": matchedAlias,
		"candidates":    candidates,
	}
}

// ResolveActor returns the actor whose name or alias is exactly ?name= (case-insensitive).
func (h *ActorHandler) ResolveActor(c *gin.Context) {
	res, err := h.Service.Resolve(c.Query("name"))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
			return
		}
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, resolveResponse(res.Actor, res.MatchedAlias, res.Candidates))
}

// mergeActorIDs reads the target (URL) and source actors of a merge request.
func (h *ActorHandler) mergeActorIDs(c *gin.Context, sourceUUID string) (targetID, sourceID uint, ok bool) {
	if _, err := uuid.Parse(c.Param("uuid")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid actor UUID"})
		return 0, 0, false
	}
	if _, err := uuid.Parse(sourceUUID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source actor UUID"})
		return 0, 0, false
	}
	target, err := h.Service.GetByUUID(c.Param("uuid"))
	if err != nil {
		writeActorNotFoundOr(c, err)
		return 0, 0, false
	}
	source, err := h.Service.GetByUUID(sourceUUID)
	if err != nil {
		writeActorNotFoundOr(c, err)
		return 0, 0, false
	}
	return target.ID, source.ID, true
}

func writeActorNotFoundOr(c *gin.Context, err error) {
	if apperrors.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
		return
	}
	response.Error(c, err)
}

func actorMergeResponse(res *core.ActorMergeResult) gin.H {
	return gin.H{
		"target":            res.Target,
		"source":            res.Source,
		"scenes":            res.Scenes,
		"shared_scenes":     res.SharedScenes,
		"ratings":           res.Ratings,
		"likes":             res.Likes,
		"aliases_added":     nonNilStrings(res.AliasesAdded),
		"image_from_source": res.ImageFromSource,
	}
}

// MergeActorPreview describes what merging ?source_uuid= into the actor of the URL would move.
func (h *ActorHandler) MergeActorPreview(c *gin.Context) {
	targetID, sourceID, ok := h.mergeActorIDs(c, c.Query("source_uuid"))
	if !ok {
		return
	}
	res, err := h.Service.MergePreview(targetID, sourceID)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, actorMergeResponse(res))
}

// MergeActor merges the actor source_uuid (deleted) into the actor of the URL (kept).
func (h *ActorHandler) MergeActor(c *gin.Context) {
	var req request.MergeActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_uuid is required"})
		return
	}
	targetID, sourceID, ok := h.mergeActorIDs(c, req.SourceUUID)
	if !ok {
		return
	}
	res, err := h.Service.Merge(targetID, sourceID)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, actorMergeResponse(res))
}

// ResolveStudio returns the studio whose name or alias is exactly ?name= (case-insensitive).
func (h *StudioHandler) ResolveStudio(c *gin.Context) {
	res, err := h.Service.Resolve(c.Query("name"))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
			return
		}
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, resolveResponse(res.Studio, res.MatchedAlias, res.Candidates))
}

// mergeStudioIDs reads the target (URL) and source studios of a merge request.
func (h *StudioHandler) mergeStudioIDs(c *gin.Context, sourceUUID string) (targetID, sourceID uint, ok bool) {
	if _, err := uuid.Parse(c.Param("uuid")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid studio UUID"})
		return 0, 0, false
	}
	if _, err := uuid.Parse(sourceUUID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source studio UUID"})
		return 0, 0, false
	}
	target, err := h.Service.GetByUUID(c.Param("uuid"))
	if err != nil {
		writeStudioNotFoundOr(c, err)
		return 0, 0, false
	}
	source, err := h.Service.GetByUUID(sourceUUID)
	if err != nil {
		writeStudioNotFoundOr(c, err)
		return 0, 0, false
	}
	return target.ID, source.ID, true
}

func writeStudioNotFoundOr(c *gin.Context, err error) {
	if apperrors.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
		return
	}
	response.Error(c, err)
}

func studioMergeResponse(res *core.StudioMergeResult) gin.H {
	return gin.H{
		"target":           res.Target,
		"source":           res.Source,
		"scenes":           res.Scenes,
		"child_studios":    res.ChildStudios,
		"ratings":          res.Ratings,
		"likes":            res.Likes,
		"aliases_added":    nonNilStrings(res.AliasesAdded),
		"logo_from_source": res.LogoFromSource,
	}
}

// MergeStudioPreview describes what merging ?source_uuid= into the studio of the URL would move.
func (h *StudioHandler) MergeStudioPreview(c *gin.Context) {
	targetID, sourceID, ok := h.mergeStudioIDs(c, c.Query("source_uuid"))
	if !ok {
		return
	}
	res, err := h.Service.MergePreview(targetID, sourceID)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, studioMergeResponse(res))
}

// MergeStudio merges the studio source_uuid (deleted) into the studio of the URL (kept).
func (h *StudioHandler) MergeStudio(c *gin.Context) {
	var req request.MergeStudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_uuid is required"})
		return
	}
	targetID, sourceID, ok := h.mergeStudioIDs(c, req.SourceUUID)
	if !ok {
		return
	}
	res, err := h.Service.Merge(targetID, sourceID)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, studioMergeResponse(res))
}

func toStudioListItems(studios []data.StudioWithCount, query string) []response.StudioListItem {
	items := make([]response.StudioListItem, len(studios))
	for i, s := range studios {
		aliases := nonNilStrings(s.Aliases)
		items[i] = response.StudioListItem{
			ID:           s.ID,
			UUID:         s.UUID,
			Name:         s.Name,
			Aliases:      aliases,
			ShortName:    s.ShortName,
			Logo:         s.Logo,
			SceneCount:   s.SceneCount,
			MatchedAlias: core.MatchedAlias(s.Name, aliases, query),
		}
	}
	return items
}

func toActorListItems(actors []data.ActorWithCount, query string) []response.ActorListItem {
	items := make([]response.ActorListItem, len(actors))
	for i, a := range actors {
		aliases := nonNilStrings(a.Aliases)
		items[i] = response.ActorListItem{
			ID:           a.ID,
			UUID:         a.UUID,
			Name:         a.Name,
			Aliases:      aliases,
			ImageURL:     a.ImageURL,
			Gender:       a.Gender,
			SceneCount:   a.SceneCount,
			MatchedAlias: core.MatchedAlias(a.Name, aliases, query),
		}
	}
	return items
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
