package api

import (
	"fmt"
	"goonhub"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/config"
	"goonhub/internal/infrastructure/logging"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// NewShareRouter creates a minimal Gin engine that serves only share-related routes.
// This is used for the dedicated share server that can be exposed on a separate public domain.
func NewShareRouter(cfg *config.Config, shareHandler *handler.ShareHandler, ogMiddleware *middleware.OGMiddleware, logger *logging.Logger) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Trust no proxies by default (same as main router)
	if len(cfg.Server.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
			logger.Error(fmt.Sprintf("Share router: failed to set trusted proxies: %v", err))
		}
	} else {
		r.SetTrustedProxies(nil)
	}

	// Middleware
	r.Use(gin.Recovery())

	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPathsRegexs([]string{
		`/api/v1/shares/.*/stream`,
	})))

	r.Use(middleware.SecurityHeaders(cfg.Environment))
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(logger))

	// CORS: allow the share BaseURL origin with read-only methods
	shareOrigins := []string{}
	if cfg.Sharing.BaseURL != "" {
		shareOrigins = append(shareOrigins, cfg.Sharing.BaseURL)
	}
	// Also allow the main app origins so the frontend can reach share API during dev
	shareOrigins = append(shareOrigins, cfg.Server.AllowedOrigins...)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     shareOrigins,
		AllowMethods:     []string{"GET", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "server": "share"})
	})

	// Thumbnails (needed for poster images and OG tags)
	r.GET("/thumbnails/:id", func(c *gin.Context) {
		id := c.Param("id")
		size := c.DefaultQuery("size", "sm")
		if size != "sm" && size != "lg" {
			size = "sm"
		}
		path := filepath.Join(cfg.Processing.ThumbnailDir, fmt.Sprintf("%s_thumb_%s.webp", id, size))
		c.Header("Content-Type", "image/webp")
		c.Header("Cache-Control", "public, max-age=31536000")
		c.File(path)
	})

	// Share API routes
	shares := r.Group("/api/v1/shares")
	{
		shares.GET("/:token", shareHandler.ResolveShareLink)
		shares.GET("/:token/stream", shareHandler.StreamShareLink)
	}

	// SPA fallback for /share/* paths only
	fsys, _ := fs.Sub(goonhub.WebDist, "web/dist")

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

		if strings.HasPrefix(filePath, "_nuxt/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else if filePath == "index.html" || filePath == "sw.js" {
			c.Header("Cache-Control", "no-cache, must-revalidate")
		}

		c.Data(http.StatusOK, contentType, content)
		return true
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// OG middleware for crawlers hitting /share/:token
		if ogMiddleware.ServeIfCrawler(c) {
			return
		}

		// Serve static assets (_nuxt/*, favicon, etc.) needed by the SPA
		clean := strings.TrimPrefix(path, "/")
		if clean != "" && (strings.HasPrefix(clean, "_nuxt/") || filepath.Ext(clean) != "") {
			if serveFile(c, clean) {
				return
			}
		}

		// Only serve SPA fallback for /share/* paths
		if strings.HasPrefix(path, "/share/") || path == "/share" {
			serveFile(c, "index.html")
			return
		}

		// Everything else is 404
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	return r
}
