package api

import (
	"fmt"
	"goonhub"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func NewRouter(logger *logging.Logger, cfg *config.Config, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, jobHandler *handler.JobHandler, sseHandler *handler.SSEHandler, tagHandler *handler.TagHandler, interactionHandler *handler.InteractionHandler, searchHandler *handler.SearchHandler, watchHistoryHandler *handler.WatchHistoryHandler, storagePathHandler *handler.StoragePathHandler, scanHandler *handler.ScanHandler, authService *core.AuthService, rbacService *core.RBACService, rateLimiter *middleware.IPRateLimiter) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New() // Empty engine, we add middleware manually
	middleware.Setup(r, logger, cfg.Server.AllowedOrigins)

	// Health Check (Unversioned)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "env": cfg.Environment})
	})

	// Serve Thumbnails (using configured thumbnail directory)
	r.GET("/thumbnails/:id", func(c *gin.Context) {
		id := c.Param("id")
		size := c.DefaultQuery("size", "sm")
		if size != "sm" && size != "lg" {
			size = "sm"
		}
		path := filepath.Join(cfg.Processing.ThumbnailDir, fmt.Sprintf("%s_thumb_%s.webp", id, size))
		c.Header("Content-Type", "image/webp")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve Sprite Sheets (using configured sprite directory)
	r.GET("/sprites/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		path := filepath.Join(cfg.Processing.SpriteDir, filename)
		c.Header("Content-Type", "image/webp")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve VTT Files (using configured VTT directory)
	r.GET("/vtt/:videoId", func(c *gin.Context) {
		videoId := c.Param("videoId")
		path := filepath.Join(cfg.Processing.VttDir, fmt.Sprintf("%s_thumbnails.vtt", videoId))
		c.Header("Content-Type", "text/vtt")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Register Routes
	RegisterRoutes(r, videoHandler, authHandler, settingsHandler, adminHandler, jobHandler, sseHandler, tagHandler, interactionHandler, searchHandler, watchHistoryHandler, storagePathHandler, scanHandler, authService, rbacService, logger, rateLimiter)

	// Serve Frontend (SPA Fallback)
	fsys, _ := fs.Sub(goonhub.WebDist, "web/dist")

	// Helper to serve a file from the embedded filesystem
	serveFile := func(c *gin.Context, filePath string) bool {
		f, err := fsys.Open(filePath)
		if err != nil {
			return false
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			return false
		}

		content, err := io.ReadAll(f)
		if err != nil {
			return false
		}

		// Determine content type from extension
		contentType := "application/octet-stream"
		switch filepath.Ext(filePath) {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js", ".mjs":
			contentType = "application/javascript"
		case ".json":
			contentType = "application/json"
		case ".svg":
			contentType = "image/svg+xml"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		case ".ico":
			contentType = "image/x-icon"
		case ".woff":
			contentType = "font/woff"
		case ".woff2":
			contentType = "font/woff2"
		case ".ttf":
			contentType = "font/ttf"
		case ".txt":
			contentType = "text/plain; charset=utf-8"
		}

		c.Data(http.StatusOK, contentType, content)
		return true
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// If path starts with /api, return 404
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Clean the path
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to serve the exact file
		if serveFile(c, path) {
			return
		}

		// For SPA routes (no extension), try path/index.html
		if filepath.Ext(path) == "" {
			if serveFile(c, path+"/index.html") {
				return
			}
		}

		// Fallback to index.html for SPA routing
		serveFile(c, "index.html")
	})

	return r
}
