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

// navLayoutsMigration is the version of the header layouts migration (docs/NAVIGATION.md).
const navLayoutsMigration = 106

// Integration test of the header layouts migration: up, down, up again, one row per user, the
// object check and the account deletion cascade. It needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationNavLayouts(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(navLayoutsMigration)
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

	hasTable := func() bool {
		var ok bool
		if err := db.QueryRow(`SELECT to_regclass('public.user_nav_layouts') IS NOT NULL`).Scan(&ok); err != nil {
			t.Fatalf("check table: %v", err)
		}
		return ok
	}
	step := func(v uint) {
		if err := m.Migrate(v); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", v, err)
		}
	}

	step(prev)
	if hasTable() {
		t.Fatal("user_nav_layouts exists before the migration")
	}
	step(navLayoutsMigration)
	if !hasTable() {
		t.Fatal("user_nav_layouts missing after the migration")
	}

	var userID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('nav-layout-migration', 'x', 'user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID) })

	if _, err := db.Exec(`INSERT INTO user_nav_layouts (user_id, layout) VALUES ($1, '{"version": 1}')`, userID); err != nil {
		t.Fatalf("insert layout: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_nav_layouts (user_id, layout) VALUES ($1, '{"version": 1}')`, userID); err == nil {
		t.Fatal("a second layout for the same user was accepted")
	}
	if _, err := db.Exec(`UPDATE user_nav_layouts SET layout = '[1, 2]' WHERE user_id = $1`, userID); err == nil {
		t.Fatal("a layout that is not an object was accepted")
	}

	step(prev)
	if hasTable() {
		t.Fatal("user_nav_layouts kept by the down migration")
	}
	step(navLayoutsMigration)
	if !hasTable() {
		t.Fatal("user_nav_layouts missing after migrating up again")
	}

	// a deleted account takes its layout with it
	if _, err := db.Exec(`INSERT INTO user_nav_layouts (user_id, layout) VALUES ($1, '{"version": 1}')`, userID); err != nil {
		t.Fatalf("insert after up again: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_nav_layouts WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("layouts after the account deletion = %d", n)
	}
}
