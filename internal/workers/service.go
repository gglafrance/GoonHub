package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SceneStore is the scene repository part the worker service needs.
type SceneStore interface {
	GetByID(id uint) (*data.Scene, error)
	UpdateSprites(id uint, spriteSheetPath, vttPath string, spriteSheetCount int) error
}

// StoragePathLister lists the storage paths (relative paths of the assignments).
type StoragePathLister interface {
	List() ([]data.StoragePath, error)
}

// ProcessingSettings gives the processing parameters of the server (the pool manager).
type ProcessingSettings interface {
	GetQualityConfig() processing.QualityConfig
	GetConfig() config.ProcessingConfig
}

// FingerprintCommitter commits a fingerprint result (jobs.FingerprintCommitter).
type FingerprintCommitter interface {
	Commit(sceneID uint, res *workerproto.FingerprintResult) error
}

// FingerprintIndexCleaner removes the indexed fingerprints of a scene (ClickHouse), so that a
// fingerprint commit interrupted after its indexing can run again without duplicates.
type FingerprintIndexCleaner interface {
	RemoveSceneFromIndex(sceneID uint) error
}

// ResultSink runs the post-completion of a job exactly as for a local job
// (processing.ResultHandler.HandleResult).
type ResultSink interface {
	HandleResult(result jobs.JobResult)
}

// RetryPolicy is the retry configuration and the dead letter queue (core.RetryScheduler).
type RetryPolicy interface {
	MaxRetries(phase string) int
	// ToDeadLetter sends a failed job to the dead letter queue.
	ToDeadLetter(jobID, phase string, sceneID uint, retryCount int, message string) error
}

// ClaimGate applies the job schedule to remote claims (core.JobQueueFeeder).
type ClaimGate interface {
	// RemoteClaimOptions reports whether the phase may run now (time windows) and since when
	// scenes count as new ("new scenes first", zero = off).
	RemoteClaimOptions(phase string) (allowed bool, newSceneSince time.Time)
}

// PushCanceller tells a push worker to stop a job (best effort).
type PushCanceller interface {
	Cancel(w *data.Worker, jobID string, generation int64)
}

// Deps are the dependencies of the Service.
type Deps struct {
	Settings           Settings
	Workers            data.WorkerRepository
	Leases             data.JobLeaseRepository
	Scenes             SceneStore
	StoragePaths       StoragePathLister
	Router             *Router
	Staging            *Staging
	Processing         ProcessingSettings
	Animated           jobs.AnimatedThumbnailGenerator
	Fingerprints       FingerprintCommitter
	FingerprintIndex   FingerprintIndexCleaner
	FingerprintMode    func() string
	DuplicationEnabled bool
	Results            ResultSink
	Retry              RetryPolicy
	Gate               ClaimGate
	MediaSecret        []byte
	Logger             *zap.Logger
	Now                func() time.Time
}

// Service implements the worker protocol on the server.
type Service struct {
	Deps
	cfg      Settings
	logger   *zap.Logger
	now      func() time.Time
	jobLocks keyedMutex
	pushMu   sync.RWMutex
	push     PushCanceller
	stopState
}

// NewService creates the worker service.
func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = zap.NewNop()
	}
	if d.FingerprintMode == nil {
		d.FingerprintMode = func() string { return workerproto.FingerprintModeAudioOnly }
	}
	return &Service{
		Deps:   d,
		cfg:    d.Settings,
		logger: d.Logger.With(zap.String("component", "remote_workers")),
		now:    d.Now,
	}
}

// SetPushCanceller sets who tells push workers about cancelled jobs.
func (s *Service) SetPushCanceller(p PushCanceller) {
	s.pushMu.Lock()
	s.push = p
	s.pushMu.Unlock()
}

// Settings returns the effective settings.
func (s *Service) Settings() Settings { return s.cfg }

// keyedMutex serializes operations per key (complete of one job, upload of one artifact).
type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedEntry
}

type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

func (k *keyedMutex) Lock(key string) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = map[string]*keyedEntry{}
	}
	e := k.locks[key]
	if e == nil {
		e = &keyedEntry{}
		k.locks[key] = e
	}
	e.refs++
	k.mu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}

// ---------------------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------------------

// Authenticate returns the worker of a bearer token. Errors: 401 unauthorized (unknown or
// revoked token), 403 worker_disabled.
func (s *Service) Authenticate(token string) (*data.Worker, error) {
	id, ok := ParseToken(token)
	if !ok {
		return nil, newError(http.StatusUnauthorized, workerproto.CodeUnauthorized, "invalid worker token")
	}
	w, err := s.Workers.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newError(http.StatusUnauthorized, workerproto.CodeUnauthorized, "invalid worker token")
		}
		return nil, fmt.Errorf("failed to load worker: %w", err)
	}
	if !TokenMatches(token, w.TokenHash) {
		return nil, newError(http.StatusUnauthorized, workerproto.CodeUnauthorized, "invalid worker token")
	}
	if !w.Enabled {
		return nil, newError(http.StatusForbidden, workerproto.CodeWorkerDisabled, "worker disabled")
	}
	return w, nil
}

