package data

import (
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// lockingClause locks the selected rows until the end of the transaction.
func lockingClause() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}

// JobLeaseRepository holds the remote worker leases on job_history rows (docs/REMOTE_WORKERS.md,
// section 6). Every guarded call is a compare-and-set on (job_id, lease_generation, worker_id,
// status = 'running'): it returns false when no row matched (the lease is lost, the job was
// cancelled or completed). Times come from the caller (server clock).
type JobLeaseRepository interface {
	// ClaimForWorker leases up to f.Limit pending jobs of f.Phase to a worker in one
	// transaction (FOR UPDATE SKIP LOCKED, shared with the local feeder): status running,
	// worker_id set, generation + 1, lease until now + f.LeaseDuration.
	ClaimForWorker(f LeaseClaimFilter, now time.Time) ([]JobHistory, error)
	// SetLeaseParams stores the phase parameters prepared for the lease.
	SetLeaseParams(jobID string, generation int64, workerID uint, params []byte) (bool, error)
	// RenewLease extends a lease to expires and records the worker progress. A lease whose
	// job started before notBefore (max assignment age) is not renewed.
	RenewLease(r LeaseRenewal) (bool, error)
	// CheckLease reports whether the lease is current, without changing anything.
	CheckLease(jobID string, generation int64, workerID uint) (bool, error)
	// ReleaseLease puts a leased job back to pending at once (generation + 1, retry_count + 1
	// with countAttempt).
	ReleaseLease(jobID string, generation int64, workerID uint, countAttempt bool) (bool, error)
	// ReleaseWorkerLeases releases every lease of a worker except the jobs in keep, and
	// returns the released rows (as they were before the release).
	ReleaseWorkerLeases(workerID uint, keep []string, countAttempt bool) ([]JobHistory, error)
	// ExpireLeases ends the remote leases expired at now (at most limit rows): a job whose
	// retry_count + 1 exceeds maxRetries[phase] is set failed ("lease expired"), the others go
	// back to pending with retry_count + 1. Returns both lists, rows as before the change.
	ExpireLeases(now time.Time, maxRetries map[string]int, limit int) (requeued, failed []JobHistory, err error)
	// MarkCommitStarted records that the commit of this generation starts, and extends the
	// lease to leaseUntil (a commit is never overtaken by the expiry sweep). started is false
	// when a commit of this generation had already started (a retried complete).
	MarkCommitStarted(jobID string, generation int64, workerID uint, leaseUntil time.Time) (ok bool, started bool, err error)
	// CompleteLease marks the job completed by this generation (committed_generation).
	CompleteLease(jobID string, generation int64, workerID uint, now time.Time) (bool, error)
	// FailLease marks the job failed with message (worker_id kept for history).
	FailLease(jobID string, generation int64, workerID uint, now time.Time, message string, retryable bool) (bool, error)
	// CancelLease cancels a running remote job (generation + 1) and returns the row as it was
	// (nil when the job is not a running remote job).
	CancelLease(jobID string, now time.Time, reason string) (*JobHistory, error)
	// CancelRemoteLeasesByPhases cancels every running remote job of the phases.
	CancelRemoteLeasesByPhases(phases []string, now time.Time, reason string) ([]JobHistory, error)
	// ExtendRemoteLeases moves every remote lease expiring before until to until (server
	// start: workers get time to reconnect).
	ExtendRemoteLeases(until time.Time) (int64, error)
	// ListRemoteRunning lists the running remote jobs (all workers when workerID is 0).
	ListRemoteRunning(workerID uint) ([]JobHistory, error)
	// GetJobs returns the rows of the given job IDs, by job ID.
	GetJobs(jobIDs []string) (map[string]JobHistory, error)
}

// LeaseClaimFilter selects the jobs a worker may claim.
type LeaseClaimFilter struct {
	Phase         string
	Limit         int
	WorkerID      uint
	LeaseDuration time.Duration
	// NewSceneSince, when not zero, claims the jobs of recent scenes first (job schedule).
	NewSceneSince time.Time
	// PreferScenes are claimed first within a priority (scene affinity: the phases of a scene
	// go to the worker that already holds another phase of it).
	PreferScenes []uint
	// PathPrefixes, when set, restricts the claim to scenes stored under one of the prefixes
	// (a worker with direct input only). Jobs whose scene is gone are claimed anyway: the
	// caller fails them.
	PathPrefixes []string
	// NoAudioOnly restricts the claim to scenes without an audio track (fingerprint on a
	// worker without chromaprint).
	NoAudioOnly bool
}

