package core

import (
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"sync"
	"time"

	"go.uber.org/zap"
)

// JobSchedulingService holds the job scheduling settings (phase priorities, time windows,
// quiet-hours pool sizes, global cap, new scenes first) and evaluates them for the job queue
// feeder. Scheduling only decides when pending jobs start: job creation is not affected.
type JobSchedulingService struct {
	repo           data.JobScheduleRepository
	poolManager    *processing.PoolManager
	jobHistoryRepo data.JobHistoryRepository
	logger         *zap.Logger
	now            func() time.Time

	mu       sync.RWMutex
	cfg      JobScheduleConfig
	compiled *compiledSchedule
}

// NewJobSchedulingService creates the service and loads the settings from the database
// (defaults when missing or invalid).
func NewJobSchedulingService(
	repo data.JobScheduleRepository,
	poolManager *processing.PoolManager,
	jobHistoryRepo data.JobHistoryRepository,
	logger *zap.Logger,
) *JobSchedulingService {
	s := &JobSchedulingService{
		repo:           repo,
		poolManager:    poolManager,
		jobHistoryRepo: jobHistoryRepo,
		logger:         logger.With(zap.String("component", "job_scheduling")),
		now:            time.Now,
	}
	cfg, err := s.load()
	if err != nil {
		s.logger.Error("Failed to load job scheduling settings, using defaults", zap.Error(err))
		cfg = DefaultJobScheduleConfig()
	}
	compiled, err := compileSchedule(cfg)
	if err != nil {
		s.logger.Error("Invalid job scheduling settings in database, using defaults", zap.Error(err))
		cfg = DefaultJobScheduleConfig()
		compiled, _ = compileSchedule(cfg)
	}
	s.cfg = cfg.Normalize()
	s.compiled = compiled
	return s
}

func (s *JobSchedulingService) load() (JobScheduleConfig, error) {
	cfg := DefaultJobScheduleConfig()
	if s.repo == nil {
		return cfg, nil
	}
	rec, err := s.repo.GetConfig()
	if err != nil {
		return cfg, fmt.Errorf("failed to load job schedule config: %w", err)
	}
	if rec != nil {
		cfg.Timezone = rec.Timezone
		cfg.QuietHoursStart = rec.QuietHoursStart
		cfg.QuietHoursEnd = rec.QuietHoursEnd
		cfg.MaxConcurrentJobs = rec.MaxConcurrentJobs
		cfg.NewScenesFirst = rec.NewScenesFirst
		cfg.NewScenesWindowHours = rec.NewScenesWindowHours
	}
	phases, err := s.repo.ListPhases()
	if err != nil {
		return cfg, fmt.Errorf("failed to load job phase schedules: %w", err)
	}
	if len(phases) > 0 {
		cfg.Phases = nil
		for _, p := range phases {
			cfg.Phases = append(cfg.Phases, PhaseScheduleConfig{
				Phase:         p.Phase,
				Priority:      p.Priority,
				WindowEnabled: p.WindowEnabled,
				WindowStart:   p.WindowStart,
				WindowEnd:     p.WindowEnd,
				QuietWorkers:  p.QuietWorkers,
			})
		}
	}
	return cfg.Normalize(), nil
}

// GetConfig returns the current settings.
func (s *JobSchedulingService) GetConfig() JobScheduleConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.cfg
	cfg.Phases = append([]PhaseScheduleConfig(nil), s.cfg.Phases...)
	return cfg
}

// UpdateConfig validates, stores and applies new settings. The feeder picks them up on its
// next tick (within seconds).
func (s *JobSchedulingService) UpdateConfig(cfg JobScheduleConfig) (JobScheduleConfig, error) {
	cfg = cfg.Normalize()
	compiled, err := compileSchedule(cfg)
	if err != nil {
		return JobScheduleConfig{}, err
	}

	if s.repo != nil {
		rec := &data.JobScheduleConfigRecord{
			Timezone:             cfg.Timezone,
			QuietHoursStart:      cfg.QuietHoursStart,
			QuietHoursEnd:        cfg.QuietHoursEnd,
			MaxConcurrentJobs:    cfg.MaxConcurrentJobs,
			NewScenesFirst:       cfg.NewScenesFirst,
			NewScenesWindowHours: cfg.NewScenesWindowHours,
		}
		phases := make([]data.JobPhaseScheduleRecord, 0, len(cfg.Phases))
		for _, p := range cfg.Phases {
			phases = append(phases, data.JobPhaseScheduleRecord{
				Phase:         p.Phase,
				Priority:      p.Priority,
				WindowEnabled: p.WindowEnabled,
				WindowStart:   p.WindowStart,
				WindowEnd:     p.WindowEnd,
				QuietWorkers:  p.QuietWorkers,
			})
		}
		if err := s.repo.Save(rec, phases); err != nil {
			return JobScheduleConfig{}, apperrors.NewInternalError("failed to save job scheduling settings", err)
		}
	}

	s.mu.Lock()
	s.cfg = cfg
	s.compiled = compiled
	s.mu.Unlock()

	s.logger.Info("Job scheduling settings updated",
		zap.String("timezone", cfg.Timezone),
		zap.String("quiet_hours", cfg.QuietHoursStart+"-"+cfg.QuietHoursEnd),
		zap.Int("max_concurrent_jobs", cfg.MaxConcurrentJobs),
		zap.Bool("new_scenes_first", cfg.NewScenesFirst),
	)
	return s.GetConfig(), nil
}

