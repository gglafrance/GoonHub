package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/cache"
	"goonhub-worker/internal/client"
)

// urlRefreshMargin: a media URL expiring sooner than this is renewed before use.
const urlRefreshMargin = 10 * time.Minute

// inputChoice is the source given to the computation.
type inputChoice struct {
	path           string // local path or URL
	mode           workerproto.InputMode
	viaURL         bool // ffmpeg reads the URL itself
	downloadMillis int64
	downloaded     int64
}

func (in *inputChoice) describe() string {
	if in.viaURL {
		return client.RedactURL(in.path)
	}
	return in.path
}

// directPath maps the assignment input to a local read-only path: by storage path id +
// relative path, or by server path prefix. ok=false with the reason otherwise. The file
// must be a regular file of the expected size (section 11 "Input selection").
func (w *Worker) directPath(a *workerproto.Assignment) (string, bool, string) {
	in := a.Input
	var candidates []string
	for _, m := range w.cfg.Storage {
		if m.StoragePathID != 0 && in.StoragePathID != nil && *in.StoragePathID == m.StoragePathID && in.RelativePath != "" {
			rel := path.Clean("/" + strings.ReplaceAll(in.RelativePath, "\\", "/"))[1:]
			if rel == "" || !filepath.IsLocal(filepath.FromSlash(rel)) {
				continue
			}
			candidates = append(candidates, filepath.Join(m.LocalPath, filepath.FromSlash(rel)))
		}
		if m.ServerPrefix != "" && in.ServerPath != "" {
			sp := path.Clean(strings.ReplaceAll(in.ServerPath, "\\", "/"))
			prefix := path.Clean(m.ServerPrefix)
			if sp == prefix || !strings.HasPrefix(sp, strings.TrimRight(prefix, "/")+"/") {
				continue
			}
			rel := strings.TrimPrefix(sp, strings.TrimRight(prefix, "/")+"/")
			if !filepath.IsLocal(filepath.FromSlash(rel)) {
				continue
			}
			candidates = append(candidates, filepath.Join(m.LocalPath, filepath.FromSlash(rel)))
		}
	}
	if len(candidates) == 0 {
		return "", false, "no storage mapping matches"
	}
	var why []string
	for _, p := range candidates {
		// Open read-only: the source is never written.
		f, err := os.Open(p)
		if err != nil {
			why = append(why, err.Error())
			continue
		}
		st, err := f.Stat()
		f.Close()
		if err != nil {
			why = append(why, err.Error())
			continue
		}
		if !st.Mode().IsRegular() {
			why = append(why, p+" is not a regular file")
			continue
		}
		if in.Size > 0 && st.Size() != in.Size {
			why = append(why, fmt.Sprintf("%s is %d bytes, the server says %d", p, st.Size(), in.Size))
			continue
		}
		return p, true, ""
	}
	return "", false, strings.Join(why, "; ")
}

// wholeFile reports whether the phase reads the whole source (download it first in
// indirect mode).
func wholeFile(p workerproto.Phase) bool { return p == workerproto.PhaseFingerprint }

