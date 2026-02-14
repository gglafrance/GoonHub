package jobs

import (
	"context"
	"fmt"
	"goonhub/pkg/chromaprint"
	"goonhub/pkg/dhash"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FingerprintResult holds the extracted fingerprint data.
// In dual mode, both AudioFingerprint and VisualFingerprint may be populated.
type FingerprintResult struct {
	AudioFingerprint  []int32  // non-nil when audio fingerprint was extracted
	VisualFingerprint []uint64 // non-nil when visual fingerprint was extracted
}

// FingerprintTypeLabel returns a label based on which fingerprints are populated.
func (r *FingerprintResult) FingerprintTypeLabel() string {
	hasAudio := len(r.AudioFingerprint) > 0
	hasVisual := len(r.VisualFingerprint) > 0
	switch {
	case hasAudio && hasVisual:
		return "dual"
	case hasAudio:
		return "audio"
	case hasVisual:
		return "visual"
	default:
		return "unknown"
	}
}

// FingerprintJob extracts audio or visual fingerprints from a video scene
type FingerprintJob struct {
	id              string
	sceneID         uint
	scenePath       string
	audioCodec      string // from scene metadata - determines audio vs visual path
	fingerprintMode string // "audio_only" or "dual"
	status          JobStatus
	error           error
	cancelled       atomic.Bool
	result          *FingerprintResult
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

func (j *FingerprintJob) GetID() string                { return j.id }
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

	// Determine which extractions to run:
	// - Audio: always when audio exists (both modes)
	// - Visual: when no audio, or when dual mode is enabled
	extractAudio := j.audioCodec != ""
	extractVisual := j.audioCodec == "" || j.fingerprintMode == "dual"

	j.result = &FingerprintResult{}

	if extractAudio {
		if err := j.extractAudioFingerprint(); err != nil {
			if j.ctx.Err() == context.DeadlineExceeded {
				j.status = JobStatusTimedOut
				j.error = fmt.Errorf("audio fingerprint extraction timed out")
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
	}

	if extractVisual {
		if err := j.extractVisualFingerprint(); err != nil {
			if j.ctx.Err() == context.DeadlineExceeded {
				j.status = JobStatusTimedOut
				j.error = fmt.Errorf("visual fingerprint extraction timed out")
				return j.error
			}
			if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
				j.status = JobStatusCancelled
				return fmt.Errorf("job cancelled")
			}
			// In dual mode, if audio succeeded but visual failed, treat as partial success
			if extractAudio && len(j.result.AudioFingerprint) > 0 {
				j.logger.Warn("Visual extraction failed in dual mode, keeping audio fingerprint",
					zap.Uint("scene_id", j.sceneID),
					zap.Error(err),
				)
			} else {
				j.error = err
				j.status = JobStatusFailed
				return err
			}
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

func (j *FingerprintJob) extractAudioFingerprint() error {
	fpResult, err := chromaprint.ExtractFingerprintWithContext(j.ctx, j.scenePath)
	if err != nil {
		return fmt.Errorf("chromaprint extraction failed: %w", err)
	}

	j.result.AudioFingerprint = fpResult.Fingerprint

	j.logger.Info("Audio fingerprint extracted",
		zap.Uint("scene_id", j.sceneID),
		zap.Int("hash_count", len(fpResult.Fingerprint)),
		zap.Float64("duration", fpResult.Duration),
	)

	return nil
}

func (j *FingerprintJob) extractVisualFingerprint() error {
	hashes, err := dhash.ExtractDHashesWithContext(j.ctx, j.scenePath)
	if err != nil {
		return fmt.Errorf("dhash extraction failed: %w", err)
	}

	if len(hashes) == 0 {
		return fmt.Errorf("no visual frames extracted")
	}

	j.result.VisualFingerprint = hashes

	j.logger.Info("Visual fingerprint extracted",
		zap.Uint("scene_id", j.sceneID),
		zap.Int("frame_count", len(hashes)),
	)

	return nil
}
