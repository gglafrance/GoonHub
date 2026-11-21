package migrator

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

const replayHeatmapMigration = 79

// Integration test of migration 000079 (user_scene_heatmaps and the replay heatmap
// settings): up, down and up again. It migrates the database down, so it needs a disposable
// database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration079ReplayHeatmap(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	previous, err := source.Prev(replayHeatmapMigration)
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
	// registered first, so it runs after the test rows are deleted
	t.Cleanup(func() { _ = db.Close() })

	exists := func(query string) bool {
		t.Helper()
		var ok bool
		if err := db.QueryRow(query).Scan(&ok); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		return ok
	}
	tableExists := func() bool {
		return exists(`SELECT to_regclass('public.user_scene_heatmaps') IS NOT NULL`)
	}
	settingsColumns := func() bool {
		return exists(`SELECT COUNT(*) = 2 FROM information_schema.columns
			WHERE table_name = 'user_settings' AND column_name IN ('show_replay_heatmap', 'record_replay_heatmap')`)
	}

	if err := m.Migrate(previous); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", previous, err)
	}
	var userID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ($1, 'x', 'user') RETURNING id`,
		fmt.Sprintf("migration-079-%d", time.Now().UnixNano())).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Logf("cleanup user: %v", err)
		}
	})
	if _, err := db.Exec(`INSERT INTO user_settings (user_id, homepage_config, parsing_rules, sort_preferences, scene_card_config)
		VALUES ($1, '{}', '{}', '{}', '{}')`, userID); err != nil {
		t.Fatalf("insert settings: %v", err)
	}

	if err := m.Migrate(replayHeatmapMigration); err != nil {
		t.Fatalf("migrate to %d: %v", replayHeatmapMigration, err)
	}
	if !tableExists() || !settingsColumns() {
		t.Fatal("expected the heatmap table and settings columns after up")
	}
	// Existing users get both settings on
	var show, record bool
	if err := db.QueryRow(`SELECT show_replay_heatmap, record_replay_heatmap FROM user_settings WHERE user_id = $1`, userID).Scan(&show, &record); err != nil {
		t.Fatalf("select settings: %v", err)
	}
	if !show || !record {
		t.Fatalf("expected both settings on, got show=%v record=%v", show, record)
	}
	var sceneID int64
	if err := db.QueryRow(`INSERT INTO scenes (title) VALUES ('migration 079 test') RETURNING id`).Scan(&sceneID); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, sceneID); err != nil {
			t.Logf("cleanup scene: %v", err)
		}
	})
	if _, err := db.Exec(`INSERT INTO user_scene_heatmaps (user_id, scene_id, buckets) VALUES ($1, $2, '{1,2,3}')`, userID, sceneID); err != nil {
		t.Fatalf("insert heatmap: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_scene_heatmaps (user_id, scene_id, buckets) VALUES ($1, $2, '{4}')`, userID, sceneID); err == nil {
		t.Fatal("expected one heatmap per user and scene")
	}

	if err := m.Migrate(previous); err != nil {
		t.Fatalf("migrate down to %d: %v", previous, err)
	}
	if tableExists() || exists(`SELECT COUNT(*) > 0 FROM information_schema.columns
		WHERE table_name = 'user_settings' AND column_name IN ('show_replay_heatmap', 'record_replay_heatmap')`) {
		t.Fatal("expected the heatmap table and settings columns to be gone after down")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if !tableExists() || !settingsColumns() {
		t.Fatal("expected the heatmap table and settings columns after up again")
	}
}
