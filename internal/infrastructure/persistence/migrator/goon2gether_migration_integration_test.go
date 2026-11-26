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

// goon2getherMigration is the version of the Goon2gether migration (lobbies, emotes).
const goon2getherMigration = 98

// Integration test of the Goon2gether migration: up, down, up again. It needs a disposable
// database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationGoon2gether(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(goon2getherMigration)
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

	tables := []string{"lobbies", "lobby_members", "lobby_queue_items", "emotes"}
	exists := func(table string) bool {
		var ok bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&ok); err != nil {
			t.Fatalf("check %s: %v", table, err)
		}
		return ok
	}
	step := func(v uint) {
		if err := m.Migrate(v); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", v, err)
		}
	}

	step(prev)
	for _, tb := range tables {
		if exists(tb) {
			t.Fatalf("%s exists before the migration", tb)
		}
	}
	step(goon2getherMigration)
	for _, tb := range tables {
		if !exists(tb) {
			t.Fatalf("%s missing after the migration", tb)
		}
	}

	// A lobby with a host, a queue item set as current; deleting the item clears it
	var userID, sceneID, lobbyID, itemID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('g2g-migration', 'x', 'admin') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID) })
	if err := db.QueryRow(`INSERT INTO scenes (title) VALUES ('g2g migration') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID) })
	if err := db.QueryRow(`INSERT INTO lobbies (uuid, invite_code, name, host_user_id) VALUES (gen_random_uuid(), 'MIGRTEST', 'm', $1) RETURNING id`, userID).Scan(&lobbyID); err != nil {
		t.Fatalf("insert lobby: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lobby_queue_items (lobby_id, scene_id) VALUES ($1, $2) RETURNING id`, lobbyID, sceneID).Scan(&itemID); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := db.Exec(`UPDATE lobbies SET current_item_id = $1 WHERE id = $2`, itemID, lobbyID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO lobby_queue_items (lobby_id, scene_id, start_s, end_s) VALUES ($1, $2, 10, 5)`, lobbyID, sceneID); err == nil {
		t.Fatal("an end before the start was accepted")
	}
	if _, err := db.Exec(`INSERT INTO lobbies (uuid, invite_code, name, visibility) VALUES (gen_random_uuid(), 'MIGRTES2', 'm', 'hidden')`); err == nil {
		t.Fatal("an unknown visibility was accepted")
	}
	if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID); err != nil {
		t.Fatal(err)
	}
	var current sql.NullInt64
	if err := db.QueryRow(`SELECT current_item_id FROM lobbies WHERE id = $1`, lobbyID).Scan(&current); err != nil || current.Valid {
		t.Fatalf("current item not cleared with its scene: %v %v", current, err)
	}

	step(prev)
	for _, tb := range tables {
		if exists(tb) {
			t.Fatalf("%s left after the down migration", tb)
		}
	}
	step(goon2getherMigration)
	for _, tb := range tables {
		if !exists(tb) {
			t.Fatalf("%s missing after migrating up again", tb)
		}
	}
}
