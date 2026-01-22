package api

import (
	"fmt"
	"goonhub"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func NewRouter(logger *logging.Logger, cfg *config.Config, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, authService *core.AuthService) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New() // Empty engine, we add middleware manually
	middleware.Setup(r, logger)

	// Health Check (Unversioned)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "env": cfg.Environment})
	})

	// Serve Thumbnails
	r.GET("/thumbnails/:id", func(c *gin.Context) {
		id := c.Param("id")
		path := fmt.Sprintf("./data/thumbnails/%s_thumb.webp", id)
		c.Header("Content-Type", "image/webp")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve Frames
	r.GET("/frames/:videoId/:frameName", func(c *gin.Context) {
		videoId := c.Param("videoId")
		frameName := c.Param("frameName")
		path := fmt.Sprintf("./data/frames/%s/%s", videoId, frameName)
		c.Header("Content-Type", "image/webp")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Register Routes
	RegisterRoutes(r, videoHandler, authHandler, authService)

	// Serve Frontend (SPA Fallback)
	// We use a custom middleware/handler for this
	fsys, _ := fs.Sub(goonhub.WebDist, "web/dist")
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// If path starts with /api, return 404
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Check if file exists in fs
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}
		if path == "" {
			path = "index.html"
		}

		f, err := fsys.Open(path)
		if err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			if !stat.IsDir() {
				c.FileFromFS(path, http.FS(fsys))
				return
			}
		}

		// Fallback to index.html for SPA
		c.FileFromFS("index.html", http.FS(fsys))
	})

	return r
}
