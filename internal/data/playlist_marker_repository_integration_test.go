package data

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestMarkerPlayRange(t *testing.T) {
	end := 50
	tests := []struct {
		name                      string
		timestamp                 int
		end                       *int
		sceneDuration, defaultDur int
		wantStart, wantEnd        int
	}{
		{"range marker", 10, &end, 600, 30, 10, 50},
		{"point marker", 10, nil, 600, 30, 10, 40},
		{"cut at the scene end", 590, nil, 600, 30, 590, 600},
		{"unknown scene duration", 590, nil, 0, 30, 590, 620},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e := MarkerPlayRange(tt.timestamp, tt.end, tt.sceneDuration, tt.defaultDur)
			if s != tt.wantStart || e != tt.wantEnd {
				t.Fatalf("got %d-%d, want %d-%d", s, e, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// Integration test of the marker playlist queries, on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestPlaylistMarkerRepository(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewPlaylistRepository(db)
	suffix := uuid.NewString()[:8]

	// f.user owns the playlist; sharer shares their markers, private does not
	sharer := User{Username: "pl-sharer-" + suffix, Password: "x", Role: "user", ShareMarkers: true}
	private := User{Username: "pl-private-" + suffix, Password: "x", Role: "user"}
	for _, u := range []*User{&sharer, &private} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	t.Cleanup(func() { db.Delete(&[]User{sharer, private}) })

	base := "/import-test/" + f.user.Username[len("import-test-"):] + "/"
	sceneA := &Scene{Title: "A", StoredPath: base + "a.mp4", Duration: 600, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	sceneB := &Scene{Title: "B", StoredPath: base + "b.mp4", Duration: 100, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	trashedAt := time.Now()
	sceneT := &Scene{Title: "T", StoredPath: base + "t.mp4", Duration: 100, TrashedAt: &trashedAt, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{sceneA, sceneB, sceneT} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}

	end := 40
	own1 := &UserSceneMarker{UserID: f.user.ID, SceneID: sceneA.ID, Timestamp: 10, EndTimestamp: &end, Label: "Kiss", Color: "#FFFFFF", ThumbnailPath: "own1.webp"}
	own2 := &UserSceneMarker{UserID: f.user.ID, SceneID: sceneB.ID, Timestamp: 90, Label: "Kiss", Color: "#FFFFFF"}
	shared := &UserSceneMarker{UserID: sharer.ID, SceneID: sceneA.ID, Timestamp: 100, Label: "Hug", Color: "#FFFFFF", ThumbnailPath: "shared.webp"}
	hidden := &UserSceneMarker{UserID: private.ID, SceneID: sceneA.ID, Timestamp: 200, Label: "Kiss", Color: "#FFFFFF"}
	inTrash := &UserSceneMarker{UserID: f.user.ID, SceneID: sceneT.ID, Timestamp: 5, Label: "Kiss", Color: "#FFFFFF"}
	extra := &UserSceneMarker{UserID: f.user.ID, SceneID: sceneA.ID, Timestamp: 300, Label: "Other", Color: "#FFFFFF"}
	for _, m := range []*UserSceneMarker{own1, own2, shared, hidden, inTrash, extra} {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Where("user_id IN ?", []uint{f.user.ID, sharer.ID, private.ID}).Delete(&UserSceneMarker{})
	})

	playlist := &Playlist{UserID: f.user.ID, Name: "Moments " + suffix, Visibility: "private", Kind: PlaylistKindMarkers}
	scenesPlaylist := &Playlist{UserID: f.user.ID, Name: "Scenes " + suffix, Visibility: "private"}
	for _, p := range []*Playlist{playlist, scenesPlaylist} {
		if err := repo.Create(p); err != nil {
			t.Fatalf("create playlist: %v", err)
		}
	}
	t.Cleanup(func() { db.Where("user_id = ?", f.user.ID).Delete(&Playlist{}) })

	t.Run("visible markers", func(t *testing.T) {
		got, err := repo.FilterVisibleMarkerIDs(f.user.ID, []uint{shared.ID, hidden.ID, own1.ID, inTrash.ID, shared.ID})
		if err != nil {
			t.Fatalf("FilterVisibleMarkerIDs: %v", err)
		}
		if len(got) != 2 || got[0] != shared.ID || got[1] != own1.ID {
			t.Fatalf("got %v, want [%d %d]", got, shared.ID, own1.ID)
		}
	})

	t.Run("resolve a label listing", func(t *testing.T) {
		got, err := repo.ResolveVisibleMarkerIDs(f.user.ID, "Kiss", MarkerListOptions{}, 10)
		if err != nil {
			t.Fatalf("ResolveVisibleMarkerIDs: %v", err)
		}
		// hidden (not shared) and inTrash (trashed scene) are left out; newest first
		if len(got) != 2 || got[0] != own2.ID || got[1] != own1.ID {
			t.Fatalf("got %v, want [%d %d]", got, own2.ID, own1.ID)
		}
		got, err = repo.ResolveVisibleMarkerIDs(f.user.ID, "", MarkerListOptions{RangesOnly: true}, 10)
		if err != nil || len(got) != 1 || got[0] != own1.ID {
			t.Fatalf("ranges only: got %v, %v", got, err)
		}
	})

	t.Run("add, dedupe and limit", func(t *testing.T) {
		added, err := repo.AddMarkers(playlist.ID, []uint{own1.ID, shared.ID, own1.ID}, 3)
		if err != nil || added != 2 {
			t.Fatalf("add: added %d, err %v", added, err)
		}
		added, err = repo.AddMarkers(playlist.ID, []uint{own1.ID}, 3)
		if err != nil || added != 0 {
			t.Fatalf("add again: added %d, err %v", added, err)
		}
		// two new markers would make 4 > 3: nothing is added
		_, err = repo.AddMarkers(playlist.ID, []uint{own2.ID, extra.ID}, 3)
		if !IsPlaylistFull(err) {
			t.Fatalf("expected full playlist, got %v", err)
		}
		added, err = repo.AddMarkers(playlist.ID, []uint{own2.ID}, 3)
		if err != nil || added != 1 {
			t.Fatalf("add last: added %d, err %v", added, err)
		}
		rows, err := repo.GetPlaylistMarkers(playlist.ID)
		if err != nil || len(rows) != 3 {
			t.Fatalf("rows %d, err %v", len(rows), err)
		}
		if rows[0].MarkerID != own1.ID || rows[1].MarkerID != shared.ID || rows[2].MarkerID != own2.ID {
			t.Fatalf("order = %d %d %d", rows[0].MarkerID, rows[1].MarkerID, rows[2].MarkerID)
		}
		if rows[1].MarkerUserID != sharer.ID || rows[1].SceneTitle != "A" || rows[1].SceneDuration != 600 {
			t.Fatalf("row = %+v", rows[1])
		}
	})

	t.Run("reorder", func(t *testing.T) {
		if err := repo.ReorderMarkers(playlist.ID, []uint{own2.ID, own1.ID, shared.ID}); err != nil {
			t.Fatalf("ReorderMarkers: %v", err)
		}
		rows, err := repo.GetPlaylistMarkers(playlist.ID)
		if err != nil || len(rows) != 3 || rows[0].MarkerID != own2.ID || rows[1].MarkerID != own1.ID || rows[2].MarkerID != shared.ID {
			t.Fatalf("after reorder: %+v, %v", rows, err)
		}
		if rows[0].Position != 0 || rows[2].Position != 2 {
			t.Fatalf("positions %d %d", rows[0].Position, rows[2].Position)
		}
	})

	t.Run("stats and thumbnails", func(t *testing.T) {
		stats, err := repo.GetMarkerStats(playlist.ID, 30)
		if err != nil {
			t.Fatalf("GetMarkerStats: %v", err)
		}
		// own1 10-40 (30), shared 100-130 (30), own2 90-100 (cut at the end of B: 10)
		if stats.MarkerCount != 3 || stats.SceneCount != 2 || stats.TotalDuration != 70 {
			t.Fatalf("stats = %+v", stats)
		}
		refs, err := repo.GetThumbnailMarkers(playlist.ID, 4)
		if err != nil || len(refs) != 2 || refs[0].ID != own1.ID || refs[1].ID != shared.ID {
			t.Fatalf("thumbnails = %+v, %v", refs, err)
		}
		found, sceneID, err := repo.HasMarker(playlist.ID, shared.ID)
		if err != nil || !found || sceneID != sceneA.ID {
			t.Fatalf("HasMarker = %v %d %v", found, sceneID, err)
		}
		if found, _, _ := repo.HasMarker(playlist.ID, extra.ID); found {
			t.Fatal("extra is not in the playlist")
		}
	})

	t.Run("progress", func(t *testing.T) {
		if err := repo.UpsertProgress(f.user.ID, playlist.ID, sceneA.ID, &shared.ID, 12); err != nil {
			t.Fatalf("UpsertProgress: %v", err)
		}
		if err := repo.UpsertProgress(f.user.ID, playlist.ID, sceneB.ID, &own2.ID, 3); err != nil {
			t.Fatalf("UpsertProgress again: %v", err)
		}
		p, err := repo.GetProgress(f.user.ID, playlist.ID)
		if err != nil || p == nil || p.LastMarkerID == nil || *p.LastMarkerID != own2.ID || *p.LastSceneID != sceneB.ID || p.LastPositionS != 3 {
			t.Fatalf("progress = %+v, %v", p, err)
		}
	})

	t.Run("trashed scene and deleted marker", func(t *testing.T) {
		// A marker of a scene sent to the trash is kept but not listed
		if err := db.Model(&Scene{}).Where("id = ?", sceneB.ID).Update("trashed_at", time.Now()).Error; err != nil {
			t.Fatalf("trash: %v", err)
		}
		rows, err := repo.GetPlaylistMarkers(playlist.ID)
		if err != nil || len(rows) != 2 {
			t.Fatalf("rows with B trashed: %d, %v", len(rows), err)
		}
		// A marker deleted by its owner leaves the playlist, and the progress forgets it
		if err := db.Delete(&UserSceneMarker{}, own2.ID).Error; err != nil {
			t.Fatalf("delete marker: %v", err)
		}
		var count int64
		db.Model(&PlaylistMarker{}).Where("playlist_id = ?", playlist.ID).Count(&count)
		if count != 2 {
			t.Fatalf("playlist rows after delete = %d, want 2", count)
		}
		p, err := repo.GetProgress(f.user.ID, playlist.ID)
		if err != nil || p == nil || p.LastMarkerID != nil {
			t.Fatalf("progress after delete = %+v, %v", p, err)
		}
	})

	t.Run("remove", func(t *testing.T) {
		removed, err := repo.RemoveMarkers(playlist.ID, []uint{own1.ID, extra.ID})
		if err != nil || removed != 1 {
			t.Fatalf("RemoveMarkers = %d, %v", removed, err)
		}
	})

	t.Run("list by kind", func(t *testing.T) {
		for kind, want := range map[string]uint{PlaylistKindMarkers: playlist.ID, PlaylistKindScenes: scenesPlaylist.ID} {
			items, _, err := repo.List(PlaylistListParams{UserID: f.user.ID, Owner: "me", Kind: kind, Limit: 10})
			if err != nil || len(items) != 1 || items[0].ID != want {
				t.Fatalf("kind %s: %+v, %v", kind, items, err)
			}
		}
		items, _, err := repo.List(PlaylistListParams{UserID: f.user.ID, Owner: "me", Sort: "scene_count_desc", Limit: 10})
		if err != nil || len(items) != 2 || items[0].ID != playlist.ID {
			t.Fatalf("item count sort: %+v, %v", items, err)
		}
	})

	t.Run("concurrent adds respect the limit", func(t *testing.T) {
		p := &Playlist{UserID: f.user.ID, Name: "Race " + suffix, Visibility: "private", Kind: PlaylistKindMarkers}
		if err := repo.Create(p); err != nil {
			t.Fatalf("create: %v", err)
		}
		ids := []uint{own1.ID, shared.ID, extra.ID, hidden.ID, inTrash.ID}
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func(id uint) {
				defer wg.Done()
				_, _ = repo.AddMarkers(p.ID, []uint{id}, 3)
			}(id)
		}
		wg.Wait()
		var rows []PlaylistMarker
		db.Where("playlist_id = ?", p.ID).Order("position").Find(&rows)
		if len(rows) != 3 {
			t.Fatalf("rows = %d, want 3", len(rows))
		}
		for i, r := range rows {
			if r.Position != i {
				t.Fatalf("positions = %+v", rows)
			}
		}
	})
}
