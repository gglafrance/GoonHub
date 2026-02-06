package streaming

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseSingleRange(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		size       int64
		wantStart  int64
		wantLength int64
		wantOk     bool
	}{
		{"open range from start", "bytes=0-", 1000, 0, 1000, true},
		{"open range from offset", "bytes=500-", 1000, 500, 500, true},
		{"closed range", "bytes=0-499", 1000, 0, 500, true},
		{"closed range middle", "bytes=200-299", 1000, 200, 100, true},
		{"suffix range", "bytes=-100", 1000, 900, 100, true},
		{"suffix larger than file", "bytes=-2000", 1000, 0, 1000, true},
		{"end beyond file size", "bytes=500-9999", 1000, 500, 500, true},
		{"single byte", "bytes=0-0", 1000, 0, 1, true},
		{"last byte", "bytes=999-999", 1000, 999, 1, true},

		// Invalid cases
		{"no bytes prefix", "chars=0-100", 1000, 0, 0, false},
		{"multipart range", "bytes=0-100, 200-300", 1000, 0, 0, false},
		{"start beyond size", "bytes=1000-", 1000, 0, 0, false},
		{"negative suffix", "bytes=-0", 1000, 0, 0, false},
		{"end before start", "bytes=500-100", 1000, 0, 0, false},
		{"no dash", "bytes=100", 1000, 0, 0, false},
		{"empty header", "", 1000, 0, 0, false},
		{"negative start", "bytes=-1-100", 1000, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, length, ok := parseSingleRange(tt.header, tt.size)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if !ok {
				return
			}
			if start != tt.wantStart {
				t.Fatalf("start = %d, want %d", start, tt.wantStart)
			}
			if length != tt.wantLength {
				t.Fatalf("length = %d, want %d", length, tt.wantLength)
			}
		})
	}
}

func TestServeVideoNoRange(t *testing.T) {
	content := "Hello, this is test video content"
	body := strings.NewReader(content)
	buf := make([]byte, 1024)

	req := httptest.NewRequest(http.MethodGet, "/video.mp4", nil)
	w := httptest.NewRecorder()

	ServeVideo(w, req, "video.mp4", time.Now(), body, buf)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatal("expected Accept-Ranges: bytes header")
	}

	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != content {
		t.Fatalf("expected body %q, got %q", content, string(respBody))
	}
}

func TestServeVideoSingleRange(t *testing.T) {
	content := "0123456789ABCDEF"
	body := strings.NewReader(content)
	buf := make([]byte, 1024)

	req := httptest.NewRequest(http.MethodGet, "/video.mp4", nil)
	req.Header.Set("Range", "bytes=4-7")
	w := httptest.NewRecorder()

	ServeVideo(w, req, "video.mp4", time.Now(), body, buf)

	resp := w.Result()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Range") != "bytes 4-7/16" {
		t.Fatalf("unexpected Content-Range: %s", resp.Header.Get("Content-Range"))
	}

	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != "4567" {
		t.Fatalf("expected body %q, got %q", "4567", string(respBody))
	}
}

func TestServeVideoOpenRange(t *testing.T) {
	content := "0123456789"
	body := strings.NewReader(content)
	buf := make([]byte, 1024)

	req := httptest.NewRequest(http.MethodGet, "/video.mp4", nil)
	req.Header.Set("Range", "bytes=5-")
	w := httptest.NewRecorder()

	ServeVideo(w, req, "video.mp4", time.Now(), body, buf)

	resp := w.Result()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != "56789" {
		t.Fatalf("expected body %q, got %q", "56789", string(respBody))
	}
}

func TestServeVideoIfModifiedSince(t *testing.T) {
	content := "test content"
	body := strings.NewReader(content)
	buf := make([]byte, 1024)
	modTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	req := httptest.NewRequest(http.MethodGet, "/video.mp4", nil)
	req.Header.Set("If-Modified-Since", modTime.Add(time.Hour).UTC().Format(http.TimeFormat))
	w := httptest.NewRecorder()

	ServeVideo(w, req, "video.mp4", modTime, body, buf)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", resp.StatusCode)
	}
}

func TestServeVideoHeadRequest(t *testing.T) {
	content := "0123456789"
	body := strings.NewReader(content)
	buf := make([]byte, 1024)

	req := httptest.NewRequest(http.MethodHead, "/video.mp4", nil)
	w := httptest.NewRecorder()

	ServeVideo(w, req, "video.mp4", time.Now(), body, buf)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Length") != "10" {
		t.Fatalf("expected Content-Length 10, got %s", resp.Header.Get("Content-Length"))
	}

	respBody, _ := io.ReadAll(resp.Body)
	if len(respBody) != 0 {
		t.Fatal("expected empty body for HEAD request")
	}
}
