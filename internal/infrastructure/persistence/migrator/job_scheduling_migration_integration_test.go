package migrator

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

// Integration test of migration 000077 (job scheduling tables): defaults that keep the previous
// behaviour, constraints, and down/up. It migrates the database down and up again, so it needs
// a disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration077JobScheduling(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	defer m.Close()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tableExists := func(name string) bool {
		var exists bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists); err != nil {
			t.Fatalf("to_regclass: %v", err)
		}
		return exists
	}

	if err := m.Migrate(74); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to 74: %v", err)
	}
	if tableExists("job_schedule_config") || tableExists("job_phase_schedules") {
		t.Fatal("tables exist before migration 077")
	}

	if err := m.Migrate(77); err != nil {
		t.Fatalf("migrate to 77: %v", err)
	}
	var tz, qs, qe string
	var maxJobs, windowHours int
	var newFirst bool
	if err := db.QueryRow(`SELECT timezone, quiet_hours_start, quiet_hours_end, max_concurrent_jobs, new_scenes_first, new_scenes_window_hours
		FROM job_schedule_config WHERE id = 1`).Scan(&tz, &qs, &qe, &maxJobs, &newFirst, &windowHours); err != nil {
		t.Fatalf("select config: %v", err)
	}
	if tz != "UTC" || qs != "01:00" || qe != "07:00" || maxJobs != 0 || newFirst || windowHours != 24 {
		t.Fatalf("unexpected defaults: %s %s %s %d %v %d", tz, qs, qe, maxJobs, newFirst, windowHours)
	}
	var phases, windowed, overridden int
	if err := db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE window_enabled), COUNT(*) FILTER (WHERE quiet_workers > 0)
		FROM job_phase_schedules`).Scan(&phases, &windowed, &overridden); err != nil {
		t.Fatalf("select phases: %v", err)
	}
	if phases != 5 || windowed != 0 || overridden != 0 {
		t.Fatalf("expected 5 phases, no window, no override; got %d, %d, %d", phases, windowed, overridden)
	}

	for _, stmt := range []string{
		`INSERT INTO job_schedule_config (id) VALUES (2)`,
		`UPDATE job_schedule_config SET quiet_hours_start = '24:00'`,
		`UPDATE job_schedule_config SET max_concurrent_jobs = -1`,
		`UPDATE job_phase_schedules SET window_end = window_start WHERE phase = 'sprites'`,
		`UPDATE job_phase_schedules SET quiet_workers = 33 WHERE phase = 'sprites'`,
		`INSERT INTO job_phase_schedules (phase) VALUES ('scan')`,
	} {
		if _, err := db.Exec(stmt); err == nil {
			t.Fatalf("expected a constraint violation for %q", stmt)
		}
	}

	if err := m.Migrate(74); err != nil {
		t.Fatalf("migrate down to 74: %v", err)
	}
	if tableExists("job_schedule_config") || tableExists("job_phase_schedules") {
		t.Fatal("tables still exist after migrating down")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if !tableExists("job_schedule_config") || !tableExists("job_phase_schedules") {
		t.Fatal("tables missing after migrating up again")
	}
}