// LeaseRenewal is one heartbeat of one job.
type LeaseRenewal struct {
	JobID      string
	Generation int64
	WorkerID   uint
	Now        time.Time
	ExpiresAt  time.Time
	NotBefore  time.Time // started_at must be after this (zero = no limit)
	Progress   int
	Stage      string
	InputMode  string
}

// JobLeaseRepositoryImpl is the GORM implementation of JobLeaseRepository.
type JobLeaseRepositoryImpl struct {
	DB *gorm.DB
}

// NewJobLeaseRepository creates a JobLeaseRepository.
func NewJobLeaseRepository(db *gorm.DB) *JobLeaseRepositoryImpl {
	return &JobLeaseRepositoryImpl{DB: db}
}

// leaseCAS is the compare-and-set condition of every guarded call.
const leaseCAS = "job_id = ? AND lease_generation = ? AND worker_id = ? AND status = 'running'"

// likePrefix escapes a path prefix for LIKE and makes it match the prefix directory only
// ("/media/a" matches "/media/a/x.mp4", not "/media/ab/x.mp4").
func likePrefix(prefix string) string {
	p := strings.TrimRight(prefix, "/")
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(p) + "/%"
}

func (r *JobLeaseRepositoryImpl) ClaimForWorker(f LeaseClaimFilter, now time.Time) ([]JobHistory, error) {
	if f.Limit <= 0 || f.WorkerID == 0 {
		return nil, nil
	}
	var jobs []JobHistory
	silent := r.DB.Session(&gorm.Session{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	err := silent.Transaction(func(tx *gorm.DB) error {
		var where strings.Builder
		args := []any{}
		where.WriteString("jh.phase = ? AND jh.status = 'pending'")
		args = append(args, f.Phase)
		if len(f.PathPrefixes) > 0 {
			where.WriteString(" AND (s.id IS NULL")
			for _, p := range f.PathPrefixes {
				where.WriteString(` OR s.stored_path LIKE ? ESCAPE '\'`)
				args = append(args, likePrefix(p))
			}
			where.WriteString(")")
		}
		if f.NoAudioOnly {
			where.WriteString(" AND (s.id IS NULL OR COALESCE(s.audio_codec, '') = '')")
		}
		prefer := make(pq.Int64Array, 0, len(f.PreferScenes))
		for _, id := range f.PreferScenes {
			prefer = append(prefer, int64(id))
		}
		order := "jh.priority DESC, (jh.scene_id = ANY(?::bigint[])) DESC"
		orderArgs := []any{prefer}
		if !f.NewSceneSince.IsZero() {
			order += ", (s.created_at IS NOT NULL AND s.created_at >= ?) DESC"
			orderArgs = append(orderArgs, f.NewSceneSince)
		}
		order += ", jh.created_at ASC"
		query := "SELECT jh.id FROM job_history jh LEFT JOIN scenes s ON s.id = jh.scene_id WHERE " +
			where.String() + " ORDER BY " + order + " LIMIT ? FOR UPDATE OF jh SKIP LOCKED"
		args = append(args, orderArgs...)
		args = append(args, f.Limit)

		var ids []uint
		if err := tx.Raw(query, args...).Scan(&ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		expires := now.Add(f.LeaseDuration)
		return tx.Raw(`
			UPDATE job_history SET
				status = 'running',
				started_at = ?,
				worker_id = ?,
				lease_generation = lease_generation + 1,
				lease_expires_at = ?,
				last_heartbeat_at = ?,
				progress = 0,
				worker_stage = NULL,
				input_mode = NULL,
				lease_params = NULL,
				committed_generation = NULL,
				commit_started_generation = NULL,
				error_message = NULL,
				completed_at = NULL
			WHERE id IN ?
			RETURNING *`, now, f.WorkerID, expires, now, ids).Scan(&jobs).Error
	})
	return jobs, err
}

func (r *JobLeaseRepositoryImpl) SetLeaseParams(jobID string, generation int64, workerID uint, params []byte) (bool, error) {
	result := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).
		UpdateColumn("lease_params", string(params))
	return result.RowsAffected > 0, result.Error
}

func (r *JobLeaseRepositoryImpl) RenewLease(l LeaseRenewal) (bool, error) {
	updates := map[string]any{
		"lease_expires_at":  l.ExpiresAt,
		"last_heartbeat_at": l.Now,
	}
	if l.Progress > 0 {
		updates["progress"] = min(l.Progress, 100)
	}
	if l.Stage != "" {
		updates["worker_stage"] = l.Stage
	}
	if l.InputMode != "" {
		updates["input_mode"] = l.InputMode
	}
	q := r.DB.Model(&JobHistory{}).Where(leaseCAS, l.JobID, l.Generation, l.WorkerID)
	if !l.NotBefore.IsZero() {
		q = q.Where("started_at >= ?", l.NotBefore)
	}
	result := q.UpdateColumns(updates)
	return result.RowsAffected > 0, result.Error
}

func (r *JobLeaseRepositoryImpl) CheckLease(jobID string, generation int64, workerID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).Count(&count).Error
	return count > 0, err
}

// releaseColumns puts a row back to pending, generation + 1.
func releaseColumns(countAttempt bool) map[string]any {
	updates := map[string]any{
		"status":            JobStatusPending,
		"worker_id":         nil,
		"lease_generation":  gorm.Expr("lease_generation + 1"),
		"lease_expires_at":  nil,
		"last_heartbeat_at": nil,
		"progress":          0,
		"worker_stage":      nil,
		"input_mode":        nil,
		"lease_params":      nil,
	}
	if countAttempt {
		updates["retry_count"] = gorm.Expr("retry_count + 1")
	}
	return updates
}

func (r *JobLeaseRepositoryImpl) ReleaseLease(jobID string, generation int64, workerID uint, countAttempt bool) (bool, error) {
	result := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).
		UpdateColumns(releaseColumns(countAttempt))
	return result.RowsAffected > 0, result.Error
}

