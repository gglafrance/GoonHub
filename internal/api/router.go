package api

import (
	"goonhub"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/infrastructure/logging"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func NewRouter(logger *logging.Logger, cfg *config.Config, videoHandler *handler.VideoHandler) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New() // Empty engine, we add middleware manually
	middleware.Setup(r, logger)

	// Health Check (Unversioned)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "env": cfg.Environment})
	})

	// Register Routes
	RegisterRoutes(r, videoHandler)

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
