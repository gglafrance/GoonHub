package data

import (
	"math/rand/v2"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration tests of the remote worker leases (JobLeaseRepository) and of the worker
// repository, on the disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).

func leaseTestWorker(t *testing.T, db *gorm.DB) *Worker {
	t.Helper()
	w := &Worker{Name: "lease-test-" + uuid.NewString()[:8], Enabled: true, ConnectionMode: "pull",
		AllowedPhases: pq.StringArray{"sprites", "animated_thumbnails", "fingerprint"},
		Capabilities:  []byte("{}"), PathMappings: []byte("[]")}
	if err := NewWorkerRepository(db).Create(w); err != nil {
		t.Fatalf("create worker: %v", err)
	}
	t.Cleanup(func() {
		db.Model(&JobHistory{}).Where("worker_id = ?", w.ID).Update("worker_id", nil)
		db.Delete(&Worker{}, w.ID)
	})
	return w
}

// leaseTestPhase is a phase name unique to one test: tests never see each other's rows.
func leaseTestPhase() string { return "lt-" + uuid.NewString()[:8] }

func leaseTestJob(t *testing.T, db *gorm.DB, phase string, sceneID uint, created time.Time) *JobHistory {
	t.Helper()
	if sceneID == 0 {
		sceneID = uint(rand.IntN(1_000_000_000) + 1_000_000)
	}
	row := &JobHistory{JobID: uuid.NewString(), SceneID: sceneID, Phase: phase, Status: JobStatusPending,
		CreatedAt: created, StartedAt: created, IsRetryable: true}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	t.Cleanup(func() { db.Delete(&JobHistory{}, row.ID) })
	return row
}

func leaseTestScene(t *testing.T, db *gorm.DB, path, audio string) *Scene {
	t.Helper()
	s := &Scene{Title: "lease test", StoredPath: path, AudioCodec: audio, Duration: 60, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("create scene: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&Scene{}, s.ID) })
	return s
}

func reloadJob(t *testing.T, db *gorm.DB, id uint) JobHistory {
	t.Helper()
	var j JobHistory
	if err := db.First(&j, id).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	return j
}

func claimFilter(phase string, w *Worker, limit int) LeaseClaimFilter {
	return LeaseClaimFilter{Phase: phase, Limit: limit, WorkerID: w.ID, LeaseDuration: time.Minute}
}

func TestLeaseClaimSetsLeaseAndGeneration(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w := leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now().Truncate(time.Microsecond)
	j := leaseTestJob(t, db, phase, 0, now.Add(-time.Hour))

	claimed, err := repo.ClaimForWorker(claimFilter(phase, w, 5), now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].JobID != j.JobID {
		t.Fatalf("claimed %+v", claimed)
	}
	got := reloadJob(t, db, j.ID)
	if got.Status != JobStatusRunning || got.WorkerID == nil || *got.WorkerID != w.ID || got.LeaseGeneration != 1 {
		t.Fatalf("lease not set: status=%s worker=%v gen=%d", got.Status, got.WorkerID, got.LeaseGeneration)
	}
	if got.LeaseExpiresAt == nil || !got.LeaseExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("lease expiry %v", got.LeaseExpiresAt)
	}
	// Nothing left to claim, neither for workers nor for the local feeder.
	if again, _ := repo.ClaimForWorker(claimFilter(phase, w, 5), now); len(again) != 0 {
		t.Fatalf("claimed a running job again")
	}
	local, err := NewJobHistoryRepository(db).ClaimPendingJobs(phase, 5, ClaimOptions{})
	if err != nil || len(local) != 0 {
		t.Fatalf("local feeder claimed a leased job: %v %v", local, err)
	}
}

func TestLeaseClaimSkipLockedNoDoubleClaim(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	jobsRepo := NewJobHistoryRepository(db)
	w1, w2 := leaseTestWorker(t, db), leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now()
	for i := 0; i < 30; i++ {
		leaseTestJob(t, db, phase, 0, now.Add(-time.Duration(i)*time.Second))
	}
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	record := func(rows []JobHistory) {
		mu.Lock()
		defer mu.Unlock()
		for _, r := range rows {
			seen[r.JobID]++
		}
	}
	for i := 0; i < 6; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			rows, err := repo.ClaimForWorker(claimFilter(phase, w1, 3), now)
			if err != nil {
				t.Errorf("claim w1: %v", err)
			}
			record(rows)
		}()
		go func() {
			defer wg.Done()
			rows, err := repo.ClaimForWorker(claimFilter(phase, w2, 3), now)
			if err != nil {
				t.Errorf("claim w2: %v", err)
			}
			record(rows)
		}()
		go func() {
			defer wg.Done()
			rows, err := jobsRepo.ClaimPendingJobs(phase, 2, ClaimOptions{})
			if err != nil {
				t.Errorf("local claim: %v", err)
			}
			record(rows)
		}()
	}
	wg.Wait()
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("job %s claimed %d times", id, n)
		}
	}
	if len(seen) != 30 {
		t.Fatalf("claimed %d of 30 jobs", len(seen))
	}
}

