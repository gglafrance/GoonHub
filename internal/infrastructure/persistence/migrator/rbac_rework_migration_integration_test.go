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

// Integration test of migration 000094 (roles and permissions rework): flags of the system
// roles, scenes:upload revoked from user, roles held by users recreated for the users.role
// foreign key, and the down migration (denials dropped, upload granted back). Needs a
// disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration094RBACRework(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	const version = 94
	prev, err := source.Prev(version)
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

	// a user holding a role missing from roles (possible before the foreign key)
	if _, err := db.Exec(`DELETE FROM users WHERE username = 'migration-094-ghost'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (username, password, role) VALUES ('migration-094-ghost', 'x', 'ghost094')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM users WHERE username = 'migration-094-ghost'`)
	})
	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}

	var adminSystem, adminSuper, userSystem, userSuper bool
	if err := db.QueryRow(`SELECT is_system, is_superuser FROM roles WHERE name = 'admin'`).Scan(&adminSystem, &adminSuper); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT is_system, is_superuser FROM roles WHERE name = 'user'`).Scan(&userSystem, &userSuper); err != nil {
		t.Fatal(err)
	}
	if !adminSystem || !adminSuper || !userSystem || userSuper {
		t.Fatalf("flags: admin %v/%v user %v/%v", adminSystem, adminSuper, userSystem, userSuper)
	}

	hasUpload := func() bool {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM role_permissions rp
			JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id
			WHERE r.name = 'user' AND p.name = 'scenes:upload'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	}
	if hasUpload() {
		t.Fatal("scenes:upload must be revoked from user")
	}

	var ghost int
	if err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE name = 'ghost094'`).Scan(&ghost); err != nil || ghost != 1 {
		t.Fatalf("role held by a user must be recreated: %d %v", ghost, err)
	}
	if _, err := db.Exec(`INSERT INTO users (username, password, role) VALUES ('migration-094-bad', 'x', 'nope094')`); err == nil {
		_, _ = db.Exec(`DELETE FROM users WHERE username = 'migration-094-bad'`)
		t.Fatal("the users.role foreign key must refuse an unknown role")
	}

	// a deny setting, then down: it must not turn into a grant
	if _, err := db.Exec(`INSERT INTO role_permissions (role_id, permission_id, effect)
		SELECT r.id, p.id, 'deny' FROM roles r, permissions p WHERE r.name = 'ghost094' AND p.name = 'scenes:view'`); err != nil {
		t.Fatalf("insert deny: %v", err)
	}
	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var ghostPerms int
	if err := db.QueryRow(`SELECT COUNT(*) FROM role_permissions rp JOIN roles r ON r.id = rp.role_id WHERE r.name = 'ghost094'`).Scan(&ghostPerms); err != nil {
		t.Fatal(err)
	}
	if ghostPerms != 0 {
		t.Fatal("deny settings must be dropped by the down migration")
	}
	if !hasUpload() {
		t.Fatal("the down migration must grant scenes:upload back to user")
	}

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up again: %v", err)
	}
	_, _ = db.Exec(`DELETE FROM users WHERE username = 'migration-094-ghost'`)
	_, _ = db.Exec(`DELETE FROM roles WHERE name = 'ghost094'`)
}
