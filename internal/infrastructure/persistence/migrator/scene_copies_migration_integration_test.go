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

// Integration test of migration 000091: the scene_copies table keeps one row per path,
// follows the deletion of its scene, scan_history gets copies_found (default 0), and both go
// away on down. Needs a disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration091SceneCopies(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	const version = 91
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
	// A scan recorded before the migration
	var scanID int64
	if err := db.QueryRow(`INSERT INTO scan_history (status) VALUES ('completed') RETURNING id`).Scan(&scanID); err != nil {
		t.Fatalf("insert scan: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scan_history WHERE id = $1`, scanID) })
	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}

	var copiesFound int
	if err := db.QueryRow(`SELECT copies_found FROM scan_history WHERE id = $1`, scanID).Scan(&copiesFound); err != nil || copiesFound != 0 {
		t.Fatalf("copies_found of an existing scan = %d (%v), want 0", copiesFound, err)
	}

	var ids [2]int64
	for i := range ids {
		if err := db.QueryRow(`INSERT INTO scenes (title, duration) VALUES ('migration 091', 600) RETURNING id`).Scan(&ids[i]); err != nil {
			t.Fatalf("insert scene: %v", err)
		}
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id IN ($1, $2)`, ids[0], ids[1]) })

	if _, err := db.Exec(`INSERT INTO scene_copies (scene_id, stored_path, size, oshash) VALUES ($1, '/m091/a.mp4', 10, '00000000000000aa'), ($2, '/m091/b.mp4', 10, '00000000000000aa')`, ids[0], ids[1]); err != nil {
		t.Fatalf("insert copies: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO scene_copies (scene_id, stored_path, size, oshash) VALUES ($1, '/m091/a.mp4', 10, '00000000000000aa')`, ids[1]); err == nil {
		t.Fatal("a path must be recorded once")
	}
	var createdSet, seenSet bool
	if err := db.QueryRow(`SELECT created_at IS NOT NULL, last_seen_at IS NOT NULL FROM scene_copies WHERE stored_path = '/m091/a.mp4'`).Scan(&createdSet, &seenSet); err != nil || !createdSet || !seenSet {
		t.Fatalf("timestamps not set (%v)", err)
	}
	// Deleting a scene removes its copy rows only
	if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, ids[0]); err != nil {
		t.Fatalf("delete scene: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM scene_copies WHERE stored_path LIKE '/m091/%'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected 1 copy left, got %d (%v)", count, err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('scene_copies') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("table still there after down (%v)", err)
	}
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'scan_history' AND column_name = 'copies_found')`).Scan(&exists); err != nil || exists {
		t.Fatalf("copies_found still there after down (%v)", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up again: %v", err)
	}
	if err := db.QueryRow(`SELECT to_regclass('scene_copies') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("table missing after up again (%v)", err)
	}
}
