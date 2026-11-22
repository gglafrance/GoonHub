// Package cache is the download cache of indirect input mode: whole-file copies of
// source videos fetched over the signed media URL with resumable Range requests, kept
// under a size limit (least recently used first out) and shared by the jobs of the same
// scene (a fingerprint download is reused by a later sprites job of that scene).
//
// The cache only holds copies: deleting an entry never touches a source media file.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/backoff"
	"goonhub-worker/internal/client"
	"goonhub-worker/internal/outbox"
	"goonhub-worker/internal/ratelimit"
)

// ErrNoSpace is returned when the file cannot fit in the cache (bigger than the limit,
// or every other entry is in use).
var ErrNoSpace = errors.New("download cache full")

// Meta is meta.json of an entry.
type Meta struct {
	Key          string    `json:"key"`
	FileName     string    `json:"filename"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	Complete     bool      `json:"complete"`
	Synced       int64     `json:"synced"` // bytes of the .part known to be on disk (fsynced)
	LastUsed     time.Time `json:"last_used"`
}

// Validator is the If-Range value of the entry.
func (m Meta) Validator() string {
	if m.ETag != "" && !strings.HasPrefix(m.ETag, "W/") {
		return m.ETag
	}
	return m.LastModified
}

type entry struct {
	meta Meta
	refs int
	// used is the bytes on disk (Size when complete, the .part size otherwise).
	used int64
	// lock serializes downloads of the key.
	lock sync.Mutex
}

// Cache is the download cache.
type Cache struct {
	dir     string
	max     int64
	log     *slog.Logger
	limiter *ratelimit.Limiter
	media   MediaGetter
	// IdleTimeout aborts a download that received nothing for this long.
	IdleTimeout time.Duration
	// Backoff bounds the retry delays (zero = 1 s -> 30 s).
	Backoff backoff.Backoff
	// SyncEvery fsyncs the partial file after this many bytes.
	SyncEvery int64

	mu      sync.Mutex
	entries map[string]*entry
}

// MediaGetter starts a GET of a media URL at an offset (client.Client).
type MediaGetter interface {
	MediaGet(ctx context.Context, mediaURL string, offset int64, ifRange string) (*http.Response, error)
}

// Open loads the cache directory.
func Open(dir string, maxBytes int64, media MediaGetter, limiter *ratelimit.Limiter, log *slog.Logger) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache %s: %w", dir, err)
	}
	c := &Cache{dir: dir, max: maxBytes, log: log, limiter: limiter, media: media,
		IdleTimeout: 60 * time.Second, SyncEvery: 64 << 20, entries: map[string]*entry{}}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}
	for _, it := range items {
		if !it.IsDir() {
			continue
		}
		edir := filepath.Join(dir, it.Name())
		var m Meta
		b, err := os.ReadFile(filepath.Join(edir, "meta.json"))
		if err != nil || json.Unmarshal(b, &m) != nil || m.Key != it.Name() || m.FileName == "" {
			_ = os.RemoveAll(edir)
			continue
		}
		e := &entry{meta: m}
		if m.Complete {
			st, err := os.Stat(c.path(m))
			if err != nil || st.Size() != m.Size {
				_ = os.RemoveAll(edir)
				continue
			}
			e.used = m.Size
		} else {
			// Keep only what was fsynced: bytes after it may be garbage after a power loss.
			part := c.path(m) + ".part"
			st, err := os.Stat(part)
			if err != nil {
				e.used = 0
				e.meta.Synced = 0
			} else {
				keep := min(st.Size(), m.Synced)
				if keep < st.Size() {
					_ = os.Truncate(part, keep)
				}
				e.used = keep
				e.meta.Synced = keep
			}
		}
		c.entries[m.Key] = e
	}
	return c, nil
}

// Dir returns the cache directory.
func (c *Cache) Dir() string { return c.dir }

// Max returns the size limit.
func (c *Cache) Max() int64 { return c.max }

// Used returns the bytes held.
func (c *Cache) Used() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var n int64
	for _, e := range c.entries {
		n += e.used
	}
	return n
}

// Key is the cache key of a scene source: the same scene and size share the entry.
func Key(sceneID uint, size int64) string {
	return "scene-" + strconv.FormatUint(uint64(sceneID), 10) + "-" + strconv.FormatInt(size, 10)
}

func (c *Cache) path(m Meta) string {
	return filepath.Join(c.dir, m.Key, m.FileName)
}

func safeFileName(name string) string {
	if err := workerproto.ValidateArtifactName(name); err == nil {
		return name
	}
	ext := strings.ToLower(filepath.Ext(name))
	if workerproto.ValidateArtifactName("source"+ext) == nil {
		return "source" + ext
	}
	return "source.bin"
}

// Handle is a reference to a complete entry; the entry is not evicted while held.
type Handle struct {
	c    *Cache
	key  string
	Path string
	Meta Meta
	once sync.Once
}

// Release drops the reference.
func (h *Handle) Release() {
	h.once.Do(func() {
		h.c.mu.Lock()
		defer h.c.mu.Unlock()
		if e := h.c.entries[h.key]; e != nil && e.refs > 0 {
			e.refs--
		}
	})
}

// Lookup returns a handle on the complete entry of key, if any.
func (c *Cache) Lookup(key string) (*Handle, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	if e == nil || !e.meta.Complete {
		return nil, false
	}
	e.refs++
	e.meta.LastUsed = time.Now().UTC()
	c.saveMetaLocked(e)
	return &Handle{c: c, key: key, Path: c.path(e.meta), Meta: e.meta}, true
}

// Invalidate removes an entry that turned out stale (the source changed). It is removed
// now when unused, else at the next eviction pass.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	if e == nil {
		return
	}
	e.meta.Complete = false
	e.meta.LastUsed = time.Time{}
	if e.refs == 0 {
		c.removeLocked(key)
	}
}

// Source gives the media URL of a download; Refresh asks a new one (the current one
// expired).
type Source interface {
	URL(ctx context.Context) (string, error)
	Refresh(ctx context.Context) (string, error)
}

// Progress receives the downloaded and total bytes.
type Progress func(done, total int64)

// Fetch returns a handle on the complete copy of key, downloading it (resuming a partial
// one) when needed. Errors from the server that end the lease are returned as is
// (client.IsLeaseGone); network errors are retried with backoff until ctx ends.
func (c *Cache) Fetch(ctx context.Context, key, fileName string, size int64, src Source, progress Progress) (*Handle, int64, error) {
	if size <= 0 {
		return nil, 0, fmt.Errorf("unknown source size")
	}
	if size > c.max {
		return nil, 0, fmt.Errorf("%w: source is %d bytes, cache limit %d", ErrNoSpace, size, c.max)
	}
	c.mu.Lock()
	e := c.entries[key]
	if e == nil {
		e = &entry{meta: Meta{Key: key, FileName: safeFileName(fileName), Size: size}}
		c.entries[key] = e
	}
	e.refs++ // protect from eviction while downloading
	c.mu.Unlock()
	release := func() {
		c.mu.Lock()
		e.refs--
		c.mu.Unlock()
	}

	e.lock.Lock()
	defer e.lock.Unlock()
	if h, ok := c.Lookup(key); ok {
		release()
		return h, 0, nil
	}
	if err := c.makeRoom(key, size); err != nil {
		release()
		return nil, 0, err
	}
	n, err := c.download(ctx, e, src, progress)
	if err != nil {
		release()
		return nil, n, err
	}
	c.mu.Lock()
	e.meta.LastUsed = time.Now().UTC()
	c.saveMetaLocked(e)
	h := &Handle{c: c, key: key, Path: c.path(e.meta), Meta: e.meta}
	c.mu.Unlock()
	// The download reference becomes the handle's.
	return h, n, nil
}

// makeRoom evicts least recently used unreferenced entries until size more bytes fit.
func (c *Cache) makeRoom(key string, size int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var used int64
	for k, e := range c.entries {
		if k != key {
			used += e.used
		}
	}
	if used+size <= c.max {
		return nil
	}
	var victims []*entry
	for k, e := range c.entries {
		if k != key && e.refs == 0 {
			victims = append(victims, e)
		}
	}
	sort.Slice(victims, func(i, j int) bool { return victims[i].meta.LastUsed.Before(victims[j].meta.LastUsed) })
	for _, v := range victims {
		if used+size <= c.max {
			break
		}
		used -= v.used
		c.log.Info("cache: evicting", "key", v.meta.Key, "bytes", v.used)
		c.removeLocked(v.meta.Key)
	}
	if used+size > c.max {
		return fmt.Errorf("%w: %d bytes in use by running jobs", ErrNoSpace, used)
	}
	return nil
}

func (c *Cache) removeLocked(key string) {
	delete(c.entries, key)
	_ = os.RemoveAll(filepath.Join(c.dir, key))
}

func (c *Cache) saveMetaLocked(e *entry) {
	dir := filepath.Join(c.dir, e.meta.Key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	b, err := json.Marshal(e.meta)
	if err != nil {
		return
	}
	if err := outbox.WriteFileAtomic(filepath.Join(dir, "meta.json"), b); err != nil {
		c.log.Warn("cache: failed to save meta", "key", e.meta.Key, "error", err)
	}
}

// download fills the .part of e up to its size, then renames it. Returns the bytes
// transferred by this call.
func (c *Cache) download(ctx context.Context, e *entry, src Source, progress Progress) (int64, error) {
	c.mu.Lock()
	c.saveMetaLocked(e)
	m := e.meta
	c.mu.Unlock()
	final := c.path(m)
	part := final + ".part"
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		return 0, fmt.Errorf("failed to create cache entry: %w", err)
	}
	f, err := os.OpenFile(part, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return 0, fmt.Errorf("failed to open cache file: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	offset := st.Size()
	if offset > m.Size {
		offset = 0
	}
	var transferred int64
	bo := c.Backoff
	refreshed := false
	for {
		if err := ctx.Err(); err != nil {
			return transferred, err
		}
		if offset == m.Size {
			break
		}
		u, err := src.URL(ctx)
		if err != nil {
			return transferred, err
		}
		n, newOffset, err := c.fetchOnce(ctx, e, f, u, offset, progress)
		transferred += n
		if newOffset > offset {
			bo.Reset()
		}
		offset = newOffset
		if err == nil {
			continue
		}
		if client.IsCode(err, workerproto.CodeURLExpired) && !refreshed {
			refreshed = true
			if _, rerr := src.Refresh(ctx); rerr != nil {
				return transferred, rerr
			}
			continue
		}
		var le *localError
		if errors.As(err, &le) || client.IsLeaseGone(err) || !client.IsTransient(err) {
			return transferred, err
		}
		refreshed = false
		c.log.Warn("cache: download interrupted, resuming", "key", m.Key, "offset", offset, "error", err)
		if werr := bo.Wait(ctx); werr != nil {
			return transferred, werr
		}
	}
	if err := f.Sync(); err != nil {
		return transferred, fmt.Errorf("failed to sync cache file: %w", err)
	}
	if err := f.Close(); err != nil {
		return transferred, fmt.Errorf("failed to close cache file: %w", err)
	}
	if err := os.Rename(part, final); err != nil {
		return transferred, fmt.Errorf("failed to finish cache file: %w", err)
	}
	c.mu.Lock()
	e.meta.Complete = true
	e.meta.Synced = e.meta.Size
	e.used = e.meta.Size
	c.saveMetaLocked(e)
	c.mu.Unlock()
	return transferred, nil
}

var errShortBody = errors.New("media body ended early")

// localError is a failure of the local disk (not retried).
type localError struct{ err error }

func (e *localError) Error() string { return e.err.Error() }
func (e *localError) Unwrap() error { return e.err }

// fetchOnce runs one GET from offset and appends to f. It returns the bytes written and
// the new offset.
func (c *Cache) fetchOnce(ctx context.Context, e *entry, f *os.File, u string, offset int64, progress Progress) (int64, int64, error) {
	c.mu.Lock()
	validator := e.meta.Validator()
	size := e.meta.Size
	c.mu.Unlock()

	reqCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	resp, err := c.media.MediaGet(reqCtx, u, offset, validator)
	if err != nil {
		return 0, offset, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusRequestedRangeNotSatisfiable:
		// Our part is longer than the source or the source changed: start over.
		if err := f.Truncate(0); err != nil {
			return 0, offset, &localError{err}
		}
		c.setSynced(e, 0)
		return 0, 0, errShortBody
	case http.StatusOK:
		if offset > 0 {
			// Range ignored or validator changed: the server sends the whole file.
			if err := f.Truncate(0); err != nil {
				return 0, offset, &localError{err}
			}
			offset = 0
			c.setSynced(e, 0)
		}
	case http.StatusPartialContent:
		start, ok := contentRangeStart(resp.Header.Get("Content-Range"))
		if !ok || start != offset {
			return 0, offset, fmt.Errorf("unexpected Content-Range %q for offset %d", resp.Header.Get("Content-Range"), offset)
		}
	}
	c.mu.Lock()
	if et := resp.Header.Get("ETag"); et != "" {
		e.meta.ETag = et
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		e.meta.LastModified = lm
	}
	c.mu.Unlock()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return 0, offset, &localError{err}
	}
	// Idle watchdog: cancel the request when no byte arrives for IdleTimeout.
	idle := c.IdleTimeout
	if idle <= 0 {
		idle = 60 * time.Second
	}
	timer := time.AfterFunc(idle, func() { cancel(fmt.Errorf("no data for %s", idle)) })
	defer timer.Stop()

	body := c.limiter.Reader(ctx, resp.Body)
	buf := make([]byte, 256<<10)
	var written int64
	lastSync := offset
	for offset < size {
		want := buf
		if rem := size - offset; rem < int64(len(want)) {
			want = want[:rem]
		}
		n, rerr := body.Read(want)
		if n > 0 {
			timer.Reset(idle)
			if _, werr := f.Write(want[:n]); werr != nil {
				return written, offset, &localError{fmt.Errorf("failed to write cache file: %w", werr)}
			}
			offset += int64(n)
			written += int64(n)
			c.addUsed(e, int64(n))
			if progress != nil {
				progress(offset, size)
			}
			if offset-lastSync >= c.SyncEvery {
				if err := f.Sync(); err == nil {
					lastSync = offset
					c.setSynced(e, offset)
				}
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				if offset < size {
					return written, offset, errShortBody
				}
				break
			}
			if cause := context.Cause(reqCtx); cause != nil && ctx.Err() == nil {
				return written, offset, fmt.Errorf("media download stalled: %w", cause)
			}
			return written, offset, fmt.Errorf("media download: %w", rerr)
		}
	}
	if err := f.Sync(); err == nil {
		c.setSynced(e, offset)
	}
	return written, offset, nil
}

func (c *Cache) addUsed(e *entry, n int64) {
	c.mu.Lock()
	e.used += n
	c.mu.Unlock()
}

func (c *Cache) setSynced(e *entry, n int64) {
	c.mu.Lock()
	e.meta.Synced = n
	if n == 0 {
		e.used = 0
	}
	c.saveMetaLocked(e)
	c.mu.Unlock()
}

// contentRangeStart parses "bytes <start>-<end>/<size>".
func contentRangeStart(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "bytes ") {
		return 0, false
	}
	v = strings.TrimPrefix(v, "bytes ")
	dash := strings.IndexByte(v, '-')
	if dash <= 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(v[:dash], 10, 64)
	return n, err == nil
}
