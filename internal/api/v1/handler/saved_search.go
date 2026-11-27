package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
)

type SavedSearchHandler struct {
	Service *core.SavedSearchService
}

func NewSavedSearchHandler(service *core.SavedSearchService) *SavedSearchHandler {
	return &SavedSearchHandler{
		Service: service,
	}
}

func (h *SavedSearchHandler) getUserID(c *gin.Context) (uint, bool) {
	user, exists := c.Get("user")
	if !exists {
		return 0, false
	}
	userPayload, ok := user.(*core.UserPayload)
	if !ok {
		return 0, false
	}
	return userPayload.UserID, true
}

func (h *SavedSearchHandler) List(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	searches, err := h.Service.List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list saved searches"})
		return
	}

	if !fileVisibility(c).Size {
		for i := range searches {
			searches[i].Filters = core.WithoutSizeFilters(searches[i].Filters)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"data": response.NewSavedSearchListResponse(searches),
	})
}

func (h *SavedSearchHandler) GetByUUID(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	uuidStr := c.Param("uuid")

	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid saved search UUID"})
		return
	}

	search, err := h.Service.GetByUUID(userID, uuidStr)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
			return
		}
		if apperrors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to access this saved search"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get saved search"})
		return
	}

	c.JSON(http.StatusOK, savedSearchForUser(c, search))
}

// savedSearchForUser returns the saved search as the user may see it: without size bounds
// nor size sort without files:view_size (saved before migration 094).
func savedSearchForUser(c *gin.Context, search *data.SavedSearch) response.SavedSearchResponse {
	if search != nil && !fileVisibility(c).Size {
		cp := *search
		cp.Filters = core.WithoutSizeFilters(cp.Filters)
		search = &cp
	}
	return response.NewSavedSearchResponse(search)
}

// refuseSizeFilters answers 403 when saved search filters use sizes without files:view_size.
func refuseSizeFilters(c *gin.Context, filters data.Filters) bool {
	if fileVisibility(c).Size || !core.SavedSearchUsesSize(filters) {
		return false
	}
	response.Error(c, apperrors.NewForbiddenError("file size filters and sorts need the files:view_size permission"))
	return true
}

func (h *SavedSearchHandler) Create(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.CreateSavedSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	input := core.CreateSavedSearchInput{
		Name:    req.Name,
		Filters: requestFiltersToData(req.Filters),
	}
	if refuseSizeFilters(c, input.Filters) {
		return
	}

	search, err := h.Service.Create(userID, input)
	if err != nil {
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create saved search"})
		return
	}

	c.JSON(http.StatusCreated, savedSearchForUser(c, search))
}

func (h *SavedSearchHandler) Update(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	uuidStr := c.Param("uuid")

	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid saved search UUID"})
		return
	}

	var req request.UpdateSavedSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	input := core.UpdateSavedSearchInput{
		Name: req.Name,
	}
	if req.Filters != nil {
		filters := requestFiltersToData(*req.Filters)
		if refuseSizeFilters(c, filters) {
			return
		}
		input.Filters = &filters
	}

	search, err := h.Service.Update(userID, uuidStr, input)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
			return
		}
		if apperrors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to modify this saved search"})
			return
		}
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update saved search"})
		return
	}

	c.JSON(http.StatusOK, savedSearchForUser(c, search))
}

func (h *SavedSearchHandler) Delete(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	uuidStr := c.Param("uuid")

	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid saved search UUID"})
		return
	}

	if err := h.Service.Delete(userID, uuidStr); err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
			return
		}
		if apperrors.IsForbidden(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this saved search"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete saved search"})
		return
	}

	c.Status(http.StatusNoContent)
}

func requestFiltersToData(f request.SavedSearchFilters) data.Filters {
	return data.Filters{
		Query:          f.Query,
		MatchType:      f.MatchType,
		SelectedTags:   f.SelectedTags,
		SelectedActors: f.SelectedActors,
		Studio:         f.Studio,
		Resolution:     f.Resolution,
		MinDuration:    f.MinDuration,
		MaxDuration:    f.MaxDuration,
		MinSize:        f.MinSize,
		MaxSize:        f.MaxSize,
		HasActors:      f.HasActors,
		HasTags:        f.HasTags,
		HasStudio:      f.HasStudio,
		SourceSites:    core.NormalizeSourceSites(f.SourceSites),
		HasSource:      f.HasSource,
		Watched:        f.Watched,
		Rated:          f.Rated,
		HasMarkers:     f.HasMarkers,
		MarkerLabels:   f.MarkerLabels,
		MinDate:        f.MinDate,
		MaxDate:        f.MaxDate,
		Liked:          f.Liked,
		MinRating:      f.MinRating,
		MaxRating:      f.MaxRating,
		MinJizzCount:   f.MinJizzCount,
		MaxJizzCount:   f.MaxJizzCount,
		Sort:           f.Sort,
	}
}
