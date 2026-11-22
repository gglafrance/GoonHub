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

// Integration test of migration 000085 (duplicate_group_members.relation): existing members
// start as unknown, the CHECK constraint holds, and down then up again works. It migrates the
// database down and up again, so it needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration085DuplicateMemberRelation(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	// The migration before 085, whatever its number
	const relationVersion = 85
	prev, err := source.Prev(relationVersion)
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
	// registered first, so it runs after the rows are deleted
	t.Cleanup(func() { _ = db.Close() })

	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}

	// A group recorded before relations existed, with a segment on one member
	var clip, comp, groupID int64
	for _, s := range []struct {
		title    string
		duration int
		id       *int64
	}{{"migration 085 clip", 1642, &clip}, {"migration 085 compilation", 5627, &comp}} {
		if err := db.QueryRow(`INSERT INTO scenes (title, duration) VALUES ($1, $2) RETURNING id`, s.title, s.duration).Scan(s.id); err != nil {
			t.Fatalf("insert scene: %v", err)
		}
	}
	if err := db.QueryRow(`INSERT INTO duplicate_groups (status, scene_count) VALUES ('unresolved', 2) RETURNING id`).Scan(&groupID); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM duplicate_groups WHERE id = $1`, groupID); err != nil {
			t.Logf("cleanup group: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM scenes WHERE id IN ($1, $2)`, clip, comp); err != nil {
			t.Logf("cleanup scenes: %v", err)
		}
	})
	if _, err := db.Exec(`INSERT INTO duplicate_group_members (group_id, scene_id, match_start, match_end, ref_scene_id, ref_start, ref_end)
		VALUES ($1, $2, 1.5, 1640, $3, 2101.5, 3740), ($1, $3, NULL, NULL, NULL, NULL, NULL)`, groupID, clip, comp); err != nil {
		t.Fatalf("insert members: %v", err)
	}

	relations := func() map[int64]string {
		rows, err := db.Query(`SELECT scene_id, relation FROM duplicate_group_members WHERE group_id = $1`, groupID)
		if err != nil {
			t.Fatalf("select relations: %v", err)
		}
		defer rows.Close()
		result := map[int64]string{}
		for rows.Next() {
			var id int64
			var relation string
			if err := rows.Scan(&id, &relation); err != nil {
				t.Fatalf("scan: %v", err)
			}
			result[id] = relation
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("rows: %v", err)
		}
		return result
	}

	if err := m.Migrate(relationVersion); err != nil {
		t.Fatalf("migrate to %d: %v", relationVersion, err)
	}
	// Existing rows are unknown until the application classifies them at startup
	if got := relations(); got[clip] != "unknown" || got[comp] != "unknown" {
		t.Fatalf("unexpected relations after up: %v", got)
	}
	if _, err := db.Exec(`UPDATE duplicate_group_members SET relation = 'contained' WHERE group_id = $1 AND scene_id = $2`, groupID, clip); err != nil {
		t.Fatalf("set relation: %v", err)
	}
	if _, err := db.Exec(`UPDATE duplicate_group_members SET relation = 'bogus' WHERE group_id = $1`, groupID); err == nil {
		t.Fatal("expected the CHECK constraint to refuse 'bogus'")
	}
	var indexes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'idx_duplicate_group_members_group_relation'`).Scan(&indexes); err != nil || indexes != 1 {
		t.Fatalf("relation index missing: %d, %v", indexes, err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var columns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'duplicate_group_members' AND column_name = 'relation'`).Scan(&columns); err != nil || columns != 0 {
		t.Fatalf("relation column still present after down: %d, %v", columns, err)
	}
	var matchEnd float64
	if err := db.QueryRow(`SELECT match_end FROM duplicate_group_members WHERE group_id = $1 AND scene_id = $2`, groupID, clip).Scan(&matchEnd); err != nil || matchEnd != 1640 {
		t.Fatalf("segment lost by down: %v, %v", matchEnd, err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if got := relations(); got[clip] != "unknown" || got[comp] != "unknown" {
		t.Fatalf("unexpected relations after up again: %v", got)
	}
}
