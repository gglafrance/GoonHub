package cache

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"goonhub-worker/internal/backoff"
	"goonhub-worker/internal/client"
)

// mediaServer serves data with Range support; cutAt > 0 breaks the first response after
// that many bytes.
type mediaServer struct {
	srv    *httptest.Server
	data   []byte
	mu     sync.Mutex
	cutAt  int
	gets   int
	ranges []string
	served int64
}

func newMediaServer(t *testing.T, n int) *mediaServer {
	t.Helper()
	m := &mediaServer{data: make([]byte, n)}
	_, _ = io.ReadFull(rand.Reader, m.data)
	mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	m.srv = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.gets++
		m.ranges = append(m.ranges, r.Header.Get("Range"))
		cut := m.cutAt
		m.cutAt = 0
		m.mu.Unlock()
		rw.Header().Set("ETag", `"v1"`)
		if cut > 0 {
			rw.Header().Set("Content-Length", "999999999")
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(m.data[:cut])
			m.mu.Lock()
			m.served += int64(cut)
			m.mu.Unlock()
			if hj, ok := rw.(http.Hijacker); ok {
				c, _, _ := hj.Hijack()
				c.Close()
			}
			return
		}
		cw := &counting{ResponseWriter: rw}
		http.ServeContent(cw, r, "v.mp4", mod, bytes.NewReader(m.data))
		m.mu.Lock()
		m.served += cw.n
		m.mu.Unlock()
	}))
	t.Cleanup(m.srv.Close)
	return m
}

type counting struct {
	http.ResponseWriter
	n int64
}

func (c *counting) Write(b []byte) (int, error) {
	n, err := c.ResponseWriter.Write(b)
	c.n += int64(n)
	return n, err
}

type staticSource string

func (s staticSource) URL(context.Context) (string, error)     { return string(s), nil }
func (s staticSource) Refresh(context.Context) (string, error) { return string(s), nil }

func open(t *testing.T, dir string, max int64) *Cache {
	t.Helper()
	cl := client.New("http://unused", "t", "test")
	c, err := Open(dir, max, cl, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	c.Backoff = backoff.Backoff{Min: 10 * time.Millisecond, Max: 50 * time.Millisecond}
	return c
}

func TestFetchResumesAfterCut(t *testing.T) {
	m := newMediaServer(t, 1<<20)
	m.cutAt = 300 << 10
	c := open(t, t.TempDir(), 10<<20)
	h, n, err := c.Fetch(context.Background(), Key(1, int64(len(m.data))), "video.mp4", int64(len(m.data)), staticSource(m.srv.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Release()
	got, _ := os.ReadFile(h.Path)
	if !bytes.Equal(got, m.data) {
		t.Fatal("downloaded content differs")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if n != int64(len(m.data)) || m.served != int64(len(m.data)) {
		t.Fatalf("transferred %d, served %d, want %d each (resume, no restart)", n, m.served, len(m.data))
	}
	if m.gets != 2 || m.ranges[1] != "bytes=307200-" {
		t.Fatalf("requests %d ranges %v", m.gets, m.ranges)
	}
}

func TestFetchReusesCompleteEntryAcrossOpen(t *testing.T) {
	m := newMediaServer(t, 100<<10)
	dir := t.TempDir()
	key := Key(2, int64(len(m.data)))
	c := open(t, dir, 10<<20)
	h, _, err := c.Fetch(context.Background(), key, "video.mp4", int64(len(m.data)), staticSource(m.srv.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	h.Release()

	c2 := open(t, dir, 10<<20) // restart
	h2, ok := c2.Lookup(key)
	if !ok {
		t.Fatal("complete entry lost across restart")
	}
	defer h2.Release()
	if h2.Meta.ETag != `"v1"` || h2.Meta.Validator() != `"v1"` {
		t.Fatalf("meta %+v", h2.Meta)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gets != 1 {
		t.Fatalf("gets %d", m.gets)
	}
}

func TestPartialResumedAfterRestartFromSyncedOffset(t *testing.T) {
	m := newMediaServer(t, 512<<10)
	dir := t.TempDir()
	key := Key(3, int64(len(m.data)))
	c := open(t, dir, 10<<20)
	c.SyncEvery = 64 << 10
	m.cutAt = 200 << 10
	ctx, cancel := context.WithCancel(context.Background())
	// Stop at the first retry wait: simulates a crash after a cut.
	c.Backoff = backoff.Backoff{Min: time.Hour, Max: time.Hour}
	go func() {
		for {
			m.mu.Lock()
			g := m.gets
			m.mu.Unlock()
			if g >= 1 {
				time.Sleep(100 * time.Millisecond)
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	if _, _, err := c.Fetch(ctx, key, "v.mp4", int64(len(m.data)), staticSource(m.srv.URL), nil); err == nil {
		t.Fatal("expected the cancelled fetch to fail")
	}
	c2 := open(t, dir, 10<<20)
	h, _, err := c2.Fetch(context.Background(), key, "v.mp4", int64(len(m.data)), staticSource(m.srv.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Release()
	got, _ := os.ReadFile(h.Path)
	if !bytes.Equal(got, m.data) {
		t.Fatal("content differs after restart resume")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if last := m.ranges[len(m.ranges)-1]; last == "" {
		t.Fatalf("restart did not resume with a Range: %v", m.ranges)
	}
}

func TestEvictionLRUAndInUse(t *testing.T) {
	m := newMediaServer(t, 40<<10)
	size := int64(len(m.data))
	dir := t.TempDir()
	c := open(t, dir, 100<<10) // room for 2 entries
	fetch := func(scene uint) *Handle {
		h, _, err := c.Fetch(context.Background(), Key(scene, size), "v.mp4", size, staticSource(m.srv.URL), nil)
		if err != nil {
			t.Fatalf("fetch %d: %v", scene, err)
		}
		return h
	}
	h1 := fetch(1)
	h1.Release()
	time.Sleep(10 * time.Millisecond)
	h2 := fetch(2) // held
	time.Sleep(10 * time.Millisecond)
	h3 := fetch(3) // evicts 1 (LRU, unused)
	if _, ok := c.Lookup(Key(1, size)); ok {
		t.Fatal("entry 1 should be evicted")
	}
	if _, err := os.Stat(filepath.Join(dir, Key(1, size))); !os.IsNotExist(err) {
		t.Fatal("entry 1 directory left")
	}
	// 2 and 3 are in use: 4 cannot fit.
	if _, _, err := c.Fetch(context.Background(), Key(4, size), "v.mp4", size, staticSource(m.srv.URL), nil); err == nil {
		t.Fatal("fetch should fail while every entry is in use")
	}
	h2.Release()
	h3.Release()
	h4 := fetch(4)
	h4.Release()
	if c.Used() > c.Max() {
		t.Fatalf("used %d above max %d", c.Used(), c.Max())
	}
	// Bigger than the cache: refused at once.
	if _, _, err := c.Fetch(context.Background(), Key(5, 1<<30), "v.mp4", 1<<30, staticSource(m.srv.URL), nil); err == nil {
		t.Fatal("oversized fetch accepted")
	}
}

func TestSafeFileName(t *testing.T) {
	if safeFileName("video.mp4") != "video.mp4" || safeFileName("../../evil.mkv") != "source.mkv" || safeFileName("a b.mp4") != "source.mp4" {
		t.Fatal("safeFileName")
	}
}
