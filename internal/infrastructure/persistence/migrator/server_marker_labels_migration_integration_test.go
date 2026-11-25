package migrator

import (
	"database/sql"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

// Integration test of migration 000097 (server-wide marker labels): the per-user free text
// labels become one referential (merged ignoring case, the most used spelling kept), markers
// point to it, the per-user default tags become global (union) and are given to every marker
// of the label, the triggers keep the label copy in sync, and down restores per-user data.
// Needs a disposable database: GOONHUB_TEST_MIGRATION_POSTGRES_DSN.
func TestMigration097ServerMarkerLabels(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	const version = 97
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

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var ua, ub, scene, tOral, tPov int64
	for _, row := range []struct {
		q   string
		dst *int64
	}{
		{`INSERT INTO users (username, password) VALUES ('m097-a', 'x') RETURNING id`, &ua},
		{`INSERT INTO users (username, password) VALUES ('m097-b', 'x') RETURNING id`, &ub},
		{`INSERT INTO scenes (title, duration) VALUES ('m097', 600) RETURNING id`, &scene},
		{`INSERT INTO tags (name) VALUES ('m097-oral') RETURNING id`, &tOral},
		{`INSERT INTO tags (name) VALUES ('m097-pov') RETURNING id`, &tPov},
	} {
		if err := db.QueryRow(row.q).Scan(row.dst); err != nil {
			t.Fatalf("%s: %v", row.q, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM scenes WHERE id = $1`, scene)
		_, _ = db.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, ua, ub)
		_, _ = db.Exec(`DELETE FROM tags WHERE id IN ($1, $2)`, tOral, tPov)
		_, _ = db.Exec(`DELETE FROM marker_labels WHERE lower(name) LIKE 'm097%'`)
	})

	// user A: "M097 Kiss" once, "m097 kiss " once; user B: "m097 kiss" twice (the most used
	// spelling), one "M097 Solo", one empty, one NULL label
	marker := func(user int64, ts int, label any, color string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(`INSERT INTO user_scene_markers (user_id, scene_id, timestamp, label, color)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, user, scene, ts, label, color).Scan(&id); err != nil {
			t.Fatalf("insert marker: %v", err)
		}
		return id
	}
	mA1 := marker(ua, 10, "M097 Kiss", "#FF0000")
	mA2 := marker(ua, 20, "m097 kiss ", "#FF0000")
	mB1 := marker(ub, 30, "m097 kiss", "#00FF00")
	mB2 := marker(ub, 40, "m097 kiss", "#00FF00")
	mB3 := marker(ub, 50, "M097 Solo", "#FFFFFF")
	mEmpty := marker(ub, 60, "", "#FFFFFF")
	mNull := marker(ub, 70, nil, "#FFFFFF")
	// default tags: A has oral on "M097 Kiss", B has oral and pov on "m097 kiss", B has pov on
	// a label without markers
	exec(`INSERT INTO marker_label_tags (user_id, label, tag_id) VALUES
		($1, 'M097 Kiss', $3), ($2, 'm097 kiss', $3), ($2, 'm097 kiss', $4), ($2, 'm097 orphan', $4)`, ua, ub, tOral, tPov)
	// mA1 has oral from its label, mB1 has pov set by hand
	exec(`INSERT INTO marker_tags (marker_id, tag_id, is_from_label) VALUES ($1, $2, TRUE), ($3, $4, FALSE)`, mA1, tOral, mB1, tPov)

	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}

	labels := map[string]int64{}
	rows, err := db.Query(`SELECT id, name FROM marker_labels WHERE lower(name) LIKE 'm097%'`)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatal(err)
		}
		labels[name] = id
	}
	rows.Close()
	if len(labels) != 3 || labels["m097 kiss"] == 0 || labels["M097 Solo"] == 0 || labels["m097 orphan"] == 0 {
		t.Fatalf("labels = %v, want m097 kiss, M097 Solo, m097 orphan", labels)
	}
	kiss := labels["m097 kiss"]

	checkMarker := func(id int64, wantLabel string, wantID int64) {
		t.Helper()
		var label string
		var labelID sql.NullInt64
		if err := db.QueryRow(`SELECT label, label_id FROM user_scene_markers WHERE id = $1`, id).Scan(&label, &labelID); err != nil {
			t.Fatalf("marker %d: %v", id, err)
		}
		if label != wantLabel || labelID.Int64 != wantID || labelID.Valid != (wantID != 0) {
			t.Fatalf("marker %d = (%q, %v), want (%q, %d)", id, label, labelID, wantLabel, wantID)
		}
	}
	for _, id := range []int64{mA1, mA2, mB1, mB2} {
		checkMarker(id, "m097 kiss", kiss)
	}
	checkMarker(mB3, "M097 Solo", labels["M097 Solo"])
	checkMarker(mEmpty, "", 0)
	checkMarker(mNull, "", 0)

	// default tags: union per label, once
	tagsOf := func(q string, args ...any) []int64 {
		t.Helper()
		rows, err := db.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			out = append(out, id)
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	if got := tagsOf(`SELECT tag_id FROM marker_label_tags WHERE label_id = $1`, kiss); len(got) != 2 || got[0] != tOral || got[1] != tPov {
		t.Fatalf("default tags of the kiss label = %v, want [%d %d]", got, tOral, tPov)
	}
	// every kiss marker has both tags; mB1 keeps pov as set by hand
	for _, id := range []int64{mA1, mA2, mB1, mB2} {
		if got := tagsOf(`SELECT tag_id FROM marker_tags WHERE marker_id = $1`, id); len(got) != 2 {
			t.Fatalf("marker %d tags = %v, want both default tags", id, got)
		}
	}
	var fromLabel bool
	if err := db.QueryRow(`SELECT is_from_label FROM marker_tags WHERE marker_id = $1 AND tag_id = $2`, mB1, tPov).Scan(&fromLabel); err != nil || fromLabel {
		t.Fatalf("a tag set by hand must stay by hand (from label = %v, %v)", fromLabel, err)
	}

	// triggers: a legacy text write resolves (or creates) the label, a label_id write wins, a
	// rename follows on the markers, a deletion empties them
	var newID int64
	var newLabel string
	if err := db.QueryRow(`INSERT INTO user_scene_markers (user_id, scene_id, timestamp, label) VALUES ($1, $2, 80, ' M097 KISS ')
		RETURNING id, label`, ua, scene).Scan(&newID, &newLabel); err != nil || newLabel != "m097 kiss" {
		t.Fatalf("text insert label = %q (%v), want the canonical name", newLabel, err)
	}
	checkMarker(newID, "m097 kiss", kiss)
	exec(`UPDATE user_scene_markers SET label = 'm097 brand new' WHERE id = $1`, newID)
	var brandNew int64
	if err := db.QueryRow(`SELECT id FROM marker_labels WHERE name = 'm097 brand new'`).Scan(&brandNew); err != nil {
		t.Fatalf("label written as text not created: %v", err)
	}
	checkMarker(newID, "m097 brand new", brandNew)
	exec(`UPDATE user_scene_markers SET label_id = $1 WHERE id = $2`, labels["M097 Solo"], newID)
	checkMarker(newID, "M097 Solo", labels["M097 Solo"])
	exec(`UPDATE marker_labels SET name = 'M097 Kissing' WHERE id = $1`, kiss)
	checkMarker(mA1, "M097 Kissing", kiss)
	exec(`DELETE FROM marker_labels WHERE id = $1`, labels["M097 Solo"])
	checkMarker(mB3, "", 0)
	checkMarker(newID, "", 0)
	if _, err := db.Exec(`INSERT INTO marker_labels (name) VALUES ('m097 KISSING')`); err == nil {
		t.Fatal("label names must be unique ignoring case")
	}

	// down: per-user default tags for the users having markers of the label
	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM marker_label_tags WHERE user_id IN ($1, $2) AND label = 'M097 Kissing'`, ua, ub).Scan(&count); err != nil || count != 4 {
		t.Fatalf("per-user default tags after down = %d (%v), want 4 (2 tags x 2 users)", count, err)
	}
	var label string
	if err := db.QueryRow(`SELECT label FROM user_scene_markers WHERE id = $1`, mA2).Scan(&label); err != nil || label != "M097 Kissing" {
		t.Fatalf("label after down = %q (%v)", label, err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('marker_labels') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("marker_labels still there after down (%v)", err)
	}
	// a text write works again without trigger
	exec(`UPDATE user_scene_markers SET label = 'free text' WHERE id = $1`, mA2)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up again: %v", err)
	}
	if err := db.QueryRow(`SELECT name FROM marker_labels ml JOIN user_scene_markers m ON m.label_id = ml.id WHERE m.id = $1`, mA2).Scan(&label); err != nil || !strings.EqualFold(label, "free text") {
		t.Fatalf("label after up again = %q (%v)", label, err)
	}
	_, _ = db.Exec(`DELETE FROM marker_labels WHERE lower(name) = 'free text'`)
}
