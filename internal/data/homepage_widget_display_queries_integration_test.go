package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of the display widget queries, on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDisplayWidgetQueries(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	q := NewDisplayWidgetQueries(db)
	suffix := f.tags[0].Name[len("import-a-"):]
	userID := f.user.ID

	mk := func(name string, duration int, preview string) *Scene {
		s := &Scene{
			Title: name, StoredPath: "/import-test/" + suffix + "/" + name + ".mp4", Duration: duration,
			ThumbnailPath: "thumb.webp", PreviewVideoPath: preview, Tags: pq.StringArray{}, Actors: pq.StringArray{},
		}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	short := mk("short", 30, "p.mp4")
	shortWatched := mk("short-watched", 20, "")
	long := mk("long", 1200, "p.mp4")
	trashed := mk("trashed", 10, "p.mp4")
	now := time.Now()
	if err := db.Model(trashed).Update("trashed_at", now).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM user_scene_watches WHERE user_id = ?", userID)
		db.Exec("DELETE FROM user_scene_likes WHERE user_id = ?", userID)
		db.Exec("DELETE FROM user_scene_jizzed WHERE user_id = ?", userID)
		db.Exec("DELETE FROM user_scene_ratings WHERE user_id = ?", userID)
	})

	watch := func(sceneID uint, at time.Time) {
		if err := db.Exec("INSERT INTO user_scene_watches (user_id, scene_id, watched_at) VALUES (?, ?, ?)", userID, sceneID, at).Error; err != nil {
			t.Fatalf("insert watch: %v", err)
		}
	}
	// This period (last 30 days): long x3, short-watched x1. Previous period: long x1.
	for i := 0; i < 3; i++ {
		watch(long.ID, now.AddDate(0, 0, -i-1))
	}
	watch(shortWatched.ID, now.AddDate(0, 0, -2))
	watch(long.ID, now.AddDate(0, 0, -40))
	// Watched in the trash: never listed
	watch(trashed.ID, now.AddDate(0, 0, -1))

	if err := db.Exec("INSERT INTO user_scene_likes (user_id, scene_id) VALUES (?, ?)", userID, long.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_scene_jizzed (user_id, scene_id, count) VALUES (?, ?, 4), (?, ?, 1)", userID, short.ID, userID, long.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_scene_ratings (user_id, scene_id, rating) VALUES (?, ?, 4.5)", userID, long.ID).Error; err != nil {
		t.Fatal(err)
	}

	ids := func(scenes []Scene) map[uint]bool {
		m := map[uint]bool{}
		for _, s := range scenes {
			m[s.ID] = true
		}
		return m
	}

	t.Run("short scenes", func(t *testing.T) {
		got, err := q.ShortScenes(userID, 60, false, 50)
		if err != nil {
			t.Fatal(err)
		}
		m := ids(got)
		if !m[short.ID] || !m[shortWatched.ID] || m[long.ID] || m[trashed.ID] {
			t.Fatalf("unexpected shorts: %v", m)
		}
		got, err = q.ShortScenes(userID, 60, true, 50)
		if err != nil {
			t.Fatal(err)
		}
		if m := ids(got); !m[short.ID] || m[shortWatched.ID] {
			t.Fatalf("unwatched_only not applied: %v", m)
		}
	})

	t.Run("sources", func(t *testing.T) {
		liked, err := q.SourceScenes(userID, WidgetSourceLiked, false, 10)
		if err != nil || len(liked) != 1 || liked[0].ID != long.ID {
			t.Fatalf("liked: %v %v", liked, err)
		}
		top, err := q.SourceScenes(userID, WidgetSourceTopJizz, false, 10)
		if err != nil || len(top) != 2 || top[0].ID != short.ID {
			t.Fatalf("top jizz must be ordered by count: %v %v", top, err)
		}
		withPreview, err := q.SourceScenes(userID, WidgetSourceRandom, true, 50)
		if err != nil {
			t.Fatal(err)
		}
		if m := ids(withPreview); m[shortWatched.ID] || m[trashed.ID] {
			t.Fatalf("requirePreview not applied: %v", m)
		}
		unwatched, err := q.SourceScenes(userID, WidgetSourceUnwatched, false, 50)
		if err != nil {
			t.Fatal(err)
		}
		if m := ids(unwatched); m[long.ID] || m[shortWatched.ID] || !m[short.ID] {
			t.Fatalf("unwatched: %v", m)
		}
		if _, err := q.SourceScenes(userID, WidgetSourceLatest, false, 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("top watched", func(t *testing.T) {
		from, to := now.AddDate(0, 0, -30), now.Add(time.Minute)
		top, err := q.TopWatchedScenes(userID, from, to, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(top) != 2 || top[0].SceneID != long.ID || top[0].Views != 3 {
			t.Fatalf("unexpected top: %+v", top)
		}
		prev, err := q.ViewCountsForScenes(userID, []uint{long.ID, shortWatched.ID}, from.AddDate(0, 0, -30), from)
		if err != nil || prev[long.ID] != 1 || prev[shortWatched.ID] != 0 {
			t.Fatalf("unexpected previous counts: %v %v", prev, err)
		}
		total, err := q.CountViews(userID, from, to)
		if err != nil || total != 5 { // includes the trashed scene watch
			t.Fatalf("unexpected total: %d %v", total, err)
		}
	})

	t.Run("unrated recently watched", func(t *testing.T) {
		got, total, err := q.UnratedRecentlyWatched(userID, now.AddDate(0, 0, -30), 10)
		if err != nil {
			t.Fatal(err)
		}
		// long is rated, trashed is excluded
		if total != 1 || len(got) != 1 || got[0].ID != shortWatched.ID {
			t.Fatalf("unexpected unrated: %d %+v", total, got)
		}
	})
}
