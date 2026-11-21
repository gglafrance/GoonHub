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

// aliasesMigration is the version of the migration that adds studios.aliases.
const aliasesMigration = 83

// Integration test of the migration of studio aliases (and actors.aliases NOT NULL, the merge
// permissions): up, down, up. It needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationActorStudioAliases(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(aliasesMigration)
	if err != nil {
		t.Fatalf("version before %d: %v", aliasesMigration, err)
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
	var actorID, studioID int64
	if err := db.QueryRow(`INSERT INTO actors (uuid, name, aliases) VALUES (gen_random_uuid(), 'migration aliases test', NULL) RETURNING id`).Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO studios (uuid, name) VALUES (gen_random_uuid(), 'migration aliases test') RETURNING id`).Scan(&studioID); err != nil {
		t.Fatalf("insert studio: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM actors WHERE id = $1`, actorID); err != nil {
			t.Logf("cleanup actor: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM studios WHERE id = $1`, studioID); err != nil {
			t.Logf("cleanup studio: %v", err)
		}
	})

	if err := m.Migrate(aliasesMigration); err != nil {
		t.Fatalf("migrate to %d: %v", aliasesMigration, err)
	}
	var actorAliases, studioAliases string
	if err := db.QueryRow(`SELECT aliases::text FROM actors WHERE id = $1`, actorID).Scan(&actorAliases); err != nil {
		t.Fatalf("select actor: %v", err)
	}
	if err := db.QueryRow(`SELECT aliases::text FROM studios WHERE id = $1`, studioID).Scan(&studioAliases); err != nil {
		t.Fatalf("select studio: %v", err)
	}
	if actorAliases != "{}" || studioAliases != "{}" {
		t.Fatalf("aliases after up: actor %q studio %q, want {} and {}", actorAliases, studioAliases)
	}
	if _, err := db.Exec(`UPDATE actors SET aliases = NULL WHERE id = $1`, actorID); err == nil {
		t.Fatal("actors.aliases must be NOT NULL after up")
	}
	var granted int
	if err := db.QueryRow(`SELECT COUNT(*) FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id
		WHERE r.name IN ('admin', 'moderator') AND p.name IN ('actors:merge', 'studios:merge')`).Scan(&granted); err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if granted != 4 {
		t.Fatalf("merge permissions granted %d times, want 4 (2 roles x 2 permissions)", granted)
	}
	var indexes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pg_indexes WHERE indexname IN
		('idx_studios_aliases', 'idx_actors_name_lower', 'idx_studios_name_lower')`).Scan(&indexes); err != nil {
		t.Fatalf("count indexes: %v", err)
	}
	if indexes != 3 {
		t.Fatalf("%d indexes, want 3", indexes)
	}

	// down
	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var hasColumn bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'studios' AND column_name = 'aliases')`).Scan(&hasColumn); err != nil {
		t.Fatalf("check column: %v", err)
	}
	if hasColumn {
		t.Fatal("studios.aliases must be dropped by down")
	}
	var perms int
	if err := db.QueryRow(`SELECT COUNT(*) FROM permissions WHERE name IN ('actors:merge', 'studios:merge')`).Scan(&perms); err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if perms != 0 {
		t.Fatalf("%d merge permissions left after down", perms)
	}
	if _, err := db.Exec(`UPDATE actors SET aliases = NULL WHERE id = $1`, actorID); err != nil {
		t.Fatalf("actors.aliases must be nullable again after down: %v", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
}
