package workers

import (
	"bytes"
	"context"
	"goonhub/internal/config"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/workerproto"
)

// Service tests: one per failure case of docs/REMOTE_WORKERS.md section 13 (and the main
// paths). They run on the test database; see helpers_test.go.

func TestClaimBuildsAssignment(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	s := e.scene("aac")
	j := e.pending(s, workerproto.PhaseSprites)
	a := e.claimJob(w, j)

	if a.Phase != workerproto.PhaseSprites || a.SceneID != s.ID || a.Lease.Generation != 1 || a.Lease.DurationSec != 60 {
		t.Fatalf("assignment %+v", a)
	}
	p := a.Params.Sprites
	if p == nil || p.SceneID != s.ID || p.FrameInterval != 5 || p.GridCols != 12 || p.GridRows != 8 || p.FrameQuality != 75 || p.TileWidth != 320 || p.TileHeight != 180 {
		t.Fatalf("sprites params %+v", p)
	}
	if a.Input.ServerPath != s.StoredPath || a.Input.Size != 18 || a.Input.Video.AudioCodec != "aac" || a.Input.FileName != "video.mp4" {
		t.Fatalf("input %+v", a.Input)
	}
	if !strings.HasPrefix(a.Input.URL, "http://127.0.0.1:8080/api/v1/workers/media/"+j.JobID+"?") {
		t.Fatalf("media URL %s", a.Input.URL)
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusRunning || len(got.LeaseParams) == 0 {
		t.Fatalf("job after claim: %s params=%s", got.Status, got.LeaseParams)
	}
}

func TestClaimRespectsPhasesRoutingScheduleAndConcurrency(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	s := e.scene("")
	sprites := e.pending(s, workerproto.PhaseSprites)
	fp := e.pending(s, workerproto.PhaseFingerprint)

	// Allowed phases: fingerprint only.
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{AllowedPhases: []string{"fingerprint"}}); err != nil {
		t.Fatal(err)
	}
	w, _ = e.svc.Workers.GetByID(w.ID)
	as, _ := e.svc.ClaimFor(w, map[workerproto.Phase]int{"sprites": 5})
	for _, a := range as {
		if a.Phase == workerproto.PhaseSprites {
			t.Fatal("claimed a phase the admin did not allow")
		}
	}

	// Routing: fingerprint local only.
	e.router.mu.Lock()
	e.router.overrides = PhaseRouting{"fingerprint": RouteLocal}
	e.router.mu.Unlock()
	if as, _ := e.svc.ClaimFor(w, map[workerproto.Phase]int{"fingerprint": 5}); len(as) != 0 {
		t.Fatal("claimed a phase routed to the local pools")
	}
	e.router.mu.Lock()
	e.router.overrides = PhaseRouting{}
	e.router.mu.Unlock()

	// Schedule: fingerprint outside its time window.
	e.gate.mu.Lock()
	e.gate.blocked["fingerprint"] = true
	e.gate.mu.Unlock()
	if as, _ := e.svc.ClaimFor(w, map[workerproto.Phase]int{"fingerprint": 5}); len(as) != 0 {
		t.Fatal("claimed a phase outside its time window")
	}
	e.gate.mu.Lock()
	e.gate.blocked = map[string]bool{}
	e.gate.mu.Unlock()

	// Max concurrency 1: one job at most.
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{AllowedPhases: []string{"sprites", "fingerprint"}, MaxConcurrency: intPtr(1)}); err != nil {
		t.Fatal(err)
	}
	w, _ = e.svc.Workers.GetByID(w.ID)
	e.claimJob(w, fp)
	if as, _ := e.svc.ClaimFor(w, map[workerproto.Phase]int{"sprites": 5}); len(as) != 0 {
		t.Fatal("max concurrency exceeded")
	}
	_ = sprites

	// Draining: nothing.
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{Draining: boolPtr(true), MaxConcurrency: intPtr(0)}); err != nil {
		t.Fatal(err)
	}
	w, _ = e.svc.Workers.GetByID(w.ID)
	if as, _ := e.svc.ClaimFor(w, map[workerproto.Phase]int{"sprites": 5}); len(as) != 0 {
		t.Fatal("a draining worker claimed")
	}
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func TestClaimSceneAffinity(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	a := e.scene("")
	b := e.scene("")
	spritesA := e.pending(a, workerproto.PhaseFingerprint)
	// Scene B's animated job has a higher priority than scene A's: affinity comes after priority,
	// so make them equal.
	animB := e.pending(b, workerproto.PhaseSprites)
	animA := e.pending(a, workerproto.PhaseSprites)
	e.db.Model(&data.JobHistory{}).Where("job_id IN ?", []string{animA.JobID, animB.JobID}).Update("priority", 2_000_000)
	e.db.Model(&data.JobHistory{}).Where("job_id = ?", animA.JobID).Update("created_at", time.Now().Add(time.Hour))

	e.claimJob(w, spritesA)
	as, err := e.svc.ClaimFor(w, map[workerproto.Phase]int{"sprites": 1})
	if err != nil || len(as) != 1 {
		t.Fatalf("claim: %v %v", as, err)
	}
	if as[0].JobID != animA.JobID {
		t.Fatalf("claimed scene %d, want the sprites job of the scene the worker already holds (%d)", as[0].SceneID, a.ID)
	}
}