// ---------------------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------------------

func workerCaps(w *data.Worker) workerproto.Capabilities {
	var caps workerproto.Capabilities
	if len(w.Capabilities) > 0 {
		_ = json.Unmarshal(w.Capabilities, &caps)
	}
	return caps
}

func workerMappings(w *data.Worker) []workerproto.PathMapping {
	var m []workerproto.PathMapping
	if len(w.PathMappings) > 0 {
		_ = json.Unmarshal(w.PathMappings, &m)
	}
	return m
}

func hasInputMode(caps workerproto.Capabilities, m workerproto.InputMode) bool {
	if len(caps.InputModes) == 0 {
		return m == workerproto.InputIndirect || m == workerproto.InputDirect
	}
	for _, x := range caps.InputModes {
		if x == m {
			return true
		}
	}
	return false
}

// leaseStatus classifies a job a call of (generation, worker) could not act on.
func leaseStatus(j *data.JobHistory, generation int64, workerID uint) workerproto.LeaseStatus {
	if j == nil {
		return workerproto.LeaseLost
	}
	sameWorker := j.WorkerID != nil && *j.WorkerID == workerID
	switch {
	case j.Status == data.JobStatusRunning && sameWorker && j.LeaseGeneration == generation:
		return workerproto.LeaseOK
	case j.Status == data.JobStatusCompleted && j.CommittedGeneration != nil && *j.CommittedGeneration == generation && sameWorker:
		return workerproto.LeaseCommitted
	case j.Status == data.JobStatusCancelled && sameWorker:
		return workerproto.LeaseCancelled
	}
	return workerproto.LeaseLost
}

func (s *Service) getJob(jobID string) (*data.JobHistory, error) {
	m, err := s.Leases.GetJobs([]string{jobID})
	if err != nil {
		return nil, fmt.Errorf("failed to load job: %w", err)
	}
	j, ok := m[jobID]
	if !ok {
		return nil, nil
	}
	return &j, nil
}

// LeaseError is the protocol error of a guarded call that matched no row: job_not_found,
// job_cancelled or lease_lost.
func (s *Service) LeaseError(jobID string, generation int64, workerID uint) error {
	return s.lostError(jobID, generation, workerID)
}

// lostError is the error of a guarded call that matched no row.
func (s *Service) lostError(jobID string, generation int64, workerID uint) error {
	j, err := s.getJob(jobID)
	if err != nil {
		return err
	}
	if j == nil {
		return errJobNotFound
	}
	if leaseStatus(j, generation, workerID) == workerproto.LeaseCancelled {
		return errJobCancelled
	}
	return errLeaseLost
}

func (s *Service) checkSession(w *data.Worker, sessionID string) error {
	if sessionID == "" || w.SessionID == "" || sessionID != w.SessionID {
		return errUnknownSession
	}
	return nil
}

func (s *Service) touch(w *data.Worker, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["last_seen_at"] = s.now()
	if err := s.Workers.Update(w.ID, fields); err != nil {
		s.logger.Warn("Failed to update worker", zap.Uint("worker_id", w.ID), zap.Error(err))
	}
}

func (s *Service) count(workerID uint, counter string, n int) {
	if n == 0 {
		return
	}
	if err := s.Workers.IncrementCounter(workerID, counter, n); err != nil {
		s.logger.Warn("Failed to update worker counter", zap.Uint("worker_id", workerID), zap.String("counter", counter), zap.Error(err))
	}
}

// ---------------------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------------------

