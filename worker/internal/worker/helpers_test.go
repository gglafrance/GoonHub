package worker_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/backoff"
	"goonhub-worker/internal/config"
	"goonhub-worker/internal/fakeserver"
	"goonhub-worker/internal/tools"
	"goonhub-worker/internal/worker"
)

func init() {
	worker.SignalSettle = 50 * time.Millisecond
}

// testLogger writes worker logs to the test log (visible with -v or on failure).
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(&tWriter{t: t}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

type tWriter struct {
	t  *testing.T
	mu sync.Mutex
}

func (w *tWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// ---------------------------------------------------------------------------------------
// ffmpeg
// ---------------------------------------------------------------------------------------

var (
	toolsOnce sync.Once
	realTools *tools.Tools
	toolsErr  error
)

// requireTools returns the real ffmpeg tools or skips.
func requireTools(t *testing.T) *tools.Tools {
	t.Helper()
	toolsOnce.Do(func() {
		realTools, toolsErr = tools.Find(context.Background(), tools.Configured{})
	})
	if toolsErr != nil {
		t.Skipf("ffmpeg/ffprobe not available: %v", toolsErr)
	}
	return realTools
}

// fakeTools are enough when the computation is replaced.
func fakeTools() *tools.Tools {
	return &tools.Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", FFmpegVersion: "test"}
}

var (
	clipMu sync.Mutex
	clips  = map[string]string{}
)

// testClip renders (once per test binary) a small clip.
func testClip(t *testing.T, seconds int, audio bool) string {
	t.Helper()
	tl := requireTools(t)
	key := fmt.Sprintf("%d-%v", seconds, audio)
	clipMu.Lock()
	defer clipMu.Unlock()
	if p, ok := clips[key]; ok {
		return p
	}
	dir, err := os.MkdirTemp("", "goonhub-worker-clips-*")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, fmt.Sprintf("clip_%s.mp4", strings.ReplaceAll(key, "-", "_")))
	args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=25", seconds)}
	if audio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds), "-c:a", "aac")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", p)
	if out, err := exec.Command(tl.FFmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("failed to render the test clip: %v: %s", err, out)
	}
	clips[key] = p
	return p
}

func videoInfo(seconds int, audio bool) workerproto.VideoInfo {
	v := workerproto.VideoInfo{DurationSec: seconds, Width: 320, Height: 240, FrameRate: 25, VideoCodec: "h264"}
	if audio {
		v.AudioCodec = "aac"
	}
	return v
}

func spritesParams(scene uint) workerproto.PhaseParams {
	return workerproto.PhaseParams{Sprites: &workerproto.SpritesParams{SceneID: scene, TileWidth: 160, TileHeight: 120,
		FrameInterval: 1, FrameQuality: 70, GridCols: 4, GridRows: 4}}
}

func animatedParams(scene uint) workerproto.PhaseParams {
	return workerproto.PhaseParams{AnimatedThumbnails: &workerproto.AnimatedThumbnailsParams{
		SceneID:     scene,
		Markers:     []workerproto.MarkerClipSpec{{MarkerID: 7, TimestampSec: 2, DurationSec: 2, Output: workerproto.MarkerClipName(7)}},
		MarkerWidth: 160, MarkerCRF: 30, MarkerTimeoutSec: 60,
		Preview: &workerproto.ScenePreviewSpec{Output: workerproto.ScenePreviewName(scene), Segments: 3, SegmentDuration: 1, Width: 160, CRF: 30},
	}}
}

func fingerprintParams() workerproto.PhaseParams {
	return workerproto.PhaseParams{Fingerprint: &workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly}}
}

// ---------------------------------------------------------------------------------------
// Fake computation
// ---------------------------------------------------------------------------------------

// fakeCompute produces valid results without ffmpeg. When gate is set, every run waits
// for a value on it (or its context) before writing the outputs.
type fakeCompute struct {
	gate     chan struct{}
	started  chan string // receives the scene id input of each run (non-blocking send)
	runs     atomic.Int32
	finished atomic.Int32
	cancels  atomic.Int32
	// sheetBytes is the size of the sprite sheet written (random content).
	sheetBytes int
}

func newFakeCompute(gated bool) *fakeCompute {
	f := &fakeCompute{started: make(chan string, 16), sheetBytes: 4096}
	if gated {
		f.gate = make(chan struct{}, 16)
	}
	return f
}

func (f *fakeCompute) Run(ctx context.Context, env compute.Env, req compute.Request, phase workerproto.Phase, params workerproto.PhaseParams) (workerproto.PhaseResult, error) {
	f.runs.Add(1)
	select {
	case f.started <- req.Input:
	default:
	}
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			f.cancels.Add(1)
			return workerproto.PhaseResult{}, ctx.Err()
		}
	}
	if err := os.MkdirAll(req.OutDir, 0o755); err != nil {
		return workerproto.PhaseResult{}, err
	}
	switch phase {
	case workerproto.PhaseSprites:
		p := params.Sprites
		sheet := workerproto.SpriteSheetName(p.SceneID, 1)
		buf := make([]byte, f.sheetBytes)
		_, _ = io.ReadFull(rand.Reader, buf)
		if err := os.WriteFile(filepath.Join(req.OutDir, sheet), buf, 0o644); err != nil {
			return workerproto.PhaseResult{}, err
		}
		vtt := workerproto.VTTName(p.SceneID)
		if err := os.WriteFile(filepath.Join(req.OutDir, vtt), []byte("WEBVTT\n"), 0o644); err != nil {
			return workerproto.PhaseResult{}, err
		}
		f.finished.Add(1)
		return workerproto.PhaseResult{Sprites: &workerproto.SpritesResult{SpriteSheets: []string{sheet}, VTT: vtt, SheetCount: 1, FrameInterval: p.FrameInterval}}, nil
	case workerproto.PhaseFingerprint:
		f.finished.Add(1)
		return workerproto.PhaseResult{Fingerprint: &workerproto.FingerprintResult{VisualFingerprint: workerproto.VisualFingerprint{1, 2, 3}}}, nil
	}
	return workerproto.PhaseResult{}, fmt.Errorf("fake compute: phase %s not supported", phase)
}