// Plan evaluates the schedule at now.
func (s *JobSchedulingService) Plan(now time.Time) SchedulePlan {
	s.mu.RLock()
	compiled := s.compiled
	s.mu.RUnlock()
	return compiled.plan(now)
}

// JobScheduleStatus is the effective scheduling state, for the admin UI.
type JobScheduleStatus struct {
	Now                  time.Time            `json:"now"`
	Timezone             string               `json:"timezone"`
	QuietHours           bool                 `json:"quiet_hours"`
	QuietHoursNextChange time.Time            `json:"quiet_hours_next_change"`
	MaxConcurrentJobs    int                  `json:"max_concurrent_jobs"`
	RunningJobs          int                  `json:"running_jobs"`
	NewSceneSince        *time.Time           `json:"new_scene_since,omitempty"`
	Phases               []PhaseScheduleState `json:"phases"`
}

// PhaseScheduleState is the effective state of one phase.
type PhaseScheduleState struct {
	Phase    string `json:"phase"`
	Priority int    `json:"priority"`
	// State is "running" (may start jobs) or "paused" (outside its time window).
	State      string     `json:"state"`
	NextChange *time.Time `json:"next_change,omitempty"`
	// Workers is the pool size in use, ConfiguredWorkers the Workers tab value, WorkerMode
	// "quiet" when the quiet-hours size applies, "day" otherwise.
	Workers           int    `json:"workers"`
	ConfiguredWorkers int    `json:"configured_workers"`
	QuietWorkers      int    `json:"quiet_workers"`
	WorkerMode        string `json:"worker_mode"`
	Pending           int    `json:"pending"`
	Queued            int    `json:"queued"`
	Running           int    `json:"running"`
}

// Status returns the effective scheduling state now. Only the phases with a worker pool are
// listed (fingerprint needs duplicate detection).
func (s *JobSchedulingService) Status() (JobScheduleStatus, error) {
	now := s.now()
	plan := s.Plan(now)
	cfg := s.GetConfig()

	status := JobScheduleStatus{
		Now:                  now,
		Timezone:             cfg.Timezone,
		QuietHours:           plan.QuietHours,
		QuietHoursNextChange: plan.QuietHoursNextChange,
		MaxConcurrentJobs:    plan.MaxConcurrentJobs,
	}
	if !plan.NewSceneSince.IsZero() {
		since := plan.NewSceneSince
		status.NewSceneSince = &since
	}

	var pending map[string]int
	if s.jobHistoryRepo != nil {
		counts, err := s.jobHistoryRepo.CountPendingByPhase()
		if err != nil {
			return JobScheduleStatus{}, apperrors.NewInternalError("failed to count pending jobs", err)
		}
		pending = counts
	}

	var effective, base processing.PoolConfig
	var queue processing.QueueStatus
	duplicationEnabled := true
	if s.poolManager != nil {
		effective = s.poolManager.GetPoolConfig()
		base = s.poolManager.GetBasePoolConfig()
		queue = s.poolManager.GetQueueStatus()
		status.RunningJobs = s.poolManager.RunningJobCount()
		duplicationEnabled = s.poolManager.IsDuplicationEnabled()
	}

	quietWorkers := map[string]int{}
	for _, p := range cfg.Phases {
		quietWorkers[p.Phase] = p.QuietWorkers
	}

	for _, phase := range plan.Order {
		if phase == "fingerprint" && !duplicationEnabled {
			continue
		}
		pp := plan.Phase(phase)
		st := PhaseScheduleState{
			Phase:        phase,
			Priority:     pp.Priority,
			State:        "running",
			QuietWorkers: quietWorkers[phase],
			WorkerMode:   "day",
			Pending:      pending[phase],
		}
		if !pp.Allowed {
			st.State = "paused"
		}
		if !pp.NextChange.IsZero() {
			next := pp.NextChange
			st.NextChange = &next
		}
		if pp.QuietWorkers > 0 {
			st.WorkerMode = "quiet"
		}
		st.Workers, st.ConfiguredWorkers, st.Queued, st.Running = phasePoolNumbers(phase, effective, base, queue)
		status.Phases = append(status.Phases, st)
	}
	return status, nil
}

func phasePoolNumbers(phase string, effective, base processing.PoolConfig, q processing.QueueStatus) (workers, configured, queued, running int) {
	switch phase {
	case "metadata":
		return effective.MetadataWorkers, base.MetadataWorkers, q.MetadataQueued, q.MetadataActive
	case "thumbnail":
		return effective.ThumbnailWorkers, base.ThumbnailWorkers, q.ThumbnailQueued, q.ThumbnailActive
	case "sprites":
		return effective.SpritesWorkers, base.SpritesWorkers, q.SpritesQueued, q.SpritesActive
	case "animated_thumbnails":
		return effective.AnimatedThumbnailsWorkers, base.AnimatedThumbnailsWorkers, q.AnimatedThumbnailsQueued, q.AnimatedThumbnailsActive
	case "fingerprint":
		return effective.FingerprintWorkers, base.FingerprintWorkers, q.FingerprintQueued, q.FingerprintActive
	}
	return 0, 0, 0, 0
}