// Register starts a new session of a worker (section 7.1). serverURL is the base URL the
// worker used to reach the server (signed media URLs point there unless workers.public_url
// is set).
func (s *Service) Register(w *data.Worker, req workerproto.RegisterRequest, serverURL string) (*workerproto.RegisterResponse, error) {
	if req.APIVersion != workerproto.APIVersion {
		return nil, newError(http.StatusUpgradeRequired, workerproto.CodeUnsupportedAPIVersion,
			"unsupported worker API version %d (server speaks %d)", req.APIVersion, workerproto.APIVersion)
	}
	mode := req.ConnectionMode
	if mode == "" {
		mode = workerproto.ConnPull
	}
	if !mode.Valid() {
		return nil, errInvalid("unknown connection_mode %q", mode)
	}
	if mode == workerproto.ConnPush && (req.PushURL == "" || req.PushToken == "") {
		return nil, errInvalid("push mode needs push_url and push_token")
	}
	for _, p := range req.Capabilities.Phases {
		if !p.Valid() {
			return nil, errInvalid("phase %q cannot be processed remotely", p)
		}
	}
	for _, m := range req.Capabilities.InputModes {
		if !m.Valid() {
			return nil, errInvalid("unknown input mode %q", m)
		}
	}
	for _, m := range req.PathMappings {
		if m.ServerPrefix == "" || !strings.HasPrefix(m.ServerPrefix, "/") {
			return nil, errInvalid("path mapping server_prefix must be an absolute server path")
		}
	}
	caps, err := json.Marshal(req.Capabilities)
	if err != nil {
		return nil, errInvalid("invalid capabilities")
	}
	mappings, err := json.Marshal(req.PathMappings)
	if err != nil {
		return nil, errInvalid("invalid path mappings")
	}
	session, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	now := s.now()

	// Resume: renew the leases the worker still holds and that it reloaded from its outbox.
	resumed := make([]workerproto.JobLeaseStatus, 0, len(req.Resume))
	keep := map[string]bool{}
	if len(req.Resume) > 0 {
		ids := make([]string, len(req.Resume))
		for i, r := range req.Resume {
			ids[i] = r.JobID
		}
		jobsByID, err := s.Leases.GetJobs(ids)
		if err != nil {
			return nil, fmt.Errorf("failed to load resumed jobs: %w", err)
		}
		for _, ref := range req.Resume {
			st := workerproto.JobLeaseStatus{JobID: ref.JobID, Generation: ref.Generation, Status: workerproto.LeaseLost}
			if j, ok := jobsByID[ref.JobID]; ok {
				st.Status = leaseStatus(&j, ref.Generation, w.ID)
				if st.Status == workerproto.LeaseOK {
					expires := now.Add(s.cfg.LeaseDuration)
					renewed, err := s.Leases.RenewLease(data.LeaseRenewal{
						JobID: ref.JobID, Generation: ref.Generation, WorkerID: w.ID,
						Now: now, ExpiresAt: expires, NotBefore: now.Add(-s.cfg.MaxAssignmentAge),
					})
					if err != nil {
						return nil, fmt.Errorf("failed to renew resumed lease: %w", err)
					}
					if renewed {
						st.LeaseExpiresAt = &expires
						keep[ref.JobID] = true
					} else {
						st.Status = workerproto.LeaseLost
					}
				}
			}
			resumed = append(resumed, st)
		}
	}

	if err := s.Workers.Update(w.ID, map[string]any{
		"session_id":      session,
		"connection_mode": string(mode),
		"push_url":        strings.TrimRight(req.PushURL, "/"),
		"push_token":      req.PushToken,
		"server_url":      strings.TrimRight(serverURL, "/"),
		"capabilities":    string(caps),
		"path_mappings":   string(mappings),
		"version":         truncate(req.Version, 50),
		"hostname":        truncate(req.Hostname, 255),
		"os":              truncate(req.OS, 20),
		"arch":            truncate(req.Arch, 20),
		"worker_draining": false,
		"last_seen_at":    now,
		"registered_at":   now,
	}); err != nil {
		return nil, fmt.Errorf("failed to store worker session: %w", err)
	}

	// Every other lease of this worker belongs to a crashed session: release it (attempt + 1).
	held, err := s.Leases.ListRemoteRunning(w.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list worker leases: %w", err)
	}
	for _, j := range held {
		if keep[j.JobID] {
			continue
		}
		if err := s.releaseCounted(w, &j, "worker restarted while holding the job"); err != nil {
			s.logger.Error("Failed to release a lease of a restarted worker", zap.String("job_id", j.JobID), zap.Error(err))
		}
	}

	s.logger.Info("Worker registered",
		zap.Uint("worker_id", w.ID), zap.String("name", w.Name), zap.String("mode", string(mode)),
		zap.String("version", req.Version), zap.Int("resumed", len(keep)), zap.Int("released", len(held)-len(keep)))

	return &workerproto.RegisterResponse{
		WorkerID:             fmt.Sprint(w.ID),
		SessionID:            session,
		APIVersion:           workerproto.APIVersion,
		ServerTime:           now.UTC(),
		HeartbeatIntervalSec: int(s.cfg.HeartbeatInterval / time.Second),
		LeaseDurationSec:     int(s.cfg.LeaseDuration / time.Second),
		ClaimMaxWaitSec:      int(s.cfg.ClaimMaxWait / time.Second),
		MaxArtifactBytes:     s.cfg.MaxArtifactBytes,
		UploadChunkBytes:     s.cfg.UploadChunkBytes,
		MaxConcurrency:       w.MaxConcurrency,
		Resumed:              resumed,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// releaseCounted gives a job back with an attempt counted; when the attempts are exhausted
// the job fails through the normal retry path instead (dead letter queue).
func (s *Service) releaseCounted(w *data.Worker, j *data.JobHistory, why string) error {
	if s.Retry != nil && j.RetryCount+1 > s.Retry.MaxRetries(j.Phase) {
		msg := fmt.Sprintf("worker %s: %s", w.Name, why)
		ok, err := s.Leases.FailLease(j.JobID, j.LeaseGeneration, w.ID, s.now(), msg, true)
		if err != nil || !ok {
			return err
		}
		s.count(w.ID, data.WorkerCounterFailed, 1)
		s.Results.HandleResult(jobs.JobResult{JobID: j.JobID, SceneID: j.SceneID, Phase: j.Phase, Status: jobs.JobStatusFailed, Error: errors.New(msg)})
		return nil
	}
	ok, err := s.Leases.ReleaseLease(j.JobID, j.LeaseGeneration, w.ID, true)
	if err != nil {
		return err
	}
	if ok {
		s.count(w.ID, data.WorkerCounterReleased, 1)
	}
	return nil
}

// ---------------------------------------------------------------------------------------
// Claim
// ---------------------------------------------------------------------------------------

// Claim leases jobs to a pull worker (section 7.2), long-polling up to wait_sec.
func (s *Service) Claim(ctx context.Context, w *data.Worker, req workerproto.ClaimRequest) (*workerproto.ClaimResponse, error) {
	if err := s.checkSession(w, req.SessionID); err != nil {
		return nil, err
	}
	s.touch(w, nil)
	wait := min(time.Duration(max(req.WaitSec, 0))*time.Second, s.cfg.ClaimMaxWait)
	deadline := s.now().Add(wait)
	for {
		assignments, err := s.ClaimFor(w, req.Slots)
		if err != nil {
			return nil, err
		}
		if len(assignments) > 0 || !s.now().Before(deadline) {
			return &workerproto.ClaimResponse{Assignments: assignments}, nil
		}
		timer := time.NewTimer(min(s.cfg.ClaimPollInterval, time.Until(deadline)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return &workerproto.ClaimResponse{Assignments: []workerproto.Assignment{}}, nil
		case <-timer.C:
		}
		// The admin may have disabled or drained the worker meanwhile.
		fresh, err := s.Workers.GetByID(w.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to reload worker: %w", err)
		}
		if fresh.SessionID != req.SessionID {
			return nil, errUnknownSession
		}
		w = fresh
	}
}

// ClaimFor leases at most slots[phase] jobs per phase to a worker, within its capabilities,
// its allowed phases, its max concurrency, the phase routing and the job schedule, and
// prepares the assignments. Used by the pull claim and by the push dispatcher.
func (s *Service) ClaimFor(w *data.Worker, slots map[workerproto.Phase]int) ([]workerproto.Assignment, error) {
	out := []workerproto.Assignment{}
	if !w.Enabled || w.Draining || w.SessionID == "" {
		return out, nil
	}
	caps := workerCaps(w)
	capPhases := map[workerproto.Phase]bool{}
	for _, p := range caps.Phases {
		capPhases[p] = true
	}
	allowed := map[string]bool{}
	for _, p := range w.AllowedPhases {
		allowed[p] = true
	}
	held, err := s.Leases.ListRemoteRunning(w.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list worker leases: %w", err)
	}
	heldPerPhase := map[string]int{}
	prefer := make([]uint, 0, len(held))
	for _, j := range held {
		heldPerPhase[j.Phase]++
		prefer = append(prefer, j.SceneID)
	}
	remaining := -1
	if w.MaxConcurrency > 0 {
		remaining = w.MaxConcurrency - len(held)
		if remaining <= 0 {
			return out, nil
		}
	}

	var prefixes []string
	if !hasInputMode(caps, workerproto.InputIndirect) {
		for _, m := range workerMappings(w) {
			prefixes = append(prefixes, m.ServerPrefix)
		}
		if len(prefixes) == 0 {
			return out, nil // direct only, and no storage it can see
		}
	}

	for _, phase := range workerproto.RemotePhases {
		p := string(phase)
		n := slots[phase]
		// the per-phase slots the worker registered with come from its config; a limit
		// set by the admin replaces them (the worker applies it too)
		if cs, ok := caps.Slots[phase]; ok && w.MaxConcurrency <= 0 {
			n = min(n, cs-heldPerPhase[p])
		}
		if remaining >= 0 {
			n = min(n, remaining)
		}
		if n <= 0 || !capPhases[phase] || !allowed[p] || !s.Router.RemoteAllowed(p) {
			continue
		}
		if phase == workerproto.PhaseFingerprint && !s.DuplicationEnabled {
			continue
		}
		filter := data.LeaseClaimFilter{
			Phase:         p,
			Limit:         n,
			WorkerID:      w.ID,
			LeaseDuration: s.cfg.LeaseDuration,
			PreferScenes:  prefer,
			PathPrefixes:  prefixes,
			NoAudioOnly:   phase == workerproto.PhaseFingerprint && !caps.AudioFingerprint,
		}
		if s.Gate != nil {
			ok, since := s.Gate.RemoteClaimOptions(p)
			if !ok {
				continue
			}
			filter.NewSceneSince = since
		}
		claimed, err := s.Leases.ClaimForWorker(filter, s.now())
		if err != nil {
			return out, fmt.Errorf("failed to claim %s jobs: %w", p, err)
		}
		for i := range claimed {
			j := &claimed[i]
			a, ok := s.prepare(w, j)
			if !ok {
				continue
			}
			out = append(out, *a)
			prefer = append(prefer, j.SceneID)
			if remaining > 0 {
				remaining--
			}
		}
	}
	if len(out) > 0 {
		s.handOver(w, out)
		s.logger.Debug("Leased jobs to worker", zap.Uint("worker_id", w.ID), zap.Int("count", len(out)))
	}
	return out, nil
}

// handOver restarts the lease clock of the assignments about to be sent: the leases were
// taken before the assignments were prepared (database, animated thumbnail plans on the
// media storage), which can take longer than two heartbeat intervals on a loaded server.
// Without this, a heartbeat the worker sends meanwhile (it does not know these jobs yet)
// would make Heartbeat release them as "not reported" while they are on their way.
func (s *Service) handOver(w *data.Worker, out []workerproto.Assignment) {
	now := s.now()
	expires := now.Add(s.cfg.LeaseDuration)
	for i := range out {
		a := &out[i]
		ok, err := s.Leases.RenewLease(data.LeaseRenewal{JobID: a.JobID, Generation: a.Lease.Generation,
			WorkerID: w.ID, Now: now, ExpiresAt: expires})
		if err != nil {
			s.logger.Error("Failed to restart the lease clock at hand-over", zap.String("job_id", a.JobID), zap.Error(err))
			continue
		}
		if ok {
			a.Lease.ExpiresAt = expires.UTC()
		}
	}
}

// failClaimed fails a job right after its claim (scene missing, no metadata...), as the local
// feeder does: no retry scheduled.
func (s *Service) failClaimed(w *data.Worker, j *data.JobHistory, msg string) {
	if _, err := s.Leases.FailLease(j.JobID, j.LeaseGeneration, w.ID, s.now(), msg, false); err != nil {
		s.logger.Error("Failed to fail a claimed job", zap.String("job_id", j.JobID), zap.Error(err))
	}
}

// prepare builds the assignment of a claimed job. Jobs that cannot be assigned are failed,
// or completed when there is nothing to do; ok is false for them.
func (s *Service) prepare(w *data.Worker, j *data.JobHistory) (*workerproto.Assignment, bool) {
	scene, err := s.Scenes.GetByID(j.SceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.failClaimed(w, j, "Scene not found")
		} else {
			s.failClaimed(w, j, "Failed to fetch scene data: "+err.Error())
		}
		return nil, false
	}
	if scene.Duration == 0 {
		s.failClaimed(w, j, "scene duration is 0: metadata not yet extracted")
		return nil, false
	}
	phase := workerproto.Phase(j.Phase)
	var params workerproto.PhaseParams
	switch phase {
	case workerproto.PhaseSprites:
		quality := s.Processing.GetQualityConfig()
		cfg := s.Processing.GetConfig()
		tileW, tileH := scene.ThumbnailWidth, scene.ThumbnailHeight
		if tileW == 0 || tileH == 0 {
			tileW, tileH = ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, quality.MaxFrameDimensionSm)
		}
		params.Sprites = &workerproto.SpritesParams{
			SceneID:       scene.ID,
			TileWidth:     tileW,
			TileHeight:    tileH,
			FrameInterval: cfg.FrameInterval,
			FrameQuality:  quality.FrameQualitySprites,
			GridCols:      cfg.GridCols,
			GridRows:      cfg.GridRows,
			Concurrency:   quality.SpritesConcurrency,
		}
	case workerproto.PhaseAnimatedThumbnails:
		plan, err := s.Animated.PrepareAnimatedThumbnails(scene.ID, j.ForceTarget)
		if err != nil {
			msg := "failed to prepare animated thumbnails: " + err.Error()
			if ok, ferr := s.Leases.FailLease(j.JobID, j.LeaseGeneration, w.ID, s.now(), msg, true); ferr == nil && ok {
				s.Results.HandleResult(jobs.JobResult{JobID: j.JobID, SceneID: j.SceneID, Phase: j.Phase, Status: jobs.JobStatusFailed, Error: errors.New(msg)})
			}
			return nil, false
		}
		if plan == nil {
			// Nothing to make: the job completes without a worker, as the local job does.
			if ok, cerr := s.Leases.CompleteLease(j.JobID, j.LeaseGeneration, w.ID, s.now()); cerr == nil && ok {
				s.Results.HandleResult(jobs.JobResult{JobID: j.JobID, SceneID: j.SceneID, Phase: j.Phase, Status: jobs.JobStatusCompleted})
			}
			return nil, false
		}
		p := plan.Params
		params.AnimatedThumbnails = &p
	case workerproto.PhaseFingerprint:
		params.Fingerprint = &workerproto.FingerprintParams{Mode: s.FingerprintMode()}
	default:
		s.failClaimed(w, j, "phase cannot be processed remotely")
		return nil, false
	}
	raw, err := json.Marshal(params)
	if err != nil {
		s.failClaimed(w, j, "failed to encode parameters")
		return nil, false
	}
	if ok, err := s.Leases.SetLeaseParams(j.JobID, j.LeaseGeneration, w.ID, raw); err != nil || !ok {
		if err != nil {
			s.logger.Error("Failed to store lease parameters", zap.String("job_id", j.JobID), zap.Error(err))
			if _, rerr := s.Leases.ReleaseLease(j.JobID, j.LeaseGeneration, w.ID, false); rerr != nil {
				s.logger.Error("Failed to release a lease", zap.String("job_id", j.JobID), zap.Error(rerr))
			}
		}
		return nil, false
	}

	now := s.now()
	expires := now.Add(s.cfg.LeaseDuration)
	if j.LeaseExpiresAt != nil {
		expires = *j.LeaseExpiresAt
	}
	input, err := s.inputSpec(w, j, scene, now)
	if err != nil {
		s.failClaimed(w, j, err.Error())
		return nil, false
	}
	return &workerproto.Assignment{
		JobID:   j.JobID,
		Phase:   phase,
		SceneID: scene.ID,
		Attempt: j.RetryCount,
		Lease: workerproto.Lease{
			Generation:  j.LeaseGeneration,
			ExpiresAt:   expires.UTC(),
			DurationSec: int(s.cfg.LeaseDuration / time.Second),
		},
		Input:      input,
		Params:     params,
		AssignedAt: now.UTC(),
	}, true
}

func (s *Service) baseURL(w *data.Worker) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	return w.ServerURL
}

