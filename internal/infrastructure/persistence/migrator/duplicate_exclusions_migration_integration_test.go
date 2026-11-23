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

// Integration test of migration 000090: the duplicate_exclusions table stores ordered pairs
// once, follows the deletion of its scenes, and goes away on down. Needs a disposable database
// of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration090DuplicateExclusions(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	const version = 90
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
	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}

	var ids [3]int64
	for i := range ids {
		if err := db.QueryRow(`INSERT INTO scenes (title, duration) VALUES ('migration 090', 600) RETURNING id`).Scan(&ids[i]); err != nil {
			t.Fatalf("insert scene: %v", err)
		}
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id IN ($1, $2, $3)`, ids[0], ids[1], ids[2]) })

	if _, err := db.Exec(`INSERT INTO duplicate_exclusions (scene_id_a, scene_id_b) VALUES ($1, $2), ($1, $3)`, ids[0], ids[1], ids[2]); err != nil {
		t.Fatalf("insert pairs: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO duplicate_exclusions (scene_id_a, scene_id_b) VALUES ($1, $2)`, ids[0], ids[1]); err == nil {
		t.Fatal("a pair must be stored once")
	}
	if _, err := db.Exec(`INSERT INTO duplicate_exclusions (scene_id_a, scene_id_b) VALUES ($1, $2)`, ids[2], ids[1]); err == nil {
		t.Fatal("an unordered pair must be rejected")
	}
	var createdSet bool
	if err := db.QueryRow(`SELECT created_at IS NOT NULL FROM duplicate_exclusions WHERE scene_id_a = $1 AND scene_id_b = $2`, ids[0], ids[1]).Scan(&createdSet); err != nil || !createdSet {
		t.Fatalf("created_at not set (%v)", err)
	}
	// Deleting a scene removes its pairs
	if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, ids[2]); err != nil {
		t.Fatalf("delete scene: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM duplicate_exclusions WHERE scene_id_a = $1`, ids[0]).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected 1 pair left, got %d (%v)", count, err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('duplicate_exclusions') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("table still there after down (%v)", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up again: %v", err)
	}
	if err := db.QueryRow(`SELECT to_regclass('duplicate_exclusions') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("table missing after up again (%v)", err)
	}
}