func (f *fakeCompute) waitStarted(t *testing.T, timeout time.Duration) string {
	t.Helper()
	select {
	case in := <-f.started:
		return in
	case <-time.After(timeout):
		t.Fatal("computation did not start")
	}
	return ""
}

func (f *fakeCompute) release() { f.gate <- struct{}{} }

// ---------------------------------------------------------------------------------------
// Worker harness
// ---------------------------------------------------------------------------------------

type harness struct {
	t       *testing.T
	srv     *fakeserver.Server
	dataDir string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	srv := fakeserver.New()
	t.Cleanup(srv.Close)
	return &harness{t: t, srv: srv, dataDir: t.TempDir()}
}

type workerOpts struct {
	mode        workerproto.ConnectionMode
	storage     string // local dir mapped as storage path 1
	compute     worker.ComputeFunc
	tools       *tools.Tools
	extraYAML   string
	listener    net.Listener
	dataDir     string
	shutdownSec int
}

// config builds a config for the harness server.
func (h *harness) config(o workerOpts) *config.Config {
	h.t.Helper()
	dataDir := o.dataDir
	if dataDir == "" {
		dataDir = h.dataDir
	}
	mode := o.mode
	if mode == "" {
		mode = workerproto.ConnPull
	}
	y := fmt.Sprintf(`server: %q
token: %q
name: test-worker
mode: %s
phases:
  sprites: 1
  animated_thumbnails: 1
  fingerprint: 1
max_concurrency: 2
data_dir: %q
min_free_disk: 1
log:
  file: "-"
shutdown_grace: %ds
`, h.srv.URL(), h.srv.Token, mode, filepath.ToSlash(dataDir), max(o.shutdownSec, 2))
	if mode == workerproto.ConnPush {
		y += fmt.Sprintf("push:\n  listen: %q\n  url: %q\n", o.listener.Addr().String(), "http://"+o.listener.Addr().String())
	}
	if o.storage != "" {
		y += fmt.Sprintf("storage:\n  - storage_path_id: 1\n    local_path: %q\n", filepath.ToSlash(o.storage))
	}
	y += o.extraYAML
	cfg, err := config.Parse([]byte(y), filepath.Join(dataDir, "config.yaml"))
	if err != nil {
		h.t.Fatalf("config: %v\n%s", err, y)
	}
	return cfg
}

type running struct {
	w      *worker.Worker
	cancel context.CancelFunc
	done   chan struct{} // closed when Run returned
	err    error
}

// start runs a worker in the background.
func (h *harness) start(o workerOpts) *running {
	h.t.Helper()
	tl := o.tools
	if tl == nil {
		tl = fakeTools()
	}
	w, err := worker.New(worker.Options{
		Config:       h.config(o),
		Tools:        tl,
		Logger:       testLogger(h.t),
		Version:      "test",
		Compute:      o.compute,
		Backoff:      backoff.Backoff{Min: 50 * time.Millisecond, Max: 400 * time.Millisecond},
		PushListener: o.listener,
	})
	if err != nil {
		h.t.Fatalf("worker.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{w: w, cancel: cancel, done: make(chan struct{})}
	go func() { r.err = w.Run(ctx); close(r.done) }()
	h.t.Cleanup(func() {
		w.Kill()
		select {
		case <-r.done:
		case <-time.After(10 * time.Second):
		}
	})
	return r
}

// stop requests a graceful stop and waits for Run to return.
func (r *running) stop(t *testing.T, timeout time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	r.cancel()
	select {
	case <-r.done:
		if r.err != nil {
			t.Fatalf("Run returned %v", r.err)
		}
	case <-time.After(timeout):
		t.Fatalf("worker did not stop within %s", timeout)
	}
	return time.Since(start)
}

// kill stops at once and waits for Run to return.
func (r *running) kill(t *testing.T) {
	t.Helper()
	r.w.Kill()
	select {
	case <-r.done:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not exit after Kill")
	}
}

// outboxEntries lists the job directories left in the outbox.
func outboxEntries(t *testing.T, dataDir string) []string {
	t.Helper()
	items, err := os.ReadDir(filepath.Join(dataDir, "jobs"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		if it.IsDir() {
			out = append(out, it.Name())
		}
	}
	return out
}

func waitOutboxEmpty(t *testing.T, dataDir string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if len(outboxEntries(t, dataDir)) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("outbox not empty: %v", outboxEntries(t, dataDir))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func isStatus(st string) func(fakeserver.Job) bool {
	return func(j fakeserver.Job) bool { return j.Status == st }
}

func mustJob(t *testing.T, srv *fakeserver.Server, id string, timeout time.Duration, cond func(fakeserver.Job) bool, what string) fakeserver.Job {
	t.Helper()
	j, ok := srv.WaitJob(id, timeout, cond)
	if !ok {
		t.Fatalf("job %s: %s not reached within %s (status %s gen %d retry %d releases %v failure %+v)",
			id, what, timeout, j.Status, j.Generation, j.RetryCount, j.Releases, j.Failure)
	}
	return j
}
