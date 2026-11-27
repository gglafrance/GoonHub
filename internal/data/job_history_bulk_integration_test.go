package data

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration tests of the bulk cancel and of the batched job history purge, on the disposable
// import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).

type bulkTestJob struct {
	status      string
	age         time.Duration
	retryAt     *time.Time
	isRetryable bool
}

// createBulkTestJob inserts a job of the given phase and status started age ago. Scene IDs are
// random (job_history has no foreign key) so that active jobs never hit the unique index.
func createBulkTestJob(t *testing.T, db *gorm.DB, phase string, j bulkTestJob) *JobHistory {
	t.Helper()
	at := time.Now().Add(-j.age)
	row := &JobHistory{
		JobID:       uuid.NewString(),
		SceneID:     uint(rand.IntN(1_000_000_000) + 1_000_000),
		Phase:       phase,
		Status:      j.status,
		CreatedAt:   at,
		StartedAt:   at,
		NextRetryAt: j.retryAt,
		IsRetryable: j.isRetryable,
	}
	if j.status != JobStatusPending && j.status != JobStatusRunning {
		row.CompletedAt = &at
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	t.Cleanup(func() { db.Delete(&JobHistory{}, row.ID) })
	return row
}

func jobStatus(t *testing.T, db *gorm.DB, id uint) (string, bool) {
	t.Helper()
	var row JobHistory
	err := db.First(&row, id).Error
	if err == gorm.ErrRecordNotFound {
		return "", false
	}
	if err != nil {
		t.Fatalf("read job %d: %v", id, err)
	}
	return row.Status, true
}

func TestCancelPendingJobsByPhases(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	phase, other := claimTestPhase(), claimTestPhase()

	var pending []*JobHistory
	for range 3 {
		pending = append(pending, createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusPending, isRetryable: true}))
	}
	running := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusRunning})
	otherPending := createBulkTestJob(t, db, other, bulkTestJob{status: JobStatusPending})

	n, err := repo.CancelPendingJobsByPhases([]string{phase}, "bulk test")
	if err != nil {
		t.Fatalf("CancelPendingJobsByPhases: %v", err)
	}
	if n != 3 {
		t.Fatalf("cancelled %d jobs, want 3", n)
	}
	for _, j := range pending {
		var row JobHistory
		if err := db.First(&row, j.ID).Error; err != nil {
			t.Fatal(err)
		}
		if row.Status != JobStatusCancelled || row.CompletedAt == nil || row.IsRetryable ||
			row.ErrorMessage == nil || *row.ErrorMessage != "bulk test" {
			t.Fatalf("job after cancel = status %s, completed_at %v, retryable %v, error %v",
				row.Status, row.CompletedAt, row.IsRetryable, row.ErrorMessage)
		}
	}
	if st, _ := jobStatus(t, db, running.ID); st != JobStatusRunning {
		t.Fatalf("running job became %s", st)
	}
	if st, _ := jobStatus(t, db, otherPending.ID); st != JobStatusPending {
		t.Fatalf("pending job of another phase became %s", st)
	}
}

// A feeder claim in progress holds row locks: the bulk UPDATE waits for it, then skips the rows
// it claimed (they are running by then) and cancels all the others.
func TestCancelPendingJobsByPhases_WaitsForConcurrentClaim(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	phase := claimTestPhase()
	for range 5 {
		createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusPending})
	}

	tx := db.Begin()
	var claimed []uint
	if err := tx.Raw(`SELECT id FROM job_history WHERE phase = ? AND status = 'pending'
		ORDER BY id LIMIT 2 FOR UPDATE SKIP LOCKED`, phase).Scan(&claimed).Error; err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Model(&JobHistory{}).Where("id IN ?", claimed).Update("status", JobStatusRunning).Error; err != nil {
		tx.Rollback()
		t.Fatal(err)
	}

	type outcome struct {
		n   int64
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		n, err := repo.CancelPendingJobsByPhases([]string{phase}, "bulk test")
		done <- outcome{n, err}
	}()
	select {
	case o := <-done:
		tx.Rollback()
		t.Fatalf("bulk cancel did not wait for the claim: %+v", o)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}

	o := <-done
	if o.err != nil {
		t.Fatalf("CancelPendingJobsByPhases: %v", o.err)
	}
	if o.n != 3 {
		t.Fatalf("cancelled %d jobs, want the 3 not claimed", o.n)
	}
	for _, id := range claimed {
		if st, _ := jobStatus(t, db, id); st != JobStatusRunning {
			t.Fatalf("claimed job %d became %s", id, st)
		}
	}
	var stillPending int64
	db.Model(&JobHistory{}).Where("phase = ? AND status = ?", phase, JobStatusPending).Count(&stillPending)
	if stillPending != 0 {
		t.Fatalf("%d jobs still pending", stillPending)
	}
}