func (s *Service) inputSpec(w *data.Worker, j *data.JobHistory, scene *data.Scene, now time.Time) (workerproto.InputSpec, error) {
	size := scene.Size
	if st, err := os.Stat(scene.StoredPath); err == nil && st.Mode().IsRegular() {
		size = st.Size()
	}
	spec := workerproto.InputSpec{
		ServerPath:    scene.StoredPath,
		StoragePathID: scene.StoragePathID,
		Size:          size,
		FileName:      filepath.Base(scene.StoredPath),
		Video: workerproto.VideoInfo{
			DurationSec: scene.Duration,
			Width:       scene.Width,
			Height:      scene.Height,
			FrameRate:   scene.FrameRate,
			VideoCodec:  scene.VideoCodec,
			AudioCodec:  scene.AudioCodec,
		},
	}
	if scene.StoragePathID != nil && s.StoragePaths != nil {
		if paths, err := s.StoragePaths.List(); err == nil {
			for _, p := range paths {
				if p.ID != *scene.StoragePathID {
					continue
				}
				if rel, err := filepath.Rel(p.Path, scene.StoredPath); err == nil && !strings.HasPrefix(rel, "..") {
					spec.RelativePath = filepath.ToSlash(rel)
				}
			}
		}
	}
	base := s.baseURL(w)
	if base == "" {
		return spec, fmt.Errorf("no server URL known for worker %s: set workers.public_url", w.Name)
	}
	spec.URLExpiresAt = now.Add(s.cfg.MediaURLTTL).UTC()
	spec.URL = workerproto.MediaURL(base, s.MediaSecret, j.JobID, j.LeaseGeneration, spec.URLExpiresAt)
	return spec, nil
}

