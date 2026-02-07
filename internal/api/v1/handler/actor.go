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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ActorHandler struct {
	Service         *core.ActorService
	ActorImageDir   string
	MaxItemsPerPage int
}

func NewActorHandler(service *core.ActorService, actorImageDir string, maxItemsPerPage int) *ActorHandler {
	return &ActorHandler{
		Service:         service,
		ActorImageDir:   actorImageDir,
		MaxItemsPerPage: maxItemsPerPage,
	}
}

func (h *ActorHandler) ListActors(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, limit = clampPagination(page, limit, 20, h.MaxItemsPerPage)
	query := c.Query("q")
	sort := c.Query("sort")

	actors, total, err := h.Service.List(page, limit, query, sort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list actors"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  toActorListItems(actors),
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func toActorListItems(actors []data.ActorWithCount) []response.ActorListItem {
	items := make([]response.ActorListItem, len(actors))
	for i, a := range actors {
		aliases := []string(a.Aliases)
		if aliases == nil {
			aliases = []string{}
		}
		items[i] = response.ActorListItem{
			ID:         a.ID,
			UUID:       a.UUID,
			Name:       a.Name,
			Aliases:    aliases,
			ImageURL:   a.ImageURL,
			Gender:     a.Gender,
			SceneCount: a.SceneCount,
		}
	}
	return items
}

func (h *ActorHandler) GetActorByUUID(c *gin.Context) {
	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid actor UUID"})
		return
	}

	actor, err := h.Service.GetByUUID(uuidStr)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get actor"})
		return
	}

	c.JSON(http.StatusOK, actor)
}

func (h *ActorHandler) GetActorScenes(c *gin.Context) {
	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid actor UUID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, limit = clampPagination(page, limit, 20, h.MaxItemsPerPage)

	actor, err := h.Service.GetByUUID(uuidStr)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get actor"})
		return
	}

	scenes, total, err := h.Service.GetActorScenes(actor.ID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get actor scenes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  response.ToSceneListItems(scenes),
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *ActorHandler) CreateActor(c *gin.Context) {
	var req request.CreateActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	input := core.CreateActorInput{
		Name:            req.Name,
		Aliases:         req.Aliases,
		ImageURL:        req.ImageURL,
		Gender:          req.Gender,
		Astrology:       req.Astrology,
		Birthplace:      req.Birthplace,
		Ethnicity:       req.Ethnicity,
		Nationality:     req.Nationality,
		CareerStartYear: req.CareerStartYear,
		CareerEndYear:   req.CareerEndYear,
		HeightCm:        req.HeightCm,
		WeightKg:        req.WeightKg,
		Measurements:    req.Measurements,
		Cupsize:         req.Cupsize,
		HairColor:       req.HairColor,
		EyeColor:        req.EyeColor,
		Tattoos:         req.Tattoos,
		Piercings:       req.Piercings,
		FakeBoobs:       req.FakeBoobs,
		SameSexOnly:     req.SameSexOnly,
	}

	if req.Birthday != nil && *req.Birthday != "" {
		t, err := time.Parse("2006-01-02", *req.Birthday)
		if err == nil {
			input.Birthday = &t
		}
	}
	if req.DateOfDeath != nil && *req.DateOfDeath != "" {
		t, err := time.Parse("2006-01-02", *req.DateOfDeath)
		if err == nil {
			input.DateOfDeath = &t
		}
	}

	actor, err := h.Service.Create(input)
	if err != nil {
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create actor"})
		return
	}

	c.JSON(http.StatusCreated, actor)
}

func (h *ActorHandler) UpdateActor(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid actor ID"})
		return
	}

	var req request.UpdateActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	input := core.UpdateActorInput{
		Name:            req.Name,
		Aliases:         req.Aliases,
		ImageURL:        req.ImageURL,
		Gender:          req.Gender,
		Astrology:       req.Astrology,
		Birthplace:      req.Birthplace,
		Ethnicity:       req.Ethnicity,
		Nationality:     req.Nationality,
		CareerStartYear: req.CareerStartYear,
		CareerEndYear:   req.CareerEndYear,
		HeightCm:        req.HeightCm,
		WeightKg:        req.WeightKg,
		Measurements:    req.Measurements,
		Cupsize:         req.Cupsize,
		HairColor:       req.HairColor,
		EyeColor:        req.EyeColor,
		Tattoos:         req.Tattoos,
		Piercings:       req.Piercings,
		FakeBoobs:       req.FakeBoobs,
		SameSexOnly:     req.SameSexOnly,
	}

	if req.Birthday != nil && *req.Birthday != "" {
		t, err := time.Parse("2006-01-02", *req.Birthday)
		if err == nil {
			input.Birthday = &t
		}
	}
	if req.DateOfDeath != nil && *req.DateOfDeath != "" {
		t, err := time.Parse("2006-01-02", *req.DateOfDeath)
		if err == nil {
			input.DateOfDeath = &t
		}
	}

	actor, err := h.Service.Update(uint(id), input)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
			return
		}
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update actor"})
		return
	}

	c.JSON(http.StatusOK, actor)
}

func (h *ActorHandler) DeleteActor(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid actor ID"})
		return
	}

	if err := h.Service.Delete(uint(id)); err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete actor"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

func (h *ActorHandler) UploadActorImage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid actor ID"})
		return
	}

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image file is required"})
		return
	}

	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size must be less than 10MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image extension. Allowed: jpg, jpeg, png, webp, gif"})
		return
	}

	if err := os.MkdirAll(h.ActorImageDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create image directory"})
		return
	}

	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	destPath := filepath.Join(h.ActorImageDir, filename)

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	imageURL := fmt.Sprintf("/actor-images/%s", filename)
	actor, err := h.Service.UpdateImageURL(uint(id), imageURL)
	if err != nil {
		os.Remove(destPath)
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update actor image"})
		return
	}

	c.JSON(http.StatusOK, actor)
}

func (h *ActorHandler) GetSceneActors(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	actors, err := h.Service.GetSceneActors(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene actors"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": actors})
}

func (h *ActorHandler) SetSceneActors(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req request.SetSceneActorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	actors, err := h.Service.SetSceneActors(uint(id), req.ActorIDs)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set scene actors"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": actors})
}
