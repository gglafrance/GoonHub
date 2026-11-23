package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

const (
	defaultTrashPageSize = 20
	maxTrashPageSize     = 100
)

// trashTotalsResponse holds the totals of the current filter and of the whole trash.
type trashTotalsResponse struct {
	Filtered data.TrashTotals `json:"filtered"`
	All      data.TrashTotals `json:"all"`
}

// trashListResponse is the trash page. data, total, page, limit and retention_days are the
// fields of the former response, kept for older clients.
type trashListResponse struct {
	Data          []core.TrashedScene `json:"data"`
	Total         int64               `json:"total"`
	Page          int                 `json:"page"`
	Limit         int                 `json:"limit"`
	RetentionDays int                 `json:"retention_days"`
	Pagination    response.Pagination `json:"pagination"`
	Totals        trashTotalsResponse `json:"totals"`
}

// parseTrashParams reads the filter and sort of the trash list from the query:
// sort, order, q, storage_path_id ("none" or 0 for no storage path), min_size, max_size
// (bytes), min_duration, max_duration (seconds), trashed_from, trashed_to (YYYY-MM-DD,
// inclusive, or RFC 3339), resolution (4k, 1440p, 1080p, 720p, 480p, 360p), codec, source
// (duplicates, other).
func parseTrashParams(c *gin.Context) (core.TrashListParams, error) {
	var p core.TrashListParams
	p.Sort = c.DefaultQuery("sort", data.TrashSortTrashedAt)
	if !data.IsValidTrashSort(p.Sort) {
		return p, fmt.Errorf("sort must be one of trashed_at, size, duration, title, purge_at")
	}
	switch order := c.Query("order"); order {
	case "", "desc":
		p.Desc = true
	case "asc":
	default:
		return p, fmt.Errorf("order must be asc or desc")
	}
	if order := c.Query("order"); order == "" && p.Sort == data.TrashSortTitle {
		p.Desc = false
	}

	f := &p.Filter
	f.Query = strings.TrimSpace(c.Query("q"))
	if raw := c.Query("storage_path_id"); raw != "" {
		var id uint
		if raw != "none" {
			n, err := strconv.ParseUint(raw, 10, 32)
			if err != nil {
				return p, fmt.Errorf("invalid storage_path_id")
			}
			id = uint(n)
		}
		f.StoragePathID = &id
	}
	var err error
	if f.MinSize, err = queryInt64(c, "min_size"); err != nil {
		return p, err
	}
	if f.MaxSize, err = queryInt64(c, "max_size"); err != nil {
		return p, err
	}
	minDuration, err := queryInt64(c, "min_duration")
	if err != nil {
		return p, err
	}
	maxDuration, err := queryInt64(c, "max_duration")
	if err != nil {
		return p, err
	}
	f.MinDuration, f.MaxDuration = int(minDuration), int(maxDuration)
	if raw := c.Query("trashed_from"); raw != "" {
		t, _, err := parseTrashDate(raw)
		if err != nil {
			return p, fmt.Errorf("invalid trashed_from")
		}
		f.TrashedFrom = &t
	}
	if raw := c.Query("trashed_to"); raw != "" {
		t, dateOnly, err := parseTrashDate(raw)
		if err != nil {
			return p, fmt.Errorf("invalid trashed_to")
		}
		// Inclusive: a date covers the whole day
		if dateOnly {
			t = t.AddDate(0, 0, 1)
		} else {
			t = t.Add(time.Nanosecond)
		}
		f.TrashedBefore = &t
	}
	if res := c.Query("resolution"); res != "" {
		heights, ok := resolutionToHeight[res]
		if !ok {
			return p, fmt.Errorf("resolution must be one of 4k, 1440p, 1080p, 720p, 480p, 360p")
		}
		f.MinHeight, f.MaxHeight = heights[0], heights[1]
	}
	f.Codec = strings.TrimSpace(c.Query("codec"))
	switch source := c.Query("source"); source {
	case "", data.TrashSourceDuplicates, data.TrashSourceOther:
		f.Source = source
	default:
		return p, fmt.Errorf("source must be duplicates or other")
	}
	return p, nil
}

