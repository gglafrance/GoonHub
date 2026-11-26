package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"goonhub/internal/together"

	"github.com/gin-gonic/gin"
)

// serveEmote serves an emote image of the emote directory. Only stored file names are
// accepted (random hex name and an image extension): no path can escape the directory.
func serveEmote(c *gin.Context, dir, filename string) {
	if !together.EmoteFilePattern.MatchString(filename) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	switch strings.TrimPrefix(filepath.Ext(filename), ".") {
	case "png":
		c.Header("Content-Type", "image/png")
	case "gif":
		c.Header("Content-Type", "image/gif")
	case "webp":
		c.Header("Content-Type", "image/webp")
	}
	// The name is random and never reused: the file never changes
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(filepath.Join(dir, filename))
}
