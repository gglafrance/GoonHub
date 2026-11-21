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

// Integration test of migration 000084 (user_settings.show_scrubber): existing settings rows get
// the scrubber off, the column goes away on down and comes back on up. It migrates the database down and up again, so it needs a disposable database of its
// own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration084ShowScrubber(t *testing.T) {
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

	const version = 84
	// the migration before (numbers are not contiguous while features are merged)
	prev, err := source.Prev(version)
	if err != nil {
		t.Fatalf("previous migration of %d: %v", version, err)
	}
	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}

	var userID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('migration-084', 'x', 'user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Logf("cleanup user %d: %v", userID, err)
		}
	})
	if _, err := db.Exec(`INSERT INTO user_settings (user_id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("insert settings: %v", err)
	}

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	var show bool
	if err := db.QueryRow(`SELECT show_scrubber FROM user_settings WHERE user_id = $1`, userID).Scan(&show); err != nil {
		t.Fatalf("select: %v", err)
	}
	if show {
		t.Fatal("expected show_scrubber false on an existing row")
	}
	// turned on, then dropped by down: up again gives the default back
	if _, err := db.Exec(`UPDATE user_settings SET show_scrubber = true WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'user_settings' AND column_name = 'show_scrubber'`).Scan(&count); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if count != 0 {
		t.Fatal("show_scrubber still exists after down")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if err := db.QueryRow(`SELECT show_scrubber FROM user_settings WHERE user_id = $1`, userID).Scan(&show); err != nil {
		t.Fatalf("select after up: %v", err)
	}
	if show {
		t.Fatal("expected show_scrubber false after down and up")
	}
}
