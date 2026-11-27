package data

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration tests of the failure counters per (scene, phase) (migration 000092) and of the
// startup re-queue of interrupted jobs, on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).

func int64Ptr(v int64) *int64 { return &v }

func TestScenePhaseFailureRepository_RecordClearList(t *testing.T) {
	db := importTestDB(t)
	repo := NewScenePhaseFailureRepository(db)
	phase := claimTestPhase()
	scene := createClaimTestScene(t, db, "failing scene", time.Now())
	other := createClaimTestScene(t, db, "other scene", time.Now())
	t.Cleanup(func() { db.Where("phase = ?", phase).Delete(&ScenePhaseFailure{}) })

	mod := time.Date(2026, 11, 20, 10, 0, 0, 123456000, time.UTC)
	rec := ScenePhaseFailureRecord{SceneID: scene.ID, Phase: phase, Error: "video file is corrupted", FailedAt: time.Now(), FileSize: int64Ptr(100), FileModTime: &mod}

	// Same file: the counter goes up
	for want := 1; want <= 3; want++ {
		row, err := repo.RecordFailure(rec)
		if err != nil {
			t.Fatal(err)
		}
		if row.Failures != want || row.FileSize == nil || *row.FileSize != 100 || !row.FileModTime.Equal(mod) {
			t.Fatalf("failure %d: got %+v", want, row)
		}
	}
	// Replaced file: the counter restarts at 1
	newMod := mod.Add(time.Hour)
	rec.FileModTime = &newMod
	rec.Error = "failed to parse duration"
	row, err := repo.RecordFailure(rec)
	if err != nil || row.Failures != 1 || row.LastError != "failed to parse duration" {
		t.Fatalf("changed file: %+v, %v", row, err)
	}
	// Unreadable file twice (no state): NULL equals NULL, the counter goes up
	rec.FileSize, rec.FileModTime = nil, nil
	if _, err := repo.RecordFailure(rec); err != nil {
		t.Fatal(err)
	}
	if row, err = repo.RecordFailure(rec); err != nil || row.Failures != 2 || row.FileSize != nil {
		t.Fatalf("unreadable file: %+v, %v", row, err)
	}

	otherRec := ScenePhaseFailureRecord{SceneID: other.ID, Phase: phase, Error: "ffmpeg failed 100%_x", FailedAt: time.Now().Add(time.Minute)}
	if _, err := repo.RecordFailure(otherRec); err != nil {
		t.Fatal(err)
	}

	rows, err := repo.List(ScenePhaseFailureFilter{Phase: phase})
	if err != nil || len(rows) != 2 || rows[0].SceneID != other.ID || rows[0].SceneTitle != "other scene" || rows[1].StoredPath != scene.StoredPath {
		t.Fatalf("list (most recent first, with the scene): %+v, %v", rows, err)
	}
	if rows, _ = repo.List(ScenePhaseFailureFilter{Phase: phase, MinFailures: 2}); len(rows) != 1 || rows[0].SceneID != scene.ID {
		t.Fatalf("min failures: %+v", rows)
	}
	if rows, _ = repo.List(ScenePhaseFailureFilter{Phase: phase, ErrorQuery: "PARSE"}); len(rows) != 1 || rows[0].SceneID != scene.ID {
		t.Fatalf("error filter (case-insensitive): %+v", rows)
	}
	if rows, _ = repo.List(ScenePhaseFailureFilter{Phase: phase, ErrorQuery: "100%_"}); len(rows) != 1 || rows[0].SceneID != other.ID {
		t.Fatalf("error filter with LIKE wildcards: %+v", rows)
	}

	got, err := repo.GetByScenePhase(scene.ID, phase)
	if err != nil || got.Failures != 2 {
		t.Fatalf("get: %+v, %v", got, err)
	}
	byIDs, err := repo.GetByIDs([]uint{got.ID, 0})
	if err != nil || len(byIDs) != 1 || byIDs[0].SceneID != scene.ID {
		t.Fatalf("get by IDs: %+v, %v", byIDs, err)
	}

	// A trashed scene leaves the list
	if err := db.Model(&Scene{}).Where("id = ?", other.ID).Update("trashed_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if rows, _ = repo.List(ScenePhaseFailureFilter{Phase: phase}); len(rows) != 1 {
		t.Fatalf("trashed scene still listed: %+v", rows)
	}

	// Success clears; delete by IDs
	if err := repo.Clear(scene.ID, phase); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByScenePhase(scene.ID, phase); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cleared counter still there: %v", err)
	}
	var otherID uint
	if err := db.Model(&ScenePhaseFailure{}).Select("id").Where("scene_id = ? AND phase = ?", other.ID, phase).Scan(&otherID).Error; err != nil {
		t.Fatal(err)
	}
	if n, err := repo.DeleteByIDs([]uint{otherID}); err != nil || n != 1 {
		t.Fatalf("delete by IDs: %d, %v", n, err)
	}
}

