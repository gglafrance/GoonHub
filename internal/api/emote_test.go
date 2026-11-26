package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServeEmote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	name := "0123456789abcdef0123456789abcdef.gif"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/emotes/:filename", func(c *gin.Context) { serveEmote(c, dir, c.Param("filename")) })

	cases := []struct {
		path string
		code int
		ct   string
	}{
		{"/emotes/" + name, http.StatusOK, "image/gif"},
		{"/emotes/..%2Fsecret.txt", http.StatusNotFound, ""},
		{"/emotes/0123456789abcdef0123456789abcdef.svg", http.StatusNotFound, ""},
		{"/emotes/ffffffffffffffffffffffffffffffff.png", http.StatusNotFound, ""},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, c.path, nil))
		if w.Code != c.code {
			t.Fatalf("%s: status %d, want %d", c.path, w.Code, c.code)
		}
		if c.ct != "" && w.Header().Get("Content-Type") != c.ct {
			t.Fatalf("%s: content type %q", c.path, w.Header().Get("Content-Type"))
		}
	}
}
