package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration tests of the job queue claim order (job priority, new scenes first, age), of the
// stuck pending recovery exclusion and of the job schedule settings (migration 000077), on the
// disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).

// claimTestPhase returns a phase name of its own so that other jobs in the database never mix in.
func claimTestPhase() string {
	return "it-" + uuid.NewString()[:8]
}

func createClaimTestScene(t *testing.T, db *gorm.DB, title string, createdAt time.Time) *Scene {
	t.Helper()
	s := &Scene{
		Title:      title,
		StoredPath: "/claim-test/" + uuid.NewString() + ".mp4",
		Tags:       pq.StringArray{},
		Actors:     pq.StringArray{},
		CreatedAt:  createdAt,
	}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("create scene: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&Scene{}, s.ID) })
	return s
}

func createClaimTestJob(t *testing.T, db *gorm.DB, sceneID uint, phase string, priority int, createdAt time.Time) string {
	t.Helper()
	j := &JobHistory{
		JobID:     uuid.NewString(),
		SceneID:   sceneID,
		Phase:     phase,
		Status:    JobStatusPending,
		Priority:  priority,
		CreatedAt: createdAt,
		StartedAt: createdAt,
	}
	if err := db.Create(j).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	t.Cleanup(func() { db.Delete(&JobHistory{}, j.ID) })
	return j.JobID
}

func claimedIDs(jobs []JobHistory) []string {
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.JobID
	}
	return ids
}

func assertJobOrder(t *testing.T, got []JobHistory, want ...string) {
	t.Helper()
	ids := claimedIDs(got)
	if len(ids) != len(want) {
		t.Fatalf("claimed %d jobs %v, want %d %v", len(ids), ids, len(want), want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("claim order %v, want %v", ids, want)
		}
	}
	for _, j := range got {
		if j.Status != JobStatusRunning {
			t.Fatalf("claimed job %s has status %s, want running", j.JobID, j.Status)
		}
	}
}

