// Package worker is the goonhub-worker runtime: session with the server, job intake
// (pull claims or push assignments), computation with goonhub/pkg/compute, resumable
// uploads, completion, the crash-safe outbox and the graceful stop
// (docs/REMOTE_WORKERS.md, section 11).
package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/backoff"
	"goonhub-worker/internal/cache"
	"goonhub-worker/internal/client"
	"goonhub-worker/internal/config"
	"goonhub-worker/internal/diskfree"
	"goonhub-worker/internal/outbox"
	"goonhub-worker/internal/ratelimit"
	"goonhub-worker/internal/tools"
)

// ComputeFunc runs one phase (compute.Run; tests may replace it).
type ComputeFunc func(ctx context.Context, env compute.Env, req compute.Request, phase workerproto.Phase, params workerproto.PhaseParams) (workerproto.PhaseResult, error)

// Options build a Worker.
type Options struct {
	Config  *config.Config
	Tools   *tools.Tools
	Accel   compute.Accel
	Logger  *slog.Logger
	Version string

	// Compute replaces compute.Run (tests).
	Compute ComputeFunc
	// Backoff bounds the retry delays of network calls (zero = 1 s -> 30 s).
	Backoff backoff.Backoff
	// PushListener replaces net.Listen(push.listen) (tests).
	PushListener net.Listener
	// FreeDisk replaces the free disk probe (tests).
	FreeDisk func(path string) (int64, error)
}

// Cancellation causes of a job context.
var (
	// errDiscard: the server said the lease is over (lost, cancelled, committed): drop the
	// outbox entry, no call.
	errDiscard = errors.New("lease ended on the server")
	// errShutdown: graceful stop: release the job.
	errShutdown = errors.New("worker shutting down")
	// errLeaseGone: no successful heartbeat for 2 x lease duration: stop computing.
	errLeaseGone = errors.New("no contact with the server for 2 x lease duration")
	// errKilled: hard stop, leave everything on disk.
	errKilled = errors.New("worker killed")
)

// Worker is one running worker.
type Worker struct {
	cfg      *config.Config
	log      *slog.Logger
	tools    *tools.Tools
	env      compute.Env
	client   *client.Client
	outbox   *outbox.Outbox
	cache    *cache.Cache
	compute  ComputeFunc
	upLimit  *ratelimit.Limiter
	backoff  backoff.Backoff
	version  string
	hostname string
	freeDisk func(string) (int64, error)

	pushToken    string
	pushListener net.Listener

	// killCtx ends on a hard stop: every goroutine returns without touching the outbox.
	killCtx context.Context
	kill    context.CancelCauseFunc
	// stopCh is closed when a graceful stop starts.
	stopCh   chan struct{}
	stopOnce sync.Once
	stopping atomic.Bool

	regMu      sync.Mutex // single-flight register
	sessMu     sync.RWMutex
	sess       session
	registered chan struct{}
	regOnce    sync.Once

	mu             sync.Mutex
	jobs           map[string]*job
	computing      map[workerproto.Phase]int
	computingTotal int
	delivering     int
	drain          bool
	diskLow        bool
	lastFreeDisk   int64
	slotFreed      chan struct{}

	wg sync.WaitGroup // job goroutines
}

// session is the server session and its timings.
type session struct {
	id               string
	workerID         string
	heartbeat        time.Duration
	lease            time.Duration
	claimWait        int
	maxArtifactBytes int64
	chunkBytes       int64
	// offset is server time - local time.
	offset      time.Duration
	lastContact time.Time
}