func TestClaimLongPollReturnsWhenAJobArrives(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{Phases: []workerproto.Phase{"animated_thumbnails"},
		Slots: map[workerproto.Phase]int{"animated_thumbnails": 1}})
	s := e.scene("")
	e.animated.plans[s.ID] = &jobs.AnimatedThumbnailsPlan{Params: workerproto.AnimatedThumbnailsParams{SceneID: s.ID,
		Markers: []workerproto.MarkerClipSpec{{MarkerID: 7, TimestampSec: 3, DurationSec: 5, Output: workerproto.MarkerClipName(7)}}}}
	e.svc.cfg.ClaimMaxWait = 3 * time.Second
	done := make(chan *workerproto.ClaimResponse, 1)
	clk := e.clock
	go func() {
		// Real time for the long-poll: the fake clock advances with it.
		stop := time.After(4 * time.Second)
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				clk.Advance(10 * time.Millisecond)
			}
		}
	}()
	go func() {
		resp, err := e.svc.Claim(context.Background(), w, workerproto.ClaimRequest{SessionID: w.SessionID,
			Slots: map[workerproto.Phase]int{"animated_thumbnails": 1}, WaitSec: 3})
		if err != nil {
			t.Errorf("claim: %v", err)
		}
		done <- resp
	}()
	time.Sleep(100 * time.Millisecond)
	j := e.pending(s, workerproto.PhaseAnimatedThumbnails)
	select {
	case resp := <-done:
		found := false
		for _, a := range resp.Assignments {
			if a.JobID == j.JobID {
				found = true
				if a.Params.AnimatedThumbnails == nil || len(a.Params.AnimatedThumbnails.Markers) != 1 {
					t.Fatalf("animated params %+v", a.Params)
				}
			}
		}
		if !found {
			t.Fatalf("long-poll answered without the new job: %+v", resp)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("long-poll never answered")
	}
}

func TestClaimUnknownSession(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	_, err := e.svc.Claim(context.Background(), w, workerproto.ClaimRequest{SessionID: "stale"})
	if protoCode(t, err) != workerproto.CodeUnknownSession {
		t.Fatalf("code %s", protoCode(t, err))
	}
	_, err = e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: "stale"})
	if protoCode(t, err) != workerproto.CodeUnknownSession {
		t.Fatal("heartbeat with an old session")
	}
}

func TestAnimatedNothingToDoCompletesWithoutWorker(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	s := e.scene("")
	j := e.pending(s, workerproto.PhaseAnimatedThumbnails)
	for i := 0; i < 5; i++ {
		e.svc.ClaimFor(w, map[workerproto.Phase]int{"animated_thumbnails": 5})
		if e.job(j.JobID).Status != data.JobStatusPending {
			break
		}
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusCompleted {
		t.Fatalf("status %s, want completed (no plan)", got.Status)
	}
	if r := e.results.forJob(j.JobID); len(r) != 1 || r[0].Status != jobs.JobStatusCompleted {
		t.Fatalf("results %+v", r)
	}
}

func TestClaimFailsJobsOfMissingScenes(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	s := e.scene("")
	j := e.pending(s, workerproto.PhaseSprites)
	e.scenes.remove(s.ID)
	for i := 0; i < 5 && e.job(j.JobID).Status == data.JobStatusPending; i++ {
		e.svc.ClaimFor(w, map[workerproto.Phase]int{"sprites": 5})
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusFailed || got.ErrorMessage == nil || *got.ErrorMessage != "Scene not found" {
		t.Fatalf("job %+v", got)
	}
}

// Worker killed / crash / power loss mid-compute; claim response lost.
func TestKilledWorkerLeaseExpiresAndRequeues(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)

	e.clock.Advance(30 * time.Second)
	if req, _ := e.svc.SweepExpired(); req != 0 {
		t.Fatal("a valid lease expired")
	}
	e.clock.Advance(31 * time.Second)
	e.svc.SweepExpired()
	got := e.job(j.JobID)
	if got.Status != data.JobStatusPending || got.RetryCount != 1 || got.LeaseGeneration != 2 || got.WorkerID != nil {
		t.Fatalf("after expiry: %+v", got)
	}
	ww, _ := e.svc.Workers.GetByID(w.ID)
	if ww.LeasesExpired != 1 {
		t.Fatalf("expired counter %d", ww.LeasesExpired)
	}
	// The worker comes back: everything about the old generation is refused.
	hb, err := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: a.Lease.Generation}}})
	if err != nil || hb.Jobs[0].Status != workerproto.LeaseLost {
		t.Fatalf("heartbeat %+v %v", hb, err)
	}
	if _, err := e.svc.Complete(w, j.JobID, e.uploadSprites(a)); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatalf("complete of an expired lease: %v", err)
	}
	if e.scenes.spriteCommits(a.SceneID) != 0 {
		t.Fatal("a zombie committed")
	}
}

func TestExpiryWithExhaustedAttemptsFailsThroughRetryPath(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	e.db.Model(&data.JobHistory{}).Where("job_id = ?", j.JobID).Update("retry_count", 3)
	e.claimJob(w, j)
	e.clock.Advance(2 * time.Minute)
	e.svc.SweepExpired()
	if got := e.job(j.JobID); got.Status != data.JobStatusFailed {
		t.Fatalf("status %s", got.Status)
	}
	r := e.results.forJob(j.JobID)
	if len(r) != 1 || r[0].Status != jobs.JobStatusFailed || !strings.Contains(r[0].Error.Error(), "lease expired on worker "+w.Name) {
		t.Fatalf("results %+v", r)
	}
}

