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

// Integration test of migration 000096 (marker playlists): up, down, up on a disposable
// database (GOONHUB_TEST_MIGRATION_POSTGRES_DSN). Existing playlists become scene playlists;
// going down drops the marker playlists.
func TestMigration096MarkerPlaylists(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	// The migration before 096 (095 may belong to another feature)
	prev, err := source.Prev(96)
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

	var userID, sceneID, markerID, scenePlaylistID int64
	if err := db.QueryRow(`INSERT INTO users (username, password) VALUES ('mig096', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Logf("cleanup user: %v", err)
		}
	})
	if err := db.QueryRow(`INSERT INTO scenes (title) VALUES ('mig096') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID); err != nil {
			t.Logf("cleanup scene: %v", err)
		}
	})
	if err := db.QueryRow(`INSERT INTO user_scene_markers (user_id, scene_id, timestamp) VALUES ($1, $2, 10) RETURNING id`, userID, sceneID).Scan(&markerID); err != nil {
		t.Fatalf("insert marker: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO playlists (user_id, name) VALUES ($1, 'scenes') RETURNING id`, userID).Scan(&scenePlaylistID); err != nil {
		t.Fatalf("insert playlist: %v", err)
	}

	if err := m.Migrate(96); err != nil {
		t.Fatalf("migrate to 96: %v", err)
	}
	var kind string
	if err := db.QueryRow(`SELECT kind FROM playlists WHERE id = $1`, scenePlaylistID).Scan(&kind); err != nil || kind != "scenes" {
		t.Fatalf("existing playlist kind = %q, %v", kind, err)
	}
	if _, err := db.Exec(`INSERT INTO playlists (user_id, name, kind) VALUES ($1, 'bad', 'actors')`, userID); err == nil {
		t.Fatal("an unknown kind must be refused")
	}
	var markerPlaylistID int64
	if err := db.QueryRow(`INSERT INTO playlists (user_id, name, kind) VALUES ($1, 'markers', 'markers') RETURNING id`, userID).Scan(&markerPlaylistID); err != nil {
		t.Fatalf("insert marker playlist: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO playlist_markers (playlist_id, marker_id, position) VALUES ($1, $2, 0)`, markerPlaylistID, markerID); err != nil {
		t.Fatalf("insert playlist marker: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO playlist_markers (playlist_id, marker_id, position) VALUES ($1, $2, 1)`, markerPlaylistID, markerID); err == nil {
		t.Fatal("a marker must be in a playlist once")
	}
	if _, err := db.Exec(`INSERT INTO playlist_progress (user_id, playlist_id, last_scene_id, last_marker_id) VALUES ($1, $2, $3, $4)`, userID, markerPlaylistID, sceneID, markerID); err != nil {
		t.Fatalf("insert progress: %v", err)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM playlists WHERE user_id = $1`, userID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("playlists after down = %d, %v (the scene playlist stays)", count, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'playlists' AND column_name = 'kind'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("kind column after down: %d, %v", count, err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
}
