package handler

import (
	"strconv"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

// Tabs, sections and custom lists of the profiles (docs/PROFILES.md). Editing needs a signed-in
// user (custom lists need scenes:view, route middleware); browsing a list is checked by the
// service for every viewer.

// SetMyTabs is PUT /profiles/me/tabs {tabs} or {reset: true}: {tabs} normalized.
func (h *ProfileHandler) SetMyTabs(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	var req request.SetProfileTabsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	var tabs data.ProfileTabs
	if !req.Reset {
		if req.Tabs == nil {
			response.BadRequest(c, "tabs is required (or reset)")
			return
		}
		tabs = data.ProfileTabs(req.Tabs)
	}
	out, err := h.profiles.SetTabs(c.Request.Context(), userID, tabs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"tabs": out})
}

// ListMyLists is GET /profiles/me/lists: {lists} with their dependencies.
func (h *ProfileHandler) ListMyLists(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	lists, err := h.profiles.MyLists(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"lists": response.NewCustomListsResponse(lists)})
}

func (h *ProfileHandler) listInput(c *gin.Context) (core.CustomListInput, bool) {
	var req request.CustomListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return core.CustomListInput{}, false
	}
	return core.CustomListInput{Title: req.Title, Filters: req.Filters, SavedSearchUUID: req.SavedSearchUUID, Audience: req.Audience}, true
}

func listIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "invalid list id")
		return 0, false
	}
	return uint(id), true
}

// CreateMyList is POST /profiles/me/lists {title, filters, saved_search_uuid?, audience?}.
func (h *ProfileHandler) CreateMyList(c *gin.Context) {
	owner, ok := h.viewer(c)
	if !ok {
		return
	}
	in, ok := h.listInput(c)
	if !ok {
		return
	}
	list, err := h.profiles.CreateList(c.Request.Context(), owner, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, response.NewCustomListResponse(*list))
}

// UpdateMyList is PUT /profiles/me/lists/:id (same body as the creation).
func (h *ProfileHandler) UpdateMyList(c *gin.Context) {
	owner, ok := h.viewer(c)
	if !ok {
		return
	}
	id, ok := listIDParam(c)
	if !ok {
		return
	}
	in, ok := h.listInput(c)
	if !ok {
		return
	}
	list, err := h.profiles.UpdateList(c.Request.Context(), owner, id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewCustomListResponse(*list))
}

// DeleteMyList is DELETE /profiles/me/lists/:id.
func (h *ProfileHandler) DeleteMyList(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	id, ok := listIDParam(c)
	if !ok {
		return
	}
	if err := h.profiles.DeleteList(c.Request.Context(), userID, id); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// BrowseList is GET /profiles/users/:username/lists/:id?page=&limit=&card_fields= (scenes:view).
func (h *ProfileHandler) BrowseList(c *gin.Context) {
	viewer, ok := h.viewer(c)
	if !ok {
		return
	}
	id, ok := listIDParam(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(profileListDefaultLimit)))
	page, limit = clampPagination(page, limit, profileListDefaultLimit, h.maxItemsPerPage)
	out, err := h.profiles.BrowseList(c.Request.Context(), viewer, c.Param("username"), id, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	list := h.listResponse(c, viewer.UserID, &core.ProfileListPage{Items: out.Items, Total: out.Total, Page: out.Page, Limit: out.Limit})
	response.OK(c, response.CustomListBrowseResponse{
		List:         response.CustomListSummary{ID: out.List.ID, Title: out.List.Title},
		Data:         list.Data,
		Pagination:   list.Pagination,
		CardSidecars: list.CardSidecars,
	})
}