// Graceful stop (Ctrl+C, SIGTERM, service stop).
func TestGracefulStopRequeuesAtOnceWithoutAttempt(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	s := e.scene("")
	j1 := e.pending(s, workerproto.PhaseSprites)
	j2 := e.pending(e.scene(""), workerproto.PhaseSprites)
	a1 := e.claimJob(w, j1)
	e.claimJob(w, j2)

	if _, err := e.svc.Release(w, j1.JobID, workerproto.ReleaseRequest{Generation: a1.Lease.Generation, Reason: workerproto.ReleaseShutdown}); err != nil {
		t.Fatal(err)
	}
	if got := e.job(j1.JobID); got.Status != data.JobStatusPending || got.RetryCount != 0 || got.LeaseGeneration != 2 {
		t.Fatalf("released job %+v", got)
	}
	// Idempotent: the same release again is lease_lost (ignored by the worker).
	if _, err := e.svc.Release(w, j1.JobID, workerproto.ReleaseRequest{Generation: a1.Lease.Generation}); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatal("second release")
	}
	resp, err := e.svc.ReleaseAll(w, workerproto.ReleaseAllRequest{SessionID: w.SessionID, Reason: workerproto.ReleaseShutdown})
	if err != nil || resp.Released != 1 {
		t.Fatalf("release-all %+v %v", resp, err)
	}
	if got := e.job(j2.JobID); got.Status != data.JobStatusPending || got.RetryCount != 0 {
		t.Fatalf("release-all job %+v", got)
	}
}

func TestCrashRecoveryReleaseCountsAnAttempt(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	if _, err := e.svc.Release(w, j.JobID, workerproto.ReleaseRequest{Generation: a.Lease.Generation, Reason: workerproto.ReleaseCrashRecovery}); err != nil {
		t.Fatal(err)
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusPending || got.RetryCount != 1 {
		t.Fatalf("job %+v", got)
	}
}

// Worker restarted quickly after a crash: register with resume.
func TestRegisterResumesValidLeasesAndReleasesTheOthers(t *testing.T) {
	e := newTestEnv(t)
	w, token := e.worker(workerproto.Capabilities{})
	keep := e.pending(e.scene(""), workerproto.PhaseSprites)
	lost := e.pending(e.scene(""), workerproto.PhaseSprites)
	done := e.pending(e.scene(""), workerproto.PhaseSprites)
	aKeep := e.claimJob(w, keep)
	e.claimJob(w, lost)
	aDone := e.claimJob(w, done)
	if _, err := e.svc.Complete(w, done.JobID, e.uploadSprites(aDone)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	e.clock.Advance(20 * time.Second)
	resp, err := e.svc.Register(w, workerproto.RegisterRequest{APIVersion: 1, Capabilities: defaultCaps(), Resume: []workerproto.JobLeaseRef{
		{JobID: keep.JobID, Generation: aKeep.Lease.Generation},
		{JobID: done.JobID, Generation: aDone.Lease.Generation},
		{JobID: "unknown-job", Generation: 1},
	}}, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]workerproto.LeaseStatus{}
	for _, r := range resp.Resumed {
		statuses[r.JobID] = r.Status
	}
	if statuses[keep.JobID] != workerproto.LeaseOK || statuses[done.JobID] != workerproto.LeaseCommitted || statuses["unknown-job"] != workerproto.LeaseLost {
		t.Fatalf("resumed %+v", resp.Resumed)
	}
	if got := e.job(keep.JobID); got.Status != data.JobStatusRunning || !got.LeaseExpiresAt.After(e.clock.Now().Add(59*time.Second)) {
		t.Fatalf("kept lease not renewed: %+v", got)
	}
	if got := e.job(lost.JobID); got.Status != data.JobStatusPending || got.RetryCount != 1 {
		t.Fatalf("unresumed lease: %+v", got)
	}
	// The old session is dead.
	w2 := e.auth(token)
	if w2.SessionID == w.SessionID {
		t.Fatal("register did not start a new session")
	}
	if _, err := e.svc.Heartbeat(w2, workerproto.HeartbeatRequest{SessionID: w.SessionID}); protoCode(t, err) != workerproto.CodeUnknownSession {
		t.Fatal("old session accepted")
	}
}

func TestRegisterRejectsOtherAPIVersions(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	if _, err := e.svc.Register(w, workerproto.RegisterRequest{APIVersion: 2}, ""); protoCode(t, err) != workerproto.CodeUnsupportedAPIVersion {
		t.Fatal("api version 2 accepted")
	}
	if _, err := e.svc.Register(w, workerproto.RegisterRequest{APIVersion: 1, ConnectionMode: "push"}, ""); protoCode(t, err) != workerproto.CodeInvalidRequest {
		t.Fatal("push without URL accepted")
	}
}

// Zombie worker / two workers think they own a job.
func TestZombieWithStaleGenerationIsRefusedEverywhere(t *testing.T) {
	e := newTestEnv(t)
	zombie, _ := e.worker(workerproto.Capabilities{})
	other, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	old := e.claimJob(zombie, j)
	staged := e.uploadSprites(old) // the zombie uploaded before being partitioned

	e.clock.Advance(2 * time.Minute)
	e.svc.SweepExpired()
	cur := e.claimJob(other, j)
	if cur.Lease.Generation != old.Lease.Generation+2 {
		t.Fatalf("generations %d then %d", old.Lease.Generation, cur.Lease.Generation)
	}

	hb, _ := e.svc.Heartbeat(zombie, workerproto.HeartbeatRequest{SessionID: zombie.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: old.Lease.Generation, Progress: 90}}})
	if hb.Jobs[0].Status != workerproto.LeaseLost {
		t.Fatalf("zombie heartbeat %+v", hb.Jobs)
	}
	if err := e.svc.LeaseError(j.JobID, old.Lease.Generation, zombie.ID); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatal("zombie upload check")
	}
	if ok, _ := e.svc.Leases.CheckLease(j.JobID, old.Lease.Generation, zombie.ID); ok {
		t.Fatal("zombie lease valid for uploads")
	}
	if _, err := e.svc.Complete(zombie, j.JobID, staged); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatal("zombie complete accepted")
	}
	if err := e.svc.Fail(zombie, j.JobID, workerproto.FailRequest{Generation: old.Lease.Generation, Error: "x"}); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatal("zombie fail accepted")
	}
	if e.job(j.JobID).Progress == 90 {
		t.Fatal("zombie progress stored")
	}
	// The current owner commits its own generation only.
	resp, err := e.svc.Complete(other, j.JobID, e.uploadSprites(cur))
	if err != nil || resp.Status != workerproto.CompleteCommitted {
		t.Fatalf("owner complete %+v %v", resp, err)
	}
	if e.scenes.spriteCommits(cur.SceneID) != 1 {
		t.Fatal("not committed exactly once")
	}
}