// New builds a worker: opens the outbox and the cache.
func New(o Options) (*Worker, error) {
	cfg := o.Config
	if cfg == nil || o.Tools == nil {
		return nil, errors.New("worker: config and tools are required")
	}
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}
	ob, err := outbox.Open(cfg.OutboxDir())
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	cl := client.New(cfg.Server, cfg.Token, fmt.Sprintf("goonhub-worker/%s (%s/%s)", o.Version, runtime.GOOS, runtime.GOARCH))
	dl := ratelimit.New(int64(cfg.Bandwidth.Download))
	ca, err := cache.Open(cfg.Cache.Dir, int64(cfg.Cache.MaxSize), cl, dl, log)
	if err != nil {
		return nil, err
	}
	ca.Backoff = o.Backoff
	fn := o.Compute
	if fn == nil {
		fn = compute.Run
	}
	fd := o.FreeDisk
	if fd == nil {
		fd = diskfree.Free
	}
	w := &Worker{
		cfg:      cfg,
		log:      log,
		tools:    o.Tools,
		env:      compute.Env{FFmpegPath: o.Tools.FFmpeg, FFprobePath: o.Tools.FFprobe, FpcalcPath: o.Tools.Fpcalc, Accel: o.Accel},
		client:   cl,
		outbox:   ob,
		cache:    ca,
		compute:  fn,
		upLimit:  ratelimit.New(int64(cfg.Bandwidth.Upload)),
		backoff:  o.Backoff,
		version:  o.Version,
		hostname: host,
		freeDisk: fd,

		pushListener: o.PushListener,
		stopCh:       make(chan struct{}),
		registered:   make(chan struct{}),
		jobs:         map[string]*job{},
		computing:    map[workerproto.Phase]int{},
		slotFreed:    make(chan struct{}, 1),
	}
	w.killCtx, w.kill = context.WithCancelCause(context.Background())
	if cfg.Mode == workerproto.ConnPush {
		w.pushToken = cfg.Push.Secret
		if w.pushToken == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				return nil, fmt.Errorf("failed to generate the push token: %w", err)
			}
			w.pushToken = hex.EncodeToString(b)
		}
	}
	return w, nil
}

// Kill stops at once: running computations are killed, nothing is released nor removed
// from the outbox (the server's lease expiry requeues the jobs; a restart resumes what it
// can). Run returns soon after.
func (w *Worker) Kill() { w.kill(errKilled) }

func (w *Worker) killed() bool { return w.killCtx.Err() != nil }

// Run runs the worker until ctx ends (graceful stop: computations cancelled and released,
// computed jobs given shutdown_grace to finish, release-all) or Kill.
func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("goonhub-worker starting",
		"version", w.version, "name", w.cfg.Name, "server", w.cfg.Server, "mode", w.cfg.Mode,
		"phases", w.cfg.EnabledPhases(), "max_concurrency", w.cfg.MaxConcurrency,
		"accel", w.env.Accel.Label(), "audio_fingerprint", w.tools.HasAudioFingerprint(),
		"data_dir", w.cfg.DataDir, "cache_dir", w.cfg.Cache.Dir)
	w.checkMappings()
	w.loadOutbox()
	w.updateDisk()

	var bg sync.WaitGroup
	var pushSrv *pushServer
	if w.cfg.Mode == workerproto.ConnPush {
		srv, err := w.startPush()
		if err != nil {
			w.kill(err)
			return err
		}
		pushSrv = srv
	}

	// Graceful stop trigger.
	go func() {
		select {
		case <-ctx.Done():
			w.beginStop()
		case <-w.killCtx.Done():
		}
	}()

	if err := w.registerLoop(); err != nil {
		w.kill(err)
		if pushSrv != nil {
			pushSrv.close()
		}
		w.wg.Wait()
		return err
	}

	hbCtx, hbCancel := context.WithCancel(w.killCtx)
	if w.sessionID() != "" {
		bg.Add(1)
		go func() { defer bg.Done(); w.heartbeatLoop(hbCtx) }()
		if w.cfg.Mode == workerproto.ConnPull {
			bg.Add(1)
			go func() { defer bg.Done(); w.claimLoop() }()
		}
	}

	select {
	case <-w.stopCh:
	case <-w.killCtx.Done():
	}
	if !w.killed() {
		w.shutdown()
	}
	if pushSrv != nil {
		pushSrv.close()
	}
	w.wg.Wait()
	hbCancel()
	bg.Wait()
	if w.killed() {
		w.log.Warn("worker killed: leases will expire on the server, the outbox is kept for the next start")
		return nil
	}
	w.kill(errors.New("stopped"))
	w.log.Info("worker stopped")
	return nil
}

// beginStop starts the graceful stop once.
func (w *Worker) beginStop() {
	w.stopOnce.Do(func() {
		w.stopping.Store(true)
		close(w.stopCh)
	})
}

