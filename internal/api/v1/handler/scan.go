package handler

import (
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ScanHandler handles HTTP requests for scan operations
type ScanHandler struct {
	scanService *core.ScanService
}

// NewScanHandler creates a new scan handler
func NewScanHandler(scanService *core.ScanService) *ScanHandler {
	return &ScanHandler{
		scanService: scanService,
	}
}

// StartScan initiates a new scan of all storage paths
// POST /api/v1/admin/scan
func (h *ScanHandler) StartScan(c *gin.Context) {
	scan, err := h.scanService.StartScan(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scan)
}

// CancelScan cancels the currently running scan
// POST /api/v1/admin/scan/cancel
func (h *ScanHandler) CancelScan(c *gin.Context) {
	if err := h.scanService.CancelScan(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scan cancelled"})
}

// GetStatus returns the current scan status
// GET /api/v1/admin/scan/status
func (h *ScanHandler) GetStatus(c *gin.Context) {
	status := h.scanService.GetStatus()
	c.JSON(http.StatusOK, status)
}

// GetHistory returns paginated scan history
// GET /api/v1/admin/scan/history
func (h *ScanHandler) GetHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	scans, total, err := h.scanService.GetHistory(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scan history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  scans,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
