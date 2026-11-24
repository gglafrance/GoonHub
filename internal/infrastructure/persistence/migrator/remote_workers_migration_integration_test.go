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

// Integration test of migration 000088 (remote workers): existing job rows keep their data with
// lease_generation 0 and no worker, the workers and worker_settings tables exist with the
// singleton row, and the migration goes down and up again. It needs a disposable database of
// its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration088RemoteWorkers(t *testing.T) {
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

	const version = 88
	prev, err := source.Prev(version)
	if err != nil {
		t.Fatalf("previous migration of %d: %v", version, err)
	}
	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}
	var jobRow int64
	if err := db.QueryRow(`INSERT INTO job_history (job_id, scene_id, phase, status) VALUES ('mig-088-job', 1, 'sprites', 'pending') RETURNING id`).Scan(&jobRow); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM job_history WHERE id = $1`, jobRow) })

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	var gen int64
	var worker sql.NullInt64
	if err := db.QueryRow(`SELECT lease_generation, worker_id FROM job_history WHERE id = $1`, jobRow).Scan(&gen, &worker); err != nil {
		t.Fatalf("select job: %v", err)
	}
	if gen != 0 || worker.Valid {
		t.Fatalf("existing job got generation %d worker %v", gen, worker)
	}
	var settings int
	if err := db.QueryRow(`SELECT count(*) FROM worker_settings WHERE id = 1`).Scan(&settings); err != nil || settings != 1 {
		t.Fatalf("worker settings singleton: %d %v", settings, err)
	}
	var workerID int64
	if err := db.QueryRow(`INSERT INTO workers (name, token_hash) VALUES ('mig-088', 'h') RETURNING id`).Scan(&workerID); err != nil {
		t.Fatalf("insert worker: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO workers (name, token_hash) VALUES ('mig-088-dup', 'h')`); err == nil {
		t.Fatal("two workers with the same token hash")
	}
	if _, err := db.Exec(`INSERT INTO workers (name, connection_mode) VALUES ('mig-088-bad', 'carrier-pigeon')`); err == nil {
		t.Fatal("unknown connection mode accepted")
	}
	if _, err := db.Exec(`UPDATE job_history SET worker_id = $1 WHERE id = $2`, workerID, jobRow); err != nil {
		t.Fatalf("lease to worker: %v", err)
	}
	// Deleting a worker keeps the job history.
	if _, err := db.Exec(`DELETE FROM workers WHERE id = $1`, workerID); err != nil {
		t.Fatalf("delete worker: %v", err)
	}
	if err := db.QueryRow(`SELECT worker_id FROM job_history WHERE id = $1`, jobRow).Scan(&worker); err != nil || worker.Valid {
		t.Fatalf("job after the worker deletion: %v %v", worker, err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM job_history WHERE id = $1`, jobRow).Scan(&settings); err != nil || settings != 1 {
		t.Fatal("job lost by the down migration")
	}
	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up again: %v", err)
	}
}
