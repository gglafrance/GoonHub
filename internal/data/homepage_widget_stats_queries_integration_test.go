package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of the stats widget queries, on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestHomepageStatsWidgetQueries(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	userID := f.user.ID
	suffix := f.tags[0].Name[len("import-a-"):]

	baseline, err := QueryLibraryStats(db, userID, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("baseline stats: %v", err)
	}

	trashedAt := time.Now()
	sceneA := &Scene{Title: "A", StoredPath: "/import-test/" + suffix + "/a.mp4", Duration: 999990, Size: 9_000_000_000_000_000, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	sceneB := &Scene{Title: "B", StoredPath: "/import-test/" + suffix + "/b.mp4", Duration: 600, Size: 1000, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	sceneC := &Scene{Title: "Trashed", StoredPath: "/import-test/" + suffix + "/c.mp4", Duration: 999999, Size: 9_100_000_000_000_000, TrashedAt: &trashedAt, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{sceneA, sceneB, sceneC} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}

	// Views (UTC instants): 23:30Z on Jan 1st is 00:30 on Friday Jan 2nd in Paris
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	watches := []UserSceneWatch{
		{UserID: userID, SceneID: sceneA.ID, WatchedAt: at("2026-01-01T23:30:00Z"), WatchDuration: 100},
		{UserID: userID, SceneID: sceneA.ID, WatchedAt: at("2026-01-02T18:00:00Z"), WatchDuration: 200},
		{UserID: userID, SceneID: sceneA.ID, WatchedAt: at("2026-01-03T18:00:00Z"), WatchDuration: 300},
		{UserID: userID, SceneID: sceneB.ID, WatchedAt: at("2026-01-03T19:00:00Z")},
		{UserID: userID, SceneID: sceneC.ID, WatchedAt: at("2026-01-10T19:00:00Z")}, // trashed scene
	}
	for i := range watches {
		if err := db.Create(&watches[i]).Error; err != nil {
			t.Fatalf("create watch: %v", err)
		}
	}
	for _, v := range []any{
		&UserSceneJizzed{UserID: userID, SceneID: sceneB.ID, Count: 4},
		&UserSceneJizzed{UserID: userID, SceneID: sceneA.ID, Count: 1},
		&UserSceneLike{UserID: userID, SceneID: sceneA.ID},
		&UserSceneRating{UserID: userID, SceneID: sceneA.ID, Rating: 4},
		&UserSceneRating{UserID: userID, SceneID: sceneB.ID, Rating: 5},
		&UserSceneMarker{UserID: userID, SceneID: sceneA.ID, Timestamp: 10, Label: "x", Color: "#FFFFFF"},
	} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create interaction: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Where("user_id = ?", userID).Delete(&UserSceneWatch{})
		db.Where("user_id = ?", userID).Delete(&UserSceneJizzed{})
		db.Where("user_id = ?", userID).Delete(&UserSceneLike{})
		db.Where("user_id = ?", userID).Delete(&UserSceneRating{})
		db.Where("user_id = ?", userID).Delete(&UserSceneMarker{})
	})

	t.Run("library stats", func(t *testing.T) {
		stats, err := QueryLibraryStats(db, userID, at("2026-01-01T00:00:00Z"), at("2026-01-03T00:00:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		// The trashed scene is excluded from the library figures
		if stats.SceneCount-baseline.SceneCount != 2 || stats.TotalDuration-baseline.TotalDuration != 999990+600 {
			t.Fatalf("library: %+v (baseline %+v)", stats, baseline)
		}
		if stats.WatchedScenes != 2 || stats.ViewsPeriod != 3 || stats.ViewsPrevPeriod != 2 {
			t.Fatalf("views: %+v", stats)
		}
		if stats.JizzTotal != 5 || stats.LikedCount != 1 || stats.MarkerCount != 1 || stats.RatedCount != 2 {
			t.Fatalf("interactions: %+v", stats)
		}
		if stats.AvgRating == nil || *stats.AvgRating != 4.5 {
			t.Fatalf("avg rating: %v", stats.AvgRating)
		}
	})

	t.Run("daily views in timezone", func(t *testing.T) {
		days, err := QueryDailyViews(db, userID, "Europe/Paris", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]int64{"2026-01-02": 2, "2026-01-03": 2, "2026-01-10": 1}
		if len(days) != len(want) {
			t.Fatalf("days: %+v", days)
		}
		for _, d := range days {
			if want[d.Day] != d.Count {
				t.Fatalf("day %s: got %d want %d", d.Day, d.Count, want[d.Day])
			}
		}
		since, err := QueryDailyViews(db, userID, "UTC", at("2026-01-05T00:00:00Z"))
		if err != nil || len(since) != 1 || since[0].Day != "2026-01-10" {
			t.Fatalf("since filter: %+v, %v", since, err)
		}
	})

	t.Run("hours and weekdays", func(t *testing.T) {
		hours, weekdays, err := QueryViewsByHourAndWeekday(db, userID, "Europe/Paris", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		byHour := map[int]int64{}
		for _, h := range hours {
			byHour[h.Bucket] = h.Count
		}
		// 23:30Z -> 00h Paris; 18:00Z -> 19h; 19:00Z -> 20h (winter time, UTC+1)
		if byHour[0] != 1 || byHour[19] != 2 || byHour[20] != 2 {
			t.Fatalf("hours: %+v", hours)
		}
		byDay := map[int]int64{}
		for _, d := range weekdays {
			byDay[d.Bucket] = d.Count
		}
		// Jan 2nd 2026 is a Friday (5), Jan 3rd a Saturday (6), Jan 10th a Saturday
		if byDay[5] != 2 || byDay[6] != 3 {
			t.Fatalf("weekdays: %+v", weekdays)
		}
	})

	t.Run("records", func(t *testing.T) {
		records, err := QueryWatchRecords(db, userID)
		if err != nil {
			t.Fatal(err)
		}
		if records.MostRewatched == nil || records.MostRewatched.ID != sceneA.ID || records.MostRewatched.Value != 3 {
			t.Fatalf("most rewatched: %+v", records.MostRewatched)
		}
		if records.MostJizzed == nil || records.MostJizzed.ID != sceneB.ID || records.MostJizzed.Value != 4 {
			t.Fatalf("most jizzed: %+v", records.MostJizzed)
		}
		// The trashed scene is longer and bigger but must be ignored
		if records.LongestScene == nil || records.LongestScene.ID != sceneA.ID {
			t.Fatalf("longest scene: %+v", records.LongestScene)
		}
		if records.BiggestFile == nil || records.BiggestFile.ID != sceneA.ID {
			t.Fatalf("biggest file: %+v", records.BiggestFile)
		}
		if records.TotalViews != 5 || records.TotalWatchTime != 600 {
			t.Fatalf("totals: %+v", records)
		}
		if records.FirstView == nil || !records.FirstView.Equal(at("2026-01-01T23:30:00Z")) {
			t.Fatalf("first view: %v", records.FirstView)
		}
	})
}
