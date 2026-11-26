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

// messageReactionsMigration is the version of the message reactions migration
// (docs/MESSAGES.md, "Reactions").
const messageReactionsMigration = 104

// Integration test of the message reactions migration: up, down, up again, the unique triple,
// the key check and the cascades. It needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationMessageReactions(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(messageReactionsMigration)
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
		if err := db.QueryRow(`SELECT to_regclass('public.message_reactions') IS NOT NULL`).Scan(&ok); err != nil {
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
		t.Fatal("message_reactions exists before the migration")
	}
	step(messageReactionsMigration)
	if !hasTable() {
		t.Fatal("message_reactions missing after the migration")
	}

	var a, b, convID, msgID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('reactions-migration-a', 'x', 'user') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('reactions-migration-b', 'x', 'user') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, a, b) })
	low, high := a, b
	if low > high {
		low, high = high, low
	}
	if err := db.QueryRow(`INSERT INTO conversations (direct_low_id, direct_high_id) VALUES ($1, $2) RETURNING id`, low, high).Scan(&convID); err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM conversations WHERE id = $1`, convID) })
	if err := db.QueryRow(`INSERT INTO messages (conversation_id, sender_id, body) VALUES ($1, $2, 'hi') RETURNING id`, convID, a).Scan(&msgID); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1, $2, '🔥'), ($1, $3, '🔥'), ($1, $2, ':pog:')`, msgID, a, b); err != nil {
		t.Fatalf("insert reactions: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1, $2, '🔥')`, msgID, a); err == nil {
		t.Fatal("a duplicate reaction was accepted")
	}
	if _, err := db.Exec(`INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1, $2, '')`, msgID, a); err == nil {
		t.Fatal("an empty key was accepted")
	}
	count := func() int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM message_reactions WHERE message_id = $1`, msgID).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	// a deleted user's reactions go with the account
	if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, b); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if n := count(); n != 2 {
		t.Fatalf("reactions after a user deletion = %d", n)
	}

	step(prev)
	if hasTable() {
		t.Fatal("message_reactions kept by the down migration")
	}
	step(messageReactionsMigration)
	if !hasTable() {
		t.Fatal("message_reactions missing after migrating up again")
	}
	// a hard deleted message takes its reactions with it
	if _, err := db.Exec(`INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1, $2, '🔥')`, msgID, a); err != nil {
		t.Fatalf("insert after up again: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM messages WHERE id = $1`, msgID); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if n := count(); n != 0 {
		t.Fatalf("reactions after a message deletion = %d", n)
	}
}