func (r *JobLeaseRepositoryImpl) ReleaseWorkerLeases(workerID uint, keep []string, countAttempt bool) ([]JobHistory, error) {
	var released []JobHistory
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Where("worker_id = ? AND status = 'running'", workerID)
		if len(keep) > 0 {
			q = q.Where("job_id NOT IN ?", keep)
		}
		if err := q.Clauses(lockingClause()).Find(&released).Error; err != nil {
			return err
		}
		if len(released) == 0 {
			return nil
		}
		ids := make([]uint, len(released))
		for i, j := range released {
			ids[i] = j.ID
		}
		return tx.Model(&JobHistory{}).Where("id IN ?", ids).UpdateColumns(releaseColumns(countAttempt)).Error
	})
	return released, err
}

func (r *JobLeaseRepositoryImpl) ExpireLeases(now time.Time, maxRetries map[string]int, limit int) ([]JobHistory, []JobHistory, error) {
	var requeued, failed []JobHistory
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var expired []JobHistory
		if err := tx.Raw(`
			SELECT * FROM job_history
			WHERE status = 'running' AND worker_id IS NOT NULL AND lease_expires_at < ?
			ORDER BY lease_expires_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED`, now, limit).Scan(&expired).Error; err != nil {
			return err
		}
		var requeueIDs, failIDs []uint
		for _, j := range expired {
			if j.RetryCount+1 > maxRetries[j.Phase] {
				failed = append(failed, j)
				failIDs = append(failIDs, j.ID)
			} else {
				requeued = append(requeued, j)
				requeueIDs = append(requeueIDs, j.ID)
			}
		}
		if len(requeueIDs) > 0 {
			if err := tx.Model(&JobHistory{}).Where("id IN ?", requeueIDs).
				UpdateColumns(releaseColumns(true)).Error; err != nil {
				return err
			}
		}
		if len(failIDs) > 0 {
			if err := tx.Model(&JobHistory{}).Where("id IN ?", failIDs).UpdateColumns(map[string]any{
				"status":           JobStatusFailed,
				"error_message":    "lease expired",
				"completed_at":     now,
				"lease_generation": gorm.Expr("lease_generation + 1"),
				"lease_expires_at": nil,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return requeued, failed, nil
}

func (r *JobLeaseRepositoryImpl) MarkCommitStarted(jobID string, generation int64, workerID uint, leaseUntil time.Time) (bool, bool, error) {
	result := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).
		Where("commit_started_generation IS DISTINCT FROM ?", generation).
		UpdateColumns(map[string]any{
			"commit_started_generation": generation,
			"lease_expires_at":          gorm.Expr("GREATEST(lease_expires_at, ?)", leaseUntil),
		})
	if result.Error != nil {
		return false, false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, true, nil
	}
	// Already started by an earlier complete of this generation?
	again := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).
		Where("commit_started_generation = ?", generation).
		UpdateColumn("lease_expires_at", gorm.Expr("GREATEST(lease_expires_at, ?)", leaseUntil))
	if again.Error != nil {
		return false, false, again.Error
	}
	return again.RowsAffected > 0, false, nil
}

func (r *JobLeaseRepositoryImpl) CompleteLease(jobID string, generation int64, workerID uint, now time.Time) (bool, error) {
	result := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).UpdateColumns(map[string]any{
		"status":               JobStatusCompleted,
		"completed_at":         now,
		"committed_generation": generation,
		"progress":             100,
		"lease_expires_at":     nil,
		"worker_stage":         nil,
		"error_message":        nil,
	})
	return result.RowsAffected > 0, result.Error
}

