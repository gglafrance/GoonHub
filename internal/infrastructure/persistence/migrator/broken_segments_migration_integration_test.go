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

// Integration test of migration 000089: the "0 to one fingerprint step" segments written by
// the bug are cleared (relation back to unknown), real segments are kept. Needs a disposable
// database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration089ClearBrokenDuplicateSegments(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	const version = 89
	prev, err := source.Prev(version)
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

	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}

	var ids [4]int64
	for i := range ids {
		if err := db.QueryRow(`INSERT INTO scenes (title, duration) VALUES ('migration 089', 600) RETURNING id`).Scan(&ids[i]); err != nil {
			t.Fatalf("insert scene: %v", err)
		}
	}
	var groupID int64
	if err := db.QueryRow(`INSERT INTO duplicate_groups (status, scene_count) VALUES ('unresolved', 4) RETURNING id`).Scan(&groupID); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM duplicate_groups WHERE id = $1`, groupID)
		_, _ = db.Exec(`DELETE FROM scenes WHERE id IN ($1, $2, $3, $4)`, ids[0], ids[1], ids[2], ids[3])
	})
	// visual bug, audio bug, a real segment starting at 0, a real segment elsewhere
	if _, err := db.Exec(`INSERT INTO duplicate_group_members
		(group_id, scene_id, match_type, relation, match_start, match_end, ref_scene_id, ref_start, ref_end) VALUES
		($1, $2, 'visual', 'partial', 0, 2, $3, 0, 2),
		($1, $3, 'audio', 'partial', 0, 0.12383976, $2, 0, 0.12383976),
		($1, $4, 'visual', 'identical', 0, 598, $5, 0, 598),
		($1, $5, 'audio', 'contained', 120, 480, $4, 0, 360)`,
		groupID, ids[0], ids[1], ids[2], ids[3]); err != nil {
		t.Fatalf("insert members: %v", err)
	}

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}
	type row struct {
		start    sql.NullFloat64
		relation string
	}
	got := map[int64]row{}
	rows, err := db.Query(`SELECT scene_id, match_start, relation FROM duplicate_group_members WHERE group_id = $1`, groupID)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	for rows.Next() {
		var id int64
		var r row
		if err := rows.Scan(&id, &r.start, &r.relation); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = r
	}
	rows.Close()
	for _, id := range ids[:2] {
		if got[id].start.Valid || got[id].relation != "unknown" {
			t.Fatalf("scene %d: broken segment not cleared: %+v", id, got[id])
		}
	}
	if !got[ids[2]].start.Valid || got[ids[2]].relation != "identical" {
		t.Fatalf("real segment starting at 0 was cleared: %+v", got[ids[2]])
	}
	if !got[ids[3]].start.Valid || got[ids[3]].start.Float64 != 120 || got[ids[3]].relation != "contained" {
		t.Fatalf("real segment was changed: %+v", got[ids[3]])
	}

	// down (no-op) then up again
	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up again: %v", err)
	}
}
