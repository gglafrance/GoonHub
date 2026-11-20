package data

import (
	"testing"

	"github.com/lib/pq"
)

// Integration tests for the marker listing filters and random sort. They reuse the import
// test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN, disposable) and fixtures.
func TestMarkerRepositoryListingFilters(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := &MarkerRepositoryImpl{DB: db}
	suffix := f.tags[0].Name[len("import-a-"):]

	studioID := f.studio.ID
	sceneA := &Scene{Title: "A", StoredPath: "/import-test/" + suffix + "/a.mp4", StudioID: &studioID, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	sceneB := &Scene{Title: "B", StoredPath: "/import-test/" + suffix + "/b.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{sceneA, sceneB} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}
	// Scene A has both tags and the actor; scene B only the first tag.
	for _, st := range []SceneTag{{SceneID: sceneA.ID, TagID: f.tags[0].ID}, {SceneID: sceneA.ID, TagID: f.tags[1].ID}, {SceneID: sceneB.ID, TagID: f.tags[0].ID}} {
		if err := db.Create(&st).Error; err != nil {
			t.Fatalf("create scene tag: %v", err)
		}
	}
	if err := db.Create(&SceneActor{SceneID: sceneA.ID, ActorID: f.actor.ID}).Error; err != nil {
		t.Fatalf("create scene actor: %v", err)
	}

	end := 30
	markers := []*UserSceneMarker{
		{UserID: f.user.ID, SceneID: sceneA.ID, Timestamp: 10, EndTimestamp: &end, Label: "Kiss", Color: "#FFFFFF"},
		{UserID: f.user.ID, SceneID: sceneA.ID, Timestamp: 40, Label: "Hug", Color: "#FFFFFF"},
		{UserID: f.user.ID, SceneID: sceneB.ID, Timestamp: 5, Label: "Kiss", Color: "#FFFFFF"},
	}
	for _, m := range markers {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Where("user_id = ?", f.user.ID).Delete(&UserSceneMarker{})
	})
	// Marker tags: first marker has both tags, the third only the first one
	for _, mt := range []MarkerTag{
		{MarkerID: markers[0].ID, TagID: f.tags[0].ID},
		{MarkerID: markers[0].ID, TagID: f.tags[1].ID},
		{MarkerID: markers[2].ID, TagID: f.tags[0].ID},
	} {
		if err := db.Create(&mt).Error; err != nil {
			t.Fatalf("create marker tag: %v", err)
		}
	}

	tagA, tagB := f.tags[0].Name, f.tags[1].Name
	tests := []struct {
		name string
		opts MarkerListOptions
		want int64
	}{
		{"no filter", MarkerListOptions{}, 3},
		{"ranges only", MarkerListOptions{RangesOnly: true}, 1},
		{"labels", MarkerListOptions{Labels: []string{"Kiss"}}, 2},
		{"one tag", MarkerListOptions{Tags: []string{tagA}}, 3},
		{"all tags required", MarkerListOptions{Tags: []string{tagA, tagB}}, 2},
		{"actor", MarkerListOptions{Actors: []string{f.actor.Name}}, 2},
		{"studio", MarkerListOptions{Studio: f.studio.Name}, 2},
		{"combined", MarkerListOptions{Labels: []string{"Kiss"}, Tags: []string{tagA}, RangesOnly: true}, 1},
		{"random sort", MarkerListOptions{SortBy: "random", Seed: 42}, 3},
		{"one marker tag", MarkerListOptions{MarkerTags: []string{tagA}}, 2},
		{"all marker tags required", MarkerListOptions{MarkerTags: []string{tagA, tagB}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := repo.GetAllMarkersForUser(f.user.ID, 0, 10, tt.opts)
			if err != nil {
				t.Fatalf("GetAllMarkersForUser: %v", err)
			}
			if total != tt.want || int64(len(got)) != tt.want {
				t.Fatalf("got %d markers (total %d), want %d", len(got), total, tt.want)
			}
		})
	}

	t.Run("random order is stable for a seed", func(t *testing.T) {
		opts := MarkerListOptions{SortBy: "random", Seed: 7}
		first, _, err := repo.GetAllMarkersForUser(f.user.ID, 0, 10, opts)
		if err != nil {
			t.Fatal(err)
		}
		second, _, err := repo.GetAllMarkersForUser(f.user.ID, 0, 10, opts)
		if err != nil {
			t.Fatal(err)
		}
		for i := range first {
			if first[i].ID != second[i].ID {
				t.Fatalf("random order changed between calls with the same seed")
			}
		}
	})

	t.Run("label groups", func(t *testing.T) {
		groups, total, err := repo.GetLabelGroupsForUser(f.user.ID, 0, 10, MarkerListOptions{SortBy: "random", Seed: 3, Tags: []string{tagB}})
		if err != nil {
			t.Fatalf("GetLabelGroupsForUser: %v", err)
		}
		if total != 2 || len(groups) != 2 {
			t.Fatalf("expected 2 label groups for scene A, got %d (total %d)", len(groups), total)
		}
		ranged, total, err := repo.GetLabelGroupsForUser(f.user.ID, 0, 10, MarkerListOptions{RangesOnly: true})
		if err != nil {
			t.Fatalf("GetLabelGroupsForUser ranges: %v", err)
		}
		if total != 1 || len(ranged) != 1 || ranged[0].Label != "Kiss" || ranged[0].Count != 1 || ranged[0].ThumbnailMarkerID != markers[0].ID {
			t.Fatalf("unexpected range groups: %+v (total %d)", ranged, total)
		}
	})

	t.Run("by label", func(t *testing.T) {
		got, total, err := repo.GetMarkersByLabelForUser(f.user.ID, "Kiss", 0, 10, MarkerListOptions{Actors: []string{f.actor.Name}})
		if err != nil {
			t.Fatalf("GetMarkersByLabelForUser: %v", err)
		}
		if total != 1 || len(got) != 1 || got[0].ID != markers[0].ID {
			t.Fatalf("unexpected markers: %+v (total %d)", got, total)
		}
	})

	t.Run("thumbnails", func(t *testing.T) {
		if _, err := repo.GetRandomThumbnailsForLabels(f.user.ID, []string{"Kiss"}, 5, MarkerListOptions{RangesOnly: true, Studio: f.studio.Name}); err != nil {
			t.Fatalf("GetRandomThumbnailsForLabels: %v", err)
		}
	})

	t.Run("by user and scenes", func(t *testing.T) {
		got, err := repo.GetByUserAndScenes(f.user.ID, []uint{sceneA.ID, sceneB.ID})
		if err != nil {
			t.Fatalf("GetByUserAndScenes: %v", err)
		}
		if len(got) != 3 || got[0].SceneID != sceneA.ID || got[0].Timestamp != 10 {
			t.Fatalf("unexpected markers: %+v", got)
		}
	})
}