func (r *JobLeaseRepositoryImpl) FailLease(jobID string, generation int64, workerID uint, now time.Time, message string, retryable bool) (bool, error) {
	updates := map[string]any{
		"status":           JobStatusFailed,
		"completed_at":     now,
		"error_message":    message,
		"lease_expires_at": nil,
		"worker_stage":     nil,
	}
	if !retryable {
		updates["is_retryable"] = false
	}
	result := r.DB.Model(&JobHistory{}).Where(leaseCAS, jobID, generation, workerID).UpdateColumns(updates)
	return result.RowsAffected > 0, result.Error
}

func cancelColumns(now time.Time, reason string) map[string]any {
	return map[string]any{
		"status":           JobStatusCancelled,
		"completed_at":     now,
		"error_message":    reason,
		"is_retryable":     false,
		"lease_generation": gorm.Expr("lease_generation + 1"),
		"lease_expires_at": nil,
		"worker_stage":     nil,
	}
}

func (r *JobLeaseRepositoryImpl) CancelLease(jobID string, now time.Time, reason string) (*JobHistory, error) {
	var before *JobHistory
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var rows []JobHistory
		if err := tx.Where("job_id = ? AND status = 'running' AND worker_id IS NOT NULL", jobID).
			Clauses(lockingClause()).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		before = &rows[0]
		return tx.Model(&JobHistory{}).Where("id = ?", before.ID).UpdateColumns(cancelColumns(now, reason)).Error
	})
	return before, err
}

func (r *JobLeaseRepositoryImpl) CancelRemoteLeasesByPhases(phases []string, now time.Time, reason string) ([]JobHistory, error) {
	if len(phases) == 0 {
		return nil, nil
	}
	var rows []JobHistory
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("phase IN ? AND status = 'running' AND worker_id IS NOT NULL", phases).
			Clauses(lockingClause()).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uint, len(rows))
		for i, j := range rows {
			ids[i] = j.ID
		}
		return tx.Model(&JobHistory{}).Where("id IN ?", ids).UpdateColumns(cancelColumns(now, reason)).Error
	})
	return rows, err
}

func (r *JobLeaseRepositoryImpl) ExtendRemoteLeases(until time.Time) (int64, error) {
	result := r.DB.Model(&JobHistory{}).
		Where("status = 'running' AND worker_id IS NOT NULL AND (lease_expires_at IS NULL OR lease_expires_at < ?)", until).
		UpdateColumn("lease_expires_at", until)
	return result.RowsAffected, result.Error
}

func (r *JobLeaseRepositoryImpl) ListRemoteRunning(workerID uint) ([]JobHistory, error) {
	var rows []JobHistory
	q := r.DB.Where("status = 'running' AND worker_id IS NOT NULL")
	if workerID != 0 {
		q = q.Where("worker_id = ?", workerID)
	}
	if err := q.Order("started_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *JobLeaseRepositoryImpl) GetJobs(jobIDs []string) (map[string]JobHistory, error) {
	out := make(map[string]JobHistory, len(jobIDs))
	if len(jobIDs) == 0 {
		return out, nil
	}
	var rows []JobHistory
	if err := r.DB.Where("job_id IN ?", jobIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, j := range rows {
		out[j.JobID] = j
	}
	return out, nil
}
