package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration tests of the scene widget queries on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN). Every query is scoped to the fixture folder or user.
func TestHomepageSceneQueries(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	q := NewHomepageSceneQueries(db)
	suffix := f.tags[0].Name[len("import-a-"):]
	root := "/import-test/" + suffix

	mk := func(name string, duration, height int, created time.Time) *Scene {
		s := &Scene{Title: name, StoredPath: root + "/" + name + ".mp4", Duration: duration, Height: height,
			Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		if !created.IsZero() {
			if err := db.Model(s).UpdateColumn("created_at", created).Error; err != nil {
				t.Fatalf("set created_at: %v", err)
			}
		}
		return s
	}
	// Feb 29 keeps the "added on this day" query away from other fixtures
	leap2024 := time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC)
	short := mk("short", 30, 720, time.Time{})
	long := mk("long", 3600, 2160, leap2024)
	longWatched := mk("long-watched", 2400, 1080, time.Time{})
	fav := mk("fav", 600, 1080, time.Time{})
	trashed := mk("trashed", 4000, 2160, time.Time{})
	now := time.Now()
	if err := db.Model(trashed).UpdateColumn("trashed_at", now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&SceneTag{SceneID: long.ID, TagID: f.tags[0].ID}).Error; err != nil {
		t.Fatal(err)
	}

	watch := func(s *Scene, at time.Time, pos int, completed bool) {
		w := &UserSceneWatch{UserID: f.user.ID, SceneID: s.ID, WatchedAt: at, LastPosition: pos, Completed: completed}
		if err := db.Create(w).Error; err != nil {
			t.Fatalf("create watch: %v", err)
		}
	}
	watch(longWatched, now.Add(-2*time.Hour), 600, false)                // in progress (25%)
	watch(fav, time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC), 600, true) // old watch
	watch(fav, time.Date(2028, 2, 29, 12, 0, 0, 0, time.UTC), 600, true) // "on this day" 2028
	watch(short, now.Add(-time.Hour), 30, true)                          // most recent
	if err := db.Create(&UserSceneJizzed{UserID: f.user.ID, SceneID: fav.ID, Count: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UserSceneLike{UserID: f.user.ID, SceneID: long.ID}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Where("user_id = ?", f.user.ID).Delete(&UserSceneWatch{})
		db.Where("user_id = ?", f.user.ID).Delete(&UserSceneJizzed{})
		db.Where("user_id = ?", f.user.ID).Delete(&UserSceneLike{})
	})

	folder := HomepageSceneFilter{Folder: root}

	t.Run("unwatched in folder", func(t *testing.T) {
		ids, total, err := q.SceneIDs(f.user.ID, folder, true, HomepageSceneOrderRandom, 10)
		if err != nil {
			t.Fatal(err)
		}
		// short, long-watched and fav are watched, trashed is excluded
		if total != 1 || len(ids) != 1 || ids[0] != long.ID {
			t.Fatalf("got %v (total %d), want only %d", ids, total, long.ID)
		}
	})

	t.Run("filters", func(t *testing.T) {
		tests := []struct {
			name string
			f    HomepageSceneFilter
			want int64
		}{
			{"all in folder", folder, 4},
			{"folder name anywhere", HomepageSceneFilter{Folder: suffix}, 4},
			{"min duration", HomepageSceneFilter{Folder: root, MinDuration: 1000}, 2},
			{"max duration", HomepageSceneFilter{Folder: root, MaxDuration: 100}, 1},
			{"4k", HomepageSceneFilter{Folder: root, MinHeight: 2160}, 1},
			{"tag", HomepageSceneFilter{Folder: root, Tags: []string{f.tags[0].Name, "missing"}}, 1},
			{"like wildcard is literal", HomepageSceneFilter{Folder: root + "%"}, 0},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, total, err := q.SceneIDs(f.user.ID, tt.f, false, HomepageSceneOrderNewest, 10)
				if err != nil {
					t.Fatal(err)
				}
				if total != tt.want {
					t.Fatalf("total = %d, want %d", total, tt.want)
				}
			})
		}
	})

	t.Run("rediscover", func(t *testing.T) {
		rows, err := q.RediscoverScenes(f.user.ID, RediscoverOptions{
			NotWatchedSince: now.AddDate(0, 0, -1), IncludeLiked: true, IncludeJizzed: true,
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		// fav (jizzed 3, last watched 2028... in the future relative to the cutoff? no: cutoff is
		// yesterday, fav's latest watch is 2028 > cutoff) -> excluded; long (liked, never watched) kept.
		if len(rows) != 1 || rows[0].SceneID != long.ID || !rows[0].Liked || rows[0].LastWatched != nil {
			t.Fatalf("unexpected rows: %+v", rows)
		}
		rows, err = q.RediscoverScenes(f.user.ID, RediscoverOptions{
			NotWatchedSince: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), IncludeLiked: true, IncludeJizzed: true,
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 || rows[0].SceneID != fav.ID || rows[0].JizzCount != 3 || rows[0].Views != 2 {
			t.Fatalf("jizzed favorite must come first: %+v", rows)
		}
		none, err := q.RediscoverScenes(f.user.ID, RediscoverOptions{NotWatchedSince: now}, 10)
		if err != nil || len(none) != 0 {
			t.Fatalf("no criterion must return nothing: %v %v", none, err)
		}
	})

	t.Run("on this day", func(t *testing.T) {
		day := time.Date(2032, 2, 29, 9, 0, 0, 0, time.UTC)
		rows, err := q.OnThisDay(f.user.ID, day, true, true, 10)
		if err != nil {
			t.Fatal(err)
		}
		var watched, added bool
		for _, r := range rows {
			if r.SceneID == fav.ID && r.Year == 2028 && r.Kind == "watched" {
				watched = true
			}
			if r.SceneID == long.ID && r.Year == 2024 && r.Kind == "added" {
				added = true
			}
		}
		if !watched || !added || rows[0].Year != 2028 {
			t.Fatalf("unexpected rows: %+v", rows)
		}
		onlyWatched, err := q.OnThisDay(f.user.ID, day, true, false, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range onlyWatched {
			if r.Kind != "watched" {
				t.Fatalf("added rows returned in watched mode: %+v", onlyWatched)
			}
		}
	})

	t.Run("recently watched and watched set", func(t *testing.T) {
		ids, err := q.RecentlyWatchedSceneIDs(f.user.ID, 2)
		if err != nil {
			t.Fatal(err)
		}
		// fav's 2028 watch is the most recent, then short (1 h ago)
		if len(ids) != 2 || ids[0] != fav.ID || ids[1] != short.ID {
			t.Fatalf("unexpected order: %v", ids)
		}
		watched, err := q.WatchedSceneIDs(f.user.ID, []uint{short.ID, long.ID})
		if err != nil || !watched[short.ID] || watched[long.ID] {
			t.Fatalf("unexpected watched set: %v %v", watched, err)
		}
	})

	t.Run("in progress", func(t *testing.T) {
		rows, err := q.InProgressScenes(f.user.ID, HomepageSceneFilter{Folder: root, MinDuration: 1200}, 0.9, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].SceneID != longWatched.ID || rows[0].LastPosition != 600 {
			t.Fatalf("unexpected rows: %+v", rows)
		}
	})
}
