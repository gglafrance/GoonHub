package core

import (
	"goonhub/internal/apperrors"
	"testing"
	"time"
)

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %s: %v", name, err)
	}
	return loc
}

func mustCompile(t *testing.T, cfg JobScheduleConfig) *compiledSchedule {
	t.Helper()
	cs, err := compileSchedule(cfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return cs
}

func setPhase(cfg *JobScheduleConfig, phase string, fn func(p *PhaseScheduleConfig)) {
	for i := range cfg.Phases {
		if cfg.Phases[i].Phase == phase {
			fn(&cfg.Phases[i])
		}
	}
}

func TestParseClockTime(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"00:00", 0, false},
		{"01:30", 90, false},
		{"23:59", 23*60 + 59, false},
		{"24:00", 0, true},
		{"12:60", 0, true},
		{"1:30", 0, true},
		{"01:3", 0, true},
		{"0130", 0, true},
		{"", 0, true},
		{"ab:cd", 0, true},
		{"-1:00", 0, true},
	}
	for _, c := range cases {
		got, err := ParseClockTime(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ParseClockTime(%q): err = %v, wantErr %v", c.in, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Fatalf("ParseClockTime(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDailyWindowContains(t *testing.T) {
	day := func(h, m int) time.Time { return time.Date(2026, 11, 21, h, m, 0, 0, time.UTC) }
	cases := []struct {
		name   string
		window DailyWindow
		at     time.Time
		want   bool
	}{
		{"same day, inside", DailyWindow{60, 420}, day(3, 0), true},
		{"same day, at start", DailyWindow{60, 420}, day(1, 0), true},
		{"same day, at end (excluded)", DailyWindow{60, 420}, day(7, 0), false},
		{"same day, before", DailyWindow{60, 420}, day(0, 59), false},
		{"same day, after", DailyWindow{60, 420}, day(12, 0), false},
		{"crossing midnight, evening", DailyWindow{22 * 60, 6 * 60}, day(23, 30), true},
		{"crossing midnight, at midnight", DailyWindow{22 * 60, 6 * 60}, day(0, 0), true},
		{"crossing midnight, morning", DailyWindow{22 * 60, 6 * 60}, day(5, 59), true},
		{"crossing midnight, at end", DailyWindow{22 * 60, 6 * 60}, day(6, 0), false},
		{"crossing midnight, afternoon", DailyWindow{22 * 60, 6 * 60}, day(15, 0), false},
		{"crossing midnight, at start", DailyWindow{22 * 60, 6 * 60}, day(22, 0), true},
	}
	for _, c := range cases {
		if got := c.window.Contains(c.at); got != c.want {
			t.Fatalf("%s: Contains(%s) = %v, want %v", c.name, c.at.Format("15:04"), got, c.want)
		}
	}
}

func TestDailyWindowNextChange(t *testing.T) {
	w := DailyWindow{Start: 22 * 60, End: 6 * 60} // 22:00-06:00
	cases := []struct {
		at   time.Time
		want time.Time
	}{
		// outside: next start the same evening
		{time.Date(2026, 11, 21, 15, 0, 0, 0, time.UTC), time.Date(2026, 11, 21, 22, 0, 0, 0, time.UTC)},
		// inside, before midnight: end next morning
		{time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC), time.Date(2026, 11, 22, 6, 0, 0, 0, time.UTC)},
		// inside, after midnight: end this morning
		{time.Date(2026, 11, 22, 2, 0, 0, 0, time.UTC), time.Date(2026, 11, 22, 6, 0, 0, 0, time.UTC)},
		// exactly at the end: outside, next start this evening
		{time.Date(2026, 11, 22, 6, 0, 0, 0, time.UTC), time.Date(2026, 11, 22, 22, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := w.NextChange(c.at); !got.Equal(c.want) {
			t.Fatalf("NextChange(%s) = %s, want %s", c.at, got, c.want)
		}
	}
}

func TestDefaultScheduleKeepsPreviousBehaviour(t *testing.T) {
	cs := mustCompile(t, DefaultJobScheduleConfig())
	for _, at := range []time.Time{
		time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC), // in the default quiet hours
		time.Date(2026, 11, 21, 15, 0, 0, 0, time.UTC),
	} {
		plan := cs.plan(at)
		if plan.MaxConcurrentJobs != 0 {
			t.Fatalf("expected no global cap, got %d", plan.MaxConcurrentJobs)
		}
		if !plan.NewSceneSince.IsZero() {
			t.Fatal("expected no new scene boost by default")
		}
		for _, phase := range SchedulablePhases {
			pp := plan.Phase(phase)
			if !pp.Allowed || pp.QuietWorkers != 0 || pp.WindowEnabled {
				t.Fatalf("%s at %s: expected allowed with no override, got %+v", phase, at.Format("15:04"), pp)
			}
		}
		if len(plan.PoolSettings().WorkerOverrides) != 0 {
			t.Fatal("expected no worker overrides by default")
		}
		if len(plan.WindowedPhases()) != 0 {
			t.Fatal("expected no windowed phase by default")
		}
	}
}

func TestPlanOrderFollowsPriority(t *testing.T) {
	cfg := DefaultJobScheduleConfig()
	setPhase(&cfg, "fingerprint", func(p *PhaseScheduleConfig) { p.Priority = 90 })
	setPhase(&cfg, "thumbnail", func(p *PhaseScheduleConfig) { p.Priority = 90 })
	setPhase(&cfg, "metadata", func(p *PhaseScheduleConfig) { p.Priority = 0 })
	plan := mustCompile(t, cfg).plan(time.Now())

	// Ties keep the pipeline order: thumbnail before fingerprint.
	want := []string{"thumbnail", "fingerprint", "animated_thumbnails", "sprites", "metadata"}
	for i := range want {
		if plan.Order[i] != want[i] {
			t.Fatalf("order = %v, want %v", plan.Order, want)
		}
	}
	if got := plan.PoolSettings().Priorities["fingerprint"]; got != 90 {
		t.Fatalf("fingerprint slot priority = %d, want 90", got)
	}
}

func TestPlanTimeWindowCrossingMidnightInTimezone(t *testing.T) {
	cfg := DefaultJobScheduleConfig()
	cfg.Timezone = "Europe/Paris"
	setPhase(&cfg, "fingerprint", func(p *PhaseScheduleConfig) {
		p.WindowEnabled = true
		p.WindowStart = "23:00"
		p.WindowEnd = "07:00"
	})
	cs := mustCompile(t, cfg)
	paris := mustLocation(t, "Europe/Paris")

	cases := []struct {
		at      time.Time
		allowed bool
		next    time.Time
	}{
		// 21:30 UTC = 22:30 Paris (winter, UTC+1): closed, opens at 23:00 Paris
		{time.Date(2026, 11, 21, 21, 30, 0, 0, time.UTC), false, time.Date(2026, 11, 21, 23, 0, 0, 0, paris)},
		// 22:30 UTC = 23:30 Paris: open until 07:00 Paris
		{time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC), true, time.Date(2026, 11, 22, 7, 0, 0, 0, paris)},
		// 05:30 UTC = 06:30 Paris: still open
		{time.Date(2026, 11, 22, 5, 30, 0, 0, time.UTC), true, time.Date(2026, 11, 22, 7, 0, 0, 0, paris)},
		// 06:30 UTC = 07:30 Paris: closed (would be open if evaluated in UTC)
		{time.Date(2026, 11, 22, 6, 30, 0, 0, time.UTC), false, time.Date(2026, 11, 22, 23, 0, 0, 0, paris)},
	}
	for _, c := range cases {
		pp := cs.plan(c.at).Phase("fingerprint")
		if pp.Allowed != c.allowed {
			t.Fatalf("at %s UTC: allowed = %v, want %v", c.at.Format("15:04"), pp.Allowed, c.allowed)
		}
		if !pp.NextChange.Equal(c.next) {
			t.Fatalf("at %s UTC: next change = %s, want %s", c.at.Format("15:04"), pp.NextChange, c.next)
		}
		// Other phases are not restricted.
		if !cs.plan(c.at).Phase("thumbnail").Allowed {
			t.Fatal("thumbnail must stay allowed")
		}
	}
	plan := cs.plan(time.Now())
	if got := plan.WindowedPhases(); len(got) != 1 || got[0] != "fingerprint" {
		t.Fatalf("windowed phases = %v, want [fingerprint]", got)
	}
}

func TestPlanTimeWindowAcrossDSTChange(t *testing.T) {
	cfg := DefaultJobScheduleConfig()
	cfg.Timezone = "Europe/Paris"
	setPhase(&cfg, "sprites", func(p *PhaseScheduleConfig) {
		p.WindowEnabled = true
		p.WindowStart = "01:00"
		p.WindowEnd = "07:00"
	})
	cs := mustCompile(t, cfg)
	// Summer (UTC+2): 00:30 UTC = 02:30 Paris, inside the window
	if !cs.plan(time.Date(2026, 7, 1, 0, 30, 0, 0, time.UTC)).Phase("sprites").Allowed {
		t.Fatal("expected sprites allowed at 02:30 Paris in summer")
	}
	// Winter (UTC+1): 00:30 UTC = 01:30 Paris, inside; 23:30 UTC = 00:30 Paris, outside
	if !cs.plan(time.Date(2026, 1, 15, 0, 30, 0, 0, time.UTC)).Phase("sprites").Allowed {
		t.Fatal("expected sprites allowed at 01:30 Paris in winter")
	}
	if cs.plan(time.Date(2026, 1, 14, 23, 30, 0, 0, time.UTC)).Phase("sprites").Allowed {
		t.Fatal("expected sprites paused at 00:30 Paris in winter")
	}
}

func TestPlanQuietHoursWorkerSwitching(t *testing.T) {
	cfg := DefaultJobScheduleConfig()
	cfg.QuietHoursStart = "22:00"
	cfg.QuietHoursEnd = "06:00"
	setPhase(&cfg, "fingerprint", func(p *PhaseScheduleConfig) { p.QuietWorkers = 12 })
	cs := mustCompile(t, cfg)

	day := cs.plan(time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC))
	if day.QuietHours {
		t.Fatal("14:00 is not in the quiet hours")
	}
	if day.Phase("fingerprint").QuietWorkers != 0 {
		t.Fatal("expected the configured pool size by day")
	}
	if _, ok := day.PoolSettings().WorkerOverrides["fingerprint"]; ok {
		t.Fatal("expected no override by day")
	}
	if want := time.Date(2026, 11, 21, 22, 0, 0, 0, time.UTC); !day.QuietHoursNextChange.Equal(want) {
		t.Fatalf("quiet hours next change = %s, want %s", day.QuietHoursNextChange, want)
	}

	night := cs.plan(time.Date(2026, 11, 22, 2, 0, 0, 0, time.UTC))
	if !night.QuietHours {
		t.Fatal("02:00 is in the quiet hours")
	}
	if got := night.PoolSettings().WorkerOverrides["fingerprint"]; got != 12 {
		t.Fatalf("fingerprint override at night = %d, want 12", got)
	}
	if _, ok := night.PoolSettings().WorkerOverrides["thumbnail"]; ok {
		t.Fatal("thumbnail has no quiet hours size, expected no override")
	}
}

func TestPlanNewScenesFirstAndCap(t *testing.T) {
	cfg := DefaultJobScheduleConfig()
	cfg.NewScenesFirst = true
	cfg.NewScenesWindowHours = 6
	cfg.MaxConcurrentJobs = 8
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	plan := mustCompile(t, cfg).plan(now)
	if want := now.Add(-6 * time.Hour); !plan.NewSceneSince.Equal(want) {
		t.Fatalf("new scene since = %s, want %s", plan.NewSceneSince, want)
	}
	if plan.PoolSettings().MaxConcurrentJobs != 8 {
		t.Fatalf("cap = %d, want 8", plan.PoolSettings().MaxConcurrentJobs)
	}
}

func TestJobScheduleConfigValidate(t *testing.T) {
	cases := []struct {
		name   string
		modify func(c *JobScheduleConfig)
		ok     bool
	}{
		{"defaults", func(c *JobScheduleConfig) {}, true},
		{"unknown timezone", func(c *JobScheduleConfig) { c.Timezone = "Mars/Olympus" }, false},
		{"empty timezone", func(c *JobScheduleConfig) { c.Timezone = "" }, false},
		{"bad quiet start", func(c *JobScheduleConfig) { c.QuietHoursStart = "25:00" }, false},
		{"empty quiet hours", func(c *JobScheduleConfig) { c.QuietHoursEnd = c.QuietHoursStart }, false},
		{"negative cap", func(c *JobScheduleConfig) { c.MaxConcurrentJobs = -1 }, false},
		{"cap too high", func(c *JobScheduleConfig) { c.MaxConcurrentJobs = maxConcurrentJobsLimit + 1 }, false},
		{"cap max", func(c *JobScheduleConfig) { c.MaxConcurrentJobs = maxConcurrentJobsLimit }, true},
		{"window hours 0", func(c *JobScheduleConfig) { c.NewScenesWindowHours = 0 }, false},
		{"window hours too high", func(c *JobScheduleConfig) { c.NewScenesWindowHours = maxNewScenesWindowHours + 1 }, false},
		{"priority too high", func(c *JobScheduleConfig) { c.Phases[0].Priority = maxPhasePriority + 1 }, false},
		{"negative priority", func(c *JobScheduleConfig) { c.Phases[0].Priority = -1 }, false},
		{"empty window", func(c *JobScheduleConfig) { c.Phases[1].WindowEnd = c.Phases[1].WindowStart }, false},
		{"bad window end", func(c *JobScheduleConfig) { c.Phases[1].WindowEnd = "7:00" }, false},
		{"quiet workers too high", func(c *JobScheduleConfig) { c.Phases[2].QuietWorkers = maxQuietWorkers + 1 }, false},
		{"quiet workers max", func(c *JobScheduleConfig) { c.Phases[2].QuietWorkers = maxQuietWorkers }, true},
		{"unknown phase", func(c *JobScheduleConfig) {
			c.Phases = append(c.Phases, PhaseScheduleConfig{Phase: "scan", WindowStart: "01:00", WindowEnd: "02:00"})
		}, false},
	}
	for _, c := range cases {
		cfg := DefaultJobScheduleConfig()
		c.modify(&cfg)
		err := cfg.Normalize().Validate()
		if c.ok && err != nil {
			t.Fatalf("%s: unexpected error %v", c.name, err)
		}
		if !c.ok {
			if err == nil {
				t.Fatalf("%s: expected a validation error", c.name)
			}
			if !apperrors.IsValidation(err) {
				t.Fatalf("%s: expected a validation error, got %T", c.name, err)
			}
		}
	}
}

func TestJobScheduleConfigNormalizeFillsMissingPhases(t *testing.T) {
	cfg := JobScheduleConfig{
		Timezone:             " UTC ",
		QuietHoursStart:      "01:00",
		QuietHoursEnd:        "07:00",
		NewScenesWindowHours: 24,
		Phases:               []PhaseScheduleConfig{{Phase: "fingerprint", Priority: 5, WindowStart: "02:00", WindowEnd: "03:00"}},
	}
	n := cfg.Normalize()
	if n.Timezone != "UTC" {
		t.Fatalf("timezone not trimmed: %q", n.Timezone)
	}
	if len(n.Phases) != len(SchedulablePhases) {
		t.Fatalf("expected %d phases, got %d", len(SchedulablePhases), len(n.Phases))
	}
	for i, phase := range SchedulablePhases {
		if n.Phases[i].Phase != phase {
			t.Fatalf("phase %d = %s, want %s", i, n.Phases[i].Phase, phase)
		}
	}
	if n.Phases[4].Priority != 5 {
		t.Fatalf("given fingerprint priority lost: %+v", n.Phases[4])
	}
	if n.Phases[1].Priority != defaultPhasePriorities["thumbnail"] {
		t.Fatalf("missing thumbnail did not get its default: %+v", n.Phases[1])
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("normalized config invalid: %v", err)
	}
}
