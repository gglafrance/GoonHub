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

// Integration test of migration 000083 (library health tables): up, down, up. It migrates the
// database down and up again, so it needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration077LibraryHealth(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	const version = 77
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	previous, err := source.Prev(version)
	if err != nil {
		t.Fatalf("version before %d: %v", version, err)
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
		t.Helper()
		var exists bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "public."+name).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		return exists
	}

	if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	for _, table := range []string{"library_health_checks", "library_health_check_items"} {
		if !tableExists(table) {
			t.Fatalf("table %s missing after up", table)
		}
	}

	// The constraints reject unknown kinds and reasons
	var sceneID, checkID int64
	if err := db.QueryRow(`INSERT INTO scenes (title) VALUES ('migration 077 test') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID) })
	if _, err := db.Exec(`INSERT INTO library_health_checks (kind) VALUES ('delete_files')`); err == nil {
		t.Fatal("an unknown kind was accepted")
	}
	if err := db.QueryRow(`INSERT INTO library_health_checks (kind) VALUES ('missing_files') RETURNING id`).Scan(&checkID); err != nil {
		t.Fatalf("insert check: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO library_health_check_items (check_id, scene_id, stored_path, reason) VALUES ($1, $2, '/x', 'gone')`, checkID, sceneID); err == nil {
		t.Fatal("an unknown reason was accepted")
	}
	if _, err := db.Exec(`INSERT INTO library_health_check_items (check_id, scene_id, stored_path, reason) VALUES ($1, $2, '/x', 'missing')`, checkID, sceneID); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	// Deleting the scene for good drops its items
	if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID); err != nil {
		t.Fatalf("delete scene: %v", err)
	}
	var items int
	if err := db.QueryRow(`SELECT COUNT(*) FROM library_health_check_items WHERE check_id = $1`, checkID).Scan(&items); err != nil || items != 0 {
		t.Fatalf("items after scene deletion: %d (err %v)", items, err)
	}

	if err := m.Migrate(previous); err != nil {
		t.Fatalf("migrate down to %d: %v", previous, err)
	}
	for _, table := range []string{"library_health_checks", "library_health_check_items"} {
		if tableExists(table) {
			t.Fatalf("table %s still there after down", table)
		}
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if !tableExists("library_health_checks") {
		t.Fatal("table library_health_checks missing after up again")
	}
}
