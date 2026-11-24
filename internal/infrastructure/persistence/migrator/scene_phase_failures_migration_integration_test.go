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

// Integration test of migration 000092: the scene_phase_failures table keeps one counter per
// (scene, phase) and follows the deletion of its scene, job_history gets startup_requeues
// (default 0) and its (scene_id, phase, created_at) index, and all go away on down (up, down,
// up). Needs a disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration092ScenePhaseFailures(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	const version = 92
	prev, err := source.Prev(version)
	if err != nil {
		t.Fatalf("previous migration: %v", err)
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

	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}
	var ids [2]int64
	for i := range ids {
		if err := db.QueryRow(`INSERT INTO scenes (title, duration) VALUES ('migration 092', 600) RETURNING id`).Scan(&ids[i]); err != nil {
			t.Fatalf("insert scene: %v", err)
		}
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id IN ($1, $2)`, ids[0], ids[1]) })
	// A job recorded before the migration
	if _, err := db.Exec(`INSERT INTO job_history (job_id, scene_id, phase, status) VALUES ('m092-job', $1, 'metadata', 'failed')`, ids[0]); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM job_history WHERE job_id = 'm092-job'`) })

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}
	var requeues int
	if err := db.QueryRow(`SELECT startup_requeues FROM job_history WHERE job_id = 'm092-job'`).Scan(&requeues); err != nil || requeues != 0 {
		t.Fatalf("startup_requeues of an existing job = %d (%v), want 0", requeues, err)
	}
	var indexed bool
	if err := db.QueryRow(`SELECT to_regclass('idx_job_history_scene_phase_created') IS NOT NULL`).Scan(&indexed); err != nil || !indexed {
		t.Fatalf("job_history index missing (%v)", err)
	}

	if _, err := db.Exec(`INSERT INTO scene_phase_failures (scene_id, phase, failures, last_error, file_size) VALUES ($1, 'sprites', 3, 'ffmpeg failed', 10), ($2, 'sprites', 1, '', NULL)`, ids[0], ids[1]); err != nil {
		t.Fatalf("insert failures: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO scene_phase_failures (scene_id, phase) VALUES ($1, 'sprites')`, ids[0]); err == nil {
		t.Fatal("a (scene, phase) must have one counter")
	}
	var lastFailedSet bool
	if err := db.QueryRow(`SELECT last_failed_at IS NOT NULL FROM scene_phase_failures WHERE scene_id = $1`, ids[1]).Scan(&lastFailedSet); err != nil || !lastFailedSet {
		t.Fatalf("last_failed_at default not set (%v)", err)
	}
	// Deleting a scene removes its counters only
	if _, err := db.Exec(`DELETE FROM job_history WHERE job_id = 'm092-job'`); err != nil {
		t.Fatalf("delete job: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, ids[0]); err != nil {
		t.Fatalf("delete scene: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM scene_phase_failures WHERE scene_id IN ($1, $2)`, ids[0], ids[1]).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected 1 counter left, got %d (%v)", count, err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('scene_phase_failures') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("table still there after down (%v)", err)
	}
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'job_history' AND column_name = 'startup_requeues')`).Scan(&exists); err != nil || exists {
		t.Fatalf("startup_requeues still there after down (%v)", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up again: %v", err)
	}
	if err := db.QueryRow(`SELECT to_regclass('scene_phase_failures') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("table missing after up again (%v)", err)
	}
}
