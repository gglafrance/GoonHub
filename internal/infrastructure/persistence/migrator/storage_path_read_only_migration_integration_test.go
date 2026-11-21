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

// Integration test of migration 000075 (storage_paths.read_only): existing paths keep
// read_only = false, the column round-trips down and up. It migrates the database down and up
// again, so it needs a disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration075StoragePathReadOnly(t *testing.T) {
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

	if err := m.Migrate(74); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to 74: %v", err)
	}

	var id int64
	if err := db.QueryRow(`INSERT INTO storage_paths (name, path) VALUES ($1, $2) RETURNING id`,
		"migration 075 test", "/migration-075-test").Scan(&id); err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM storage_paths WHERE id = $1`, id); err != nil {
			t.Logf("cleanup storage path %d: %v", id, err)
		}
	})

	if err := m.Migrate(75); err != nil {
		t.Fatalf("migrate to 75: %v", err)
	}
	var readOnly bool
	if err := db.QueryRow(`SELECT read_only FROM storage_paths WHERE id = $1`, id).Scan(&readOnly); err != nil {
		t.Fatalf("select: %v", err)
	}
	if readOnly {
		t.Fatal("existing storage path must not become read-only")
	}
	var defaultRO bool
	if err := db.QueryRow(`SELECT read_only FROM storage_paths WHERE path = './data/videos'`).Scan(&defaultRO); err != nil {
		t.Fatalf("select default path: %v", err)
	}
	if defaultRO {
		t.Fatal("default storage path must not become read-only")
	}
	if _, err := db.Exec(`UPDATE storage_paths SET read_only = true WHERE id = $1`, id); err != nil {
		t.Fatalf("set read_only: %v", err)
	}

	if err := m.Migrate(74); err != nil {
		t.Fatalf("migrate down to 74: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'storage_paths' AND column_name = 'read_only'`).Scan(&count); err != nil {
		t.Fatalf("columns: %v", err)
	}
	if count != 0 {
		t.Fatal("read_only column should be dropped by the down migration")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if err := db.QueryRow(`SELECT read_only FROM storage_paths WHERE id = $1`, id).Scan(&readOnly); err != nil {
		t.Fatalf("select after up: %v", err)
	}
	if readOnly {
		t.Fatal("read_only must default to false after up again")
	}
}