// Complete response lost: the retried complete is a no-op.
func TestDuplicateCompleteIsANoop(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	req := e.uploadSprites(a)
	resp, err := e.svc.Complete(w, j.JobID, req)
	if err != nil || resp.Status != workerproto.CompleteCommitted {
		t.Fatalf("complete %+v %v", resp, err)
	}
	sheet := filepath.Join(e.dirs.SpriteDir, workerproto.SpriteSheetName(a.SceneID, 1))
	if b, err := os.ReadFile(sheet); err != nil || string(b) != "sheet one" {
		t.Fatalf("sheet not committed: %v", err)
	}
	if _, err := os.Stat(e.svc.Staging.Dir(j.JobID, a.Lease.Generation)); !os.IsNotExist(err) {
		t.Fatal("staging kept after the commit")
	}
	resp, err = e.svc.Complete(w, j.JobID, req)
	if err != nil || resp.Status != workerproto.CompleteAlreadyCommitted {
		t.Fatalf("second complete %+v %v", resp, err)
	}
	if e.scenes.spriteCommits(a.SceneID) != 1 || len(e.results.forJob(j.JobID)) != 1 {
		t.Fatal("committed or handled twice")
	}
	got := e.job(j.JobID)
	if got.Status != data.JobStatusCompleted || got.CommittedGeneration == nil || *got.CommittedGeneration != a.Lease.Generation {
		t.Fatalf("job %+v", got)
	}
	hb, _ := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: a.Lease.Generation}}})
	if hb.Jobs[0].Status != workerproto.LeaseCommitted {
		t.Fatalf("heartbeat after commit: %+v", hb.Jobs)
	}
	ww, _ := e.svc.Workers.GetByID(w.ID)
	if ww.JobsCompleted != 1 {
		t.Fatalf("completed counter %d", ww.JobsCompleted)
	}
}

