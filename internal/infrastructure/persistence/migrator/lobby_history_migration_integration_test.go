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

// lobbyHistoryMigration is the version of the Goon2gether history migration (lobby_history,
// the play of the current item on lobbies).
const lobbyHistoryMigration = 107

// Integration test of the Goon2gether history migration: up, down, up again. It needs a
// disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationLobbyHistory(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(lobbyHistoryMigration)
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

	columns := [][2]string{
		{"lobbies", "play_started_at"}, {"lobbies", "play_completed"},
		{"lobby_history", "outcome"}, {"lobby_history", "added_by_anon"},
	}
	has := func(table, column string) bool {
		var ok bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`, table, column).Scan(&ok)
		if err != nil {
			t.Fatalf("check %s.%s: %v", table, column, err)
		}
		return ok
	}
	step := func(v uint) {
		if err := m.Migrate(v); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", v, err)
		}
	}
	expect := func(want bool, when string) {
		for _, c := range columns {
			if has(c[0], c[1]) != want {
				t.Fatalf("%s.%s present=%v %s", c[0], c[1], !want, when)
			}
		}
	}

	step(prev)
	expect(false, "before the migration")

	// A lobby created before the migration: no play recorded
	var userID, lobbyID, sceneID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('g2g-history-migration', 'x', 'admin') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID) })
	if err := db.QueryRow(`INSERT INTO lobbies (uuid, invite_code, name, host_user_id) VALUES (gen_random_uuid(), 'HISTRY01', 'm', $1) RETURNING id`, userID).Scan(&lobbyID); err != nil {
		t.Fatalf("insert lobby: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM lobbies WHERE id = $1`, lobbyID) })
	if err := db.QueryRow(`INSERT INTO scenes (title, stored_path) VALUES ('history migration', '/migration-test/history.mp4') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID) })

	step(lobbyHistoryMigration)
	expect(true, "after the migration")
	var started sql.NullTime
	var completed bool
	if err := db.QueryRow(`SELECT play_started_at, play_completed FROM lobbies WHERE id = $1`, lobbyID).Scan(&started, &completed); err != nil || started.Valid || completed {
		t.Fatalf("default play: %v %v %v", started, completed, err)
	}
	insert := `INSERT INTO lobby_history (lobby_id, scene_id, start_s, end_s, added_by, added_by_anon, started_at, ended_at, outcome)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW(), $7)`
	if _, err := db.Exec(insert, lobbyID, sceneID, 5.0, 20.0, userID, 1, "completed"); err != nil {
		t.Fatalf("insert history: %v", err)
	}
	for _, bad := range [][]any{
		{lobbyID, sceneID, nil, nil, userID, nil, "watched"},  // unknown outcome
		{lobbyID, sceneID, 20.0, 5.0, userID, nil, "skipped"}, // end before start
		{lobbyID, sceneID, nil, nil, userID, 0, "skipped"},    // anonymous number 0
	} {
		if _, err := db.Exec(insert, bad...); err == nil {
			t.Fatalf("invalid history row accepted: %v", bad)
		}
	}
	// Deleted with the lobby
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lobby_history WHERE lobby_id = $1`, lobbyID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("history rows: %d %v", n, err)
	}
	if _, err := db.Exec(`DELETE FROM lobbies WHERE id = $1`, lobbyID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM lobby_history WHERE lobby_id = $1`, lobbyID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("history rows left after the lobby deletion: %d %v", n, err)
	}

	step(prev)
	expect(false, "after the down migration")
	step(lobbyHistoryMigration)
	expect(true, "after migrating up again")
}