// queryInt64 reads a non-negative integer query param (0 when absent).
func queryInt64(c *gin.Context, key string) (int64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}

// parseTrashDate parses YYYY-MM-DD (UTC day, dateOnly) or RFC 3339.
func parseTrashDate(raw string) (time.Time, bool, error) {
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, true, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	return t, false, err
}

// ListTrash returns a page of the trash with filters, sort and totals.
// GET /admin/trash?page=&limit=&sort=&order=&q=&storage_path_id=&min_size=&max_size=
// &min_duration=&max_duration=&trashed_from=&trashed_to=&resolution=&codec=&source=
func (h *AdminHandler) ListTrash(c *gin.Context) {
	params, err := parseTrashParams(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	params.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	params.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultTrashPageSize)))
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > maxTrashPageSize {
		params.Limit = defaultTrashPageSize
	}

	list, err := h.SceneService.ListTrash(params)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, trashListResponse{
		Data:          list.Items,
		Total:         list.Filtered.Count,
		Page:          list.Page,
		Limit:         list.Limit,
		RetentionDays: list.RetentionDays,
		Pagination:    response.NewPagination(list.Page, list.Limit, list.Filtered.Count),
		Totals:        trashTotalsResponse{Filtered: list.Filtered, All: list.All},
	})
}

// ListTrashIDs returns the IDs of the trashed scenes matching the filter, in the list's sort,
// for "select all": at most the bulk actions' maximum, with their sizes and the total.
// GET /admin/trash/ids?<filters> -> {"ids": [...], "items": [{"id", "size"}], "total": n, "limit": 500}
func (h *AdminHandler) ListTrashIDs(c *gin.Context) {
	params, err := parseTrashParams(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, total, err := h.SceneService.TrashIDs(params, core.MaxTrashBulk)
	if err != nil {
		response.Error(c, err)
		return
	}
	ids := make([]uint, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	response.OK(c, gin.H{"ids": ids, "items": items, "total": total, "limit": core.MaxTrashBulk})
}

// TrashFacets returns the filter options of the whole trash (codecs, storage paths, number of
// scenes from the duplicates).
// GET /admin/trash/facets
func (h *AdminHandler) TrashFacets(c *gin.Context) {
	facets, err := h.SceneService.TrashFacets()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, facets)
}

type trashBulkRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required,min=1"`
}

func bindTrashBulk(c *gin.Context) ([]uint, bool) {
	var req trashBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "scene_ids is required")
		return nil, false
	}
	if len(req.SceneIDs) > core.MaxTrashBulk {
		response.BadRequest(c, fmt.Sprintf("too many scenes (max %d)", core.MaxTrashBulk))
		return nil, false
	}
	return req.SceneIDs, true
}

// RestoreTrashBulk restores several scenes from the trash, reporting each failure.
// POST /admin/trash/restore-bulk {"scene_ids": [...]}
func (h *AdminHandler) RestoreTrashBulk(c *gin.Context) {
	ids, ok := bindTrashBulk(c)
	if !ok {
		return
	}
	response.OK(c, h.SceneService.RestoreScenesFromTrash(ids))
}

// DeleteTrashBulk permanently deletes several trashed scenes (files included, except on
// read-only storage paths) through the single permanent delete, reporting each failure.
// Scenes not in the trash are refused.
// POST /admin/trash/delete-bulk {"scene_ids": [...]}
func (h *AdminHandler) DeleteTrashBulk(c *gin.Context) {
	ids, ok := bindTrashBulk(c)
	if !ok {
		return
	}
	response.OK(c, h.SceneService.DeleteTrashedScenes(ids))
}