func TestLeaseClaimOrderingAndFilters(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w := leaseTestWorker(t, db)
	now := time.Now()

	t.Run("priority, then preferred scenes, then age", func(t *testing.T) {
		phase := leaseTestPhase()
		old := leaseTestJob(t, db, phase, 0, now.Add(-time.Hour))
		preferred := leaseTestJob(t, db, phase, 0, now.Add(-time.Minute))
		urgent := leaseTestJob(t, db, phase, 0, now)
		db.Model(&JobHistory{}).Where("id = ?", urgent.ID).Update("priority", 5)
		var order []string
		for i := 0; i < 3; i++ {
			f := claimFilter(phase, w, 1)
			f.PreferScenes = []uint{preferred.SceneID}
			rows, err := repo.ClaimForWorker(f, now)
			if err != nil || len(rows) != 1 {
				t.Fatalf("claim %d: %v %v", i, rows, err)
			}
			order = append(order, rows[0].JobID)
		}
		want := []string{urgent.JobID, preferred.JobID, old.JobID}
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("claim order %v, want urgent, preferred, old", order)
			}
		}
	})

	t.Run("path prefixes and audio", func(t *testing.T) {
		phase := leaseTestPhase()
		base := "/lease-test-" + uuid.NewString()[:8]
		inside := leaseTestScene(t, db, base+"/a/video.mp4", "")
		sibling := leaseTestScene(t, db, base+"2/a/video.mp4", "")
		withAudio := leaseTestScene(t, db, base+"/b/video.mp4", "aac")
		jInside := leaseTestJob(t, db, phase, inside.ID, now)
		leaseTestJob(t, db, phase, sibling.ID, now)
		jAudio := leaseTestJob(t, db, phase, withAudio.ID, now)

		f := claimFilter(phase, w, 10)
		f.PathPrefixes = []string{base + "/"}
		f.NoAudioOnly = true
		rows, err := repo.ClaimForWorker(f, now)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(rows) != 1 || rows[0].JobID != jInside.JobID {
			t.Fatalf("claimed %v, want only the scene under the prefix without audio", rows)
		}
		f.NoAudioOnly = false
		rows, err = repo.ClaimForWorker(f, now)
		if err != nil || len(rows) != 1 || rows[0].JobID != jAudio.JobID {
			t.Fatalf("claimed %v %v, want the scene with audio (the sibling prefix never)", rows, err)
		}
	})
}

