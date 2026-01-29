package handler

import (
	"fmt"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StudioHandler struct {
	Service       *core.StudioService
	StudioLogoDir string
}

func NewStudioHandler(service *core.StudioService, studioLogoDir string) *StudioHandler {
	return &StudioHandler{
		Service:       service,
		StudioLogoDir: studioLogoDir,
	}
}

func (h *StudioHandler) ListStudios(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	query := c.Query("q")

	studios, total, err := h.Service.List(page, limit, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list studios"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  toStudioListItems(studios),
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func toStudioListItems(studios []data.StudioWithCount) []response.StudioListItem {
	items := make([]response.StudioListItem, len(studios))
	for i, s := range studios {
		items[i] = response.StudioListItem{
			ID:         s.ID,
			UUID:       s.UUID,
			Name:       s.Name,
			ShortName:  s.ShortName,
			Logo:       s.Logo,
			VideoCount: s.VideoCount,
		}
	}
	return items
}

func (h *StudioHandler) GetStudioByUUID(c *gin.Context) {
	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid studio UUID"})
		return
	}

	studio, err := h.Service.GetByUUID(uuidStr)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get studio"})
		return
	}

	c.JSON(http.StatusOK, studio)
}

func (h *StudioHandler) GetStudioVideos(c *gin.Context) {
	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid studio UUID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	studio, err := h.Service.GetByUUID(uuidStr)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get studio"})
		return
	}

	videos, total, err := h.Service.GetStudioVideos(studio.ID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get studio videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  videos,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *StudioHandler) CreateStudio(c *gin.Context) {
	var req request.CreateStudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	input := core.CreateStudioInput{
		Name:        req.Name,
		ShortName:   req.ShortName,
		URL:         req.URL,
		Description: req.Description,
		Rating:      req.Rating,
		Logo:        req.Logo,
		Favicon:     req.Favicon,
		Poster:      req.Poster,
		PornDBID:    req.PornDBID,
		ParentID:    req.ParentID,
		NetworkID:   req.NetworkID,
	}

	studio, err := h.Service.Create(input)
	if err != nil {
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create studio"})
		return
	}

	c.JSON(http.StatusCreated, studio)
}

func (h *StudioHandler) UpdateStudio(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid studio ID"})
		return
	}

	var req request.UpdateStudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	input := core.UpdateStudioInput{
		Name:        req.Name,
		ShortName:   req.ShortName,
		URL:         req.URL,
		Description: req.Description,
		Rating:      req.Rating,
		Logo:        req.Logo,
		Favicon:     req.Favicon,
		Poster:      req.Poster,
		PornDBID:    req.PornDBID,
		ParentID:    req.ParentID,
		NetworkID:   req.NetworkID,
	}

	studio, err := h.Service.Update(uint(id), input)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
			return
		}
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update studio"})
		return
	}

	c.JSON(http.StatusOK, studio)
}

func (h *StudioHandler) DeleteStudio(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid studio ID"})
		return
	}

	if err := h.Service.Delete(uint(id)); err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete studio"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

var allowedLogoExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

func (h *StudioHandler) UploadStudioLogo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid studio ID"})
		return
	}

	file, err := c.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Logo file is required"})
		return
	}

	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size must be less than 10MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedLogoExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image extension. Allowed: jpg, jpeg, png, webp, gif"})
		return
	}

	if err := os.MkdirAll(h.StudioLogoDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create logo directory"})
		return
	}

	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	destPath := filepath.Join(h.StudioLogoDir, filename)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create destination file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save logo"})
		return
	}

	logoURL := fmt.Sprintf("/studio-logos/%s", filename)
	studio, err := h.Service.UpdateLogoURL(uint(id), logoURL)
	if err != nil {
		os.Remove(destPath)
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update studio logo"})
		return
	}

	c.JSON(http.StatusOK, studio)
}

func (h *StudioHandler) GetVideoStudio(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	studio, err := h.Service.GetVideoStudio(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video studio"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": studio})
}

func (h *StudioHandler) SetVideoStudio(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req request.SetVideoStudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	studio, err := h.Service.SetVideoStudio(uint(id), req.StudioID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set video studio"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": studio})
}