// shutdown is the graceful stop (section 11 "Graceful stop").
func (w *Worker) shutdown() {
	w.log.Info("graceful stop: cancelling computations and releasing their jobs")
	// 1. computations: cancel and release (each job goroutine releases itself).
	var delivering []*job
	w.mu.Lock()
	for _, j := range w.jobs {
		if j.isDelivering() {
			delivering = append(delivering, j)
		} else {
			j.cancel(errShutdown)
		}
	}
	w.mu.Unlock()

	// 2. computed jobs: finish within the grace, then release.
	if len(delivering) > 0 {
		grace := w.cfg.ShutdownGrace.D()
		w.log.Info("graceful stop: letting computed jobs finish their upload", "jobs", len(delivering), "grace", grace)
		deadline := time.NewTimer(grace)
		defer deadline.Stop()
		for _, j := range delivering {
			select {
			case <-j.done:
			case <-deadline.C:
				for _, j := range delivering {
					j.cancel(errShutdown)
				}
			case <-w.killCtx.Done():
				return
			}
		}
	}
	w.wg.Wait()

	// 3. safety net.
	sid := w.sessionID()
	if sid != "" && !w.killed() {
		ctx, cancel := context.WithTimeout(w.killCtx, 5*time.Second)
		defer cancel()
		res, err := w.client.ReleaseAll(ctx, &workerproto.ReleaseAllRequest{SessionID: sid, Reason: workerproto.ReleaseShutdown})
		if err != nil {
			w.log.Warn("release-all failed (leases will expire on the server)", "error", err)
		} else if res.Released > 0 {
			w.log.Info("release-all released leftover leases", "released", res.Released)
		}
	}
}

// checkMappings logs the state of the direct-mode mounts.
func (w *Worker) checkMappings() {
	for _, m := range w.cfg.Storage {
		st, err := os.Stat(m.LocalPath)
		switch {
		case err != nil:
			w.log.Warn("storage mapping not accessible: its jobs fall back to indirect mode", "local_path", m.LocalPath, "error", err)
		case !st.IsDir():
			w.log.Warn("storage mapping is not a directory: its jobs fall back to indirect mode", "local_path", m.LocalPath)
		default:
			w.log.Info("direct mode mapping (mount it read-only)", "storage_path_id", m.StoragePathID, "server_prefix", m.ServerPrefix, "local_path", m.LocalPath)
		}
	}
}

// updateDisk refreshes the free disk view; below min_free_disk the worker stops taking
// jobs.
func (w *Worker) updateDisk() int64 {
	free := int64(-1)
	for _, dir := range []string{w.cfg.DataDir, w.cfg.Cache.Dir} {
		n, err := w.freeDisk(dir)
		if err != nil {
			continue
		}
		if free < 0 || n < free {
			free = n
		}
	}
	low := free >= 0 && free < int64(w.cfg.MinFreeDisk)
	w.mu.Lock()
	if low && !w.diskLow {
		w.log.Warn("free disk below min_free_disk: not taking jobs", "free", config.FormatBytes(free), "min_free_disk", config.FormatBytes(int64(w.cfg.MinFreeDisk)))
	} else if !low && w.diskLow {
		w.log.Info("free disk back above min_free_disk", "free", config.FormatBytes(free))
	}
	w.diskLow = low
	if free >= 0 {
		w.lastFreeDisk = free
	}
	w.mu.Unlock()
	if !low {
		w.signalSlot()
	}
	return free
}

func (w *Worker) signalSlot() {
	select {
	case w.slotFreed <- struct{}{}:
	default:
	}
}

// accepting reports whether new jobs may start.
func (w *Worker) acceptingLocked() bool {
	return !w.stopping.Load() && !w.drain && !w.diskLow && w.delivering < w.cfg.MaxPendingUploads && w.sessionID() != ""
}

// freeSlotsLocked returns the claimable slots per phase: each phase's free slots capped
// by the free global concurrency. When several phases have work the server may return
// more than the global cap: the extra assignments are released at once (overloaded, no
// attempt counted), which beats leaving a phase out of the claim for a whole long-poll.
func (w *Worker) freeSlotsLocked() map[workerproto.Phase]int {
	slots := map[workerproto.Phase]int{}
	for p, n := range w.freeSlotsPerPhaseLocked() {
		if n > 0 {
			slots[p] = n
		}
	}
	return slots
}

// freeSlotsPerPhaseLocked is the push status view: each phase's free slots capped by the
// free global concurrency (not summed).
func (w *Worker) freeSlotsPerPhaseLocked() map[workerproto.Phase]int {
	slots := map[workerproto.Phase]int{}
	accepting := w.acceptingLocked()
	remaining := w.cfg.MaxConcurrency - w.computingTotal
	for _, p := range w.cfg.EnabledPhases() {
		n := 0
		if accepting {
			n = max(0, min(w.cfg.Phases[p]-w.computing[p], remaining))
		}
		slots[p] = n
	}
	return slots
}