func TestLeaseCompareAndSet(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w, other := leaseTestWorker(t, db), leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now()
	j := leaseTestJob(t, db, phase, 0, now)
	if _, err := repo.ClaimForWorker(claimFilter(phase, w, 1), now); err != nil {
		t.Fatal(err)
	}
	renew := func(gen int64, worker uint, notBefore time.Time) bool {
		ok, err := repo.RenewLease(LeaseRenewal{JobID: j.JobID, Generation: gen, WorkerID: worker, Now: now,
			ExpiresAt: now.Add(2 * time.Minute), NotBefore: notBefore, Progress: 40, Stage: "computing", InputMode: "indirect"})
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !renew(1, w.ID, time.Time{}) {
		t.Fatal("current lease not renewed")
	}
	got := reloadJob(t, db, j.ID)
	if got.Progress != 40 || got.WorkerStage == nil || *got.WorkerStage != "computing" || got.InputMode == nil || *got.InputMode != "indirect" {
		t.Fatalf("progress not stored: %+v", got)
	}
	if renew(0, w.ID, time.Time{}) || renew(2, w.ID, time.Time{}) || renew(1, other.ID, time.Time{}) {
		t.Fatal("renewed with a wrong generation or worker")
	}
	if renew(1, w.ID, now.Add(time.Hour)) {
		t.Fatal("renewed a lease older than the max assignment age")
	}
	if ok, _ := repo.CheckLease(j.JobID, 1, w.ID); !ok {
		t.Fatal("check lease")
	}
	if ok, _ := repo.CompleteLease(j.JobID, 2, w.ID, now); ok {
		t.Fatal("completed with a stale generation")
	}
	if ok, _ := repo.FailLease(j.JobID, 1, other.ID, now, "x", true); ok {
		t.Fatal("failed by another worker")
	}
}

func TestLeaseReleaseAndCommitMarks(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w := leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now()
	j := leaseTestJob(t, db, phase, 0, now)
	repo.ClaimForWorker(claimFilter(phase, w, 1), now)

	if ok, err := repo.ReleaseLease(j.JobID, 1, w.ID, false); err != nil || !ok {
		t.Fatalf("release: %v %v", ok, err)
	}
	got := reloadJob(t, db, j.ID)
	if got.Status != JobStatusPending || got.WorkerID != nil || got.LeaseGeneration != 2 || got.RetryCount != 0 || got.LeaseExpiresAt != nil {
		t.Fatalf("after release: %+v", got)
	}
	if ok, _ := repo.ReleaseLease(j.JobID, 1, w.ID, false); ok {
		t.Fatal("released twice")
	}

	repo.ClaimForWorker(claimFilter(phase, w, 1), now) // generation 3
	if ok, _ := repo.ReleaseLease(j.JobID, 3, w.ID, true); !ok {
		t.Fatal("counted release")
	}
	if got := reloadJob(t, db, j.ID); got.RetryCount != 1 || got.LeaseGeneration != 4 {
		t.Fatalf("counted release: retry=%d gen=%d", got.RetryCount, got.LeaseGeneration)
	}

	repo.ClaimForWorker(claimFilter(phase, w, 1), now) // generation 5
	ok, first, err := repo.MarkCommitStarted(j.JobID, 5, w.ID, now.Add(time.Hour))
	if err != nil || !ok || !first {
		t.Fatalf("mark commit: %v %v %v", ok, first, err)
	}
	ok, first, _ = repo.MarkCommitStarted(j.JobID, 5, w.ID, now.Add(time.Hour))
	if !ok || first {
		t.Fatalf("second mark: ok=%v first=%v, want ok and not first", ok, first)
	}
	if got := reloadJob(t, db, j.ID); !got.LeaseExpiresAt.After(now.Add(59 * time.Minute)) {
		t.Fatalf("commit did not extend the lease: %v", got.LeaseExpiresAt)
	}
	if ok, _, _ := repo.MarkCommitStarted(j.JobID, 4, w.ID, now); ok {
		t.Fatal("stale generation marked")
	}
	if ok, _ := repo.CompleteLease(j.JobID, 5, w.ID, now); !ok {
		t.Fatal("complete")
	}
	got = reloadJob(t, db, j.ID)
	if got.Status != JobStatusCompleted || got.CommittedGeneration == nil || *got.CommittedGeneration != 5 || got.WorkerID == nil {
		t.Fatalf("after complete: %+v", got)
	}
}

func TestLeaseReleaseWorkerLeasesKeeps(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w := leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now()
	a := leaseTestJob(t, db, phase, 0, now)
	b := leaseTestJob(t, db, phase, 0, now.Add(time.Second))
	repo.ClaimForWorker(claimFilter(phase, w, 2), now)
	released, err := repo.ReleaseWorkerLeases(w.ID, []string{a.JobID}, false)
	if err != nil || len(released) != 1 || released[0].JobID != b.JobID {
		t.Fatalf("released %v %v", released, err)
	}
	if got := reloadJob(t, db, a.ID); got.Status != JobStatusRunning {
		t.Fatal("kept lease released")
	}
	held, _ := repo.ListRemoteRunning(w.ID)
	if len(held) != 1 || held[0].JobID != a.JobID {
		t.Fatalf("held %v", held)
	}
}

func TestLeaseExpirySweep(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w := leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now()
	fresh := leaseTestJob(t, db, phase, 0, now)
	expiring := leaseTestJob(t, db, phase, 0, now.Add(time.Second))
	exhausted := leaseTestJob(t, db, phase, 0, now.Add(2*time.Second))
	repo.ClaimForWorker(claimFilter(phase, w, 3), now)
	past := now.Add(-time.Second)
	db.Model(&JobHistory{}).Where("id IN ?", []uint{expiring.ID, exhausted.ID}).Update("lease_expires_at", past)
	db.Model(&JobHistory{}).Where("id = ?", exhausted.ID).Update("retry_count", 3)

	requeued, failed, err := repo.ExpireLeases(now, map[string]int{phase: 3}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(requeued) != 1 || requeued[0].JobID != expiring.JobID || len(failed) != 1 || failed[0].JobID != exhausted.JobID {
		t.Fatalf("requeued %v failed %v", requeued, failed)
	}
	if requeued[0].WorkerID == nil || *requeued[0].WorkerID != w.ID {
		t.Fatal("expired rows must carry their worker (as before the change)")
	}
	got := reloadJob(t, db, expiring.ID)
	if got.Status != JobStatusPending || got.RetryCount != 1 || got.LeaseGeneration != 2 || got.WorkerID != nil {
		t.Fatalf("requeued row: %+v", got)
	}
	got = reloadJob(t, db, exhausted.ID)
	if got.Status != JobStatusFailed || got.LeaseGeneration != 2 || got.RetryCount != 3 {
		t.Fatalf("exhausted row: %+v", got)
	}
	if got := reloadJob(t, db, fresh.ID); got.Status != JobStatusRunning || got.LeaseGeneration != 1 {
		t.Fatal("a valid lease was expired")
	}
	// The old generation is dead at once.
	if ok, _ := repo.CheckLease(expiring.JobID, 1, w.ID); ok {
		t.Fatal("expired generation still valid")
	}
}

func TestLeaseCancelAndExtend(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	w := leaseTestWorker(t, db)
	phase := leaseTestPhase()
	now := time.Now()
	a := leaseTestJob(t, db, phase, 0, now)
	b := leaseTestJob(t, db, phase, 0, now.Add(time.Second))
	repo.ClaimForWorker(claimFilter(phase, w, 2), now)

	n, err := repo.ExtendRemoteLeases(now.Add(time.Hour))
	if err != nil || n < 2 {
		t.Fatalf("extend: %d %v", n, err)
	}
	if got := reloadJob(t, db, a.ID); !got.LeaseExpiresAt.After(now.Add(59 * time.Minute)) {
		t.Fatal("lease not extended")
	}
	before, err := repo.CancelLease(a.JobID, now, "job was cancelled")
	if err != nil || before == nil || before.LeaseGeneration != 1 {
		t.Fatalf("cancel: %v %v", before, err)
	}
	got := reloadJob(t, db, a.ID)
	if got.Status != JobStatusCancelled || got.LeaseGeneration != 2 || got.WorkerID == nil {
		t.Fatalf("cancelled row: %+v", got)
	}
	if again, _ := repo.CancelLease(a.JobID, now, "x"); again != nil {
		t.Fatal("cancelled twice")
	}
	rows, err := repo.CancelRemoteLeasesByPhases([]string{phase}, now, "bulk")
	if err != nil || len(rows) != 1 || rows[0].JobID != b.JobID {
		t.Fatalf("bulk cancel: %v %v", rows, err)
	}
}

func TestLocalRecoveryIgnoresRemoteLeases(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobLeaseRepository(db)
	jobsRepo := NewJobHistoryRepository(db)
	w := leaseTestWorker(t, db)
	phase := leaseTestPhase()
	old := time.Now().Add(-2 * time.Hour)
	remote := leaseTestJob(t, db, phase, 0, old)
	repo.ClaimForWorker(claimFilter(phase, w, 1), old)
	local := leaseTestJob(t, db, phase, 0, old)
	db.Model(&JobHistory{}).Where("id = ?", local.ID).Updates(map[string]any{"status": JobStatusRunning, "started_at": old})

	if _, err := jobsRepo.MarkOrphanedRunningAsFailed(time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := reloadJob(t, db, remote.ID); got.Status != JobStatusRunning {
		t.Fatal("orphan recovery touched a remote lease")
	}
	if got := reloadJob(t, db, local.ID); got.Status != JobStatusFailed {
		t.Fatal("orphan recovery skipped a local job")
	}
	db.Model(&JobHistory{}).Where("id = ?", local.ID).Update("status", JobStatusRunning)
	if _, err := jobsRepo.MarkRunningAsInterrupted(); err != nil {
		t.Fatal(err)
	}
	if got := reloadJob(t, db, remote.ID); got.Status != JobStatusRunning {
		t.Fatal("shutdown interrupted a remote lease")
	}
	if _, err := jobsRepo.ResetJobsToPending([]string{remote.JobID}); err != nil {
		t.Fatal(err)
	}
	if got := reloadJob(t, db, remote.ID); got.Status != JobStatusRunning {
		t.Fatal("reset to pending touched a remote lease")
	}
}

func TestWorkerRepository(t *testing.T) {
	db := importTestDB(t)
	repo := NewWorkerRepository(db)
	w := leaseTestWorker(t, db)
	hash := "abc"
	if err := repo.Update(w.ID, map[string]any{"token_hash": hash, "capabilities": `{"accel":"nvidia"}`,
		"allowed_phases": pq.StringArray{"sprites"}}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(w.ID)
	if err != nil || got.TokenHash == nil || *got.TokenHash != hash || string(got.Capabilities) != `{"accel": "nvidia"}` {
		t.Fatalf("worker: %+v %v (caps %s)", got, err, got.Capabilities)
	}
	if len(got.AllowedPhases) != 1 || got.AllowedPhases[0] != "sprites" {
		t.Fatalf("allowed phases %v", got.AllowedPhases)
	}
	for _, c := range []string{WorkerCounterCompleted, WorkerCounterCompleted, WorkerCounterFailed} {
		if err := repo.IncrementCounter(w.ID, c, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.IncrementCounter(w.ID, "name", 1); err == nil {
		t.Fatal("unknown counter accepted")
	}
	got, _ = repo.GetByID(w.ID)
	if got.JobsCompleted != 2 || got.JobsFailed != 1 {
		t.Fatalf("counters %d %d", got.JobsCompleted, got.JobsFailed)
	}
	if err := repo.Update(999_999_999, map[string]any{"name": "x"}); err != gorm.ErrRecordNotFound {
		t.Fatalf("update of a missing worker: %v", err)
	}

	first, err := repo.EnsureMediaSecret("first-secret-" + uuid.NewString())
	if err != nil || first == "" {
		t.Fatalf("ensure secret: %q %v", first, err)
	}
	second, _ := repo.EnsureMediaSecret("another")
	if second != first {
		t.Fatal("the media secret changed on a second start")
	}
	if err := repo.UpdatePhaseRouting([]byte(`{"sprites":"remote"}`)); err != nil {
		t.Fatal(err)
	}
	s, _ := repo.GetSettings()
	if string(s.PhaseRouting) != `{"sprites": "remote"}` {
		t.Fatalf("routing %s", s.PhaseRouting)
	}
	repo.UpdatePhaseRouting([]byte(`{}`))
	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int, 0, len(list))
	for _, x := range list {
		ids = append(ids, int(x.ID))
	}
	if !sort.IntsAreSorted(ids) {
		t.Fatal("workers not listed by id")
	}
}
