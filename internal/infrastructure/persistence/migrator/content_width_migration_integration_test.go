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

// Integration test of migration 000087 (user_settings.content_width): existing settings rows get
// the default width, the CHECK constraint rejects other values, the column goes away on down and
// comes back with the default on up. It migrates the database down and up again, so it needs a
// disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration087ContentWidth(t *testing.T) {
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

	const version = 87
	// the migration before (numbers are not contiguous while features are merged)
	prev, err := source.Prev(version)
	if err != nil {
		t.Fatalf("previous migration of %d: %v", version, err)
	}
	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}

	var userID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('migration-087', 'x', 'user') RETURNING id`).Scan(&userID); err != nil {
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

	width := func() string {
		t.Helper()
		var v string
		if err := db.QueryRow(`SELECT content_width FROM user_settings WHERE user_id = $1`, userID).Scan(&v); err != nil {
			t.Fatalf("select: %v", err)
		}
		return v
	}

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	if got := width(); got != "default" {
		t.Fatalf("expected the default width on an existing row, got %q", got)
	}
	if _, err := db.Exec(`UPDATE user_settings SET content_width = 'full' WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("update to full: %v", err)
	}
	if got := width(); got != "full" {
		t.Fatalf("expected full, got %q", got)
	}
	for _, bad := range []string{"huge", "", "Wide"} {
		if _, err := db.Exec(`UPDATE user_settings SET content_width = $2 WHERE user_id = $1`, userID, bad); err == nil {
			t.Fatalf("expected the CHECK constraint to reject %q", bad)
		}
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'user_settings' AND column_name = 'content_width'`).Scan(&count); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if count != 0 {
		t.Fatal("content_width still exists after down")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if got := width(); got != "default" {
		t.Fatalf("expected default after down and up, got %q", got)
	}
}
