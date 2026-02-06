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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func NewRouter(logger *logging.Logger, cfg *config.Config, sceneHandler *handler.SceneHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, jobHandler *handler.JobHandler, poolConfigHandler *handler.PoolConfigHandler, processingConfigHandler *handler.ProcessingConfigHandler, triggerConfigHandler *handler.TriggerConfigHandler, dlqHandler *handler.DLQHandler, retryConfigHandler *handler.RetryConfigHandler, sseHandler *handler.SSEHandler, tagHandler *handler.TagHandler, actorHandler *handler.ActorHandler, studioHandler *handler.StudioHandler, interactionHandler *handler.InteractionHandler, actorInteractionHandler *handler.ActorInteractionHandler, studioInteractionHandler *handler.StudioInteractionHandler, searchHandler *handler.SearchHandler, watchHistoryHandler *handler.WatchHistoryHandler, storagePathHandler *handler.StoragePathHandler, scanHandler *handler.ScanHandler, explorerHandler *handler.ExplorerHandler, pornDBHandler *handler.PornDBHandler, savedSearchHandler *handler.SavedSearchHandler, homepageHandler *handler.HomepageHandler, markerHandler *handler.MarkerHandler, importHandler *handler.ImportHandler, streamStatsHandler *handler.StreamStatsHandler, authService *core.AuthService, rbacService *core.RBACService, rateLimiter *middleware.IPRateLimiter) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New() // Empty engine, we add middleware manually

	// SECURITY: Configure trusted proxies to prevent X-Forwarded-For spoofing
	// Only proxies in this list are trusted to set X-Forwarded-For headers
	if len(cfg.Server.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
			logger.Error(fmt.Sprintf("Failed to set trusted proxies: %v", err))
		}
	} else {
		// No trusted proxies configured - trust no proxies (use direct client IP)
		r.SetTrustedProxies(nil)
	}

	middleware.Setup(r, logger, cfg.Server.AllowedOrigins, cfg.Environment)

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

	// Serve Actor Images (using configured actor image directory)
	r.GET("/actor-images/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		path := filepath.Join(cfg.Processing.ActorImageDir, filename)
		ext := filepath.Ext(filename)
		switch ext {
		case ".jpg", ".jpeg":
			c.Header("Content-Type", "image/jpeg")
		case ".png":
			c.Header("Content-Type", "image/png")
		case ".webp":
			c.Header("Content-Type", "image/webp")
		case ".gif":
			c.Header("Content-Type", "image/gif")
		default:
			c.Header("Content-Type", "application/octet-stream")
		}
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve Studio Logos (using configured studio logo directory)
	r.GET("/studio-logos/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		path := filepath.Join(cfg.Processing.StudioLogoDir, filename)
		ext := filepath.Ext(filename)
		switch ext {
		case ".jpg", ".jpeg":
			c.Header("Content-Type", "image/jpeg")
		case ".png":
			c.Header("Content-Type", "image/png")
		case ".webp":
			c.Header("Content-Type", "image/webp")
		case ".gif":
			c.Header("Content-Type", "image/gif")
		default:
			c.Header("Content-Type", "application/octet-stream")
		}
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve Marker Thumbnails (using configured marker thumbnail directory)
	r.GET("/marker-thumbnails/:id", func(c *gin.Context) {
		id := c.Param("id")
		// Validate ID is numeric to prevent path traversal attacks
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid marker ID"})
			return
		}
		path := filepath.Join(cfg.Processing.MarkerThumbnailDir, fmt.Sprintf("marker_%s.webp", id))
		c.Header("Content-Type", "image/webp")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve Animated Marker Thumbnails (MP4 clips)
	r.GET("/marker-thumbnails/:id/animated", func(c *gin.Context) {
		id := c.Param("id")
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid marker ID"})
			return
		}
		path := filepath.Join(cfg.Processing.MarkerThumbnailDir, fmt.Sprintf("marker_%s.mp4", id))
		c.Header("Content-Type", "video/mp4")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Serve Scene Preview Videos (MP4 clips for hover preview)
	r.GET("/scene-previews/:id", func(c *gin.Context) {
		id := c.Param("id")
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scene ID"})
			return
		}
		path := filepath.Join(cfg.Processing.ScenePreviewDir, fmt.Sprintf("%s_preview.mp4", id))
		c.Header("Content-Type", "video/mp4")
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
		c.File(path)
	})

	// Register Routes
	RegisterRoutes(r, sceneHandler, authHandler, settingsHandler, adminHandler, jobHandler, poolConfigHandler, processingConfigHandler, triggerConfigHandler, dlqHandler, retryConfigHandler, sseHandler, tagHandler, actorHandler, studioHandler, interactionHandler, actorInteractionHandler, studioInteractionHandler, searchHandler, watchHistoryHandler, storagePathHandler, scanHandler, explorerHandler, pornDBHandler, savedSearchHandler, homepageHandler, markerHandler, importHandler, streamStatsHandler, authService, rbacService, logger, rateLimiter)

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

		// Cache hashed _nuxt/ assets immutably (filenames contain content hashes)
		if strings.HasPrefix(filePath, "_nuxt/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else if filePath == "index.html" || filePath == "sw.js" {
			// HTML and service worker must always be revalidated
			c.Header("Cache-Control", "no-cache, must-revalidate")
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
