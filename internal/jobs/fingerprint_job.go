package jobs

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"goonhub/pkg/fingerprint"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FingerprintJob extracts perceptual hashes from video frames for duplicate detection.
type FingerprintJob struct {
	id              string
	sceneID         uint
	scenePath       string
	duration        int
	intervalSec     int
	repo            data.SceneRepository
	fingerprintRepo data.FingerprintRepository
	logger          *zap.Logger
	status          JobStatus
	error           error
	cancelled       atomic.Bool
	ctx             context.Context
	cancelFn        context.CancelFunc
	progressCb      func(jobID string, progress int)
}

func NewFingerprintJob(
	sceneID uint,
	scenePath string,
	duration int,
	intervalSec int,
	repo data.SceneRepository,
	fingerprintRepo data.FingerprintRepository,
	logger *zap.Logger,
) *FingerprintJob {
	return &FingerprintJob{
		id:              uuid.New().String(),
		sceneID:         sceneID,
		scenePath:       scenePath,
		duration:        duration,
		intervalSec:     intervalSec,
		repo:            repo,
		fingerprintRepo: fingerprintRepo,
		logger:          logger,
		status:          JobStatusPending,
	}
}

func NewFingerprintJobWithID(
	jobID string,
	sceneID uint,
	scenePath string,
	duration int,
	intervalSec int,
	repo data.SceneRepository,
	fingerprintRepo data.FingerprintRepository,
	logger *zap.Logger,
) *FingerprintJob {
	return &FingerprintJob{
		id:              jobID,
		sceneID:         sceneID,
		scenePath:       scenePath,
		duration:        duration,
		intervalSec:     intervalSec,
		repo:            repo,
		fingerprintRepo: fingerprintRepo,
		logger:          logger,
		status:          JobStatusPending,
	}
}

func (j *FingerprintJob) GetID() string       { return j.id }
func (j *FingerprintJob) GetSceneID() uint     { return j.sceneID }
func (j *FingerprintJob) GetPhase() string     { return "fingerprint" }
func (j *FingerprintJob) GetStatus() JobStatus { return j.status }
func (j *FingerprintJob) GetError() error      { return j.error }

func (j *FingerprintJob) SetProgressCallback(cb func(jobID string, progress int)) {
	j.progressCb = cb
}

func (j *FingerprintJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *FingerprintJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *FingerprintJob) ExecuteWithContext(ctx context.Context) error {
	j.ctx, j.cancelFn = context.WithCancel(ctx)
	defer j.cancelFn()

	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting fingerprint extraction",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.Int("duration", j.duration),
		zap.Int("interval_sec", j.intervalSec),
	)

	// Update scene fingerprint status
	if err := j.repo.UpdateFingerprintStatus(j.sceneID, "processing", 0); err != nil {
		j.logger.Error("Failed to update fingerprint status",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
	}

	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		if err := j.repo.UpdateFingerprintStatus(j.sceneID, "cancelled", 0); err != nil {
			j.logger.Error("Failed to update fingerprint status",
				zap.Uint("scene_id", j.sceneID),
				zap.Error(err),
			)
		}
		return fmt.Errorf("job cancelled")
	}

	// Extract all hashes using streaming ffmpeg process
	hashes, err := fingerprint.ExtractAllHashes(j.ctx, j.scenePath, j.duration, j.intervalSec, func(progress int) {
		if j.progressCb != nil {
			j.progressCb(j.id, progress)
		}
	})
	if err != nil {
		if j.ctx.Err() != nil {
			j.status = JobStatusCancelled
			if statusErr := j.repo.UpdateFingerprintStatus(j.sceneID, "cancelled", 0); statusErr != nil {
				j.logger.Error("Failed to update fingerprint status",
					zap.Uint("scene_id", j.sceneID),
					zap.Error(statusErr),
				)
			}
			return fmt.Errorf("job cancelled")
		}
		j.handleError(fmt.Errorf("hash extraction failed: %w", err))
		return err
	}

	// Delete existing fingerprints for this scene (supports re-fingerprinting)
	if err := j.fingerprintRepo.DeleteBySceneID(j.sceneID); err != nil {
		j.handleError(fmt.Errorf("failed to clear old fingerprints: %w", err))
		return err
	}

	// Bulk insert new fingerprints
	fingerprints := make([]data.SceneFingerprint, len(hashes))
	for i, h := range hashes {
		fingerprints[i] = data.SceneFingerprint{
			SceneID:    j.sceneID,
			FrameIndex: i,
			HashValue:  int64(h),
		}
	}

	if err := j.fingerprintRepo.BulkInsert(fingerprints); err != nil {
		j.handleError(fmt.Errorf("failed to save fingerprints: %w", err))
		return err
	}

	// Update scene status
	if err := j.repo.UpdateFingerprintStatus(j.sceneID, "completed", len(hashes)); err != nil {
		j.logger.Error("Failed to update fingerprint status to completed",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
	}

	j.status = JobStatusCompleted
	j.logger.Info("Fingerprint extraction completed",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.Int("hash_count", len(hashes)),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *FingerprintJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	if statusErr := j.repo.UpdateFingerprintStatus(j.sceneID, "failed", 0); statusErr != nil {
		j.logger.Error("Failed to update fingerprint status",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(statusErr),
		)
	}
}
