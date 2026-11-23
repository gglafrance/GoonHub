package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type WatchHistoryHandler struct {
	Service        *core.WatchHistoryService
	HeatmapService *core.ReplayHeatmapService
	// card data of the history scenes (see WithCardData)
	MarkerService   *core.MarkerService
	InteractionRepo data.InteractionRepository
	TagRepo         data.TagRepository
	ActorRepo       data.ActorRepository
}

func NewWatchHistoryHandler(service *core.WatchHistoryService, heatmapService *core.ReplayHeatmapService) *WatchHistoryHandler {
	return &WatchHistoryHandler{Service: service, HeatmapService: heatmapService}
}

func (h *WatchHistoryHandler) RecordWatch(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req request.RecordWatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.Service.RecordWatch(payload.UserID, uint(sceneID), req.Duration, req.Position, req.Completed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record watch"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WatchHistoryHandler) GetResumePosition(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	position, err := h.Service.GetResumePosition(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get resume position"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"position": position})
}

func (h *WatchHistoryHandler) GetSceneHistory(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	const maxLimit = 100
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	watches, err := h.Service.GetSceneHistory(payload.UserID, uint(sceneID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"watches": watches})
}

func (h *WatchHistoryHandler) GetUserHistory(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	const maxLimit = 100
	page := 1
	limit := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if parsed, err := strconv.Atoi(pageStr); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	entries, total, err := h.Service.GetUserHistory(payload.UserID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": response.ToWatchHistoryEntriesResponse(entries),
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

func (h *WatchHistoryHandler) GetUserHistoryByDateRange(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	limit := 2000
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 5000 {
				limit = 5000
			}
		}
	}

	// Custom date range takes precedence over range param
	sinceStr := c.Query("since")
	untilStr := c.Query("until")
	if sinceStr != "" && untilStr != "" {
		sinceTime, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'since' date format, expected YYYY-MM-DD"})
			return
		}
		untilTime, err := time.Parse("2006-01-02", untilStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'until' date format, expected YYYY-MM-DD"})
			return
		}
		// Set until to end of day
		untilTime = untilTime.Add(24*time.Hour - time.Nanosecond)

		entries, err := h.Service.GetUserHistoryByTimeRange(payload.UserID, sinceTime, untilTime, limit)
		if err != nil {
			response.InternalError(c, "Failed to get history")
			return
		}

		response.OK(c, h.historyEntriesResponse(c, entries, payload.UserID))
		return
	}

	rangeDays := 30
	if rangeStr := c.Query("range"); rangeStr != "" {
		if parsed, err := strconv.Atoi(rangeStr); err == nil && parsed >= 0 {
			rangeDays = parsed
		}
	}

	entries, err := h.Service.GetUserHistoryByDateRange(payload.UserID, rangeDays, limit)
	if err != nil {
		response.InternalError(c, "Failed to get history")
		return
	}

	response.OK(c, h.historyEntriesResponse(c, entries, payload.UserID))
}

func (h *WatchHistoryHandler) GetDailyActivity(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	rangeDays := 30
	if rangeStr := c.Query("range"); rangeStr != "" {
		if parsed, err := strconv.Atoi(rangeStr); err == nil && parsed >= 0 {
			rangeDays = parsed
		}
	}

	counts, err := h.Service.GetDailyActivity(payload.UserID, rangeDays)
	if err != nil {
		response.InternalError(c, "Failed to get activity data")
		return
	}

	response.OK(c, gin.H{
		"counts": counts,
	})
}

// RecordReplayHeatmap adds watched ranges of a scene to the current user's "most replayed"
// histogram and returns the updated histogram.
func (h *WatchHistoryHandler) RecordReplayHeatmap(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	var req request.RecordReplayHeatmapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	ranges := make([]core.WatchedRange, len(req.Ranges))
	for i, r := range req.Ranges {
		ranges[i] = core.WatchedRange{Start: r[0], End: r[1]}
	}

	result, err := h.HeatmapService.Record(payload.UserID, uint(sceneID), ranges, req.Duration)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, result)
}

// GetReplayHeatmap returns the current user's "most replayed" histogram of a scene.
func (h *WatchHistoryHandler) GetReplayHeatmap(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	heatmap, err := h.HeatmapService.Get(payload.UserID, uint(sceneID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, heatmap)
}
