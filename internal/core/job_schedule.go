package core

import (
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/core/processing"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SchedulablePhases are the phases the job scheduler manages, in pipeline order. The order is
// also the tie-breaker between phases of equal priority.
var SchedulablePhases = []string{"metadata", "thumbnail", "sprites", "animated_thumbnails", "fingerprint"}

const (
	maxPhasePriority        = 100
	maxQuietWorkers         = 32
	maxConcurrentJobsLimit  = 256
	maxNewScenesWindowHours = 720
)

// PhaseScheduleConfig holds the scheduling settings of one phase.
type PhaseScheduleConfig struct {
	Phase string `json:"phase"`
	// Priority (0-100, higher first) decides which phase gets the next free slot when the
	// global limit on running jobs is reached.
	Priority int `json:"priority"`
	// WindowEnabled restricts the phase to the daily window [WindowStart, WindowEnd) ("HH:MM",
	// in the schedule timezone; the window may cross midnight). Outside it, pending jobs wait.
	WindowEnabled bool   `json:"window_enabled"`
	WindowStart   string `json:"window_start"`
	WindowEnd     string `json:"window_end"`
	// QuietWorkers is the pool size during quiet hours (0 = keep the configured size).
	QuietWorkers int `json:"quiet_workers"`
}

// JobScheduleConfig holds all the job scheduling settings.
type JobScheduleConfig struct {
	Timezone             string                `json:"timezone"`
	QuietHoursStart      string                `json:"quiet_hours_start"`
	QuietHoursEnd        string                `json:"quiet_hours_end"`
	MaxConcurrentJobs    int                   `json:"max_concurrent_jobs"`
	NewScenesFirst       bool                  `json:"new_scenes_first"`
	NewScenesWindowHours int                   `json:"new_scenes_window_hours"`
	Phases               []PhaseScheduleConfig `json:"phases"`
}

var defaultPhasePriorities = map[string]int{
	"metadata":            50,
	"thumbnail":           40,
	"animated_thumbnails": 30,
	"sprites":             20,
	"fingerprint":         10,
}

// DefaultJobScheduleConfig returns the settings that keep the behaviour from before job
// scheduling: every phase runs at any time with its configured pool size, no global limit.
func DefaultJobScheduleConfig() JobScheduleConfig {
	cfg := JobScheduleConfig{
		Timezone:             "UTC",
		QuietHoursStart:      "01:00",
		QuietHoursEnd:        "07:00",
		NewScenesWindowHours: 24,
	}
	for _, phase := range SchedulablePhases {
		cfg.Phases = append(cfg.Phases, defaultPhaseSchedule(phase))
	}
	return cfg
}

func defaultPhaseSchedule(phase string) PhaseScheduleConfig {
	return PhaseScheduleConfig{
		Phase:       phase,
		Priority:    defaultPhasePriorities[phase],
		WindowStart: "01:00",
		WindowEnd:   "07:00",
	}
}

// Normalize returns a copy with one entry per schedulable phase, in pipeline order: missing
// phases get their defaults, unknown phases are dropped later by Validate.
func (c JobScheduleConfig) Normalize() JobScheduleConfig {
	out := c
	out.Timezone = strings.TrimSpace(c.Timezone)
	out.QuietHoursStart = strings.TrimSpace(c.QuietHoursStart)
	out.QuietHoursEnd = strings.TrimSpace(c.QuietHoursEnd)
	byPhase := make(map[string]PhaseScheduleConfig, len(c.Phases))
	var unknown []PhaseScheduleConfig
	for _, p := range c.Phases {
		p.WindowStart = strings.TrimSpace(p.WindowStart)
		p.WindowEnd = strings.TrimSpace(p.WindowEnd)
		if isSchedulablePhase(p.Phase) {
			byPhase[p.Phase] = p
		} else {
			unknown = append(unknown, p)
		}
	}
	out.Phases = nil
	for _, phase := range SchedulablePhases {
		p, ok := byPhase[phase]
		if !ok {
			p = defaultPhaseSchedule(phase)
		}
		out.Phases = append(out.Phases, p)
	}
	// Keep unknown phases so that Validate reports them.
	out.Phases = append(out.Phases, unknown...)
	return out
}

func isSchedulablePhase(phase string) bool {
	for _, p := range SchedulablePhases {
		if p == phase {
			return true
		}
	}
	return false
}

// Validate checks the settings; errors are validation errors.
func (c JobScheduleConfig) Validate() error {
	if _, err := loadScheduleLocation(c.Timezone); err != nil {
		return apperrors.NewValidationErrorWithField("timezone", err.Error())
	}
	if _, err := ParseClockTime(c.QuietHoursStart); err != nil {
		return apperrors.NewValidationErrorWithField("quiet_hours_start", err.Error())
	}
	if _, err := ParseClockTime(c.QuietHoursEnd); err != nil {
		return apperrors.NewValidationErrorWithField("quiet_hours_end", err.Error())
	}
	if c.QuietHoursStart == c.QuietHoursEnd {
		return apperrors.NewValidationErrorWithField("quiet_hours_end", "quiet hours must start and end at different times")
	}
	if c.MaxConcurrentJobs < 0 || c.MaxConcurrentJobs > maxConcurrentJobsLimit {
		return apperrors.NewValidationErrorWithField("max_concurrent_jobs",
			fmt.Sprintf("max_concurrent_jobs must be between 0 (no limit) and %d", maxConcurrentJobsLimit))
	}
	if c.NewScenesWindowHours < 1 || c.NewScenesWindowHours > maxNewScenesWindowHours {
		return apperrors.NewValidationErrorWithField("new_scenes_window_hours",
			fmt.Sprintf("new_scenes_window_hours must be between 1 and %d", maxNewScenesWindowHours))
	}
	seen := map[string]bool{}
	for _, p := range c.Phases {
		if !isSchedulablePhase(p.Phase) {
			return apperrors.NewValidationErrorWithField("phases", fmt.Sprintf("unknown phase %q", p.Phase))
		}
		if seen[p.Phase] {
			return apperrors.NewValidationErrorWithField("phases", fmt.Sprintf("phase %q is listed twice", p.Phase))
		}
		seen[p.Phase] = true
		if p.Priority < 0 || p.Priority > maxPhasePriority {
			return apperrors.NewValidationErrorWithField("phases",
				fmt.Sprintf("%s: priority must be between 0 and %d", p.Phase, maxPhasePriority))
		}
		if _, err := ParseClockTime(p.WindowStart); err != nil {
			return apperrors.NewValidationErrorWithField("phases", fmt.Sprintf("%s: window start: %v", p.Phase, err))
		}
		if _, err := ParseClockTime(p.WindowEnd); err != nil {
			return apperrors.NewValidationErrorWithField("phases", fmt.Sprintf("%s: window end: %v", p.Phase, err))
		}
		if p.WindowStart == p.WindowEnd {
			return apperrors.NewValidationErrorWithField("phases",
				fmt.Sprintf("%s: the window must start and end at different times", p.Phase))
		}
		if p.QuietWorkers < 0 || p.QuietWorkers > maxQuietWorkers {
			return apperrors.NewValidationErrorWithField("phases",
				fmt.Sprintf("%s: quiet hours workers must be between 0 (unchanged) and %d", p.Phase, maxQuietWorkers))
		}
	}
	return nil
}

func loadScheduleLocation(name string) (*time.Location, error) {
	if name == "" {
		return nil, fmt.Errorf("timezone is required")
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q", name)
	}
	return loc, nil
}

// ParseClockTime parses a "HH:MM" time of day (00:00-23:59) into minutes since midnight.
func ParseClockTime(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("time must be HH:MM, got %q", s)
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time must be HH:MM between 00:00 and 23:59, got %q", s)
	}
	return h*60 + m, nil
}