// Artifact missing at complete; sha256 mismatch; invalid result.
func TestCompleteRefusesMissingOrMismatchedArtifacts(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	sheet := workerproto.SpriteSheetName(a.SceneID, 1)
	vtt := workerproto.VTTName(a.SceneID)
	sheetBody := []byte("sheet one")
	sheetSum, _, _ := workerproto.SHA256Reader(bytes.NewReader(sheetBody))
	vttSum, _, _ := workerproto.SHA256Reader(bytes.NewReader([]byte("WEBVTT\n")))
	req := workerproto.CompleteRequest{Generation: a.Lease.Generation, Phase: workerproto.PhaseSprites,
		Result:    workerproto.PhaseResult{Sprites: &workerproto.SpritesResult{SpriteSheets: []string{sheet}, VTT: vtt, SheetCount: 1, FrameInterval: 5}},
		Artifacts: []workerproto.ArtifactRef{{Name: sheet, Size: 9, SHA256: sheetSum}, {Name: vtt, Size: 7, SHA256: vttSum}}}

	// Sheet half uploaded, VTT not at all.
	if _, err := e.svc.Staging.Put(j.JobID, a.Lease.Generation, sheet, 0, 9, sheetSum, bytes.NewReader(sheetBody[:4])); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.Complete(w, j.JobID, req)
	ae := AsError(err)
	if ae.Code != workerproto.CodeArtifactMissing || ae.Details[sheet] != "partial" || ae.Details[vtt] != "missing" {
		t.Fatalf("complete with missing artifacts: %+v", ae)
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusRunning {
		t.Fatal("job changed by a refused complete")
	}

	// Wrong result for the scene.
	bad := req
	bad.Result = workerproto.PhaseResult{Sprites: &workerproto.SpritesResult{SpriteSheets: []string{"1_sheet_001.webp"}, VTT: vtt, SheetCount: 1}}
	bad.Artifacts = []workerproto.ArtifactRef{{Name: "1_sheet_001.webp", Size: 1, SHA256: sheetSum}, req.Artifacts[1]}
	if _, err := e.svc.Complete(w, j.JobID, bad); protoCode(t, err) != workerproto.CodeInvalidResult {
		t.Fatalf("invalid result accepted: %v", err)
	}

	// Upload the rest, then complete.
	if _, err := e.svc.Staging.Put(j.JobID, a.Lease.Generation, sheet, 4, 9, sheetSum, bytes.NewReader(sheetBody[4:])); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Staging.Put(j.JobID, a.Lease.Generation, vtt, 0, 7, vttSum, strings.NewReader("WEBVTT\n")); err != nil {
		t.Fatal(err)
	}
	wrong := req
	wrong.Artifacts = []workerproto.ArtifactRef{{Name: sheet, Size: 9, SHA256: vttSum}, req.Artifacts[1]}
	if ae := AsError(func() error { _, err := e.svc.Complete(w, j.JobID, wrong); return err }()); ae.Code != workerproto.CodeArtifactMissing || ae.Details[sheet] != "sha256" {
		t.Fatalf("declared sha256 differs from the upload: %+v", ae)
	}
	if resp, err := e.svc.Complete(w, j.JobID, req); err != nil || resp.Status != workerproto.CompleteCommitted {
		t.Fatalf("complete %+v %v", resp, err)
	}
}

// Network cut during upload: resume at the received offset.
func TestUploadResumeAndChecks(t *testing.T) {
	e := newTestEnv(t)
	st := e.svc.Staging
	body := bytes.Repeat([]byte("0123456789"), 1000)
	sum, size, _ := workerproto.SHA256Reader(bytes.NewReader(body))
	job := "job-upload-test"

	// A cut mid-body: the bytes received are kept.
	if _, err := st.Put(job, 1, "a.webp", 0, size, sum, &cutReader{data: body[:3000], cutAfter: 2500}); err == nil {
		t.Fatal("cut body not reported")
	}
	status, _ := st.Status(job, 1, "a.webp")
	if status.Received != 2500 || status.Complete || status.Size != size || status.SHA256 != sum {
		t.Fatalf("status after the cut %+v", status)
	}
	// Wrong offset: 409 with the received count.
	_, err := st.Put(job, 1, "a.webp", 1000, size, sum, bytes.NewReader(body[1000:]))
	if ae := AsError(err); ae.Code != workerproto.CodeOffsetMismatch || ae.Details["received"] != "2500" {
		t.Fatalf("wrong offset: %+v", ae)
	}
	// Another declaration at an offset: refused.
	_, err = st.Put(job, 1, "a.webp", 2500, size+1, sum, bytes.NewReader(body[2500:]))
	if protoCode(t, err) != workerproto.CodeOffsetMismatch {
		t.Fatal("changed declaration accepted mid-upload")
	}
	// Resume.
	status, err = st.Put(job, 1, "a.webp", 2500, size, sum, bytes.NewReader(body[2500:]))
	if err != nil || !status.Complete || status.Received != size {
		t.Fatalf("resume %+v %v", status, err)
	}
	// Re-upload of a complete artifact: idempotent, the body is not read.
	status, err = st.Put(job, 1, "a.webp", 0, size, sum, &cutReader{data: nil, cutAfter: 0})
	if err != nil || !status.Complete {
		t.Fatalf("re-upload %+v %v", status, err)
	}
	// sha256 mismatch: 422 and the part is gone.
	badSum, _, _ := workerproto.SHA256Reader(strings.NewReader("other"))
	_, err = st.Put(job, 1, "b.webp", 0, 5, badSum, strings.NewReader("hello"))
	if protoCode(t, err) != workerproto.CodeSHA256Mismatch {
		t.Fatal("sha256 mismatch accepted")
	}
	if status, _ := st.Status(job, 1, "b.webp"); status.Received != 0 || status.SHA256 != "" {
		t.Fatalf("mismatched upload kept: %+v", status)
	}
	// Names, sizes.
	if _, err := st.Put(job, 1, "../x", 0, 1, sum, strings.NewReader("x")); protoCode(t, err) != workerproto.CodeInvalidArtifactName {
		t.Fatal("path traversal name accepted")
	}
	if _, err := st.Put(job, 1, ".hidden", 0, 1, sum, strings.NewReader("x")); protoCode(t, err) != workerproto.CodeInvalidArtifactName {
		t.Fatal("dot name accepted")
	}
	small := NewStaging(t.TempDir(), 10)
	if _, err := small.Put(job, 1, "a.webp", 0, 11, sum, strings.NewReader("x")); protoCode(t, err) != workerproto.CodeArtifactTooLarge {
		t.Fatal("too large accepted")
	}
	if _, err := st.Put("../../etc", 1, "a.webp", 0, 1, sum, strings.NewReader("x")); protoCode(t, err) != workerproto.CodeInvalidRequest {
		t.Fatal("bad job id accepted")
	}
	// Staged files of another generation are separate.
	if status, _ := st.Status(job, 2, "a.webp"); status.Received != 0 {
		t.Fatal("generations share uploads")
	}
}

// cutReader returns data then fails after cutAfter bytes, like a broken connection.
type cutReader struct {
	data     []byte
	cutAfter int
	read     int
}

func (r *cutReader) Read(p []byte) (int, error) {
	if r.read >= r.cutAfter {
		return 0, errCut
	}
	n := copy(p, r.data[r.read:min(len(r.data), r.cutAfter)])
	r.read += n
	return n, nil
}

var errCut = &Error{Message: "connection reset"}

// Server restart with leases in the database.
func TestServerRestartKeepsLeasesAndStaging(t *testing.T) {
	e := newTestEnv(t)
	w, token := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	req := e.uploadSprites(a)

	// The server is down for longer than the lease.
	e.clock.Advance(90 * time.Second)
	restarted := e.newService(e.svc.cfg)
	restarted.Start(nil)
	restarted.Stop()
	got := e.job(j.JobID)
	if got.Status != data.JobStatusRunning || !got.LeaseExpiresAt.After(e.clock.Now().Add(50*time.Second)) {
		t.Fatalf("lease after restart: %+v", got)
	}
	if n, _ := data.NewJobHistoryRepository(e.db).MarkOrphanedRunningAsFailed(time.Second); n > 0 {
		if e.job(j.JobID).Status != data.JobStatusRunning {
			t.Fatal("orphan recovery failed a remote lease")
		}
	}
	// The worker reconnects within the grace: same session, heartbeat ok, complete commits
	// the staging kept across the restart.
	w = e.auth(token)
	hb, err := restarted.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: a.Lease.Generation}}})
	if err != nil || hb.Jobs[0].Status != workerproto.LeaseOK {
		t.Fatalf("heartbeat after restart %+v %v", hb, err)
	}
	if resp, err := restarted.Complete(w, j.JobID, req); err != nil || resp.Status != workerproto.CompleteCommitted {
		t.Fatalf("complete after restart %+v %v", resp, err)
	}
}

