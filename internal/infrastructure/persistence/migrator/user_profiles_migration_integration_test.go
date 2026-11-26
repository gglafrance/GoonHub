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

// Integration test of migration 000100 (user profiles): up, down, up again, and the
// constraints. It migrates the database down and up, so it needs a disposable database of its
// own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration100UserProfiles(t *testing.T) {
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

	tables := []string{"user_profiles", "user_profile_favorite_scenes", "user_profile_favorite_markers"}
	exists := func(name string) bool {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1`, name).Scan(&n); err != nil {
			t.Fatalf("check %s: %v", name, err)
		}
		return n > 0
	}
	if err := m.Migrate(100); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to 100: %v", err)
	}
	for _, step := range []struct {
		down    bool
		present bool
	}{{false, true}, {true, false}, {false, true}} {
		if step.down {
			if err := m.Steps(-1); err != nil {
				t.Fatalf("down: %v", err)
			}
		} else if err := m.Migrate(100); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("up: %v", err)
		}
		for _, name := range tables {
			if exists(name) != step.present {
				t.Fatalf("down=%v: %s present = %v, want %v", step.down, name, !step.present, step.present)
			}
		}
	}

	var userID, sceneID, markerID int64
	if err := db.QueryRow(`INSERT INTO users (username, password) VALUES ('mig100-a', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE username LIKE 'mig100-%'`); err != nil {
			t.Logf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM scenes WHERE stored_path = '/mig099/a.mp4'`); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	if err := db.QueryRow(`INSERT INTO scenes (title, stored_path) VALUES ('mig099', '/mig099/a.mp4') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO user_scene_markers (user_id, scene_id, timestamp) VALUES ($1, $2, 5) RETURNING id`, userID, sceneID).Scan(&markerID); err != nil {
		t.Fatal(err)
	}
	mustFail := func(name, q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	mustFail("bad banner type", `INSERT INTO user_profiles (user_id, banner_type) VALUES ($1, 'video')`, userID)
	mustFail("layout not an array", `INSERT INTO user_profiles (user_id, layout) VALUES ($1, '{}')`, userID)
	if _, err := db.Exec(`INSERT INTO user_profiles (user_id, banner_type, banner_scene_id, layout) VALUES ($1, 'scene', $2, '[]')`, userID, sceneID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO user_profile_favorite_scenes (user_id, scene_id, position) VALUES ($1, $2, 0)`, userID, sceneID); err != nil {
		t.Fatal(err)
	}
	mustFail("negative position", `INSERT INTO user_profile_favorite_markers (user_id, marker_id, position) VALUES ($1, $2, -1)`, userID, markerID)
	if _, err := db.Exec(`INSERT INTO user_profile_favorite_markers (user_id, marker_id, position) VALUES ($1, $2, 0)`, userID, markerID); err != nil {
		t.Fatal(err)
	}

	// Deleting the scene clears the banner and the showcase entry; the marker goes with it
	if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID); err != nil {
		t.Fatal(err)
	}
	var banner sql.NullInt64
	var n int
	if err := db.QueryRow(`SELECT banner_scene_id FROM user_profiles WHERE user_id = $1`, userID).Scan(&banner); err != nil || banner.Valid {
		t.Fatalf("banner scene not cleared: %v %v", banner, err)
	}
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM user_profile_favorite_scenes) + (SELECT COUNT(*) FROM user_profile_favorite_markers)`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("showcases not cascaded: %d %v", n, err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_profiles`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("profile not cascaded: %d %v", n, err)
	}
}
