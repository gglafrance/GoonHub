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

// profileListFiguresMigration is the version of the custom list figures migration
// (docs/PROFILES.md, "Figures").
const profileListFiguresMigration = 105

// Integration test of the custom list figures migration: up, down, up again, the default and
// the check. It needs a disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationProfileListFigures(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(profileListFiguresMigration)
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

	hasColumn := func() bool {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'user_profile_lists' AND column_name = 'figures'`).Scan(&n); err != nil {
			t.Fatalf("check column: %v", err)
		}
		return n == 1
	}
	step := func(v uint) {
		if err := m.Migrate(v); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", v, err)
		}
	}

	step(prev)
	var userID, listID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('list-figures-migration', 'x', 'user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID) })
	// A list made before the migration
	if err := db.QueryRow(`INSERT INTO user_profile_lists (user_id, title) VALUES ($1, 'Top O') RETURNING id`, userID).Scan(&listID); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if hasColumn() {
		t.Fatal("figures exists before the migration")
	}

	step(profileListFiguresMigration)
	var figures string
	if err := db.QueryRow(`SELECT figures FROM user_profile_lists WHERE id = $1`, listID).Scan(&figures); err != nil || figures != "auto" {
		t.Fatalf("existing list: %q %v", figures, err)
	}
	for _, v := range []string{"none", "jizz", "rating", "liked", "views", "auto"} {
		if _, err := db.Exec(`UPDATE user_profile_lists SET figures = $1 WHERE id = $2`, v, listID); err != nil {
			t.Fatalf("figures %q refused: %v", v, err)
		}
	}
	if _, err := db.Exec(`UPDATE user_profile_lists SET figures = 'list' WHERE id = $1`, listID); err == nil {
		t.Fatal("an unknown figures value was accepted")
	}

	step(prev)
	if hasColumn() {
		t.Fatal("figures kept by the down migration")
	}
	step(profileListFiguresMigration)
	if !hasColumn() {
		t.Fatal("figures missing after migrating up again")
	}
	if _, err := db.Exec(`DELETE FROM user_profile_lists WHERE id = $1`, listID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