// DailyWindow is a daily time range [Start, End) in minutes since midnight, evaluated on the
// wall clock of the time passed in. End before Start means the window crosses midnight.
type DailyWindow struct {
	Start int
	End   int
}

// Contains reports whether t (wall clock in its own location) falls in the window.
func (w DailyWindow) Contains(t time.Time) bool {
	m := t.Hour()*60 + t.Minute()
	if w.Start < w.End {
		return m >= w.Start && m < w.End
	}
	return m >= w.Start || m < w.End
}

// NextChange returns the next time after t at which Contains changes: the end of the window
// when t is inside it, its next start otherwise.
func (w DailyWindow) NextChange(t time.Time) time.Time {
	boundary := w.Start
	if w.Contains(t) {
		boundary = w.End
	}
	for days := 0; days <= 2; days++ {
		y, mo, d := t.Date()
		candidate := time.Date(y, mo, d+days, boundary/60, boundary%60, 0, 0, t.Location())
		if candidate.After(t) {
			return candidate
		}
	}
	return t.Add(24 * time.Hour)
}

// SchedulePlan is the scheduling decision at a given time.
type SchedulePlan struct {
	// Order lists the phases by priority, highest first.
	Order  []string
	Phases map[string]PhasePlan
	// QuietHours reports whether the quiet hours are in progress; QuietHoursNextChange is when
	// they start or end next.
	QuietHours           bool
	QuietHoursNextChange time.Time
	// MaxConcurrentJobs caps the running jobs across phases (0 = no cap).
	MaxConcurrentJobs int
	// NewSceneSince, when not zero, makes the jobs of scenes created since then go first.
	NewSceneSince time.Time
}

