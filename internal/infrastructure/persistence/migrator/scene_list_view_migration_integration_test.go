package migrator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

// Integration test of migration 000084 (user_settings.scene_list_view): existing settings rows
// get the default view (grids, default table columns), and the column goes away on down. It
// migrates the database down and up again, so it needs a disposable database of its own:
// GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration084SceneListView(t *testing.T) {
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

	const version = 84
	// the migration before (numbers are not contiguous while features are merged)
	prev, err := source.Prev(version)
	if err != nil {
		t.Fatalf("previous migration of %d: %v", version, err)
	}
	if err := m.Migrate(prev); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to %d: %v", prev, err)
	}

	var userID int64
	if err := db.QueryRow(`INSERT INTO users (username, password, role) VALUES ('migration-084', 'x', 'user') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Logf("cleanup user %d: %v", userID, err)
		}
	})
	if _, err := db.Exec(`INSERT INTO user_settings (user_id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("insert settings: %v", err)
	}

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	var raw []byte
	if err := db.QueryRow(`SELECT scene_list_view FROM user_settings WHERE user_id = $1`, userID).Scan(&raw); err != nil {
		t.Fatalf("select: %v", err)
	}
	var view struct {
		Pages   map[string]string `json:"pages"`
		Columns []string          `json:"columns"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	wantColumns := []string{"thumbnail", "title", "duration", "resolution", "file_size", "rating", "views", "studio", "actors", "added_at"}
	if view.Pages == nil || len(view.Pages) != 0 || !slices.Equal(view.Columns, wantColumns) {
		t.Fatalf("unexpected default scene_list_view: %s", raw)
	}

	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'user_settings' AND column_name = 'scene_list_view')`).Scan(&exists); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if exists {
		t.Fatal("scene_list_view still exists after down")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
}
