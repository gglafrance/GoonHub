package worker_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/fakeserver"
)

// End-to-end tests: a real worker against the fake server of internal/fakeserver
// (lease 3 s, heartbeat 1 s). Tests named *Real run the real computation with ffmpeg and
// skip without it.

func TestPullSpritesIndirectReal(t *testing.T) {
	tl := requireTools(t)
	clip := testClip(t, 10, false)
	h := newHarness(t)
	id := h.srv.AddJob(workerproto.PhaseSprites, 42, clip, videoInfo(10, false), spritesParams(42))
	h.start(workerOpts{tools: tl})

	j := mustJob(t, h.srv, id, 60*time.Second, isStatus(fakeserver.Completed), "completed")
	res := j.Committed.Result.Sprites
	if res == nil || res.SheetCount < 1 || res.VTT != workerproto.VTTName(42) {
		t.Fatalf("unexpected sprites result: %+v", res)
	}
	vtt := j.CommittedFiles[workerproto.VTTName(42)]
	if !bytes.HasPrefix(vtt, []byte("WEBVTT")) || !bytes.Contains(vtt, []byte("42_sheet_001.webp#xywh=")) {
		t.Fatalf("unexpected VTT:\n%s", vtt)
	}
	if sheet := j.CommittedFiles[workerproto.SpriteSheetName(42, 1)]; len(sheet) < 12 || string(sheet[8:12]) != "WEBP" {
		t.Fatalf("sheet is not a WebP (%d bytes)", len(sheet))
	}
	if j.Committed.Stats.InputMode != workerproto.InputIndirect {
		t.Fatalf("input mode %q, want indirect", j.Committed.Stats.InputMode)
	}
	if mediaBytes, reqs, _, _ := h.srv.Counters(); reqs == 0 || mediaBytes == 0 {
		t.Fatalf("indirect sprites should read the media URL (requests %d bytes %d)", reqs, mediaBytes)
	}
	if j.RetryCount != 0 || j.Completes != 1 {
		t.Fatalf("retry %d completes %d, want 0 and 1", j.RetryCount, j.Completes)
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
}

func TestPullAllPhasesDirectReal(t *testing.T) {
	tl := requireTools(t)
	clip := testClip(t, 10, false)
	h := newHarness(t)
	sprites := h.srv.AddJob(workerproto.PhaseSprites, 5, clip, videoInfo(10, false), spritesParams(5))
	animated := h.srv.AddJob(workerproto.PhaseAnimatedThumbnails, 5, clip, videoInfo(10, false), animatedParams(5))
	fp := h.srv.AddJob(workerproto.PhaseFingerprint, 5, clip, videoInfo(10, false), fingerprintParams())
	h.start(workerOpts{tools: tl, storage: filepath.Dir(clip)})

	for _, id := range []string{sprites, animated, fp} {
		j := mustJob(t, h.srv, id, 90*time.Second, isStatus(fakeserver.Completed), "completed")
		if j.Committed.Stats.InputMode != workerproto.InputDirect {
			t.Fatalf("%s: input mode %q, want direct", j.Phase, j.Committed.Stats.InputMode)
		}
	}
	a := h.srv.Job(animated).Committed.Result.AnimatedThumbnails
	if len(a.Markers) != 1 || a.Markers[0].File != workerproto.MarkerClipName(7) || a.Preview == nil {
		t.Fatalf("unexpected animated result: %+v", a)
	}
	if len(h.srv.Job(animated).CommittedFiles[workerproto.MarkerClipName(7)]) == 0 {
		t.Fatal("marker clip not uploaded")
	}
	f := h.srv.Job(fp).Committed.Result.Fingerprint
	if len(f.VisualFingerprint) < 3 || len(f.AudioFingerprint) != 0 {
		t.Fatalf("silent clip: want a visual fingerprint only, got visual %d audio %d", len(f.VisualFingerprint), len(f.AudioFingerprint))
	}
	if _, reqs, _, _ := h.srv.Counters(); reqs != 0 {
		t.Fatalf("direct mode made %d media requests", reqs)
	}
	// The source was only read.
	if st, err := os.Stat(clip); err != nil || st.Size() == 0 {
		t.Fatalf("source changed: %v", err)
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
}

func TestFingerprintAudioReal(t *testing.T) {
	tl := requireTools(t)
	if tl.Fpcalc == "" {
		t.Skip("fpcalc (chromaprint) not available: audio fingerprint not tested here")
	}
	clip := testClip(t, 20, true)
	h := newHarness(t)
	id := h.srv.AddJob(workerproto.PhaseFingerprint, 9, clip, videoInfo(20, true), fingerprintParams())
	h.start(workerOpts{tools: tl})
	j := mustJob(t, h.srv, id, 60*time.Second, isStatus(fakeserver.Completed), "completed")
	if len(j.Committed.Result.Fingerprint.AudioFingerprint) == 0 {
		t.Fatalf("no audio fingerprint: %+v", j.Committed.Result.Fingerprint)
	}
}

func TestCacheReuseAcrossPhasesReal(t *testing.T) {
	tl := requireTools(t)
	clip := testClip(t, 10, false)
	st, _ := os.Stat(clip)
	h := newHarness(t)
	fp := h.srv.AddJob(workerproto.PhaseFingerprint, 77, clip, videoInfo(10, false), fingerprintParams())
	r := h.start(workerOpts{tools: tl})
	mustJob(t, h.srv, fp, 60*time.Second, isStatus(fakeserver.Completed), "fingerprint completed")
	mediaBytes, reqs, _, _ := h.srv.Counters()
	if mediaBytes != st.Size() || reqs != 1 {
		t.Fatalf("fingerprint download: %d bytes in %d requests, want %d in 1", mediaBytes, reqs, st.Size())
	}

	sp := h.srv.AddJob(workerproto.PhaseSprites, 77, clip, videoInfo(10, false), spritesParams(77))
	mustJob(t, h.srv, sp, 60*time.Second, isStatus(fakeserver.Completed), "sprites completed")
	mediaBytes2, reqs2, _, _ := h.srv.Counters()
	if mediaBytes2 != mediaBytes || reqs2 != reqs {
		t.Fatalf("sprites of the same scene should use the cached copy: media bytes %d -> %d, GETs %d -> %d", mediaBytes, mediaBytes2, reqs, reqs2)
	}
	r.stop(t, 10*time.Second)
	// The cached copy stays (LRU), the outbox is empty.
	cached, _ := filepath.Glob(filepath.Join(h.dataDir, "cache", "scene-77-*", filepath.Base(clip)))
	if len(cached) != 1 {
		t.Fatalf("cached copy missing: %v", cached)
	}
	waitOutboxEmpty(t, h.dataDir, time.Second)
}

func TestPushModeReal(t *testing.T) {
	tl := requireTools(t)
	clip := testClip(t, 10, false)
	h := newHarness(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	id := h.srv.AddJob(workerproto.PhaseSprites, 3, clip, videoInfo(10, false), spritesParams(3))
	h.start(workerOpts{tools: tl, mode: workerproto.ConnPush, listener: ln})

	// Wait for the register, then check the push auth.
	deadline := time.Now().Add(10 * time.Second)
	for h.srv.PushToken() == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+workerproto.PathPushStatus, nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong push token: status %d, want 401", resp.StatusCode)
	}

	// Nothing is claimed in push mode until the server assigns.
	time.Sleep(1500 * time.Millisecond)
	if j := h.srv.Job(id); j.Status != fakeserver.Pending {
		t.Fatalf("push worker claimed by itself: %s", j.Status)
	}
	ids, err := h.srv.PushDispatch(context.Background())
	if err != nil || len(ids) != 1 {
		t.Fatalf("dispatch: %v %v", ids, err)
	}
	j := mustJob(t, h.srv, id, 60*time.Second, isStatus(fakeserver.Completed), "completed")
	if j.Committed.Result.Sprites == nil {
		t.Fatal("no sprites result")
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
}

func TestPushBusyAndCancel(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, 1000)
	a := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	b := h.srv.AddJob(workerproto.PhaseSprites, 2, src, videoInfo(10, false), spritesParams(2))
	h.start(workerOpts{mode: workerproto.ConnPush, listener: ln, compute: fc.Run})
	deadline := time.Now().Add(10 * time.Second)
	for h.srv.PushToken() == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	ids, err := h.srv.PushDispatch(context.Background())
	if err != nil || len(ids) != 1 || ids[0] != a {
		t.Fatalf("first dispatch: %v %v (sprites has 1 slot)", ids, err)
	}
	fc.waitStarted(t, 10*time.Second)
	// The sprites slot is busy: status reports 0 free, the job stays pending.
	ids, _ = h.srv.PushDispatch(context.Background())
	if len(ids) != 0 || h.srv.Job(b).Status != fakeserver.Pending {
		t.Fatalf("second dispatch assigned %v while busy", ids)
	}
	// Cancel by the user: push cancel, then the heartbeat confirms.
	gen := h.srv.Job(a).Generation
	h.srv.Cancel(a)
	if err := h.srv.PushCancel(context.Background(), a, gen); err != nil {
		t.Fatal(err)
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
	if fc.cancels.Load() != 1 {
		t.Fatalf("computation not cancelled (cancels %d)", fc.cancels.Load())
	}
	// The slot is free again: b gets assigned and completes.
	deadline = time.Now().Add(10 * time.Second)
	for {
		ids, _ = h.srv.PushDispatch(context.Background())
		if len(ids) == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	fc.release()
	mustJob(t, h.srv, b, 20*time.Second, isStatus(fakeserver.Completed), "b completed")
	if j := h.srv.Job(a); j.Status != fakeserver.Cancelled || j.Completes != 0 {
		t.Fatalf("cancelled job: %+v", j)
	}
}

// writeSource writes a dummy source file (fake computations do not read it).
func writeSource(t *testing.T, n int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(p, bytes.Repeat([]byte{1}, n), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGracefulStopReleasesJobs(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	a := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	b := h.srv.AddJob(workerproto.PhaseFingerprint, 1, src, videoInfo(10, false), fingerprintParams())
	r := h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	fc.waitStarted(t, 10*time.Second)

	took := r.stop(t, 10*time.Second)
	if took > 5*time.Second {
		t.Fatalf("graceful stop took %s", took)
	}
	for _, id := range []string{a, b} {
		j := h.srv.Job(id)
		if j.Status != fakeserver.Pending || j.RetryCount != 0 || len(j.Releases) != 1 || j.Releases[0] != workerproto.ReleaseShutdown {
			t.Fatalf("job %s after graceful stop: status %s retry %d releases %v (want pending, 0, [shutdown])", id, j.Status, j.RetryCount, j.Releases)
		}
	}
	if fc.cancels.Load() != 2 {
		t.Fatalf("computations cancelled: %d, want 2", fc.cancels.Load())
	}
	if e := outboxEntries(t, h.dataDir); len(e) != 0 {
		t.Fatalf("outbox not empty after graceful stop: %v", e)
	}
	var releaseAlls int
	h.srv.Snapshot(func(s *fakeserver.Server) { releaseAlls = s.ReleaseAlls })
	if releaseAlls != 1 {
		t.Fatalf("release-all calls %d, want 1", releaseAlls)
	}
}

func TestGracefulStopFinishesComputedJobWithinGrace(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(false)
	src := writeSource(t, 1000)
	h.srv.FailNextCompletes(2) // the complete needs a few retries
	a := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	r := h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src), shutdownSec: 5})
	fc.waitStarted(t, 10*time.Second)
	time.Sleep(100 * time.Millisecond)
	r.stop(t, 10*time.Second)
	if j := h.srv.Job(a); j.Status != fakeserver.Completed {
		t.Fatalf("computed job should complete during the grace: %s releases %v", j.Status, j.Releases)
	}
}

func TestKilledMidComputeLeaseExpires(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	r := h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	gen := h.srv.Job(id).Generation
	r.kill(t)

	// Nothing released: the lease expires on the server (3 s), attempt + 1.
	j := mustJob(t, h.srv, id, 10*time.Second, isStatus(fakeserver.Pending), "pending after lease expiry")
	if j.RetryCount != 1 || len(j.Releases) != 0 || j.Generation != gen+1 {
		t.Fatalf("after expiry: retry %d releases %v gen %d (want 1, none, %d)", j.RetryCount, j.Releases, j.Generation, gen+1)
	}
	if e := outboxEntries(t, h.dataDir); len(e) != 1 {
		t.Fatalf("killed worker should leave its outbox entry: %v", e)
	}

	// Restart with the same data dir: the stale entry is dropped, the job is done again.
	fc2 := newFakeCompute(false)
	h.start(workerOpts{compute: fc2.Run, storage: filepath.Dir(src)})
	j = mustJob(t, h.srv, id, 20*time.Second, isStatus(fakeserver.Completed), "completed after restart")
	if j.Completes != 1 {
		t.Fatalf("completes %d", j.Completes)
	}
	found := false
	h.srv.Snapshot(func(s *fakeserver.Server) {
		for _, ref := range s.Resumes {
			if ref.JobID == id && ref.Generation == gen {
				found = true
			}
		}
	})
	if !found {
		t.Fatal("restart did not send the outbox entry in resume")
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
}

func TestCrashRestartBeforeExpiryReleasesPartialCompute(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	r := h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	r.kill(t)

	// Quick restart, lease still valid: the partial computation is released with
	// crash_recovery (counts an attempt), then claimed again and completed.
	fc2 := newFakeCompute(false)
	h.start(workerOpts{compute: fc2.Run, storage: filepath.Dir(src)})
	j := mustJob(t, h.srv, id, 20*time.Second, isStatus(fakeserver.Completed), "completed")
	if len(j.Releases) != 1 || j.Releases[0] != workerproto.ReleaseCrashRecovery || j.RetryCount != 1 {
		t.Fatalf("releases %v retry %d, want [crash_recovery] and 1", j.Releases, j.RetryCount)
	}
}

func TestCrashAfterComputeResumesUpload(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(false)
	src := writeSource(t, 1000)
	h.srv.FailNextCompletes(1000) // keeps the job in "completing"
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	r := h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	deadline := time.Now().Add(10 * time.Second)
	for fc.finished.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	r.kill(t)
	h.srv.FailNextCompletes(0)

	fc2 := newFakeCompute(false)
	h.start(workerOpts{compute: fc2.Run, storage: filepath.Dir(src)})
	j := mustJob(t, h.srv, id, 20*time.Second, isStatus(fakeserver.Completed), "completed")
	if fc2.runs.Load() != 0 {
		t.Fatalf("the computed result should be resumed, not recomputed (runs %d)", fc2.runs.Load())
	}
	if j.RetryCount != 0 || len(j.Releases) != 0 {
		t.Fatalf("retry %d releases %v, want none", j.RetryCount, j.Releases)
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
}

func TestNetworkCutDuringUploadResumes(t *testing.T) {
	h := newHarness(t)
	h.srv.ChunkBytes = 4 << 20
	fc := newFakeCompute(false)
	fc.sheetBytes = 1 << 20
	src := writeSource(t, 1000)
	h.srv.CutNextUpload(300 << 10)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	j := mustJob(t, h.srv, id, 30*time.Second, isStatus(fakeserver.Completed), "completed")

	var total int64
	for _, ref := range j.Committed.Artifacts {
		total += ref.Size
	}
	_, _, uploaded, puts := h.srv.Counters()
	if uploaded != total {
		t.Fatalf("uploaded %d bytes for %d of artifacts: the cut upload restarted instead of resuming", uploaded, total)
	}
	if puts < 3 {
		t.Fatalf("expected the cut PUT and a resumed one (puts %d)", puts)
	}
	sheet := j.CommittedFiles[workerproto.SpriteSheetName(1, 1)]
	if int64(len(sheet)) != 1<<20 {
		t.Fatalf("committed sheet %d bytes", len(sheet))
	}
}

func TestNetworkDownShorterThanLease(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	h.srv.SetDown(true)
	time.Sleep(1500 * time.Millisecond)
	fc.release() // finishes while the server is unreachable: uploads retry
	time.Sleep(500 * time.Millisecond)
	h.srv.SetDown(false)
	j := mustJob(t, h.srv, id, 20*time.Second, isStatus(fakeserver.Completed), "completed")
	if j.RetryCount != 0 {
		t.Fatalf("retry %d, want 0 (outage shorter than the lease)", j.RetryCount)
	}
}

func TestServerRestartLosesLeasesWorkerDiscards(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	gen := h.srv.Job(id).Generation
	h.srv.Restart()

	// unknown_session -> register with resume -> lease_lost -> the computation stops,
	// the job is claimed again with a new generation.
	fc.waitStarted(t, 15*time.Second)
	if fc.cancels.Load() != 1 {
		t.Fatalf("first computation not cancelled (cancels %d)", fc.cancels.Load())
	}
	fc.release()
	j := mustJob(t, h.srv, id, 20*time.Second, isStatus(fakeserver.Completed), "completed")
	if j.CommittedGeneration <= gen+1 {
		t.Fatalf("committed generation %d, want > %d", j.CommittedGeneration, gen+1)
	}
	var registers int
	h.srv.Snapshot(func(s *fakeserver.Server) { registers = s.RegisterCalls })
	if registers < 2 {
		t.Fatalf("register calls %d, want a re-register", registers)
	}
}

func TestZombieDuringComputeDropsJob(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	h.srv.Steal(id) // another worker owns it now

	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
	if fc.cancels.Load() != 1 {
		t.Fatalf("zombie computation not stopped")
	}
	j := h.srv.Job(id)
	if j.Status != fakeserver.Running || j.Worker != "other" || j.Completes != 0 || len(j.Releases) != 0 || j.Failure != nil {
		t.Fatalf("the zombie touched the stolen job: %+v", j)
	}
}

func TestZombieAtCompleteDropsJob(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(false)
	src := writeSource(t, 1000)
	h.srv.FailNextCompletes(1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	deadline := time.Now().Add(10 * time.Second)
	for fc.finished.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	h.srv.Steal(id)
	h.srv.FailNextCompletes(0)
	waitOutboxEmpty(t, h.dataDir, 10*time.Second)
	if j := h.srv.Job(id); j.Status != fakeserver.Running || j.Completes != 0 {
		t.Fatalf("zombie committed: %+v", j)
	}
}

func TestCancelledOnServerStopsComputation(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(true)
	src := writeSource(t, 1000)
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: filepath.Dir(src)})
	fc.waitStarted(t, 10*time.Second)
	h.srv.Cancel(id)
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
	if fc.cancels.Load() != 1 {
		t.Fatal("cancelled job kept computing")
	}
}

func TestIndirectDownloadResumesAfterCut(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(false)
	src := writeSource(t, 3<<20)
	id := h.srv.AddJob(workerproto.PhaseFingerprint, 8, src, videoInfo(10, false), fingerprintParams())
	// Cut the network shortly after the download started.
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if b, _, _, _ := h.srv.Counters(); b > 0 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		h.srv.SetDown(true)
		time.Sleep(300 * time.Millisecond)
		h.srv.SetDown(false)
	}()
	h.start(workerOpts{compute: fc.Run, extraYAML: "bandwidth:\n  download: 4MiB\n"})
	j := mustJob(t, h.srv, id, 30*time.Second, isStatus(fakeserver.Completed), "completed")
	if j.Committed.Stats.InputMode != workerproto.InputIndirect || j.Committed.Stats.BytesDownloaded == 0 {
		t.Fatalf("stats %+v", j.Committed.Stats)
	}
	cached, _ := filepath.Glob(filepath.Join(h.dataDir, "cache", "scene-8-*", "source.mp4"))
	if len(cached) != 1 {
		t.Fatalf("download not in the cache: %v", cached)
	}
	got, _ := os.ReadFile(cached[0])
	want, _ := os.ReadFile(src)
	if !bytes.Equal(got, want) {
		t.Fatal("resumed download differs from the source")
	}
	var ranges int
	h.srv.Snapshot(func(s *fakeserver.Server) { ranges = s.MediaRanges })
	mediaBytes, _, _, _ := h.srv.Counters()
	t.Logf("media bytes served %d for %d (range requests %d)", mediaBytes, len(want), ranges)
	if ranges == 0 {
		t.Fatal("the download was not resumed with a Range request after the cut")
	}
}

func TestDirectFallsBackToIndirectOnSizeMismatch(t *testing.T) {
	h := newHarness(t)
	fc := newFakeCompute(false)
	src := writeSource(t, 1000)
	mount := t.TempDir()
	// Same name, other size: the mapping must not be trusted.
	if err := os.WriteFile(filepath.Join(mount, filepath.Base(src)), []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, src, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{compute: fc.Run, storage: mount})
	j := mustJob(t, h.srv, id, 20*time.Second, isStatus(fakeserver.Completed), "completed")
	if j.Committed.Stats.InputMode != workerproto.InputIndirect {
		t.Fatalf("input mode %s, want indirect after the size mismatch", j.Committed.Stats.InputMode)
	}
	in := <-fc.started
	if !strings.HasPrefix(in, "http://") {
		t.Fatalf("compute input %q, want the media URL", in)
	}
}

func TestComputeFailureReportsFail(t *testing.T) {
	tl := requireTools(t)
	h := newHarness(t)
	bad := filepath.Join(t.TempDir(), "broken.mp4")
	if err := os.WriteFile(bad, []byte("not a video at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	id := h.srv.AddJob(workerproto.PhaseSprites, 1, bad, videoInfo(10, false), spritesParams(1))
	h.start(workerOpts{tools: tl, storage: filepath.Dir(bad)})
	j := mustJob(t, h.srv, id, 30*time.Second, isStatus(fakeserver.Failed), "failed")
	if j.Failure.Kind != workerproto.FailureCompute || !j.Failure.Retryable {
		t.Fatalf("failure %+v", j.Failure)
	}
	waitOutboxEmpty(t, h.dataDir, 5*time.Second)
}