// Server commit fails (DB down) on a fingerprint: retried complete, committed once.
func TestFingerprintCommitAtMostOncePerGeneration(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene("aac"), workerproto.PhaseFingerprint)
	a := e.claimJob(w, j)
	if a.Params.Fingerprint == nil || a.Params.Fingerprint.Mode != workerproto.FingerprintModeAudioOnly {
		t.Fatalf("fingerprint params %+v", a.Params)
	}
	req := workerproto.CompleteRequest{Generation: a.Lease.Generation, Phase: workerproto.PhaseFingerprint,
		Result: workerproto.PhaseResult{Fingerprint: &workerproto.FingerprintResult{AudioFingerprint: workerproto.AudioFingerprint{1, 2, 3}}}}
	e.fp.failN = 1
	if _, err := e.svc.Complete(w, j.JobID, req); protoCode(t, err) != workerproto.CodeCommitFailed {
		t.Fatalf("failed commit: %v", err)
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusRunning {
		t.Fatal("job left running after a failed commit")
	}
	if resp, err := e.svc.Complete(w, j.JobID, req); err != nil || resp.Status != workerproto.CompleteCommitted {
		t.Fatalf("retried complete %+v %v", resp, err)
	}
	if resp, err := e.svc.Complete(w, j.JobID, req); err != nil || resp.Status != workerproto.CompleteAlreadyCommitted {
		t.Fatalf("third complete %+v %v", resp, err)
	}
	if e.fp.commits != 2 || e.index.removed != 1 {
		t.Fatalf("commits %d, index resets %d: want 2 and 1", e.fp.commits, e.index.removed)
	}
	r := e.results.forJob(j.JobID)
	if len(r) != 1 {
		t.Fatalf("results %+v", r)
	}
	if _, ok := r[0].Data.(*jobs.FingerprintResult); !ok {
		t.Fatalf("result data %T", r[0].Data)
	}
}

// Worker lacks fpcalc: only scenes without audio.
func TestFingerprintWithoutChromaprintOnlyClaimsSilentScenes(t *testing.T) {
	e := newTestEnv(t)
	caps := defaultCaps()
	caps.AudioFingerprint = false
	w, _ := e.worker(caps)
	// The NoAudioOnly filter joins the scenes table: insert real scene rows.
	silent := &data.Scene{Title: "silent", StoredPath: filepath.Join(t.TempDir(), "a.mp4"), Duration: 60}
	loud := &data.Scene{Title: "loud", StoredPath: filepath.Join(t.TempDir(), "b.mp4"), Duration: 60, AudioCodec: "aac"}
	for _, s := range []*data.Scene{silent, loud} {
		if err := e.db.Exec(`INSERT INTO scenes (title, stored_path, duration, audio_codec, tags, actors, created_at, updated_at) VALUES (?, ?, ?, ?, '{}', '{}', now(), now()) RETURNING id`,
			s.Title, s.StoredPath, s.Duration, s.AudioCodec).Error; err != nil {
			t.Fatal(err)
		}
		e.db.Raw(`SELECT id FROM scenes WHERE stored_path = ?`, s.StoredPath).Scan(&s.ID)
		id := s.ID
		t.Cleanup(func() { e.db.Exec(`DELETE FROM scenes WHERE id = ?`, id) })
		e.scenes.add(s)
	}
	jl := e.pending(loud, workerproto.PhaseFingerprint)
	js := e.pending(silent, workerproto.PhaseFingerprint)
	e.claimJob(w, js)
	for i := 0; i < 3; i++ {
		as, _ := e.svc.ClaimFor(w, map[workerproto.Phase]int{"fingerprint": 5})
		for _, a := range as {
			if a.JobID == jl.JobID {
				t.Fatal("assigned a scene with audio to a worker without chromaprint")
			}
		}
	}
}

// Source unreadable / ffmpeg error; non-retryable failure.
func TestFailUsesRetryPathOrDeadLetter(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j1 := e.pending(e.scene(""), workerproto.PhaseSprites)
	j2 := e.pending(e.scene(""), workerproto.PhaseSprites)
	a1 := e.claimJob(w, j1)
	a2 := e.claimJob(w, j2)

	if err := e.svc.Fail(w, j1.JobID, workerproto.FailRequest{Generation: a1.Lease.Generation, Kind: workerproto.FailureCompute, Error: "ffmpeg exit 1", Retryable: true}); err != nil {
		t.Fatal(err)
	}
	got := e.job(j1.JobID)
	if got.Status != data.JobStatusFailed || !got.IsRetryable || !strings.Contains(*got.ErrorMessage, "worker "+w.Name) {
		t.Fatalf("failed job %+v", got)
	}
	r := e.results.forJob(j1.JobID)
	if len(r) != 1 || r[0].Status != jobs.JobStatusFailed {
		t.Fatalf("results %+v", r)
	}
	if err := e.svc.Fail(w, j2.JobID, workerproto.FailRequest{Generation: a2.Lease.Generation, Kind: workerproto.FailureUnsupported, Error: "no", Retryable: false}); err != nil {
		t.Fatal(err)
	}
	if got := e.job(j2.JobID); got.IsRetryable {
		t.Fatal("non-retryable failure is retryable")
	}
	if len(e.retry.dlq) != 1 || e.retry.dlq[0] != j2.JobID {
		t.Fatalf("dlq %v", e.retry.dlq)
	}
	ww, _ := e.svc.Workers.GetByID(w.ID)
	if ww.JobsFailed != 2 || ww.LastError == "" {
		t.Fatalf("worker stats %d %q", ww.JobsFailed, ww.LastError)
	}
}

