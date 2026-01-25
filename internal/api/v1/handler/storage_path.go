package handler

import (
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type StoragePathHandler struct {
	Service *core.StoragePathService
}

func NewStoragePathHandler(service *core.StoragePathService) *StoragePathHandler {
	return &StoragePathHandler{
		Service: service,
	}
}

func (h *StoragePathHandler) List(c *gin.Context) {
	paths, err := h.Service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list storage paths"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"storage_paths": paths})
}

func (h *StoragePathHandler) Create(c *gin.Context) {
	var req request.CreateStoragePathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	storagePath, err := h.Service.Create(req.Name, req.Path, req.IsDefault)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, storagePath)
}

func (h *StoragePathHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid storage path ID"})
		return
	}

	var req request.UpdateStoragePathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	storagePath, err := h.Service.Update(uint(id), req.Name, req.Path, req.IsDefault)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, storagePath)
}

func (h *StoragePathHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid storage path ID"})
		return
	}

	if err := h.Service.Delete(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Storage path deleted successfully"})
}

func (h *StoragePathHandler) ValidatePath(c *gin.Context) {
	var req request.ValidatePathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.Service.ValidatePath(req.Path); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid":   false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Path is valid and accessible",
	})
}
