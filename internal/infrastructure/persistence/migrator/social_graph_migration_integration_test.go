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

// Integration test of migration 000095 (social graph): up, down, up again, and the
// constraints. It migrates the database down and up, so it needs a disposable database of its
// own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration095SocialGraph(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
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

	tables := []string{"user_follows", "user_friendships", "friend_requests", "user_privacy_settings", "user_social_settings", "user_friends"}
	exists := func(name string) bool {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1`, name).Scan(&n); err != nil {
			t.Fatalf("check %s: %v", name, err)
		}
		return n > 0
	}

	for _, step := range []struct {
		version uint
		present bool
	}{{95, true}, {94, false}, {95, true}} {
		if err := m.Migrate(step.version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", step.version, err)
		}
		for _, name := range tables {
			if exists(name) != step.present {
				t.Fatalf("at %d: %s present = %v, want %v", step.version, name, !step.present, step.present)
			}
		}
	}

	var a, b int64
	if err := db.QueryRow(`INSERT INTO users (username, password) VALUES ('mig095-a', 'x') RETURNING id`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO users (username, password) VALUES ('mig095-b', 'x') RETURNING id`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE username LIKE 'mig095-%'`); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	low, high := a, b
	if low > high {
		low, high = high, low
	}
	mustFail := func(name, q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	mustFail("self follow", `INSERT INTO user_follows (follower_id, followed_id) VALUES ($1, $1)`, a)
	mustFail("unordered friendship", `INSERT INTO user_friendships (user_low_id, user_high_id) VALUES ($1, $2)`, high, low)
	mustFail("self request", `INSERT INTO friend_requests (sender_id, receiver_id) VALUES ($1, $1)`, a)
	mustFail("bad audience", `INSERT INTO user_privacy_settings (user_id, item, audience) VALUES ($1, 'stats', 'admins')`, a)
	mustFail("bad policy", `INSERT INTO user_social_settings (user_id, friend_request_policy) VALUES ($1, 'friends')`, a)

	if _, err := db.Exec(`INSERT INTO user_friendships (user_low_id, user_high_id) VALUES ($1, $2)`, low, high); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_friends WHERE (user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1)`, a, b).Scan(&n); err != nil || n != 2 {
		t.Fatalf("user_friends both directions: %d %v", n, err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_friendships`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("friendship not cascaded: %d %v", n, err)
	}
}
