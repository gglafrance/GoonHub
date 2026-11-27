package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServeMarkerThumbnailCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker_1.webp"), []byte("webp"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/marker-thumbnails/:id", func(c *gin.Context) {
		serveMarkerThumbnail(c, filepath.Join(dir, "marker_"+c.Param("id")+".webp"), "image/webp")
	})

	tests := []struct {
		name       string
		url        string
		wantStatus int
		wantCache  string
	}{
		{"versioned", "/marker-thumbnails/1?v=1700000000123", http.StatusOK, "public, max-age=31536000, immutable"},
		{"unversioned", "/marker-thumbnails/1", http.StatusOK, "no-cache"},
		{"missing", "/marker-thumbnails/2?v=1", http.StatusNotFound, "no-store"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.url, nil))
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
			if tt.wantStatus == http.StatusOK {
				if ct := w.Header().Get("Content-Type"); ct != "image/webp" {
					t.Fatalf("Content-Type = %q, want image/webp", ct)
				}
				if w.Header().Get("Last-Modified") == "" {
					t.Fatal("missing Last-Modified (revalidation of unversioned URLs)")
				}
			}
		})
	}
}
