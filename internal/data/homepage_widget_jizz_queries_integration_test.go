package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of the dated o queries of the stats widgets, on the disposable import test
// database (GOONHUB_TEST_IMPORT_POSTGRES_DSN). Library-wide counts are checked as deltas.
func TestHomepageJizzWidgetQueries(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	userID := f.user.ID
	suffix := f.tags[0].Name[len("import-a-"):]

	baseShare, err := QueryJizzedSceneShare(db, userID)
	if err != nil {
		t.Fatalf("baseline share: %v", err)
	}

	trashedAt := time.Now()
	sceneA := &Scene{Title: "A", StoredPath: "/import-test/" + suffix + "/ja.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	sceneB := &Scene{Title: "B", StoredPath: "/import-test/" + suffix + "/jb.mp4", ThumbnailPath: "b.jpg", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	sceneC := &Scene{Title: "Trashed", StoredPath: "/import-test/" + suffix + "/jc.mp4", TrashedAt: &trashedAt, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{sceneA, sceneB, sceneC} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}

	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	// 23:30Z on Jan 31st is 00:30 on Sunday Feb 1st in Paris
	events := []UserSceneJizzEvent{
		{UserID: userID, SceneID: sceneA.ID, JizzedAt: at("2026-01-31T23:30:00Z")},
		{UserID: userID, SceneID: sceneB.ID, JizzedAt: at("2026-02-02T20:00:00Z")},
		{UserID: userID, SceneID: sceneB.ID, JizzedAt: at("2026-02-02T21:00:00Z")},
		{UserID: userID, SceneID: sceneB.ID, JizzedAt: at("2026-02-03T20:00:00Z")},
		{UserID: userID, SceneID: sceneC.ID, JizzedAt: at("2026-03-05T20:00:00Z")}, // trashed scene
	}
	for i := range events {
		if err := db.Create(&events[i]).Error; err != nil {
			t.Fatalf("create event: %v", err)
		}
	}
	for _, v := range []any{
		&UserSceneJizzed{UserID: userID, SceneID: sceneA.ID, Count: 1},
		&UserSceneJizzed{UserID: userID, SceneID: sceneB.ID, Count: 3},
		&UserSceneJizzed{UserID: userID, SceneID: sceneC.ID, Count: 1},
	} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create count: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Where("user_id = ?", userID).Delete(&UserSceneJizzEvent{})
		db.Where("user_id = ?", userID).Delete(&UserSceneJizzed{})
	})

	t.Run("period stats", func(t *testing.T) {
		stats, err := QueryJizzPeriodStats(db, userID, at("2026-01-01T00:00:00Z"), at("2026-02-01T00:00:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		// Time series keep the trashed scene's event (it happened)
		if stats.Period != 4 || stats.PrevPeriod != 1 || stats.Total != 5 {
			t.Fatalf("period stats: %+v", stats)
		}
		if stats.First == nil || !stats.First.Equal(events[0].JizzedAt) || stats.Last == nil || !stats.Last.Equal(events[4].JizzedAt) {
			t.Fatalf("first/last: %v %v", stats.First, stats.Last)
		}
	})

	t.Run("daily and monthly in the timezone", func(t *testing.T) {
		days, err := QueryDailyJizz(db, userID, "Europe/Paris", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if len(days) != 4 || days[0].Day != "2026-02-01" || days[1].Day != "2026-02-02" || days[1].Count != 2 {
			t.Fatalf("daily: %+v", days)
		}
		months, err := QueryMonthlyJizz(db, userID, "Europe/Paris", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if len(months) != 2 || months[0].Day != "2026-02" || months[0].Count != 4 || months[1].Day != "2026-03" {
			t.Fatalf("monthly: %+v", months)
		}
		utc, err := QueryMonthlyJizz(db, userID, "UTC", at("2026-02-01T00:00:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		if len(utc) != 2 || utc[0].Day != "2026-02" || utc[0].Count != 3 {
			t.Fatalf("monthly UTC since Feb: %+v", utc)
		}
	})

	t.Run("hour and weekday", func(t *testing.T) {
		hours, weekdays, err := QueryJizzByHourAndWeekday(db, userID, "Europe/Paris", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		byHour := map[int]int64{}
		for _, h := range hours {
			byHour[h.Bucket] = h.Count
		}
		// 20:00Z = 21:00 Paris (winter)
		if byHour[0] != 1 || byHour[21] != 3 || byHour[22] != 1 {
			t.Fatalf("hours: %+v", hours)
		}
		byDay := map[int]int64{}
		for _, d := range weekdays {
			byDay[d.Bucket] = d.Count
		}
		// Sunday Feb 1st (7), Monday Feb 2nd (1) x2, Tuesday Feb 3rd (2), Thursday Mar 5th (4)
		if byDay[7] != 1 || byDay[1] != 2 || byDay[2] != 1 || byDay[4] != 1 {
			t.Fatalf("weekdays: %+v", weekdays)
		}
	})

	t.Run("scene rankings skip trashed scenes", func(t *testing.T) {
		top, err := QueryTopJizzScenes(db, userID, time.Time{}, time.Now(), 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(top) != 2 || top[0].ID != sceneB.ID || top[0].Count != 3 || top[0].ThumbnailPath != "b.jpg" || top[1].ID != sceneA.ID {
			t.Fatalf("top: %+v", top)
		}
		windowed, err := QueryTopJizzScenes(db, userID, at("2026-02-03T00:00:00Z"), time.Now(), 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(windowed) != 1 || windowed[0].Count != 1 {
			t.Fatalf("windowed top: %+v", windowed)
		}
		best, err := QueryMostJizzedSceneInADay(db, userID, "Europe/Paris")
		if err != nil {
			t.Fatal(err)
		}
		if best == nil || best.ID != sceneB.ID || best.Count != 2 || best.Day != "2026-02-02" {
			t.Fatalf("most in a day: %+v", best)
		}
	})

	t.Run("display queries", func(t *testing.T) {
		q := NewDisplayWidgetQueries(db)
		from, to := at("2026-02-01T00:00:00Z"), at("2026-04-01T00:00:00Z")
		top, err := q.TopJizzedScenes(userID, from, to, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(top) != 1 || top[0].SceneID != sceneB.ID || top[0].Views != 3 {
			t.Fatalf("top jizzed: %+v", top)
		}
		counts, err := q.JizzCountsForScenes(userID, []uint{sceneA.ID, sceneB.ID}, at("2026-01-01T00:00:00Z"), from)
		if err != nil {
			t.Fatal(err)
		}
		if counts[sceneA.ID] != 1 || counts[sceneB.ID] != 0 {
			t.Fatalf("counts: %+v", counts)
		}
		total, err := q.CountJizz(userID, from, to)
		if err != nil {
			t.Fatal(err)
		}
		if total != 4 {
			t.Fatalf("count: %d", total)
		}
	})

	t.Run("library share", func(t *testing.T) {
		share, err := QueryJizzedSceneShare(db, userID)
		if err != nil {
			t.Fatal(err)
		}
		if share.JizzedScenes-baseShare.JizzedScenes != 2 || share.LibraryCount-baseShare.LibraryCount != 2 {
			t.Fatalf("share: %+v (baseline %+v)", share, baseShare)
		}
	})
}