// User cancels the job; scene deleted while leased.
func TestCancelledJobAndDeletedScene(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	ok, err := e.svc.CancelJob(j.JobID)
	if err != nil || !ok {
		t.Fatalf("cancel %v %v", ok, err)
	}
	hb, _ := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: a.Lease.Generation}}})
	if hb.Jobs[0].Status != workerproto.LeaseCancelled {
		t.Fatalf("heartbeat after cancel %+v", hb.Jobs)
	}
	if _, err := e.svc.Complete(w, j.JobID, e.uploadSprites(a)); protoCode(t, err) != workerproto.CodeJobCancelled {
		t.Fatalf("complete after cancel: %v", err)
	}
	if r := e.results.forJob(j.JobID); len(r) != 1 || r[0].Status != jobs.JobStatusCancelled {
		t.Fatalf("results %+v", r)
	}
	if ok, _ := e.svc.CancelJob("not-a-remote-job"); ok {
		t.Fatal("cancelled a job that is not remote")
	}

	s := e.scene("")
	j2 := e.pending(s, workerproto.PhaseSprites)
	a2 := e.claimJob(w, j2)
	req := e.uploadSprites(a2)
	e.scenes.remove(s.ID)
	if _, err := e.svc.Complete(w, j2.JobID, req); protoCode(t, err) != workerproto.CodeSceneGone {
		t.Fatalf("complete of a deleted scene: %v", err)
	}
	if got := e.job(j2.JobID); got.Status != data.JobStatusCancelled {
		t.Fatalf("job of a deleted scene: %s", got.Status)
	}
	if _, err := os.Stat(e.svc.Staging.Dir(j2.JobID, a2.Lease.Generation)); !os.IsNotExist(err) {
		t.Fatal("staging of a deleted scene kept")
	}
}

// Claim response or push answer lost: the worker does not report the lease.
func TestHeartbeatReleasesUnreportedLeases(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	e.claimJob(w, j)
	if _, err := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID}); err != nil {
		t.Fatal(err)
	}
	if e.job(j.JobID).Status != data.JobStatusRunning {
		t.Fatal("a just claimed lease was released")
	}
	e.clock.Advance(25 * time.Second)
	if _, err := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID}); err != nil {
		t.Fatal(err)
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusPending || got.RetryCount != 0 {
		t.Fatalf("unreported lease: %+v", got)
	}
}

func TestHeartbeatStoresProgressAndHonoursMaxAssignmentAge(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	hb, err := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID, FreeDiskBytes: 1234, Draining: true,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: a.Lease.Generation, Progress: 42, Stage: workerproto.StageComputing, InputMode: workerproto.InputDirect}}})
	if err != nil || hb.Jobs[0].Status != workerproto.LeaseOK || hb.Jobs[0].LeaseExpiresAt == nil {
		t.Fatalf("heartbeat %+v %v", hb, err)
	}
	got := e.job(j.JobID)
	if got.Progress != 42 || *got.WorkerStage != "computing" || *got.InputMode != "direct" {
		t.Fatalf("progress %+v", got)
	}
	ww, _ := e.svc.Workers.GetByID(w.ID)
	if ww.FreeDiskBytes != 1234 || !ww.WorkerDraining {
		t.Fatalf("worker state %+v", ww)
	}
	e.svc.cfg.MaxAssignmentAge = time.Second
	e.clock.Advance(2 * time.Second)
	hb, _ = e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: a.Lease.Generation}}})
	if hb.Jobs[0].Status != workerproto.LeaseLost {
		t.Fatal("a lease older than the max assignment age was renewed")
	}
}

func TestMediaURLAuthorization(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	s := e.scene("")
	j := e.pending(s, workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	u, _ := url.Parse(a.Input.URL)
	path, err := e.svc.MediaFile(j.JobID, u.Query())
	if err != nil || path != s.StoredPath {
		t.Fatalf("media %q %v", path, err)
	}
	fresh, err := e.svc.MediaURL(w, j.JobID, workerproto.MediaURLRequest{Generation: a.Lease.Generation})
	if err != nil || fresh.URL == "" {
		t.Fatalf("fresh URL %v", err)
	}
	if _, err := e.svc.MediaURL(w, j.JobID, workerproto.MediaURLRequest{Generation: 99}); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatal("fresh URL for a stale generation")
	}
	q := u.Query()
	q.Set("sig", strings.Repeat("0", 64))
	if _, err := e.svc.MediaFile(j.JobID, q); protoCode(t, err) != workerproto.CodeUnauthorized {
		t.Fatal("bad signature accepted")
	}
	e.clock.Advance(7 * time.Hour)
	if _, err := e.svc.MediaFile(j.JobID, u.Query()); protoCode(t, err) != workerproto.CodeURLExpired {
		t.Fatal("expired URL accepted")
	}
	e.clock.Advance(-7 * time.Hour)
	e.svc.Release(w, j.JobID, workerproto.ReleaseRequest{Generation: a.Lease.Generation})
	if _, err := e.svc.MediaFile(j.JobID, u.Query()); protoCode(t, err) != workerproto.CodeLeaseLost {
		t.Fatal("URL of a released lease accepted")
	}
}

