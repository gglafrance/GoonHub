package jobs

import (
	"context"
	"errors"
	"fmt"
	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FingerprintResult holds the extracted fingerprint data (the computation result shared with
// remote workers). In dual mode, both AudioFingerprint and VisualFingerprint may be populated.
type FingerprintResult = workerproto.FingerprintResult

// FingerprintJob extracts audio or visual fingerprints from a video scene
type FingerprintJob struct {
	id              string
	sceneID         uint
	scenePath       string
	audioCodec      string // from scene metadata - determines audio vs visual path
	fingerprintMode string // "audio_only" or "dual"
	sceneDuration   int    // seconds, scales the timeout of long videos
	status          JobStatus
	error           error
	cancelled       atomic.Bool
	result          *FingerprintResult
	committer       *FingerprintCommitter
	env             compute.Env
	ctx             context.Context
	cancelFn        context.CancelFunc
	logger          *zap.Logger
}

// NewFingerprintJob creates a new FingerprintJob
func NewFingerprintJob(
	sceneID uint,
	scenePath string,
	audioCodec string,
	fingerprintMode string,
	logger *zap.Logger,
) *FingerprintJob {
	return &FingerprintJob{
		id:              uuid.New().String(),
		sceneID:         sceneID,
		scenePath:       scenePath,
		audioCodec:      audioCodec,
		fingerprintMode: fingerprintMode,
		logger:          logger,
		status:          JobStatusPending,
	}
}

// SetCommitter sets the commit step (PostgreSQL save + matching). Without one, the job only
// extracts the fingerprints.
func (j *FingerprintJob) SetCommitter(c *FingerprintCommitter) { j.committer = c }

// SetComputeEnv sets the compute environment (binaries, acceleration; default: software).
func (j *FingerprintJob) SetComputeEnv(env compute.Env) { j.env = env }

// SetSceneDuration sets the video duration (seconds), used to scale the job timeout.
func (j *FingerprintJob) SetSceneDuration(seconds int) {
	j.sceneDuration = seconds
}

// fingerprintTimeoutPerVideoSecond is the time allowed per second of video beyond the pool
// timeout: long videos (hours) must not time out after a fixed 10 minutes and redo all the work.
const fingerprintTimeoutPerVideoSecond = 500 * time.Millisecond

// JobTimeout implements TimeoutScaler: the pool timeout, raised for long videos.
func (j *FingerprintJob) JobTimeout(base time.Duration) time.Duration {
	scaled := time.Duration(j.sceneDuration) * fingerprintTimeoutPerVideoSecond
	if scaled > base {
		return scaled
	}
	return base
}

// NewFingerprintJobWithID creates a FingerprintJob with a pre-assigned job ID.
// Used by JobQueueFeeder when creating jobs from pending DB records.
func NewFingerprintJobWithID(
	jobID string,
	sceneID uint,
	scenePath string,
	audioCodec string,
	fingerprintMode string,
	logger *zap.Logger,
) *FingerprintJob {
	return &FingerprintJob{
		id:              jobID,
		sceneID:         sceneID,
		scenePath:       scenePath,
		audioCodec:      audioCodec,
		fingerprintMode: fingerprintMode,
		logger:          logger,
		status:          JobStatusPending,
	}
}

func (j *FingerprintJob) GetID() string                 { return j.id }
func (j *FingerprintJob) GetSceneID() uint              { return j.sceneID }
func (j *FingerprintJob) GetPhase() string              { return "fingerprint" }
func (j *FingerprintJob) GetStatus() JobStatus          { return j.status }
func (j *FingerprintJob) GetError() error               { return j.error }
func (j *FingerprintJob) GetResult() *FingerprintResult { return j.result }

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

	j.logger.Info("Starting fingerprint extraction job",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.String("audio_codec", j.audioCodec),
		zap.String("fingerprint_mode", j.fingerprintMode),
	)

	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}

	res, err := compute.Fingerprint(j.ctx, j.env, compute.Request{
		Input: j.scenePath,
		Video: workerproto.VideoInfo{DurationSec: j.sceneDuration, AudioCodec: j.audioCodec},
	}, workerproto.FingerprintParams{Mode: j.fingerprintMode})
	if err != nil {
		step := "visual"
		if compute.FailedStep(err) == compute.StepAudioFingerprint {
			step = "audio"
		}
		if j.ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded) {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("%s fingerprint extraction timed out", step)
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.error = err
		j.status = JobStatusFailed
		return err
	}
	j.result = res

	if res.AudioError != "" {
		j.logger.Warn("Audio extraction failed in dual mode, falling back to visual fingerprint",
			zap.Uint("scene_id", j.sceneID),
			zap.String("error", res.AudioError),
		)
	}
	if res.VisualError != "" {
		j.logger.Warn("Visual extraction failed in dual mode, keeping audio fingerprint",
			zap.Uint("scene_id", j.sceneID),
			zap.String("error", res.VisualError),
		)
	}
	if len(res.AudioFingerprint) > 0 {
		j.logger.Info("Audio fingerprint extracted",
			zap.Uint("scene_id", j.sceneID),
			zap.Int("hash_count", len(res.AudioFingerprint)),
			zap.Float64("duration", res.AudioDurationSec),
		)
	}
	if len(res.VisualFingerprint) > 0 {
		j.logger.Info("Visual fingerprint extracted",
			zap.Uint("scene_id", j.sceneID),
			zap.Int("frame_count", len(res.VisualFingerprint)),
		)
	}

	// Commit: a failed save is logged and does not fail the job (as before)
	if j.committer != nil {
		if err := j.committer.Commit(j.sceneID, res); err != nil {
			j.logger.Error("Failed to save fingerprint to database",
				zap.Uint("scene_id", j.sceneID),
				zap.Error(err),
			)
		}
	}

	j.status = JobStatusCompleted
	j.logger.Info("Fingerprint extraction completed",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.String("type", j.result.FingerprintTypeLabel()),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}
