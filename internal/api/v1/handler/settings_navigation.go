package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
)

// maxNavLayoutBodyBytes caps the body of PUT /settings/navigation (a layout is a few KB).
const maxNavLayoutBodyBytes = 64 << 10

// GetNavLayout answers the header layout of the user (GET /settings/navigation): the stored one
// or the default one, without the items the user lacks the permission for.
func (h *SettingsHandler) GetNavLayout(c *gin.Context) {
	view, err := h.NavLayout.Get(c.Request.Context(), middleware.GetAccess(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}

// UpdateNavLayout validates and stores the header layout of the user
// (PUT /settings/navigation, body {layout}). Items the user lacks the permission for are left
// out of the stored layout; the answer is the stored layout.
func (h *SettingsHandler) UpdateNavLayout(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxNavLayoutBodyBytes)
	var req request.UpdateNavLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request body"))
		return
	}
	view, err := h.NavLayout.Save(c.Request.Context(), middleware.GetAccess(c), *req.Layout)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}

// ResetNavLayout drops the stored header layout of the user (DELETE /settings/navigation) and
// answers the default layout.
func (h *SettingsHandler) ResetNavLayout(c *gin.Context) {
	view, err := h.NavLayout.Reset(c.Request.Context(), middleware.GetAccess(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}