func TestStagingJanitor(t *testing.T) {
	e := newTestEnv(t)
	w, _ := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	a := e.claimJob(w, j)
	e.uploadSprites(a)
	sum, _, _ := workerproto.SHA256Reader(strings.NewReader("x"))
	if _, err := e.svc.Staging.Put(j.JobID, a.Lease.Generation-1, "old.webp", 0, 1, sum, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Staging.Put("abandoned-job", 3, "x.webp", 0, 1, sum, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if n := e.svc.CleanStaging(); n != 0 {
		t.Fatal("fresh staging deleted")
	}
	e.clock.Advance(25 * time.Hour)
	if n := e.svc.CleanStaging(); n != 2 {
		t.Fatalf("deleted %d generations, want the stale generation and the abandoned job", n)
	}
	if _, err := os.Stat(e.svc.Staging.Dir(j.JobID, a.Lease.Generation)); err != nil {
		t.Fatal("the staging of the current lease was deleted")
	}
	if _, err := os.Stat(filepath.Join(e.svc.Staging.RemoteRoot(), "abandoned-job")); !os.IsNotExist(err) {
		t.Fatal("abandoned job directory kept")
	}
}

func TestAdminWorkerLifecycle(t *testing.T) {
	e := newTestEnv(t)
	w, token := e.worker(workerproto.Capabilities{})
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	e.claimJob(w, j)

	list, err := e.svc.ListWorkers()
	if err != nil {
		t.Fatal(err)
	}
	var v *WorkerView
	for i := range list {
		if list[i].ID == w.ID {
			v = &list[i]
		}
	}
	if v == nil || v.Status != StatusOnline || len(v.Jobs) != 1 || v.Jobs[0].JobID != j.JobID || !v.HasToken || len(v.Capabilities.Phases) != 3 {
		t.Fatalf("view %+v", v)
	}
	e.clock.Advance(time.Minute)
	if g, _ := e.svc.GetWorker(w.ID); g.Status != StatusOffline {
		t.Fatalf("status %s, want offline", g.Status)
	}

	// Disable: leases released at once (no attempt), the token refused with 403.
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{Enabled: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if got := e.job(j.JobID); got.Status != data.JobStatusPending || got.RetryCount != 0 {
		t.Fatalf("job of a disabled worker %+v", got)
	}
	if _, err := e.svc.Authenticate(token); protoCode(t, err) != workerproto.CodeWorkerDisabled {
		t.Fatal("disabled worker accepted")
	}
	e.svc.UpdateWorker(w.ID, WorkerInput{Enabled: boolPtr(true)})

	// Rotate: the old token stops working.
	rotated, err := e.svc.RotateToken(w.ID)
	if err != nil || rotated.Token == "" || rotated.Token == token {
		t.Fatalf("rotate %v", err)
	}
	if _, err := e.svc.Authenticate(token); protoCode(t, err) != workerproto.CodeUnauthorized {
		t.Fatal("old token accepted")
	}
	e.auth(rotated.Token)

	// Revoke.
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{RevokeToken: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Authenticate(rotated.Token); protoCode(t, err) != workerproto.CodeUnauthorized {
		t.Fatal("revoked token accepted")
	}
	if g, _ := e.svc.GetWorker(w.ID); g.Status != StatusRevoked || g.HasToken {
		t.Fatalf("revoked view %+v", g)
	}

	// Validation.
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{AllowedPhases: []string{"metadata"}}); err == nil {
		t.Fatal("metadata allowed remotely")
	}
	if _, err := e.svc.UpdateWorker(w.ID, WorkerInput{PushURL: strPtr("ftp://x")}); err == nil {
		t.Fatal("bad push URL accepted")
	}
	if err := e.svc.DeleteWorker(w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.GetWorker(w.ID); err == nil {
		t.Fatal("deleted worker still there")
	}
}

func strPtr(s string) *string { return &s }

func TestRouterModes(t *testing.T) {
	r := NewRouter(true, []string{"sprites"}, nil)
	if r.Mode("sprites") != RouteBoth || r.Mode("fingerprint") != RouteRemote || r.Mode("metadata") != RouteLocal {
		t.Fatalf("defaults %+v", r.View())
	}
	if !r.LocalAllowed("metadata") || r.RemoteAllowed("thumbnail") {
		t.Fatal("server-only phases must stay local")
	}
	if _, err := r.Update(map[string]string{"sprites": "remote", "fingerprint": "local"}); err != nil {
		t.Fatal(err)
	}
	if r.LocalAllowed("sprites") || r.RemoteAllowed("fingerprint") {
		t.Fatal("overrides ignored")
	}
	if got := r.RemoteOnlyPhases(); len(got) != 2 || got[0] != "sprites" || got[1] != "animated_thumbnails" {
		t.Fatalf("remote only %v", got)
	}
	if _, err := r.Update(map[string]string{"metadata": "remote"}); err == nil {
		t.Fatal("metadata routed remotely")
	}
	if _, err := r.Update(map[string]string{"sprites": "elsewhere"}); err == nil {
		t.Fatal("unknown mode accepted")
	}
	off := NewRouter(false, nil, nil)
	for _, p := range workerproto.RemotePhases {
		if off.Mode(string(p)) != RouteLocal {
			t.Fatal("with the worker API disabled every phase is local")
		}
	}
}

func TestTokens(t *testing.T) {
	token, hash, err := NewToken(42)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "ghw_42_") || len(hash) != 64 {
		t.Fatalf("token %s hash %s", token, hash)
	}
	if id, ok := ParseToken(token); !ok || id != 42 {
		t.Fatal("parse")
	}
	if !TokenMatches(token, &hash) || TokenMatches(token+"x", &hash) || TokenMatches(token, nil) {
		t.Fatal("match")
	}
	for _, bad := range []string{"", "ghw_", "ghw_x_abc", "ghw_0_abc", "ghx_1_abc", "ghw_1_"} {
		if _, ok := ParseToken(bad); ok {
			t.Fatalf("parsed %q", bad)
		}
	}
}

func TestSettingsDefaultsAndCIDRs(t *testing.T) {
	s, err := NewSettings(defaultWorkersConfig(), "/tmp/staging")
	if err != nil {
		t.Fatal(err)
	}
	if s.StartupLeaseGrace != s.LeaseDuration || s.OfflineAfter != 3*s.HeartbeatInterval {
		t.Fatalf("defaults %+v", s)
	}
	if !s.AddressAllowed("100.101.102.103") || !s.AddressAllowed("127.0.0.1") || s.AddressAllowed("192.168.1.10") || s.AddressAllowed("garbage") {
		t.Fatal("CIDR check")
	}
	cfg := defaultWorkersConfig()
	cfg.LeaseDuration = 10 * time.Second
	if _, err := NewSettings(cfg, ""); err == nil {
		t.Fatal("lease shorter than 3 heartbeats accepted")
	}
}

func defaultWorkersConfig() config.WorkersConfig {
	return config.WorkersConfig{Enabled: true, HeartbeatInterval: 10 * time.Second, LeaseDuration: 60 * time.Second,
		AllowedCIDRs: []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48", "127.0.0.0/8", "::1/128"}}
}
