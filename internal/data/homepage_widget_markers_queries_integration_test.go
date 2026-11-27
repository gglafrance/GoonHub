package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of the marker widget queries on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestHomepageWidgetMarkerQueries(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	suffix := f.tags[0].Name[len("import-a-"):]

	visible := &Scene{Title: "Visible", StoredPath: "/import-test/" + suffix + "/v.mp4", ThumbnailPath: "v.webp", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	trashed := &Scene{Title: "Trashed", StoredPath: "/import-test/" + suffix + "/t.mp4", ThumbnailPath: "t.webp", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{visible, trashed} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}
	now := time.Now()
	if err := db.Model(&Scene{}).Where("id = ?", trashed.ID).Update("trashed_at", &now).Error; err != nil {
		t.Fatalf("trash scene: %v", err)
	}
	for _, st := range []SceneTag{{SceneID: visible.ID, TagID: f.tags[0].ID}, {SceneID: trashed.ID, TagID: f.tags[1].ID}} {
		if err := db.Create(&st).Error; err != nil {
			t.Fatalf("scene tag: %v", err)
		}
	}

	end := 40
	markers := []*UserSceneMarker{
		{UserID: f.user.ID, SceneID: visible.ID, Timestamp: 10, Label: "Kiss", Color: "#FFFFFF", ThumbnailPath: "k1.webp"},
		{UserID: f.user.ID, SceneID: visible.ID, Timestamp: 30, EndTimestamp: &end, Label: "Kiss", Color: "#FFFFFF", AnimatedThumbnailPath: "k2.mp4"},
		{UserID: f.user.ID, SceneID: visible.ID, Timestamp: 50, Label: "Cum", Color: "#FFFFFF", ThumbnailPath: "c1.webp"},
		{UserID: f.user.ID, SceneID: trashed.ID, Timestamp: 5, Label: "Kiss", Color: "#FFFFFF", ThumbnailPath: "x.webp"},
	}
	for _, m := range markers {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
	}
	t.Cleanup(func() { db.Where("user_id = ?", f.user.ID).Delete(&UserSceneMarker{}) })

	t.Run("recent excludes trashed scenes", func(t *testing.T) {
		got, err := ListWidgetMarkers(db, f.user.ID, WidgetMarkerQuery{Order: "recent", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[0].ID != markers[2].ID || got[0].SceneTitle != "Visible" {
			t.Fatalf("unexpected markers: %+v", got)
		}
		if got[0].UpdatedAt.IsZero() {
			t.Fatal("widget markers miss updated_at (thumbnail version)")
		}
	})
	t.Run("labels, ranges and flags", func(t *testing.T) {
		got, err := ListWidgetMarkers(db, f.user.ID, WidgetMarkerQuery{Labels: []string{"Kiss"}, RangesOnly: true, Order: "random", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].EndTimestamp == nil || *got[0].EndTimestamp != 40 || !got[0].HasAnimated || got[0].HasThumbnail {
			t.Fatalf("unexpected markers: %+v", got)
		}
	})
	t.Run("label tiles", func(t *testing.T) {
		tiles, err := ListMarkerLabelTiles(db, f.user.ID, nil, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(tiles) != 2 || tiles[0].Label != "Kiss" || tiles[0].Count != 2 || tiles[0].CoverMarkerID == nil || *tiles[0].CoverMarkerID != markers[0].ID {
			t.Fatalf("unexpected label tiles: %+v", tiles)
		}
		if v := tiles[0].CoverMarkerVersion; v == nil || *v != MarkerThumbnailVersion(markers[0].UpdatedAt) {
			t.Fatalf("cover marker version = %v, want %d", v, MarkerThumbnailVersion(markers[0].UpdatedAt))
		}
		ordered, err := ListMarkerLabelTiles(db, f.user.ID, []string{"Cum", "Kiss"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(ordered) != 2 || ordered[0].Label != "Cum" {
			t.Fatalf("explicit order not kept: %+v", ordered)
		}
	})
	t.Run("tag tiles", func(t *testing.T) {
		tiles, err := ListSceneTagTiles(db, []string{f.tags[0].Name, f.tags[1].Name}, 10)
		if err != nil {
			t.Fatal(err)
		}
		// The second tag is only on a trashed scene
		if len(tiles) != 1 || tiles[0].Label != f.tags[0].Name || tiles[0].Count != 1 || tiles[0].CoverSceneID == nil || *tiles[0].CoverSceneID != visible.ID {
			t.Fatalf("unexpected tag tiles: %+v", tiles)
		}
	})
}