// ---------------------------------------------------------------------------------------
// Heartbeat
// ---------------------------------------------------------------------------------------

// Heartbeat renews every lease of the worker (section 7.3). Leases the server holds for the
// worker but that the worker did not report for 2 heartbeat intervals (a claim or push
// answer lost on the way) are released at once, without counting an attempt.
func (s *Service) Heartbeat(w *data.Worker, req workerproto.HeartbeatRequest) (*workerproto.HeartbeatResponse, error) {
	if err := s.checkSession(w, req.SessionID); err != nil {
		return nil, err
	}
	now := s.now()
	s.touch(w, map[string]any{"free_disk_bytes": max(req.FreeDiskBytes, 0), "worker_draining": req.Draining})

	resp := &workerproto.HeartbeatResponse{ServerTime: now.UTC(), Drain: w.Draining, MaxConcurrency: w.MaxConcurrency, Jobs: []workerproto.JobLeaseStatus{}}
	reported := map[string]bool{}
	var missing []string
	expires := now.Add(s.cfg.LeaseDuration)
	for _, hb := range req.Jobs {
		reported[hb.JobID] = true
		ok, err := s.Leases.RenewLease(data.LeaseRenewal{
			JobID: hb.JobID, Generation: hb.Generation, WorkerID: w.ID,
			Now: now, ExpiresAt: expires, NotBefore: now.Add(-s.cfg.MaxAssignmentAge),
			Progress: hb.Progress, Stage: string(hb.Stage), InputMode: string(hb.InputMode),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to renew lease: %w", err)
		}
		st := workerproto.JobLeaseStatus{JobID: hb.JobID, Generation: hb.Generation, Status: workerproto.LeaseOK}
		if ok {
			e := expires.UTC()
			st.LeaseExpiresAt = &e
		} else {
			st.Status = workerproto.LeaseLost
			missing = append(missing, hb.JobID)
		}
		resp.Jobs = append(resp.Jobs, st)
	}
	if len(missing) > 0 {
		jobsByID, err := s.Leases.GetJobs(missing)
		if err != nil {
			return nil, fmt.Errorf("failed to load jobs: %w", err)
		}
		for i := range resp.Jobs {
			st := &resp.Jobs[i]
			if st.Status != workerproto.LeaseLost {
				continue
			}
			if j, ok := jobsByID[st.JobID]; ok {
				if ls := leaseStatus(&j, st.Generation, w.ID); ls != workerproto.LeaseOK {
					st.Status = ls
				}
			}
		}
	}

	held, err := s.Leases.ListRemoteRunning(w.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list worker leases: %w", err)
	}
	for _, j := range held {
		if reported[j.JobID] || j.LastHeartbeatAt == nil || now.Sub(*j.LastHeartbeatAt) < 2*s.cfg.HeartbeatInterval {
			continue
		}
		if ok, err := s.Leases.ReleaseLease(j.JobID, j.LeaseGeneration, w.ID, false); err != nil {
			s.logger.Error("Failed to release an unreported lease", zap.String("job_id", j.JobID), zap.Error(err))
		} else if ok {
			s.count(w.ID, data.WorkerCounterReleased, 1)
			s.logger.Info("Released a lease the worker does not report", zap.Uint("worker_id", w.ID), zap.String("job_id", j.JobID))
		}
	}
	return resp, nil
}

// ---------------------------------------------------------------------------------------
// Release, fail, media URL, cancel
// ---------------------------------------------------------------------------------------

// Release gives one job back (section 7.8).
func (s *Service) Release(w *data.Worker, jobID string, req workerproto.ReleaseRequest) (*workerproto.ReleaseResponse, error) {
	reason := req.Reason
	if reason == "" {
		reason = workerproto.ReleaseShutdown
	}
	switch reason {
	case workerproto.ReleaseShutdown, workerproto.ReleaseDiskFull, workerproto.ReleaseOverloaded, workerproto.ReleaseCrashRecovery:
	default:
		return nil, errInvalid("unknown release reason %q", reason)
	}
	if reason.CountsAsAttempt() {
		j, err := s.getJob(jobID)
		if err != nil {
			return nil, err
		}
		if j == nil {
			return nil, errJobNotFound
		}
		if leaseStatus(j, req.Generation, w.ID) != workerproto.LeaseOK {
			return nil, s.lostError(jobID, req.Generation, w.ID)
		}
		if err := s.releaseCounted(w, j, "released after a worker crash"); err != nil {
			return nil, fmt.Errorf("failed to release job: %w", err)
		}
		return &workerproto.ReleaseResponse{Released: 1}, nil
	}
	ok, err := s.Leases.ReleaseLease(jobID, req.Generation, w.ID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to release job: %w", err)
	}
	if !ok {
		return nil, s.lostError(jobID, req.Generation, w.ID)
	}
	s.count(w.ID, data.WorkerCounterReleased, 1)
	s.logger.Info("Worker released a job", zap.Uint("worker_id", w.ID), zap.String("job_id", jobID), zap.String("reason", string(reason)))
	return &workerproto.ReleaseResponse{Released: 1}, nil
}

// ReleaseAll gives back every job of the worker (graceful shutdown).
func (s *Service) ReleaseAll(w *data.Worker, req workerproto.ReleaseAllRequest) (*workerproto.ReleaseResponse, error) {
	if err := s.checkSession(w, req.SessionID); err != nil {
		return nil, err
	}
	if req.Reason.CountsAsAttempt() {
		held, err := s.Leases.ListRemoteRunning(w.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list worker leases: %w", err)
		}
		for i := range held {
			if err := s.releaseCounted(w, &held[i], "released after a worker crash"); err != nil {
				return nil, fmt.Errorf("failed to release job: %w", err)
			}
		}
		return &workerproto.ReleaseResponse{Released: len(held)}, nil
	}
	released, err := s.Leases.ReleaseWorkerLeases(w.ID, nil, false)
	if err != nil {
		return nil, fmt.Errorf("failed to release jobs: %w", err)
	}
	s.count(w.ID, data.WorkerCounterReleased, len(released))
	if len(released) > 0 {
		s.logger.Info("Worker released all its jobs", zap.Uint("worker_id", w.ID), zap.Int("count", len(released)))
	}
	return &workerproto.ReleaseResponse{Released: len(released)}, nil
}

// ReleaseWorker releases every lease of a worker at once, without counting an attempt
// (admin: disabled, revoked or deleted worker).
func (s *Service) ReleaseWorker(workerID uint) (int, error) {
	released, err := s.Leases.ReleaseWorkerLeases(workerID, nil, false)
	if err != nil {
		return 0, err
	}
	s.count(workerID, data.WorkerCounterReleased, len(released))
	return len(released), nil
}

// Fail reports a failed job (section 7.7): the existing retry path, or the dead letter queue
// when the failure is not retryable.
func (s *Service) Fail(w *data.Worker, jobID string, req workerproto.FailRequest) error {
	msg := strings.TrimSpace(req.Error)
	if msg == "" {
		msg = "failed"
	}
	if req.Kind != "" {
		msg = fmt.Sprintf("%s (%s)", msg, req.Kind)
	}
	msg = fmt.Sprintf("worker %s: %s", w.Name, msg)
	j, err := s.getJob(jobID)
	if err != nil {
		return err
	}
	if j == nil {
		return errJobNotFound
	}
	ok, err := s.Leases.FailLease(jobID, req.Generation, w.ID, s.now(), msg, req.Retryable)
	if err != nil {
		return fmt.Errorf("failed to record job failure: %w", err)
	}
	if !ok {
		return s.lostError(jobID, req.Generation, w.ID)
	}
	s.count(w.ID, data.WorkerCounterFailed, 1)
	if err := s.Workers.RecordError(w.ID, msg, s.now()); err != nil {
		s.logger.Warn("Failed to record worker error", zap.Error(err))
	}
	if !req.Retryable && s.Retry != nil {
		if err := s.Retry.ToDeadLetter(jobID, j.Phase, j.SceneID, s.Retry.MaxRetries(j.Phase), msg); err != nil {
			s.logger.Error("Failed to move a job to the dead letter queue", zap.String("job_id", jobID), zap.Error(err))
		}
	}
	s.Results.HandleResult(jobs.JobResult{JobID: jobID, SceneID: j.SceneID, Phase: j.Phase, Status: jobs.JobStatusFailed, Error: errors.New(msg)})
	return nil
}

// MediaURL issues a fresh signed media URL for a leased job.
func (s *Service) MediaURL(w *data.Worker, jobID string, req workerproto.MediaURLRequest) (*workerproto.MediaURLResponse, error) {
	ok, err := s.Leases.CheckLease(jobID, req.Generation, w.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check lease: %w", err)
	}
	if !ok {
		return nil, s.lostError(jobID, req.Generation, w.ID)
	}
	base := s.baseURL(w)
	if base == "" {
		return nil, fmt.Errorf("no server URL known for worker %s: set workers.public_url", w.Name)
	}
	exp := s.now().Add(s.cfg.MediaURLTTL).UTC()
	return &workerproto.MediaURLResponse{URL: workerproto.MediaURL(base, s.MediaSecret, jobID, req.Generation, exp), ExpiresAt: exp}, nil
}

// MediaFile authorizes a signed media URL and returns the source file to serve (read-only).
func (s *Service) MediaFile(jobID string, query map[string][]string) (string, error) {
	gen, err := workerproto.VerifyMedia(s.MediaSecret, jobID, query, s.now())
	if err != nil {
		if err.Error() == "url expired" {
			return "", newError(http.StatusForbidden, workerproto.CodeURLExpired, "media URL expired")
		}
		return "", newError(http.StatusForbidden, workerproto.CodeUnauthorized, "invalid media URL")
	}
	j, err := s.getJob(jobID)
	if err != nil {
		return "", err
	}
	if j == nil {
		return "", errJobNotFound
	}
	if j.Status != data.JobStatusRunning || j.WorkerID == nil || j.LeaseGeneration != gen {
		if j.Status == data.JobStatusCancelled {
			return "", errJobCancelled
		}
		return "", errLeaseLost
	}
	scene, err := s.Scenes.GetByID(j.SceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errSceneGone
		}
		return "", fmt.Errorf("failed to load scene: %w", err)
	}
	return scene.StoredPath, nil
}

