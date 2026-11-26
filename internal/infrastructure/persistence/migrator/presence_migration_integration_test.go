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

// Integration test of migration 000098 (user_presence): up, down, up again, and the cascade.
// It migrates the database down and up, so it needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration098UserPresence(t *testing.T) {
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

	exists := func() bool {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'user_presence'`).Scan(&n); err != nil {
			t.Fatalf("check: %v", err)
		}
		return n > 0
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("up: %v", err)
	}
	if !exists() {
		t.Fatal("user_presence missing after up")
	}
	if err := m.Steps(-1); err != nil {
		t.Fatalf("down: %v", err)
	}
	if exists() {
		t.Fatal("user_presence still there after down")
	}
	if err := m.Steps(1); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if !exists() {
		t.Fatal("user_presence missing after up again")
	}

	var id int64
	if err := db.QueryRow(`INSERT INTO users (username, password) VALUES ('mig098-a', 'x') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE username LIKE 'mig098-%'`); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	if _, err := db.Exec(`INSERT INTO user_presence (user_id, last_seen_at) VALUES ($1, NOW())`, id); err != nil {
		t.Fatal(err)
	}
	var invisible bool
	if err := db.QueryRow(`SELECT invisible FROM user_presence WHERE user_id = $1`, id).Scan(&invisible); err != nil || invisible {
		t.Fatalf("invisible default: %v %v", invisible, err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_presence WHERE user_id = $1`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("presence not cascaded: %d %v", n, err)
	}
}
