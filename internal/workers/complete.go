package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/workerproto"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Complete verifies and commits the result of a job (section 7.6). It is idempotent per
// (job, generation): a retried complete of a committed generation answers already_committed.
func (s *Service) Complete(w *data.Worker, jobID string, req workerproto.CompleteRequest) (*workerproto.CompleteResponse, error) {
	unlock := s.jobLocks.Lock(jobID)
	defer unlock()

	// 1. Lease check (nothing changes yet).
	j, err := s.getJob(jobID)
	if err != nil {
		return nil, err
	}
	if j == nil {
		return nil, errJobNotFound
	}
	switch leaseStatus(j, req.Generation, w.ID) {
	case workerproto.LeaseCommitted:
		return &workerproto.CompleteResponse{Status: workerproto.CompleteAlreadyCommitted}, nil
	case workerproto.LeaseCancelled:
		return nil, errJobCancelled
	case workerproto.LeaseLost:
		return nil, errLeaseLost
	}
	phase := workerproto.Phase(j.Phase)
	scene, err := s.Scenes.GetByID(j.SceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, cerr := s.Leases.CancelLease(jobID, s.now(), "Scene deleted"); cerr != nil {
				s.logger.Error("Failed to cancel the job of a deleted scene", zap.String("job_id", jobID), zap.Error(cerr))
			}
			if s.Staging != nil {
				_ = s.Staging.RemoveJob(jobID)
			}
			return nil, errSceneGone
		}
		return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "failed to load scene: %v", err)
	}

	// 2. Validate the result against the assignment.
	if req.Phase != phase {
		return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "the job phase is %s, not %s", phase, req.Phase)
	}
	if err := req.Result.Validate(phase); err != nil {
		return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "%s", err.Error())
	}
	if err := workerproto.ValidateArtifacts(req.Artifacts, req.Result); err != nil {
		return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "%s", err.Error())
	}
	var params workerproto.PhaseParams
	if len(j.LeaseParams) > 0 {
		if err := json.Unmarshal(j.LeaseParams, &params); err != nil {
			return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "the parameters of the lease are unreadable")
		}
	}
	switch phase {
	case workerproto.PhaseSprites:
		if err := req.Result.Sprites.Validate(scene.ID); err != nil {
			return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "%s", err.Error())
		}
	case workerproto.PhaseAnimatedThumbnails:
		if params.AnimatedThumbnails == nil {
			return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "no animated thumbnails parameters stored with the lease")
		}
		if err := req.Result.AnimatedThumbnails.Validate(params.AnimatedThumbnails); err != nil {
			return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "%s", err.Error())
		}
	case workerproto.PhaseFingerprint:
		if err := req.Result.Fingerprint.Validate(); err != nil {
			return nil, newError(http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, "%s", err.Error())
		}
	}

	// 3. Every artifact complete with its declared size and sha256.
	if len(req.Artifacts) > 0 {
		problems, err := s.Staging.Verify(jobID, req.Generation, req.Artifacts)
		if err != nil {
			return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "failed to verify artifacts: %v", err)
		}
		if len(problems) > 0 {
			return nil, &Error{Status: http.StatusUnprocessableEntity, Code: workerproto.CodeArtifactMissing,
				Message: fmt.Sprintf("%d artifacts are missing or incomplete", len(problems)), Details: problems}
		}
	}

	// 4. Commit: mark it started (and keep the lease alive), files then database.
	ok, firstAttempt, err := s.Leases.MarkCommitStarted(jobID, req.Generation, w.ID, s.now().Add(s.cfg.LeaseDuration))
	if err != nil {
		return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "failed to start the commit: %v", err)
	}
	if !ok {
		return nil, s.lostError(jobID, req.Generation, w.ID)
	}
	dir := s.Staging.Dir(jobID, req.Generation)
	var resultData any
	switch phase {
	case workerproto.PhaseSprites:
		res, err := jobs.CommitSprites(s.Scenes, s.Processing.GetConfig().SpriteDir, s.Processing.GetConfig().VttDir, scene.ID, req.Result.Sprites, dir)
		if err != nil {
			return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "sprites commit failed: %v", err)
		}
		resultData = res
	case workerproto.PhaseAnimatedThumbnails:
		committed, preview, err := s.Animated.CommitAnimatedThumbnails(scene.ID, req.Result.AnimatedThumbnails, dir)
		if err != nil {
			// Best effort per item, as the local job: logged, not a job failure.
			s.logger.Warn("Some animated thumbnails could not be committed",
				zap.String("job_id", jobID), zap.Int("committed", committed), zap.Bool("preview", preview), zap.Error(err))
		}
	case workerproto.PhaseFingerprint:
		if !firstAttempt && s.FingerprintIndex != nil {
			// A previous complete of this generation may have indexed already: start clean.
			if err := s.FingerprintIndex.RemoveSceneFromIndex(scene.ID); err != nil {
				return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "failed to reset the fingerprint index: %v", err)
			}
		}
		if err := s.Fingerprints.Commit(scene.ID, req.Result.Fingerprint); err != nil {
			return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "fingerprint commit failed: %v", err)
		}
		resultData = req.Result.Fingerprint
	}

	done, err := s.Leases.CompleteLease(jobID, req.Generation, w.ID, s.now())
	if err != nil {
		return nil, newError(http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "failed to mark the job completed: %v", err)
	}
	if !done {
		// Cancelled during the commit: the outputs are in place (harmless), the job is not.
		return nil, s.lostError(jobID, req.Generation, w.ID)
	}
	s.count(w.ID, data.WorkerCounterCompleted, 1)
	s.logger.Info("Remote job committed",
		zap.String("job_id", jobID), zap.String("phase", string(phase)), zap.Uint("scene_id", scene.ID),
		zap.Uint("worker_id", w.ID), zap.String("input_mode", string(req.Stats.InputMode)),
		zap.String("accel", req.Stats.Accel), zap.Int64("compute_ms", req.Stats.ComputeMillis))

	// 5. Post-completion exactly as a local job.
	s.Results.HandleResult(jobs.JobResult{JobID: jobID, SceneID: scene.ID, Phase: j.Phase, Status: jobs.JobStatusCompleted, Data: resultData})

	// 6. The staging of the job is no longer needed.
	if err := s.Staging.RemoveJob(jobID); err != nil {
		s.logger.Warn("Failed to delete the staging of a committed job", zap.String("job_id", jobID), zap.Error(err))
	}
	return &workerproto.CompleteResponse{Status: workerproto.CompleteCommitted}, nil
}
