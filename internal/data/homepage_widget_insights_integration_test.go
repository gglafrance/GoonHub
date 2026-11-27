package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Integration tests of the insight widget queries, on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN). Library-wide counts are checked as deltas because the
// database may hold rows of other tests.
func TestInsightsWidgetQueries(t *testing.T) {
	db := importTestDB(t)
	q := NewInsightsWidgetQueries(db)
	f := newImportFixture(t, db)
	suffix := f.tags[0].Name[len("import-a-"):]

	curationBefore, err := q.CurationCounts(f.user.ID)
	if err != nil {
		t.Fatalf("CurationCounts: %v", err)
	}
	coverageBefore, err := q.Coverage()
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}

	studioID := f.studio.ID
	fp := "audio"
	trashedAt := time.Now()
	// a: tagged (both tags) + actor + studio + thumbnail/sprites/fingerprint
	a := &Scene{Title: "A", StoredPath: "/import-test/" + suffix + "/a.mp4", StudioID: &studioID,
		ThumbnailPath: "a.webp", SpriteSheetPath: "a_sheet.webp", FingerprintType: &fp,
		Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	// b: first tag only, no actor, no studio, no generated files
	b := &Scene{Title: "B", StoredPath: "/import-test/" + suffix + "/b.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	// trashed: must be ignored everywhere
	trashed := &Scene{Title: "T", StoredPath: "/import-test/" + suffix + "/t.mp4", TrashedAt: &trashedAt, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{a, b, trashed} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}
	for _, v := range []any{
		&SceneTag{SceneID: a.ID, TagID: f.tags[0].ID},
		&SceneTag{SceneID: a.ID, TagID: f.tags[1].ID},
		&SceneTag{SceneID: b.ID, TagID: f.tags[0].ID},
		&SceneTag{SceneID: trashed.ID, TagID: f.tags[1].ID},
		&SceneActor{SceneID: a.ID, ActorID: f.actor.ID},
		&SceneActor{SceneID: trashed.ID, ActorID: f.actor.ID},
	} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create association: %v", err)
		}
	}

	now := time.Now()
	recentFrom := now.AddDate(0, 0, -30)
	baseFrom := recentFrom.AddDate(0, 0, -180)
	watches := []UserSceneWatch{
		// recent: a x3, b x1; baseline: b x2; too old: a x5 (ignored); trashed (ignored)
		{UserID: f.user.ID, SceneID: a.ID, WatchedAt: now.Add(-24 * time.Hour)},
		{UserID: f.user.ID, SceneID: a.ID, WatchedAt: now.Add(-48 * time.Hour)},
		{UserID: f.user.ID, SceneID: a.ID, WatchedAt: now.Add(-72 * time.Hour)},
		{UserID: f.user.ID, SceneID: b.ID, WatchedAt: now.Add(-24 * time.Hour)},
		{UserID: f.user.ID, SceneID: b.ID, WatchedAt: now.AddDate(0, 0, -60)},
		{UserID: f.user.ID, SceneID: b.ID, WatchedAt: now.AddDate(0, 0, -90)},
		{UserID: f.user.ID, SceneID: trashed.ID, WatchedAt: now.Add(-24 * time.Hour)},
	}
	for i := 0; i < 5; i++ {
		watches = append(watches, UserSceneWatch{UserID: f.user.ID, SceneID: a.ID, WatchedAt: now.AddDate(-2, 0, 0)})
	}
	for i := range watches {
		if err := db.Create(&watches[i]).Error; err != nil {
			t.Fatalf("create watch: %v", err)
		}
	}
	for _, v := range []any{
		&UserSceneJizzed{UserID: f.user.ID, SceneID: a.ID, Count: 4},
		&UserSceneLike{UserID: f.user.ID, SceneID: a.ID},
		&UserSceneRating{UserID: f.user.ID, SceneID: a.ID, Rating: 4.5},
	} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create interaction: %v", err)
		}
	}

	t.Run("view totals", func(t *testing.T) {
		totals, err := q.ViewTotals(f.user.ID, baseFrom, recentFrom)
		if err != nil {
			t.Fatal(err)
		}
		if totals.Recent != 4 || totals.Base != 2 {
			t.Fatalf("unexpected totals: %+v", totals)
		}
	})

	t.Run("tag trends", func(t *testing.T) {
		rows, err := q.TagTrends(f.user.ID, baseFrom, recentFrom)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]InsightsTrendRow{}
		for _, r := range rows {
			got[r.Name] = r
		}
		tagA, tagB := got[f.tags[0].Name], got[f.tags[1].Name]
		if tagA.RecentViews != 4 || tagA.BaseViews != 2 || tagB.RecentViews != 3 || tagB.BaseViews != 0 {
			t.Fatalf("unexpected tag rows: %+v", rows)
		}
	})

	t.Run("actor trends", func(t *testing.T) {
		rows, err := q.ActorTrends(f.user.ID, baseFrom, recentFrom)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].RecentViews != 3 || rows[0].BaseViews != 0 || rows[0].UUID != f.actor.UUID.String() {
			t.Fatalf("unexpected actor rows: %+v", rows)
		}
	})

	t.Run("top performers", func(t *testing.T) {
		for _, order := range []string{"views", "jizz", "scenes"} {
			rows, err := q.TopPerformers(f.user.ID, order, 10)
			if err != nil {
				t.Fatalf("%s: %v", order, err)
			}
			var found *InsightsPerformerRow
			for i := range rows {
				if rows[i].ID == f.actor.ID {
					found = &rows[i]
				}
			}
			if found == nil {
				t.Fatalf("%s: actor missing from %+v", order, rows)
			}
			// a only (trashed scene ignored): 3 recent + 5 old views, 4 jizz, 1 liked
			if found.SceneCount != 1 || found.Views != 8 || found.Jizz != 4 || found.Liked != 1 || found.LastWatched == nil {
				t.Fatalf("%s: unexpected stats %+v", order, found)
			}
		}
		if _, err := q.TopPerformers(f.user.ID, "nope", 10); err == nil {
			t.Fatal("unknown order must fail")
		}
	})

	t.Run("curation counts", func(t *testing.T) {
		after, err := q.CurationCounts(f.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		delta := InsightsCurationCounts{
			Total:       after.Total - curationBefore.Total,
			NoTags:      after.NoTags - curationBefore.NoTags,
			NoActors:    after.NoActors - curationBefore.NoActors,
			NoStudio:    after.NoStudio - curationBefore.NoStudio,
			NoThumbnail: after.NoThumbnail - curationBefore.NoThumbnail,
			NoSprites:   after.NoSprites - curationBefore.NoSprites,
		}
		want := InsightsCurationCounts{Total: 2, NoTags: 0, NoActors: 1, NoStudio: 1, NoThumbnail: 1, NoSprites: 1}
		if delta != want {
			t.Fatalf("unexpected curation delta: %+v, want %+v", delta, want)
		}
		// Watched but not rated (user scoped): b only (a is rated, trashed ignored)
		if after.WatchedNoRate != 1 {
			t.Fatalf("watched not rated = %d, want 1", after.WatchedNoRate)
		}
	})

	t.Run("coverage", func(t *testing.T) {
		after, err := q.Coverage()
		if err != nil {
			t.Fatal(err)
		}
		if after.Total-coverageBefore.Total != 2 || after.Thumbnail-coverageBefore.Thumbnail != 1 ||
			after.Sprites-coverageBefore.Sprites != 1 || after.Fingerprint-coverageBefore.Fingerprint != 1 ||
			after.Preview != coverageBefore.Preview {
			t.Fatalf("unexpected coverage: before %+v after %+v", coverageBefore, after)
		}
	})

	t.Run("active job counts", func(t *testing.T) {
		prefix := uuid.NewString()[:8]
		old := now.Add(-48 * time.Hour)
		recent := now.Add(-time.Hour)
		jobs := []JobHistory{
			{JobID: prefix + "-1", SceneID: a.ID, Phase: "insights_test", Status: "pending"},
			{JobID: prefix + "-2", SceneID: b.ID, Phase: "insights_test", Status: "running"},
			{JobID: prefix + "-3", SceneID: a.ID, Phase: "insights_test", Status: "failed", CompletedAt: &recent},
			{JobID: prefix + "-4", SceneID: b.ID, Phase: "insights_test", Status: "failed", CompletedAt: &old},
			{JobID: prefix + "-5", SceneID: trashed.ID, Phase: "insights_test", Status: "completed", CompletedAt: &recent},
		}
		for i := range jobs {
			if err := db.Create(&jobs[i]).Error; err != nil {
				t.Fatalf("create job: %v", err)
			}
		}
		t.Cleanup(func() { db.Where("job_id LIKE ?", prefix+"-%").Delete(&JobHistory{}) })

		rows, err := q.ActiveJobCounts(now.Add(-24 * time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int64{}
		for _, r := range rows {
			if r.Phase == "insights_test" {
				got[r.Status] = r.Count
			}
		}
		if got["pending"] != 1 || got["running"] != 1 || got["failed"] != 1 || got["completed"] != 0 {
			t.Fatalf("unexpected job counts: %v", got)
		}
	})
}
