package handler

import (
	"fmt"
	"strconv"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

// Copy triage (Duplicates page > Copies): the files the library scan recorded as copies of a
// scene. Admin only (registered under /admin/duplicates/copies).

// parseSceneCopyFilter reads storage_path_id, q, min_size and max_size (bytes).
func parseSceneCopyFilter(c *gin.Context) (data.SceneCopyFilter, error) {
	var f data.SceneCopyFilter
	if raw := c.Query("storage_path_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || id == 0 {
			return f, fmt.Errorf("invalid storage_path_id")
		}
		v := uint(id)
		f.StoragePathID = &v
	}
	f.Search = c.Query("q")
	for _, b := range []struct {
		key string
		dst *int64
	}{{"min_size", &f.MinSize}, {"max_size", &f.MaxSize}} {
		if raw := c.Query(b.key); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || v < 0 {
				return f, fmt.Errorf("invalid %s", b.key)
			}
			*b.dst = v
		}
	}
	if f.MinSize > 0 && f.MaxSize > 0 && f.MinSize > f.MaxSize {
		return f, fmt.Errorf("min_size is larger than max_size")
	}
	return f, nil
}

func parseCopyID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "Invalid copy ID")
		return 0, false
	}
	return uint(id), true
}

// ListCopies returns a page of the scenes with copies, their copies and the totals.
// GET /api/v1/admin/duplicates/copies?page&limit&storage_path_id&q&min_size&max_size&sort_by
func (h *ScanHandler) ListCopies(c *gin.Context) {
	f, err := parseSceneCopyFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, limit = clampPagination(page, limit, 20, 100)
	sortBy := c.DefaultQuery("sort_by", data.SceneCopySortNewest)
	switch sortBy {
	case data.SceneCopySortNewest, data.SceneCopySortSize, data.SceneCopySortCopies:
	default:
		response.BadRequest(c, "sort_by must be one of newest, size, copies")
		return
	}
	list, err := h.copyService.List(data.SceneCopyQuery{SceneCopyFilter: f, Sort: sortBy, Page: page, Limit: limit})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{
		"data":         list.Scenes,
		"pagination":   response.NewPagination(page, limit, list.Totals.Scenes),
		"totals":       list.Totals,
		"last_scan_at": list.LastScanAt,
	})
}

// CopyIDs returns the IDs of the copies matching the filter, for "select all": at most the
// bulk maximum, with the total.
// GET /api/v1/admin/duplicates/copies/ids -> {"ids": [...], "total": n, "limit": 500}
func (h *ScanHandler) CopyIDs(c *gin.Context) {
	f, err := parseSceneCopyFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ids, total, err := h.copyService.SelectableIDs(f, core.MaxBulkSceneCopies)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"ids": ids, "total": total, "limit": core.MaxBulkSceneCopies})
}

// MakeCopyMain makes a copy the scene's main file (the old main file becomes a copy).
// POST /api/v1/admin/duplicates/copies/:id/make-main
func (h *ScanHandler) MakeCopyMain(c *gin.Context) {
	id, ok := parseCopyID(c)
	if !ok {
		return
	}
	if err := h.copyService.MakeMain(id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "Copy is now the main file"})
}

// SeparateCopy turns a copy into its own scene.
// POST /api/v1/admin/duplicates/copies/:id/separate -> {"scene_id": n}
func (h *ScanHandler) SeparateCopy(c *gin.Context) {
	id, ok := parseCopyID(c)
	if !ok {
		return
	}
	scene, err := h.copyService.Separate(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"scene_id": scene.ID})
}

// ForgetCopy removes a copy row; the file stays on disk.
// POST /api/v1/admin/duplicates/copies/:id/forget
func (h *ScanHandler) ForgetCopy(c *gin.Context) {
	id, ok := parseCopyID(c)
	if !ok {
		return
	}
	if err := h.copyService.Forget(id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "Copy forgotten"})
}

// deleteCopyFilesRequest must carry confirm=true: deleting a file cannot be undone.
type deleteCopyFilesRequest struct {
	Confirm bool `json:"confirm"`
}

// DeleteCopyFile permanently deletes a copy's file from disk (with the safety checks of
// SceneCopyService.DeleteFile), then its row.
// POST /api/v1/admin/duplicates/copies/:id/delete-file {"confirm": true} -> {"freed_bytes": n}
func (h *ScanHandler) DeleteCopyFile(c *gin.Context) {
	id, ok := parseCopyID(c)
	if !ok {
		return
	}
	var req deleteCopyFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		response.BadRequest(c, "confirm must be true: the file is deleted from disk")
		return
	}
	freed, err := h.copyService.DeleteFile(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"freed_bytes": freed})
}

type copyIDsRequest struct {
	CopyIDs []uint `json:"copy_ids" binding:"required"`
	Confirm bool   `json:"confirm"`
}

func bindCopyIDs(c *gin.Context) (*copyIDsRequest, bool) {
	var req copyIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.CopyIDs) == 0 {
		response.BadRequest(c, "copy_ids is required")
		return nil, false
	}
	if len(req.CopyIDs) > core.MaxBulkSceneCopies {
		response.BadRequest(c, fmt.Sprintf("too many copies (max %d)", core.MaxBulkSceneCopies))
		return nil, false
	}
	return &req, true
}

// ForgetCopiesBulk removes the rows of the given copies; the files stay on disk.
// POST /api/v1/admin/duplicates/copies/forget-bulk {"copy_ids": [...]}
func (h *ScanHandler) ForgetCopiesBulk(c *gin.Context) {
	req, ok := bindCopyIDs(c)
	if !ok {
		return
	}
	response.OK(c, h.copyService.ForgetBulk(req.CopyIDs))
}

// DeleteCopyFilesBulk permanently deletes the files of the given copies, each with the safety
// checks of the single action; per-copy results.
// POST /api/v1/admin/duplicates/copies/delete-files-bulk {"copy_ids": [...], "confirm": true}
func (h *ScanHandler) DeleteCopyFilesBulk(c *gin.Context) {
	req, ok := bindCopyIDs(c)
	if !ok {
		return
	}
	if !req.Confirm {
		response.BadRequest(c, "confirm must be true: the files are deleted from disk")
		return
	}
	response.OK(c, h.copyService.DeleteFilesBulk(req.CopyIDs))
}
