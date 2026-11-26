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

// goon2getherPolishMigration is the version of the Goon2gether polish migration (auto-next
// mode, anonymous members).
const goon2getherPolishMigration = 101

// Integration test of the Goon2gether polish migration: up, down, up again. It needs a
// disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationGoon2getherPolish(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(goon2getherPolishMigration)
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
		{"lobbies", "auto_next"}, {"lobbies", "anon_seq"},
		{"lobby_members", "anonymous"}, {"lobby_members", "anon_number"},
		{"lobby_queue_items", "added_by_anon"},
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

	// A lobby created before the migration gets the default mode
	var userID, lobbyID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('g2g-polish-migration', 'x', 'admin') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID) })
	if err := db.QueryRow(`INSERT INTO lobbies (uuid, invite_code, name, host_user_id) VALUES (gen_random_uuid(), 'POLISH01', 'm', $1) RETURNING id`, userID).Scan(&lobbyID); err != nil {
		t.Fatalf("insert lobby: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM lobbies WHERE id = $1`, lobbyID) })
	if _, err := db.Exec(`INSERT INTO lobby_members (lobby_id, user_id) VALUES ($1, $2)`, lobbyID, userID); err != nil {
		t.Fatalf("insert member: %v", err)
	}

	step(goon2getherPolishMigration)
	expect(true, "after the migration")
	var mode string
	var anon bool
	if err := db.QueryRow(`SELECT auto_next FROM lobbies WHERE id = $1`, lobbyID).Scan(&mode); err != nil || mode != "next" {
		t.Fatalf("default auto_next: %q %v", mode, err)
	}
	if err := db.QueryRow(`SELECT anonymous FROM lobby_members WHERE lobby_id = $1`, lobbyID).Scan(&anon); err != nil || anon {
		t.Fatalf("default anonymous: %v %v", anon, err)
	}
	if _, err := db.Exec(`UPDATE lobbies SET auto_next = 'shuffle' WHERE id = $1`, lobbyID); err == nil {
		t.Fatal("an unknown auto_next mode was accepted")
	}
	if _, err := db.Exec(`UPDATE lobby_members SET anonymous = TRUE WHERE lobby_id = $1`, lobbyID); err == nil {
		t.Fatal("an anonymous member without a number was accepted")
	}
	if _, err := db.Exec(`UPDATE lobby_members SET anonymous = TRUE, anon_number = 1 WHERE lobby_id = $1`, lobbyID); err != nil {
		t.Fatalf("anonymous member: %v", err)
	}

	step(prev)
	expect(false, "after the down migration")
	step(goon2getherPolishMigration)
	expect(true, "after migrating up again")
}