func TestScenePhaseFailureRepository_SceneDeletionCascades(t *testing.T) {
	db := importTestDB(t)
	repo := NewScenePhaseFailureRepository(db)
	phase := claimTestPhase()
	scene := createClaimTestScene(t, db, "deleted scene", time.Now())
	if _, err := repo.RecordFailure(ScenePhaseFailureRecord{SceneID: scene.ID, Phase: phase, FailedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := db.Unscoped().Delete(&Scene{}, scene.ID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&ScenePhaseFailure{}).Where("phase = ?", phase).Count(&count)
	if count != 0 {
		t.Fatalf("counter of a deleted scene kept: %d", count)
	}
}

// createRequeueTestJob creates a finished job with a status and an error message.
func createRequeueTestJob(t *testing.T, db *gorm.DB, sceneID uint, phase, status, errMsg string, createdAt time.Time, retryable bool) string {
	t.Helper()
	j := &JobHistory{
		JobID: uuid.NewString(), SceneID: sceneID, Phase: phase, Status: status,
		CreatedAt: createdAt, StartedAt: createdAt, RetryCount: 2, Priority: 1,
	}
	if errMsg != "" {
		j.ErrorMessage = &errMsg
	}
	now := time.Now()
	j.CompletedAt = &now
	if err := db.Create(j).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := db.Model(&JobHistory{}).Where("id = ?", j.ID).Update("is_retryable", retryable).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&JobHistory{}, j.ID) })
	return j.JobID
}

func TestRequeueInterruptedJobs(t *testing.T) {
	db := importTestDB(t)
	repo := NewJobHistoryRepository(db)
	phase := claimTestPhase()
	old := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-time.Hour)
	s := func(title string) uint { return createClaimTestScene(t, db, title, old).ID }

	shutdown := createRequeueTestJob(t, db, s("shutdown"), phase, JobStatusFailed, JobErrorInterruptedByShutdown, old, true)
	orphan := createRequeueTestJob(t, db, s("orphan"), phase, JobStatusFailed, JobErrorOrphanRecovered, old, true)
	stuck := createRequeueTestJob(t, db, s("stuck"), phase, JobStatusFailed, JobErrorStuckPendingRecovered, old, true)
	realFailure := createRequeueTestJob(t, db, s("real failure"), phase, JobStatusFailed, "video file is corrupted", old, true)
	cancelled := createRequeueTestJob(t, db, s("cancelled"), phase, JobStatusCancelled, "job was cancelled", old, true)
	retriedByHand := createRequeueTestJob(t, db, s("retried by hand"), phase, JobStatusFailed, JobErrorInterruptedByShutdown, old, false)

	superseded := s("superseded")
	supersededJob := createRequeueTestJob(t, db, superseded, phase, JobStatusFailed, JobErrorInterruptedByShutdown, old, true)
	createRequeueTestJob(t, db, superseded, phase, JobStatusCompleted, "", newer, true)

	active := s("active")
	activeOld := createRequeueTestJob(t, db, active, phase, JobStatusFailed, JobErrorInterruptedByShutdown, old, true)
	createClaimTestJob(t, db, active, "other-"+phase, 0, newer) // other phase: no effect
	if err := db.Model(&JobHistory{}).Where("job_id = ?", activeOld).Update("created_at", newer.Add(time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	activePending := createClaimTestJob(t, db, active, phase, 0, old) // older but active: dedup wins

	trashed := s("trashed")
	trashedJob := createRequeueTestJob(t, db, trashed, phase, JobStatusFailed, JobErrorInterruptedByShutdown, old, true)
	db.Model(&Scene{}).Where("id = ?", trashed).Update("trashed_at", time.Now())

	if _, err := repo.RequeueInterruptedJobs(); err != nil {
		t.Fatal(err)
	}
	status := func(jobID string) *JobHistory {
		j, err := repo.GetByJobID(jobID)
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	for _, id := range []string{shutdown, orphan, stuck} {
		j := status(id)
		if j.Status != JobStatusPending || j.ErrorMessage != nil || j.CompletedAt != nil || j.RetryCount != 2 || j.Priority != 1 {
			t.Fatalf("interrupted job %s not put back as it was: %+v", id, j)
		}
	}
	for _, id := range []string{realFailure, retriedByHand, supersededJob, activeOld, trashedJob} {
		if j := status(id); j.Status != JobStatusFailed {
			t.Fatalf("job %s must stay failed, got %s", id, j.Status)
		}
	}
	if j := status(cancelled); j.Status != JobStatusCancelled {
		t.Fatalf("cancelled job touched: %s", j.Status)
	}
	if j := status(activePending); j.Status != JobStatusPending {
		t.Fatalf("active job touched: %s", j.Status)
	}

	// Interrupted again: the job that ran is not put back twice, the stuck one always is
	for _, id := range []string{shutdown, orphan} {
		db.Model(&JobHistory{}).Where("job_id = ?", id).Updates(map[string]any{"status": JobStatusFailed, "error_message": JobErrorInterruptedByShutdown})
	}
	db.Model(&JobHistory{}).Where("job_id = ?", stuck).Updates(map[string]any{"status": JobStatusFailed, "error_message": JobErrorStuckPendingRecovered})
	if _, err := repo.RequeueInterruptedJobs(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{shutdown, orphan} {
		if j := status(id); j.Status != JobStatusFailed {
			t.Fatalf("job %s interrupted twice must stay failed, got %s", id, j.Status)
		}
	}
	if j := status(stuck); j.Status != JobStatusPending {
		t.Fatalf("stuck pending job must be put back again, got %s", j.Status)
	}
}
