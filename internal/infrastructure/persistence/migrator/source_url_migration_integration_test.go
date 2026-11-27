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

// Integration test of migration 000074 (scenes.source_url): a description that is only a URL
// becomes the source, both ways. It migrates the database down and up again, so it needs a
// disposable database of its own: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration074SourceURL(t *testing.T) {
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
	// registered first, so it runs after the scenes are deleted
	t.Cleanup(func() { _ = db.Close() })

	if err := m.Migrate(73); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate to 73: %v", err)
	}

	cases := []struct {
		description string
		wantDesc    string
		wantSource  string
	}{
		{"https://www.lpsg.com/threads/x.1/post-2", "", "https://www.lpsg.com/threads/x.1/post-2"},
		{"  http://example.com/watch?v=1&t=2 \n", "", "http://example.com/watch?v=1&t=2"},
		{"HTTPS://EXAMPLE.COM/X", "", "HTTPS://EXAMPLE.COM/X"},
		{"Look at https://example.com", "Look at https://example.com", ""},
		{"https://a.com https://b.com", "https://a.com https://b.com", ""},
		{"https://example.com\nwith a second line", "https://example.com\nwith a second line", ""},
		{"ftp://example.com/file", "ftp://example.com/file", ""},
		{"", "", ""},
	}
	ids := make([]int64, len(cases))
	for i, c := range cases {
		if err := db.QueryRow(`INSERT INTO scenes (title, description) VALUES ($1, $2) RETURNING id`,
			"migration 074 test", c.description).Scan(&ids[i]); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			if _, err := db.Exec(`DELETE FROM scenes WHERE id = $1`, id); err != nil {
				t.Logf("cleanup scene %d: %v", id, err)
			}
		}
	})

	if err := m.Migrate(74); err != nil {
		t.Fatalf("migrate to 74: %v", err)
	}
	for i, c := range cases {
		var desc, src string
		if err := db.QueryRow(`SELECT description, source_url FROM scenes WHERE id = $1`, ids[i]).Scan(&desc, &src); err != nil {
			t.Fatalf("select: %v", err)
		}
		if desc != c.wantDesc || src != c.wantSource {
			t.Fatalf("case %q: got description %q, source %q; want %q, %q", c.description, desc, src, c.wantDesc, c.wantSource)
		}
	}

	// down: the sources go back into the (empty) descriptions
	if err := m.Migrate(73); err != nil {
		t.Fatalf("migrate down to 73: %v", err)
	}
	for i, c := range cases {
		var desc string
		if err := db.QueryRow(`SELECT description FROM scenes WHERE id = $1`, ids[i]).Scan(&desc); err != nil {
			t.Fatalf("select: %v", err)
		}
		want := c.wantDesc
		if c.wantSource != "" {
			want = c.wantSource
		}
		if desc != want {
			t.Fatalf("down, case %q: got description %q, want %q", c.description, desc, want)
		}
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
}
