package handler

import (
	"goonhub/internal/streaming"
	"net/http"

	"github.com/gin-gonic/gin"
)

type StreamStatsHandler struct {
	StreamManager *streaming.Manager
}

func NewStreamStatsHandler(streamManager *streaming.Manager) *StreamStatsHandler {
	return &StreamStatsHandler{
		StreamManager: streamManager,
	}
}

func (h *StreamStatsHandler) GetStreamStats(c *gin.Context) {
	stats := h.StreamManager.Stats()

	c.JSON(http.StatusOK, gin.H{
		"global_count":    stats.Stream.GlobalCount,
		"max_global":      stats.Stream.MaxGlobal,
		"max_per_ip":      stats.Stream.MaxPerIP,
		"active_ips":      stats.Stream.ActiveIPs,
		"path_cache_size": stats.CacheSize,
	})
}