func TestCancelClaimedJobs(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	phase := claimTestPhase()
	claimed := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusRunning})
	finished := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusCompleted})

	n, err := repo.CancelClaimedJobs([]string{claimed.JobID, finished.JobID, uuid.NewString()}, "bulk test")
	if err != nil {
		t.Fatalf("CancelClaimedJobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("cancelled %d jobs, want 1", n)
	}
	if st, _ := jobStatus(t, db, claimed.ID); st != JobStatusCancelled {
		t.Fatalf("claimed job is %s, want cancelled", st)
	}
	if st, _ := jobStatus(t, db, finished.ID); st != JobStatusCompleted {
		t.Fatalf("completed job became %s", st)
	}
	if n, err := repo.CancelClaimedJobs(nil, "bulk test"); err != nil || n != 0 {
		t.Fatalf("CancelClaimedJobs(nil) = %d, %v", n, err)
	}
}

func TestPurgeFinishedBatch(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	phase := claimTestPhase()
	day := 24 * time.Hour
	retryAt := time.Now().Add(time.Hour)

	oldCompleted := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusCompleted, age: 40 * day})
	oldCompleted2 := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusCompleted, age: 35 * day})
	oldCancelled := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusCancelled, age: 31 * day})
	newCompleted := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusCompleted, age: 2 * day})
	oldFailed := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusFailed, age: 100 * day})
	oldTimedOut := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusTimedOut, age: 100 * day})
	retrying := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusFailed, age: 100 * day, retryAt: &retryAt, isRetryable: true})
	recentFailed := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusFailed, age: 40 * day})
	oldPending := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusPending, age: 200 * day})
	oldRunning := createBulkTestJob(t, db, phase, bulkTestJob{status: JobStatusRunning, age: 200 * day})

	dlq := &DLQEntry{JobID: oldFailed.JobID, SceneID: oldFailed.SceneID, Phase: phase, OriginalError: "boom", LastError: "boom"}
	if err := db.Create(dlq).Error; err != nil {
		t.Fatalf("create DLQ entry: %v", err)
	}
	t.Cleanup(func() { db.Delete(&DLQEntry{}, dlq.ID) })

	completed := []string{JobStatusCompleted, JobStatusCancelled}
	failed := []string{JobStatusFailed, JobStatusTimedOut}

	counts, err := repo.CountPurgeable(completed, time.Now().Add(-30*day))
	if err != nil {
		t.Fatalf("CountPurgeable: %v", err)
	}
	if counts[JobStatusCompleted] < 2 || counts[JobStatusCancelled] < 1 {
		t.Fatalf("counts = %v, want at least 2 completed and 1 cancelled", counts)
	}

	// Batches of one row: the loop ends on the first short batch.
	batches := 0
	for {
		n, err := repo.PurgeFinishedBatch(completed, time.Now().Add(-30*day), 1)
		if err != nil {
			t.Fatalf("PurgeFinishedBatch: %v", err)
		}
		batches++
		if n < 1 {
			break
		}
		if n != 1 {
			t.Fatalf("batch deleted %d rows, want at most 1", n)
		}
	}
	if batches < 4 {
		t.Fatalf("purge took %d batches, want at least 4 (3 rows, 1 per batch)", batches)
	}

	if _, err := repo.PurgeFinishedBatch(failed, time.Now().Add(-90*day), 5000); err != nil {
		t.Fatalf("PurgeFinishedBatch: %v", err)
	}

	for _, gone := range []*JobHistory{oldCompleted, oldCompleted2, oldCancelled, oldFailed, oldTimedOut} {
		if st, ok := jobStatus(t, db, gone.ID); ok {
			t.Fatalf("old %s job %d was not purged", st, gone.ID)
		}
	}
	for _, kept := range []*JobHistory{newCompleted, retrying, recentFailed, oldPending, oldRunning} {
		if _, ok := jobStatus(t, db, kept.ID); !ok {
			t.Fatalf("%s job %d (age %s) was purged", kept.Status, kept.ID, time.Since(kept.StartedAt).Round(time.Hour))
		}
	}
	var dlqCount int64
	db.Model(&DLQEntry{}).Where("id = ?", dlq.ID).Count(&dlqCount)
	if dlqCount != 1 {
		t.Fatal("the DLQ entry of a purged job was deleted")
	}
}

func TestPurgeFinishedBatch_RefusesActiveStatuses(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	for _, statuses := range [][]string{{JobStatusPending}, {JobStatusCompleted, JobStatusRunning}, {}} {
		if _, err := repo.PurgeFinishedBatch(statuses, time.Now(), 10); err == nil {
			t.Fatalf("PurgeFinishedBatch(%v) succeeded, want an error", statuses)
		}
		if _, err := repo.CountPurgeable(statuses, time.Now()); err == nil {
			t.Fatalf("CountPurgeable(%v) succeeded, want an error", statuses)
		}
	}
	if _, err := repo.PurgeFinishedBatch([]string{JobStatusCompleted}, time.Now(), 0); err == nil {
		t.Fatal("PurgeFinishedBatch with batch size 0 succeeded, want an error")
	}
}
