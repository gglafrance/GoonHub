package migrator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// splitFixture builds actors and studios split by alias, at the version before the
// aliases migration, in a disposable database (GOONHUB_TEST_MIGRATION_POSTGRES_DSN).
type splitFixture struct {
	t      *testing.T
	db     *sql.DB
	suffix string
	ids    map[string]int64
	uuids  map[string]string
}

func (f *splitFixture) n(name string) string { return name + " " + f.suffix }

func (f *splitFixture) actor(key, name string, aliases []string, image string) {
	f.t.Helper()
	if aliases == nil {
		aliases = []string{}
	}
	id := uuid.NewString()
	var dbID int64
	if err := f.db.QueryRow(`INSERT INTO actors (uuid, name, aliases, image_url) VALUES ($1, $2, $3, $4) RETURNING id`,
		id, name, pq.Array(aliases), image).Scan(&dbID); err != nil {
		f.t.Fatalf("insert actor %s: %v", key, err)
	}
	f.ids["actor:"+key], f.uuids["actor:"+key] = dbID, id
}

func (f *splitFixture) studio(key, name string, aliases []string, logo string, parent *int64) {
	f.t.Helper()
	if aliases == nil {
		aliases = []string{}
	}
	id := uuid.NewString()
	var dbID int64
	if err := f.db.QueryRow(`INSERT INTO studios (uuid, name, aliases, logo, parent_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		id, name, pq.Array(aliases), logo, parent).Scan(&dbID); err != nil {
		f.t.Fatalf("insert studio %s: %v", key, err)
	}
	f.ids["studio:"+key], f.uuids["studio:"+key] = dbID, id
}

func (f *splitFixture) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(q, args...); err != nil {
		f.t.Fatalf("exec %q: %v", q, err)
	}
}

func (f *splitFixture) insertID(q string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.db.QueryRow(q, args...).Scan(&id); err != nil {
		f.t.Fatalf("insert %q: %v", q, err)
	}
	return id
}

func (f *splitFixture) deleted(table, key string) bool {
	f.t.Helper()
	var del bool
	if err := f.db.QueryRow(`SELECT deleted_at IS NOT NULL FROM `+table+` WHERE id = $1`, f.ids[strings.TrimSuffix(table, "s")+":"+key]).Scan(&del); err != nil {
		f.t.Fatalf("deleted %s %s: %v", table, key, err)
	}
	return del
}

func (f *splitFixture) aliases(table, key string) []string {
	f.t.Helper()
	var a pq.StringArray
	if err := f.db.QueryRow(`SELECT aliases FROM `+table+` WHERE id = $1`, f.ids[strings.TrimSuffix(table, "s")+":"+key]).Scan(&a); err != nil {
		f.t.Fatalf("aliases %s %s: %v", table, key, err)
	}
	return a
}

func sameList(a, b []string) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// TestMigrationMergesActorsSplitByAlias checks that the aliases migration merges the actors
// (and studios) that were split into one entity per alias: a clean split, mutual aliases,
// an ambiguous name (skipped), a chain (merged into its final target) and a loop of three
// (skipped), with their scenes, ratings, likes, homepage sections and saved filters; then
// down (the merges stay) and up again.
func TestMigrationMergesActorsSplitByAlias(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_MIGRATION_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOONHUB_TEST_MIGRATION_POSTGRES_DSN not set")
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatalf("migration source: %v", err)
	}
	prev, err := source.Prev(aliasesMigration)
	if err != nil {
		t.Fatalf("version before %d: %v", aliasesMigration, err)
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
	f := &splitFixture{t: t, db: db, suffix: uuid.NewString()[:8], ids: map[string]int64{}, uuids: map[string]string{}}
	like := "%" + f.suffix + "%"
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM scenes WHERE title LIKE $1`,
			`DELETE FROM actors WHERE name LIKE $1`,
			`UPDATE studios SET parent_id = NULL, network_id = NULL WHERE name LIKE $1`,
			`DELETE FROM studios WHERE name LIKE $1`,
			`DELETE FROM users WHERE username LIKE $1`,
		} {
			if _, err := db.Exec(q, like); err != nil {
				t.Logf("cleanup %q: %v", q, err)
			}
		}
	})

	// Studios have no aliases before this migration: give them some, as a database that
	// already had the column would (the migration keeps an existing column).
	f.exec(`ALTER TABLE studios ADD COLUMN IF NOT EXISTS aliases TEXT[] NOT NULL DEFAULT '{}'`)

	// Actors
	f.actor("jane", f.n("Jane Doe"), []string{f.n("Janie")}, "")                            // target of a clean split
	f.actor("janie", "  "+strings.ToUpper(f.n("janie"))+" ", []string{f.n("JD")}, "/b.jpg") // split: Jane's alias, other case, spaces
	f.actor("m1", f.n("Mutual One"), []string{f.n("Mutual Two")}, "/m1.jpg")                // mutual, oldest: kept
	f.actor("m2", f.n("Mutual Two"), []string{f.n("Mutual One")}, "")                       // mutual: merged into m1
	f.actor("amb", f.n("Ambiguous"), nil, "")                                               // alias of two actors: skipped
	f.actor("p", f.n("Owner P"), []string{f.n("ambiguous")}, "")
	f.actor("q", f.n("Owner Q"), []string{f.n("AMBIGUOUS")}, "")
	f.actor("top", f.n("Chain Top"), []string{f.n("Chain Mid")}, "/top.jpg") // chain low -> mid -> top
	f.actor("mid", f.n("Chain Mid"), []string{f.n("Chain Low")}, "")
	f.actor("low", f.n("Chain Low"), nil, "")
	f.actor("ra", f.n("Ring A"), []string{f.n("Ring B")}, "") // loop of three: skipped
	f.actor("rb", f.n("Ring B"), []string{f.n("Ring C")}, "")
	f.actor("rc", f.n("Ring C"), []string{f.n("Ring A")}, "")
	f.actor("gone", f.n("Gone Owner"), []string{f.n("Lonely")}, "") // a deleted actor's alias does not count
	f.actor("lonely", f.n("Lonely"), nil, "")
	f.exec(`UPDATE actors SET deleted_at = NOW() WHERE id = $1`, f.ids["actor:gone"])

	u1 := f.insertID(`INSERT INTO users (username, password) VALUES ($1, 'x') RETURNING id`, f.n("split-u1"))
	u2 := f.insertID(`INSERT INTO users (username, password) VALUES ($1, 'x') RETURNING id`, f.n("split-u2"))
	s1 := f.insertID(`INSERT INTO scenes (title, actors) VALUES ($1, $2) RETURNING id`, f.n("s1"), pq.Array([]string{"x"}))
	s2 := f.insertID(`INSERT INTO scenes (title) VALUES ($1) RETURNING id`, f.n("s2"))
	s3 := f.insertID(`INSERT INTO scenes (title) VALUES ($1) RETURNING id`, f.n("s3"))
	jane, janie := f.ids["actor:jane"], f.ids["actor:janie"]
	f.exec(`INSERT INTO scene_actors (scene_id, actor_id) VALUES ($1, $3), ($2, $3), ($2, $4), ($5, $6)`,
		s1, s2, janie, jane, s3, f.ids["actor:low"])
	f.exec(`INSERT INTO user_actor_ratings (user_id, actor_id, rating) VALUES ($1, $3, 5), ($1, $4, 2), ($2, $4, 4)`, u1, u2, jane, janie)
	f.exec(`INSERT INTO user_actor_likes (user_id, actor_id) VALUES ($1, $2)`, u2, janie)
	f.exec(`INSERT INTO user_settings (user_id, homepage_config) VALUES ($1, $2::jsonb)
		ON CONFLICT (user_id) DO UPDATE SET homepage_config = EXCLUDED.homepage_config`,
		u1, `{"sections":[{"type":"actor","config":{"actor_uuid":"`+f.uuids["actor:janie"]+`"}}]}`)
	janieName := "  " + strings.ToUpper(f.n("janie")) + " "
	searchFilters, _ := json.Marshal(map[string]any{"selected_actors": []string{janieName, f.n("Jane Doe"), "Other"}, "query": "q"})
	f.exec(`INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (gen_random_uuid(), $1, 'split', $2::jsonb)`, u1, string(searchFilters))
	markerFilters, _ := json.Marshal(map[string]any{"actors": []string{janieName}})
	f.exec(`INSERT INTO saved_marker_filters (uuid, user_id, name, filters) VALUES (gen_random_uuid(), $1, 'split', $2::jsonb)`, u2, string(markerFilters))

	// Studios: a clean split with a child studio, and an ambiguous name
	f.studio("bz", f.n("Brazzers"), []string{f.n("BZ")}, "", nil)
	f.studio("bzsplit", f.n("bz"), nil, "/bz.png", nil)
	split := f.ids["studio:bzsplit"]
	f.studio("child", f.n("BZ Child"), nil, "", &split)
	f.studio("samb", f.n("Studio Amb"), nil, "", nil)
	f.studio("so1", f.n("Studio Owner 1"), []string{f.n("studio amb")}, "", nil)
	f.studio("so2", f.n("Studio Owner 2"), []string{f.n("Studio Amb")}, "", nil)
	s4 := f.insertID(`INSERT INTO scenes (title, studio_id, studio) VALUES ($1, $2, $3) RETURNING id`, f.n("s4"), split, f.n("bz"))
	f.exec(`INSERT INTO user_studio_ratings (user_id, studio_id, rating) VALUES ($1, $2, 3)`, u1, split)
	studioFilters, _ := json.Marshal(map[string]any{"studio": f.n("bz")})
	f.exec(`INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (gen_random_uuid(), $1, 'split studio', $2::jsonb)`, u2, string(studioFilters))

	if err := m.Migrate(aliasesMigration); err != nil {
		t.Fatalf("migrate to %d: %v", aliasesMigration, err)
	}

	check := func(stage string, withStudioAliases bool) {
		t.Helper()
		// clean split
		if !f.deleted("actors", "janie") || f.deleted("actors", "jane") {
			t.Fatalf("%s: janie must be merged into jane", stage)
		}
		if got := f.aliases("actors", "jane"); !sameList(got, []string{f.n("Janie"), f.n("JD")}) {
			t.Fatalf("%s: jane aliases = %v", stage, got)
		}
		var image string
		db.QueryRow(`SELECT image_url FROM actors WHERE id = $1`, jane).Scan(&image)
		if image != "/b.jpg" {
			t.Fatalf("%s: jane had no photo, must take janie's: %q", stage, image)
		}
		var credits int
		db.QueryRow(`SELECT COUNT(*) FROM scene_actors WHERE actor_id = $1 AND scene_id IN ($2, $3)`, jane, s1, s2).Scan(&credits)
		var left int
		db.QueryRow(`SELECT COUNT(*) FROM scene_actors WHERE actor_id = $1`, janie).Scan(&left)
		if credits != 2 || left != 0 {
			t.Fatalf("%s: scene credits of jane %d (want 2), left on janie %d", stage, credits, left)
		}
		var denorm pq.StringArray
		db.QueryRow(`SELECT actors FROM scenes WHERE id = $1`, s1).Scan(&denorm)
		if !sameList(denorm, []string{f.n("Jane Doe")}) {
			t.Fatalf("%s: denormalized actors of s1 = %v", stage, denorm)
		}
		var r1, r2 float64
		db.QueryRow(`SELECT rating FROM user_actor_ratings WHERE user_id = $1 AND actor_id = $2`, u1, jane).Scan(&r1)
		db.QueryRow(`SELECT rating FROM user_actor_ratings WHERE user_id = $1 AND actor_id = $2`, u2, jane).Scan(&r2)
		var leftRatings, likes int
		db.QueryRow(`SELECT COUNT(*) FROM user_actor_ratings WHERE actor_id = $1`, janie).Scan(&leftRatings)
		db.QueryRow(`SELECT COUNT(*) FROM user_actor_likes WHERE actor_id = $1 AND user_id = $2`, jane, u2).Scan(&likes)
		if r1 != 5 || r2 != 4 || leftRatings != 0 || likes != 1 {
			t.Fatalf("%s: ratings u1 %v (keep 5) u2 %v (moved 4), left %d, likes %d", stage, r1, r2, leftRatings, likes)
		}
		var homepage string
		db.QueryRow(`SELECT homepage_config::text FROM user_settings WHERE user_id = $1`, u1).Scan(&homepage)
		if strings.Contains(homepage, f.uuids["actor:janie"]) || !strings.Contains(homepage, f.uuids["actor:jane"]) {
			t.Fatalf("%s: homepage not updated: %s", stage, homepage)
		}
		var raw string
		db.QueryRow(`SELECT filters::text FROM saved_searches WHERE user_id = $1 AND name = 'split'`, u1).Scan(&raw)
		var ss struct {
			SelectedActors []string `json:"selected_actors"`
			Query          string   `json:"query"`
		}
		json.Unmarshal([]byte(raw), &ss)
		if !sameList(ss.SelectedActors, []string{f.n("Jane Doe"), "Other"}) || ss.Query != "q" {
			t.Fatalf("%s: saved search = %s", stage, raw)
		}
		db.QueryRow(`SELECT filters::text FROM saved_marker_filters WHERE user_id = $1 AND name = 'split'`, u2).Scan(&raw)
		if !strings.Contains(raw, f.n("Jane Doe")) || strings.Contains(raw, janieName) {
			t.Fatalf("%s: saved marker filter = %s", stage, raw)
		}

		// mutual: the oldest is kept
		if f.deleted("actors", "m1") || !f.deleted("actors", "m2") {
			t.Fatalf("%s: m2 must be merged into m1 (oldest)", stage)
		}
		if got := f.aliases("actors", "m1"); !sameList(got, []string{f.n("Mutual Two")}) {
			t.Fatalf("%s: m1 aliases = %v (its own name must not be an alias)", stage, got)
		}
		// ambiguous, loop and alias of a deleted actor: untouched
		for _, key := range []string{"amb", "p", "q", "ra", "rb", "rc", "lonely"} {
			if f.deleted("actors", key) {
				t.Fatalf("%s: actor %s must be left alone", stage, key)
			}
		}
		// chain: everything into the final target
		if !f.deleted("actors", "mid") || !f.deleted("actors", "low") || f.deleted("actors", "top") {
			t.Fatalf("%s: the chain must be merged into top", stage)
		}
		if got := f.aliases("actors", "top"); !sameList(got, []string{f.n("Chain Mid"), f.n("Chain Low")}) {
			t.Fatalf("%s: top aliases = %v", stage, got)
		}
		var topScene int
		db.QueryRow(`SELECT COUNT(*) FROM scene_actors WHERE actor_id = $1 AND scene_id = $2`, f.ids["actor:top"], s3).Scan(&topScene)
		if topScene != 1 {
			t.Fatalf("%s: low's scene must move to top", stage)
		}

		// studios
		if !f.deleted("studios", "bzsplit") || f.deleted("studios", "bz") {
			t.Fatalf("%s: bz split must be merged into Brazzers", stage)
		}
		for _, key := range []string{"samb", "so1", "so2"} {
			if f.deleted("studios", key) {
				t.Fatalf("%s: studio %s must be left alone", stage, key)
			}
		}
		var studioID, parentID int64
		var studioName, logo string
		db.QueryRow(`SELECT studio_id, studio FROM scenes WHERE id = $1`, s4).Scan(&studioID, &studioName)
		db.QueryRow(`SELECT parent_id FROM studios WHERE id = $1`, f.ids["studio:child"]).Scan(&parentID)
		db.QueryRow(`SELECT logo FROM studios WHERE id = $1`, f.ids["studio:bz"]).Scan(&logo)
		bz := f.ids["studio:bz"]
		if studioID != bz || studioName != f.n("Brazzers") || parentID != bz || logo != "/bz.png" {
			t.Fatalf("%s: scene studio %d %q, child parent %d, logo %q", stage, studioID, studioName, parentID, logo)
		}
		var studioRatings int
		db.QueryRow(`SELECT COUNT(*) FROM user_studio_ratings WHERE studio_id = $1 AND user_id = $2`, bz, u1).Scan(&studioRatings)
		db.QueryRow(`SELECT filters->>'studio' FROM saved_searches WHERE user_id = $1 AND name = 'split studio'`, u2).Scan(&raw)
		if studioRatings != 1 || raw != f.n("Brazzers") {
			t.Fatalf("%s: studio rating moved %d, saved search studio %q", stage, studioRatings, raw)
		}
		if withStudioAliases {
			if got := f.aliases("studios", "bz"); !sameList(got, []string{f.n("BZ")}) {
				t.Fatalf("%s: Brazzers aliases = %v", stage, got)
			}
		}
	}
	check("after up", true)

	// down reverts the schema only: the merges stay
	if err := m.Migrate(prev); err != nil {
		t.Fatalf("migrate down to %d: %v", prev, err)
	}
	if !f.deleted("actors", "janie") || !f.deleted("actors", "m2") || !f.deleted("studios", "bzsplit") {
		t.Fatal("down must not un-merge")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up again: %v", err)
	}
	// studio aliases were dropped by down, the rest is unchanged and nothing more is merged
	check("after down and up", false)
}
