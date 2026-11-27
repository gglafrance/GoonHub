package data

import (
	"testing"
	"time"

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
		{"query on the label, ignoring case", MarkerListOptions{Query: "hUG"}, 1},
		{"query on the scene title", MarkerListOptions{Query: "b"}, 1},
		{"live scenes only", MarkerListOptions{LiveScenesOnly: true}, 3},
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

	t.Run("live scenes only drops trashed scenes", func(t *testing.T) {
		if err := db.Model(&Scene{}).Where("id = ?", sceneB.ID).Update("trashed_at", time.Now()).Error; err != nil {
			t.Fatalf("trash scene: %v", err)
		}
		defer db.Model(&Scene{}).Where("id = ?", sceneB.ID).Update("trashed_at", nil)
		got, total, err := repo.GetAllMarkersForUser(f.user.ID, 0, 10, MarkerListOptions{LiveScenesOnly: true})
		if err != nil {
			t.Fatalf("GetAllMarkersForUser: %v", err)
		}
		if total != 2 || len(got) != 2 {
			t.Fatalf("got %d markers (total %d), want 2", len(got), total)
		}
		for _, m := range got {
			if m.SceneID == sceneB.ID {
				t.Fatalf("marker of a trashed scene listed: %+v", m)
			}
		}
	})

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
		if err := repo.UpdateThumbnailPath(markers[1].ID, "marker_hug.webp"); err != nil {
			t.Fatalf("UpdateThumbnailPath: %v", err)
		}
		saved, err := repo.GetByID(markers[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		got, err := repo.GetRandomThumbnailsForLabels(f.user.ID, []string{"Hug"}, 5, MarkerListOptions{})
		if err != nil {
			t.Fatalf("GetRandomThumbnailsForLabels: %v", err)
		}
		refs := got["Hug"]
		if len(refs) != 1 || refs[0].ID != markers[1].ID || !refs[0].UpdatedAt.Equal(saved.UpdatedAt) {
			t.Fatalf("unexpected thumbnail refs: %+v (marker updated at %v)", refs, saved.UpdatedAt)
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

// The thumbnail URLs are versioned by updated_at: writing a thumbnail path (the file is
// rewritten under the same name when a marker moves) must move it, and only that column
// must change (the user may have edited the marker meanwhile).
func TestMarkerRepositoryThumbnailPathBumpsUpdatedAt(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := &MarkerRepositoryImpl{DB: db}
	suffix := f.tags[0].Name[len("import-a-"):]

	scene := &Scene{Title: "T", StoredPath: "/import-test/" + suffix + "/thumb.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := db.Create(scene).Error; err != nil {
		t.Fatalf("create scene: %v", err)
	}
	marker := &UserSceneMarker{UserID: f.user.ID, SceneID: scene.ID, Timestamp: 10, Label: "Kiss", Color: "#FFFFFF", ThumbnailPath: "marker_x.webp"}
	if err := db.Create(marker).Error; err != nil {
		t.Fatalf("create marker: %v", err)
	}
	t.Cleanup(func() { db.Where("user_id = ?", f.user.ID).Delete(&UserSceneMarker{}) })

	// An old updated_at, as for a marker made a while ago
	old := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	if err := db.Model(&UserSceneMarker{}).Where("id = ?", marker.ID).UpdateColumn("updated_at", old).Error; err != nil {
		t.Fatalf("set old updated_at: %v", err)
	}

	tests := []struct {
		name   string
		update func(id uint, path string) error
		path   string
		get    func(m *UserSceneMarker) string
	}{
		{"static", repo.UpdateThumbnailPath, "marker_x.webp", func(m *UserSceneMarker) string { return m.ThumbnailPath }},
		{"animated", repo.UpdateAnimatedThumbnailPath, "marker_x.mp4", func(m *UserSceneMarker) string { return m.AnimatedThumbnailPath }},
	}
	previous := old
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.update(marker.ID, tt.path); err != nil {
				t.Fatalf("update: %v", err)
			}
			got, err := repo.GetByID(marker.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.get(got) != tt.path {
				t.Fatalf("path = %q, want %q", tt.get(got), tt.path)
			}
			if MarkerThumbnailVersion(got.UpdatedAt) <= MarkerThumbnailVersion(previous) {
				t.Fatalf("updated_at not bumped: %v, before %v", got.UpdatedAt, previous)
			}
			if got.Timestamp != 10 || got.Label != "Kiss" {
				t.Fatalf("other columns changed: %+v", got)
			}
			previous = got.UpdatedAt
		})
	}
}

// TestMarkerRepositorySharedMarkersVisibility checks that listings cover the viewer's markers
// plus those of the users sharing theirs, while counts used for writes stay per owner. It runs
// in a rolled back transaction: a sharing user would otherwise show up in the listings of the
// other tests using the same database.
func TestMarkerRepositorySharedMarkersVisibility(t *testing.T) {
	db := importTestDB(t)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	repo := &MarkerRepositoryImpl{DB: tx}

	suffix := time.Now().Format("150405.000000000")
	viewer := &User{Username: "shared-viewer-" + suffix, Password: "x", Role: "user"}
	owner := &User{Username: "shared-owner-" + suffix, Password: "x", Role: "user"}
	for _, u := range []*User{viewer, owner} {
		if err := tx.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	scene := &Scene{Title: "Shared", StoredPath: "/shared-test/" + suffix + ".mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := tx.Create(scene).Error; err != nil {
		t.Fatalf("create scene: %v", err)
	}
	label := "SharedLabel-" + suffix
	for _, m := range []*UserSceneMarker{
		{UserID: viewer.ID, SceneID: scene.ID, Timestamp: 10, Label: label, Color: "#FFFFFF"},
		{UserID: owner.ID, SceneID: scene.ID, Timestamp: 20, Label: label, Color: "#FFFFFF"},
		{UserID: owner.ID, SceneID: scene.ID, Timestamp: 30, Label: label, Color: "#FFFFFF"},
	} {
		if err := tx.Create(m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
	}

	check := func(t *testing.T, wantVisible int) {
		t.Helper()
		markers, err := repo.GetByUserAndScene(viewer.ID, scene.ID)
		if err != nil {
			t.Fatalf("GetByUserAndScene: %v", err)
		}
		if len(markers) != wantVisible {
			t.Fatalf("GetByUserAndScene: got %d markers, want %d", len(markers), wantVisible)
		}
		_, total, err := repo.GetMarkersByLabelForUser(viewer.ID, label, 0, 50, MarkerListOptions{})
		if err != nil {
			t.Fatalf("GetMarkersByLabelForUser: %v", err)
		}
		if total != int64(wantVisible) {
			t.Fatalf("GetMarkersByLabelForUser: got %d, want %d", total, wantVisible)
		}
		ids, err := repo.GetSceneIDsByLabels(viewer.ID, []string{label})
		if err != nil || len(ids) != 1 {
			t.Fatalf("GetSceneIDsByLabels: got %v (%v), want the scene", ids, err)
		}
		tiles, err := ListMarkerLabelTiles(tx, viewer.ID, []string{label}, 10)
		if err != nil {
			t.Fatalf("ListMarkerLabelTiles: %v", err)
		}
		if len(tiles) != 1 || tiles[0].Count != int64(wantVisible) {
			t.Fatalf("ListMarkerLabelTiles: got %+v, want one tile of %d", tiles, wantVisible)
		}
		count, err := repo.CountByUserAndScene(viewer.ID, scene.ID)
		if err != nil || count != 1 {
			t.Fatalf("CountByUserAndScene: got %d (%v), want the viewer's own 1", count, err)
		}
	}

	t.Run("not shared", func(t *testing.T) { check(t, 1) })
	if err := (&UserRepositoryImpl{DB: tx}).UpdateShareMarkers(owner.ID, true); err != nil {
		t.Fatalf("UpdateShareMarkers: %v", err)
	}
	t.Run("shared", func(t *testing.T) { check(t, 3) })
}
