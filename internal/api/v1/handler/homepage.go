package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
)

type HomepageHandler struct {
	homepageService *core.HomepageService
}

func NewHomepageHandler(homepageService *core.HomepageService) *HomepageHandler {
	return &HomepageHandler{
		homepageService: homepageService,
	}
}

func (h *HomepageHandler) GetHomepageData(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, apperrors.NewUnauthorizedError("user not authenticated"))
		return
	}

	data, err := h.homepageService.GetHomepageData(userPayload.UserID)
	if err != nil {
		response.Error(c, apperrors.NewInternalError("failed to fetch homepage data", err))
		return
	}

	response.OK(c, response.ToHomepageResponse(data))
}

func (h *HomepageHandler) GetSectionData(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, apperrors.NewUnauthorizedError("user not authenticated"))
		return
	}

	sectionID := c.Param("id")
	if sectionID == "" {
		response.Error(c, apperrors.NewValidationError("section ID is required"))
		return
	}

	data, err := h.homepageService.GetSectionData(userPayload.UserID, sectionID)
	if err != nil {
		// Check if section not found
		if strings.Contains(err.Error(), "section not found") {
			response.Error(c, apperrors.NewNotFoundError("section", sectionID))
			return
		}
		response.Error(c, apperrors.NewInternalError("failed to fetch section data", err))
		return
	}

	response.OK(c, response.ToHomepageSectionDataResponse(data))
}
