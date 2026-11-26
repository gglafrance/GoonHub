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

// friendMessagesMigration is the version of the messages migration (docs/MESSAGES.md).
const friendMessagesMigration = 102

// Integration test of the messages migration: up, down, up again, with the defaults of the
// new settings columns. It needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigrationFriendMessages(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(friendMessagesMigration)
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

	tables := []string{"conversations", "conversation_participants", "messages"}
	hasTable := func(name string) bool {
		var ok bool
		if err := db.QueryRow(`SELECT to_regclass('public.' || $1) IS NOT NULL`, name).Scan(&ok); err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		return ok
	}
	hasColumn := func(table, column string) bool {
		var ok bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`, table, column).Scan(&ok); err != nil {
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
		for _, name := range tables {
			if hasTable(name) != want {
				t.Fatalf("table %s present=%v %s", name, !want, when)
			}
		}
		for _, c := range [][2]string{{"user_social_settings", "message_policy"}, {"user_social_settings", "message_toasts"},
			{"share_links", "start_s"}, {"share_links", "end_s"}, {"share_links", "label"}, {"share_links", "marker_id"}} {
			if hasColumn(c[0], c[1]) != want {
				t.Fatalf("%s.%s present=%v %s", c[0], c[1], !want, when)
			}
		}
	}

	step(prev)
	expect(false, "before the migration")

	// Settings saved before the migration get the defaults
	var userID, otherID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('messages-migration-a', 'x', 'admin') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('messages-migration-b', 'x', 'admin') RETURNING id`).Scan(&otherID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, userID, otherID) })
	if _, err := db.Exec(`INSERT INTO user_social_settings (user_id, friend_request_policy) VALUES ($1, 'mutuals')`, userID); err != nil {
		t.Fatalf("insert settings: %v", err)
	}

	step(friendMessagesMigration)
	expect(true, "after the migration")
	var policy string
	var toasts bool
	if err := db.QueryRow(`SELECT message_policy, message_toasts FROM user_social_settings WHERE user_id = $1`, userID).Scan(&policy, &toasts); err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if policy != "friends" || !toasts {
		t.Fatalf("defaults = %q %v", policy, toasts)
	}

	// One direct conversation per ordered pair; the pair must be ordered
	low, high := userID, otherID
	if low > high {
		low, high = high, low
	}
	var convID int64
	if err := db.QueryRow(`INSERT INTO conversations (direct_low_id, direct_high_id) VALUES ($1, $2) RETURNING id`, low, high).Scan(&convID); err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO conversations (direct_low_id, direct_high_id) VALUES ($1, $2)`, low, high); err == nil {
		t.Fatal("a second conversation for the pair was accepted")
	}
	if _, err := db.Exec(`INSERT INTO conversations (direct_low_id, direct_high_id) VALUES ($1, $2)`, high, low); err == nil {
		t.Fatal("an unordered pair was accepted")
	}
	if _, err := db.Exec(`INSERT INTO messages (conversation_id, sender_id, attachment_kind) VALUES ($1, $2, 'video')`, convID, userID); err == nil {
		t.Fatal("an unknown attachment kind was accepted")
	}
	if _, err := db.Exec(`INSERT INTO messages (conversation_id, sender_id, body) VALUES ($1, $2, 'hello')`, convID, userID); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// Share link ranges: the end needs a start after it
	var sceneID int64
	if err := db.QueryRow(`INSERT INTO scenes (title) VALUES ('messages migration') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID) })
	if _, err := db.Exec(`INSERT INTO share_links (token, scene_id, user_id, start_s, end_s, label) VALUES ('msg-mig-ok', $1, $2, 10, 40, 'Kiss')`, sceneID, userID); err != nil {
		t.Fatalf("insert a range link: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO share_links (token, scene_id, user_id, start_s, end_s) VALUES ('msg-mig-bad', $1, $2, 40, 40)`, sceneID, userID); err == nil {
		t.Fatal("an empty range was accepted")
	}

	step(prev)
	expect(false, "after the down migration")
	step(friendMessagesMigration)
	expect(true, "after migrating up again")
}
