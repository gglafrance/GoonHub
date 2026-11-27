package handler

import (
	"strconv"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// maxMarkerLabelSuggestions bounds GET /markers/labels (the label pickers list the whole
// referential).
const maxMarkerLabelSuggestions = 2000

// SetLabelService provides the marker label service (routes /marker-labels and the default
// tags of /markers/label-tags).
func (h *MarkerHandler) SetLabelService(service *core.MarkerLabelService) {
	h.labels = service
}

func labelInput(req request.MarkerLabelRequest) core.MarkerLabelInput {
	return core.MarkerLabelInput{Name: req.Name, Color: req.Color, Description: req.Description}
}

func parseLabelID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("labelID"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "Invalid label ID")
		return 0, false
	}
	return uint(id), true
}

// ListMarkerLabels returns every label with its usage (all users) and default tags
// (GET /marker-labels, marker_labels:manage).
func (h *MarkerHandler) ListMarkerLabels(c *gin.Context) {
	labels, err := h.labels.List()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"data": labels})
}

// CreateMarkerLabel creates a label (POST /marker-labels, body {name, color?, description?}).
// A name used by another label (ignoring case) answers 409 with that label in "existing".
func (h *MarkerHandler) CreateMarkerLabel(c *gin.Context) {
	var req request.MarkerLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	label, err := h.labels.Create(labelInput(req))
	if err != nil {
		if writeEntityConflict(c, err) {
			return
		}
		response.Error(c, err)
		return
	}
	response.Created(c, label)
}

// UpdateMarkerLabel renames, recolors or describes a label (PUT /marker-labels/:labelID,
// body {name?, color?, description?}). A name used by another label answers 409 with that
// label in "existing", to merge into it instead.
func (h *MarkerHandler) UpdateMarkerLabel(c *gin.Context) {
	id, ok := parseLabelID(c)
	if !ok {
		return
	}
	var req request.MarkerLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	label, err := h.labels.Update(id, labelInput(req))
	if err != nil {
		if writeEntityConflict(c, err) {
			return
		}
		response.Error(c, err)
		return
	}
	response.OK(c, label)
}

// DeleteMarkerLabel deletes a label (DELETE /marker-labels/:labelID); its markers stay,
// without label.
func (h *MarkerHandler) DeleteMarkerLabel(c *gin.Context) {
	id, ok := parseLabelID(c)
	if !ok {
		return
	}
	res, err := h.labels.Delete([]uint{id})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, res)
}

// DeleteMarkerLabels deletes several labels (POST /marker-labels/delete, body {ids}).
func (h *MarkerHandler) DeleteMarkerLabels(c *gin.Context) {
	var req request.DeleteMarkerLabelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "ids are required")
		return
	}
	res, err := h.labels.Delete(req.IDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, res)
}

// MergeMarkerLabels merges source labels into a target (POST /marker-labels/merge, body
// {target_id, source_ids, dry_run}).
func (h *MarkerHandler) MergeMarkerLabels(c *gin.Context) {
	var req request.MergeMarkerLabelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "target_id and source_ids are required")
		return
	}
	res, err := h.labels.Merge(req.TargetID, req.SourceIDs, req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, res)
}

// GetMarkerLabelTags returns the default tags of a label (GET /marker-labels/:labelID/tags).
func (h *MarkerHandler) GetMarkerLabelTags(c *gin.Context) {
	id, ok := parseLabelID(c)
	if !ok {
		return
	}
	tags, err := h.labels.GetDefaultTags(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"tags": tags})
}

// SetMarkerLabelTags replaces the default tags of a label (PUT /marker-labels/:labelID/tags,
// body {tag_ids}).
func (h *MarkerHandler) SetMarkerLabelTags(c *gin.Context) {
	id, ok := parseLabelID(c)
	if !ok {
		return
	}
	var req request.SetLabelTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	tags, err := h.labels.SetDefaultTags(id, req.TagIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"tags": tags})
}
