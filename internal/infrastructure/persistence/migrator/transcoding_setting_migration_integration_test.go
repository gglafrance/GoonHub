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

// Integration test of migration 000076 (app_settings.transcoding_enabled, on by default):
// up, down and up again. It migrates the database down, so it needs a disposable database
// of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration076TranscodingSetting(t *testing.T) {
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
	defer db.Close()

	hasColumn := func() bool {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_name = 'app_settings' AND column_name = 'transcoding_enabled'`).Scan(&n); err != nil {
			t.Fatalf("columns: %v", err)
		}
		return n == 1
	}

	if err := m.Migrate(74); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to 74: %v", err)
	}
	if hasColumn() {
		t.Fatal("column present before 076")
	}
	// An existing settings row gets the default
	if _, err := db.Exec(`INSERT INTO app_settings (id, trash_retention_days) VALUES (1, 7)
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("insert settings: %v", err)
	}

	if err := m.Migrate(76); err != nil {
		t.Fatalf("migrate to 76: %v", err)
	}
	var enabled bool
	if err := db.QueryRow(`SELECT transcoding_enabled FROM app_settings WHERE id = 1`).Scan(&enabled); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !enabled {
		t.Fatal("transcoding must be enabled by default")
	}

	if err := m.Migrate(74); err != nil {
		t.Fatalf("migrate down to 74: %v", err)
	}
	if hasColumn() {
		t.Fatal("column still present after down")
	}
	if err := m.Migrate(76); err != nil {
		t.Fatalf("migrate up again to 76: %v", err)
	}
	if !hasColumn() {
		t.Fatal("column missing after up again")
	}
}