// CancelJob cancels a running remote job (user cancel). It returns false when the job is
// not leased to a remote worker.
func (s *Service) CancelJob(jobID string) (bool, error) {
	before, err := s.Leases.CancelLease(jobID, s.now(), "job was cancelled")
	if err != nil {
		return false, err
	}
	if before == nil {
		return false, nil
	}
	s.afterCancel([]data.JobHistory{*before})
	return true, nil
}

// CancelPhases cancels every running remote job of the phases (bulk cancel with running jobs).
func (s *Service) CancelPhases(phases []string) (int, error) {
	rows, err := s.Leases.CancelRemoteLeasesByPhases(phases, s.now(), "Cancelled by bulk cancel")
	if err != nil {
		return 0, err
	}
	s.afterCancel(rows)
	return len(rows), nil
}

func (s *Service) afterCancel(rows []data.JobHistory) {
	s.pushMu.RLock()
	push := s.push
	s.pushMu.RUnlock()
	for _, j := range rows {
		s.Results.HandleResult(jobs.JobResult{JobID: j.JobID, SceneID: j.SceneID, Phase: j.Phase, Status: jobs.JobStatusCancelled})
		if push != nil && j.WorkerID != nil {
			if w, err := s.Workers.GetByID(*j.WorkerID); err == nil && w.ConnectionMode == string(workerproto.ConnPush) {
				push.Cancel(w, j.JobID, j.LeaseGeneration)
			}
		}
		if s.Staging != nil {
			if err := s.Staging.RemoveJob(j.JobID); err != nil {
				s.logger.Warn("Failed to delete the staging of a cancelled job", zap.String("job_id", j.JobID), zap.Error(err))
			}
		}
	}
}
