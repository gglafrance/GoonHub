package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// serveMarkerThumbnail serves a marker thumbnail file. A marker's thumbnail is rewritten under
// the same name when the marker moves, so the frontend appends its version (?v=, see
// data.MarkerThumbnailVersion): a versioned URL never changes content and is cached for a year,
// an unversioned one is revalidated on each use (Last-Modified), and a missing file (thumbnail
// not made yet, or being remade) is never cached.
func serveMarkerThumbnail(c *gin.Context, path, contentType string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusNotFound, gin.H{"error": "marker thumbnail not found"})
		return
	}
	if c.Query("v") != "" {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-cache")
	}
	c.Header("Content-Type", contentType)
	c.File(path)
}
