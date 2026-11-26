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

// profileTabsListsMigration is the version of the profile tabs and custom lists migration
// (docs/PROFILES.md).
const profileTabsListsMigration = 103

// Integration test of the profile tabs and custom lists migration: up, down, up again, with its
// checks. It needs a disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationProfileTabsLists(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(profileTabsListsMigration)
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

	state := func() (table, column bool) {
		if err := db.QueryRow(`SELECT to_regclass('public.user_profile_lists') IS NOT NULL,
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'user_profiles' AND column_name = 'tabs')`).Scan(&table, &column); err != nil {
			t.Fatalf("state: %v", err)
		}
		return
	}
	step := func(v uint) {
		if err := m.Migrate(v); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", v, err)
		}
	}

	step(profileTabsListsMigration)
	if table, column := state(); !table || !column {
		t.Fatalf("after up: table %v, column %v", table, column)
	}
	var userID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('mig103', 'x', 'user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID) })
	if _, err := db.Exec(`INSERT INTO user_profile_lists (user_id, title) VALUES ($1, 'Top O')`, userID); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	for name, q := range map[string]string{
		"bad audience": `INSERT INTO user_profile_lists (user_id, title, audience) VALUES ($1, 'x', 'world')`,
		"empty title":  `INSERT INTO user_profile_lists (user_id, title) VALUES ($1, '')`,
		"array filter": `INSERT INTO user_profile_lists (user_id, title, filters) VALUES ($1, 'x', '[]')`,
	} {
		if _, err := db.Exec(q, userID); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if _, err := db.Exec(`INSERT INTO user_profiles (user_id, tabs) VALUES ($1, '{}')`, userID); err == nil {
		t.Fatal("tabs object accepted")
	}

	step(prev)
	if table, column := state(); table || column {
		t.Fatalf("after down: table %v, column %v", table, column)
	}
	step(profileTabsListsMigration)
	if table, column := state(); !table || !column {
		t.Fatalf("after up again: table %v, column %v", table, column)
	}
}