func TestClaimPendingJobsOrder(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	now := time.Now()

	oldScene := createClaimTestScene(t, db, "old", now.Add(-30*24*time.Hour))
	oldScene2 := createClaimTestScene(t, db, "old 2", now.Add(-20*24*time.Hour))
	newScene := createClaimTestScene(t, db, "new", now.Add(-2*time.Hour))
	newScene2 := createClaimTestScene(t, db, "new 2", now.Add(-1*time.Hour))
	retryScene := createClaimTestScene(t, db, "retry", now.Add(-40*24*time.Hour))

	setup := func(phase string) (oldJob, old2Job, newJob, new2Job, retryJob, goneJob string) {
		// created in this order: old jobs first (bulk run), then the new scenes, then a retry
		oldJob = createClaimTestJob(t, db, oldScene.ID, phase, 0, now.Add(-50*time.Minute))
		old2Job = createClaimTestJob(t, db, oldScene2.ID, phase, 0, now.Add(-40*time.Minute))
		// job of a scene that no longer exists
		goneJob = createClaimTestJob(t, db, 999999999, phase, 0, now.Add(-35*time.Minute))
		newJob = createClaimTestJob(t, db, newScene.ID, phase, 0, now.Add(-30*time.Minute))
		new2Job = createClaimTestJob(t, db, newScene2.ID, phase, 0, now.Add(-20*time.Minute))
		retryJob = createClaimTestJob(t, db, retryScene.ID, phase, 1, now.Add(-10*time.Minute))
		return
	}

	t.Run("priority then oldest first", func(t *testing.T) {
		phase := claimTestPhase()
		oldJob, old2Job, newJob, new2Job, retryJob, goneJob := setup(phase)
		got, err := repo.ClaimPendingJobs(phase, 10, ClaimOptions{})
		if err != nil {
			t.Fatal(err)
		}
		assertJobOrder(t, got, retryJob, oldJob, old2Job, goneJob, newJob, new2Job)
	})

	t.Run("new scenes first within a priority", func(t *testing.T) {
		phase := claimTestPhase()
		oldJob, old2Job, newJob, new2Job, retryJob, goneJob := setup(phase)
		got, err := repo.ClaimPendingJobs(phase, 10, ClaimOptions{NewSceneSince: now.Add(-24 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		assertJobOrder(t, got, retryJob, newJob, new2Job, oldJob, old2Job, goneJob)
	})

	t.Run("limit and no double claim", func(t *testing.T) {
		phase := claimTestPhase()
		oldJob, old2Job, newJob, new2Job, retryJob, goneJob := setup(phase)
		opts := ClaimOptions{NewSceneSince: now.Add(-24 * time.Hour)}
		first, err := repo.ClaimPendingJobs(phase, 2, opts)
		if err != nil {
			t.Fatal(err)
		}
		assertJobOrder(t, first, retryJob, newJob)
		second, err := repo.ClaimPendingJobs(phase, 10, opts)
		if err != nil {
			t.Fatal(err)
		}
		assertJobOrder(t, second, new2Job, oldJob, old2Job, goneJob)
		third, err := repo.ClaimPendingJobs(phase, 10, opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(third) != 0 {
			t.Fatalf("expected nothing left to claim, got %v", claimedIDs(third))
		}
	})

	t.Run("concurrent claims never return the same job", func(t *testing.T) {
		phase := claimTestPhase()
		setup(phase)
		opts := ClaimOptions{NewSceneSince: now.Add(-24 * time.Hour)}
		results := make(chan []JobHistory, 3)
		errs := make(chan error, 3)
		for i := 0; i < 3; i++ {
			go func() {
				got, err := repo.ClaimPendingJobs(phase, 2, opts)
				errs <- err
				results <- got
			}()
		}
		seen := map[string]bool{}
		for i := 0; i < 3; i++ {
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
			for _, j := range <-results {
				if seen[j.JobID] {
					t.Fatalf("job %s claimed twice", j.JobID)
				}
				seen[j.JobID] = true
			}
		}
		// A claimer may come back short when it races another one; the rest is still pending.
		rest, err := repo.ClaimPendingJobs(phase, 10, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range rest {
			if seen[j.JobID] {
				t.Fatalf("job %s claimed twice", j.JobID)
			}
			seen[j.JobID] = true
		}
		if len(seen) != 6 {
			t.Fatalf("expected the 6 jobs claimed, got %d", len(seen))
		}
	})
}

func TestMarkStuckPendingJobsAsFailedExcludesPhases(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	old := time.Now().Add(-48 * time.Hour)
	windowed := claimTestPhase()
	other := claimTestPhase()
	keep := createClaimTestJob(t, db, 1, windowed, 0, old)
	fail := createClaimTestJob(t, db, 2, other, 0, old)

	if _, err := repo.MarkStuckPendingJobsAsFailed(24*time.Hour, []string{windowed}); err != nil {
		t.Fatal(err)
	}
	k, err := repo.GetByJobID(keep)
	if err != nil {
		t.Fatal(err)
	}
	if k.Status != JobStatusPending {
		t.Fatalf("job of an excluded phase is %s, want pending", k.Status)
	}
	f, err := repo.GetByJobID(fail)
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != JobStatusFailed {
		t.Fatalf("stuck job of another phase is %s, want failed", f.Status)
	}
}

func TestJobScheduleRepository(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobScheduleRepository(db)

	origCfg, err := repo.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	origPhases, err := repo.ListPhases()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if origCfg != nil {
			if err := repo.Save(origCfg, origPhases); err != nil {
				t.Errorf("restore job schedule: %v", err)
			}
		}
	})

	// Seeded by the migration
	if origCfg == nil || origCfg.Timezone == "" {
		t.Fatalf("expected the seeded singleton row, got %+v", origCfg)
	}
	if len(origPhases) != 5 {
		t.Fatalf("expected 5 seeded phases, got %d", len(origPhases))
	}

	cfg := &JobScheduleConfigRecord{
		Timezone: "Europe/Paris", QuietHoursStart: "22:00", QuietHoursEnd: "06:30",
		MaxConcurrentJobs: 8, NewScenesFirst: true, NewScenesWindowHours: 48,
	}
	phases := []JobPhaseScheduleRecord{
		{Phase: "fingerprint", Priority: 5, WindowEnabled: true, WindowStart: "01:00", WindowEnd: "07:00", QuietWorkers: 12},
		{Phase: "thumbnail", Priority: 90, WindowStart: "01:00", WindowEnd: "07:00"},
	}
	if err := repo.Save(cfg, phases); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := repo.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Timezone != "Europe/Paris" || got.QuietHoursEnd != "06:30" || got.MaxConcurrentJobs != 8 || !got.NewScenesFirst || got.NewScenesWindowHours != 48 {
		t.Fatalf("config not saved: %+v", got)
	}
	gotPhases, err := repo.ListPhases()
	if err != nil {
		t.Fatal(err)
	}
	if len(gotPhases) != 5 {
		t.Fatalf("upsert must not add rows, got %d phases", len(gotPhases))
	}
	for _, p := range gotPhases {
		switch p.Phase {
		case "fingerprint":
			if p.Priority != 5 || !p.WindowEnabled || p.QuietWorkers != 12 {
				t.Fatalf("fingerprint not saved: %+v", p)
			}
		case "thumbnail":
			if p.Priority != 90 || p.WindowEnabled {
				t.Fatalf("thumbnail not saved: %+v", p)
			}
		}
	}

	// The database rejects what the service would never send
	bad := []JobPhaseScheduleRecord{{Phase: "scan", Priority: 1, WindowStart: "01:00", WindowEnd: "02:00"}}
	if err := repo.Save(cfg, bad); err == nil {
		t.Fatal("expected the phase check constraint to reject scan")
	}
	badWindow := []JobPhaseScheduleRecord{{Phase: "sprites", WindowStart: "03:00", WindowEnd: "03:00"}}
	if err := repo.Save(cfg, badWindow); err == nil {
		t.Fatal("expected an empty window to be rejected")
	}
}