// PhasePlan is the scheduling decision for one phase.
type PhasePlan struct {
	Priority      int
	WindowEnabled bool
	// Allowed reports whether the phase may start new jobs now.
	Allowed bool
	// NextChange is when Allowed changes next (zero without a window).
	NextChange time.Time
	// QuietWorkers is the pool size to use now (0 = the configured size).
	QuietWorkers int
}

// Phase returns the plan of a phase; a phase the plan does not know is allowed.
func (p SchedulePlan) Phase(phase string) PhasePlan {
	if pp, ok := p.Phases[phase]; ok {
		return pp
	}
	return PhasePlan{Allowed: true}
}

// PoolSettings converts the plan into the settings applied to the worker pools.
func (p SchedulePlan) PoolSettings() processing.ScheduleSettings {
	settings := processing.ScheduleSettings{
		WorkerOverrides:   map[string]int{},
		Priorities:        map[string]int{},
		MaxConcurrentJobs: p.MaxConcurrentJobs,
	}
	for phase, pp := range p.Phases {
		settings.Priorities[phase] = pp.Priority
		if pp.QuietWorkers > 0 {
			settings.WorkerOverrides[phase] = pp.QuietWorkers
		}
	}
	return settings
}

// WindowedPhases returns the phases restricted to a time window.
func (p SchedulePlan) WindowedPhases() []string {
	var phases []string
	for _, phase := range p.Order {
		if p.Phases[phase].WindowEnabled {
			phases = append(phases, phase)
		}
	}
	return phases
}

// compiledSchedule is a validated JobScheduleConfig ready to be evaluated.
type compiledSchedule struct {
	loc             *time.Location
	quiet           DailyWindow
	maxConcurrent   int
	newScenesFirst  bool
	newScenesWindow time.Duration
	phases          map[string]compiledPhase
	order           []string
}

type compiledPhase struct {
	priority      int
	windowEnabled bool
	window        DailyWindow
	quietWorkers  int
}

func compileSchedule(c JobScheduleConfig) (*compiledSchedule, error) {
	c = c.Normalize()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	loc, _ := loadScheduleLocation(c.Timezone)
	qs, _ := ParseClockTime(c.QuietHoursStart)
	qe, _ := ParseClockTime(c.QuietHoursEnd)
	cs := &compiledSchedule{
		loc:             loc,
		quiet:           DailyWindow{Start: qs, End: qe},
		maxConcurrent:   c.MaxConcurrentJobs,
		newScenesFirst:  c.NewScenesFirst,
		newScenesWindow: time.Duration(c.NewScenesWindowHours) * time.Hour,
		phases:          map[string]compiledPhase{},
	}
	for _, p := range c.Phases {
		ws, _ := ParseClockTime(p.WindowStart)
		we, _ := ParseClockTime(p.WindowEnd)
		cs.phases[p.Phase] = compiledPhase{
			priority:      p.Priority,
			windowEnabled: p.WindowEnabled,
			window:        DailyWindow{Start: ws, End: we},
			quietWorkers:  p.QuietWorkers,
		}
	}
	cs.order = append([]string(nil), SchedulablePhases...)
	pipelineIndex := map[string]int{}
	for i, phase := range SchedulablePhases {
		pipelineIndex[phase] = i
	}
	sort.SliceStable(cs.order, func(i, j int) bool {
		pi, pj := cs.phases[cs.order[i]].priority, cs.phases[cs.order[j]].priority
		if pi != pj {
			return pi > pj
		}
		return pipelineIndex[cs.order[i]] < pipelineIndex[cs.order[j]]
	})
	return cs, nil
}

// plan evaluates the schedule at now.
func (cs *compiledSchedule) plan(now time.Time) SchedulePlan {
	local := now.In(cs.loc)
	quiet := cs.quiet.Contains(local)
	plan := SchedulePlan{
		Order:                append([]string(nil), cs.order...),
		Phases:               make(map[string]PhasePlan, len(cs.phases)),
		QuietHours:           quiet,
		QuietHoursNextChange: cs.quiet.NextChange(local),
		MaxConcurrentJobs:    cs.maxConcurrent,
	}
	if cs.newScenesFirst {
		plan.NewSceneSince = now.Add(-cs.newScenesWindow)
	}
	for phase, cp := range cs.phases {
		pp := PhasePlan{
			Priority:      cp.priority,
			WindowEnabled: cp.windowEnabled,
			Allowed:       true,
		}
		if cp.windowEnabled {
			pp.Allowed = cp.window.Contains(local)
			pp.NextChange = cp.window.NextChange(local)
		}
		if quiet {
			pp.QuietWorkers = cp.quietWorkers
		}
		plan.Phases[phase] = pp
	}
	return plan
}