// resolveInput picks the source of a computation (direct, cached copy, download, URL).
func (w *Worker) resolveInput(j *job, forceIndirect bool) (*inputChoice, error) {
	a := &j.entry.Assignment
	if !forceIndirect {
		if p, ok, why := w.directPath(a); ok {
			return &inputChoice{path: p, mode: workerproto.InputDirect}, nil
		} else if len(w.cfg.Storage) > 0 {
			j.log.Info("direct input not usable, using the media URL", "reason", why)
		}
	}
	if a.Input.URL == "" {
		return nil, &computeError{kind: workerproto.FailureInput, retryable: true,
			err: errors.New("no readable input: no storage mapping matches and the assignment has no media URL")}
	}
	if j.handle != nil {
		j.handle.Release()
		j.handle = nil
	}
	key := ""
	if a.Input.Size > 0 && a.SceneID != 0 {
		key = cache.Key(a.SceneID, a.Input.Size)
	}
	// A cached copy of the scene (from a previous fingerprint download) serves every phase.
	if key != "" {
		if h, ok := w.cache.Lookup(key); ok {
			valid, err := w.validateCached(j, h)
			if err != nil {
				h.Release()
				return nil, err
			}
			if valid {
				j.handle = h
				j.log.Info("using the cached copy of the source", "path", h.Path)
				return &inputChoice{path: h.Path, mode: workerproto.InputIndirect}, nil
			}
			h.Release()
			j.log.Info("cached copy is stale (source changed): dropping it")
			w.cache.Invalidate(key)
		}
	}
	if wholeFile(a.Phase) && key != "" {
		j.setStage(workerproto.StageDownloading)
		start := time.Now()
		h, n, err := w.cache.Fetch(j.ctx, key, a.Input.FileName, a.Input.Size, &jobSource{w: w, j: j},
			func(done, total int64) { j.progress.Store(int32(done * 100 / total)) })
		switch {
		case err == nil:
			j.handle = h
			j.log.Info("source downloaded to the cache", "bytes", n, "ms", time.Since(start).Milliseconds())
			return &inputChoice{path: h.Path, mode: workerproto.InputIndirect, downloadMillis: time.Since(start).Milliseconds(), downloaded: n}, nil
		case j.ctx.Err() != nil:
			return nil, context.Cause(j.ctx)
		case errors.Is(err, errDiscard) || client.IsLeaseGone(err):
			j.cancel(errDiscard)
			return nil, errDiscard
		case errors.Is(err, cache.ErrNoSpace):
			j.log.Warn("source does not fit in the cache: reading the URL directly", "error", err)
		default:
			var ce *computeError
			if errors.As(err, &ce) {
				return nil, err
			}
			return nil, &computeError{kind: workerproto.FailureInput, retryable: true, err: fmt.Errorf("source download failed: %w", err)}
		}
	}
	u, err := w.ensureURL(j)
	if err != nil {
		return nil, err
	}
	return &inputChoice{path: u, mode: workerproto.InputIndirect, viaURL: true}, nil
}

// validateCached checks a cached copy against the server's validators (ETag /
// Last-Modified) when both are known. A transport error keeps the copy (same scene, same
// size); a lease error ends the job.
func (w *Worker) validateCached(j *job, h *cache.Handle) (bool, error) {
	if h.Meta.ETag == "" && h.Meta.LastModified == "" {
		return true, nil
	}
	u, err := w.ensureURL(j)
	if err != nil {
		return false, err
	}
	info, err := w.client.MediaHead(j.ctx, u)
	if client.IsCode(err, workerproto.CodeURLExpired) {
		if u, err = w.refreshURL(j); err != nil {
			return false, err
		}
		info, err = w.client.MediaHead(j.ctx, u)
	}
	switch {
	case err == nil:
	case j.ctx.Err() != nil:
		return false, context.Cause(j.ctx)
	case client.IsLeaseGone(err):
		j.cancel(errDiscard)
		return false, errDiscard
	default:
		return true, nil
	}
	if h.Meta.ETag != "" && info.ETag != "" {
		return h.Meta.ETag == info.ETag, nil
	}
	if h.Meta.LastModified != "" && info.LastModified != "" {
		return h.Meta.LastModified == info.LastModified, nil
	}
	return true, nil
}

// ensureURL returns the media URL, renewed first when it expires within 10 minutes.
func (w *Worker) ensureURL(j *job) (string, error) {
	u, exp := j.url()
	if !exp.IsZero() && exp.Sub(w.serverNow()) < urlRefreshMargin {
		return w.refreshURL(j)
	}
	return u, nil
}

// jobSource gives the cache the job's media URL.
type jobSource struct {
	w *Worker
	j *job
}

func (s *jobSource) URL(context.Context) (string, error)     { return s.w.ensureURL(s.j) }
func (s *jobSource) Refresh(context.Context) (string, error) { return s.w.refreshURL(s.j) }
