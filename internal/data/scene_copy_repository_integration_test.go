package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of the scene copies (library scan): CreateBatch records each path once and
// keeps the first row on a conflict, GetAllStoredPathSet and ListBySceneID return them,
// TouchLastSeen updates last_seen_at of the given paths only, GetScanLookupEntries reports
// trashed scenes, scan_history stores copies_found, and deleting a scene removes its copy rows.
// Runs on the disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestSceneCopyRepository(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	suffix := f.tags[0].Name[len("import-a-"):]
	repo := NewSceneCopyRepository(db)
	sceneRepo := NewSceneRepository(db)
	base := "/import-test/" + suffix

	scenes := make([]*Scene, 2)
	for i := range scenes {
		scenes[i] = &Scene{Title: fmt.Sprintf("copies-%d", i), Size: 1000, StoredPath: fmt.Sprintf("%s/scene-%d.mp4", base, i), Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(scenes[i]).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}
	a, b := scenes[0], scenes[1]
	paths := []string{base + "/copy-2.mp4", base + "/copy-1.mp4", base + "/copy-3.mp4"}
	t.Cleanup(func() { db.Where("stored_path LIKE ?", base+"/%").Delete(&SceneCopy{}) })

	if err := repo.CreateBatch([]*SceneCopy{
		{SceneID: a.ID, StoredPath: paths[0], Size: 1000, Oshash: "00000000000003e8"},
		{SceneID: a.ID, StoredPath: paths[1], Size: 1000, Oshash: "00000000000003e8"},
		{SceneID: b.ID, StoredPath: paths[2], Size: 1000, Oshash: "00000000000003e9"},
	}); err != nil {
		t.Fatalf("create copies: %v", err)
	}
	// A path already recorded is left as it is
	if err := repo.CreateBatch([]*SceneCopy{{SceneID: b.ID, StoredPath: paths[0], Size: 1, Oshash: "ffffffffffffffff"}}); err != nil {
		t.Fatalf("create a recorded path again: %v", err)
	}

	set, err := repo.GetAllStoredPathSet()
	if err != nil {
		t.Fatalf("path set: %v", err)
	}
	for _, p := range paths {
		if _, ok := set[p]; !ok {
			t.Fatalf("path %s missing from the set", p)
		}
	}

	list, err := repo.ListBySceneID(a.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].StoredPath != paths[1] || list[1].StoredPath != paths[0] {
		t.Fatalf("copies of scene a not ordered by path: %+v", list)
	}
	for _, c := range list {
		if c.SceneID != a.ID || c.Oshash != "00000000000003e8" || c.Size != 1000 || c.CreatedAt.IsZero() || c.LastSeenAt.IsZero() {
			t.Fatalf("copy row incomplete or overwritten: %+v", c)
		}
	}
	if empty, err := repo.ListBySceneID(0); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("a scene without copies must give an empty list: %v (%v)", empty, err)
	}

	// TouchLastSeen updates the given paths only
	seen := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	if err := repo.TouchLastSeen([]string{paths[0], paths[2], base + "/unknown.mp4"}, seen); err != nil {
		t.Fatalf("touch: %v", err)
	}
	var rows []SceneCopy
	if err := db.Where("stored_path LIKE ?", base+"/%").Order("stored_path").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		touched := r.LastSeenAt.Equal(seen)
		if touched == (r.StoredPath == paths[1]) {
			t.Fatalf("last_seen_at of %s = %v (touched %v)", r.StoredPath, r.LastSeenAt, touched)
		}
	}

	// GetScanLookupEntries reports trashed scenes
	if _, err := sceneRepo.MoveToTrash(b.ID); err != nil {
		t.Fatalf("trash: %v", err)
	}
	entries, err := sceneRepo.GetScanLookupEntries()
	if err != nil {
		t.Fatalf("lookup entries: %v", err)
	}
	found := 0
	for _, e := range entries {
		switch e.ID {
		case a.ID:
			found++
			if e.IsTrashed || e.IsDeleted {
				t.Fatalf("scene a flagged: %+v", e)
			}
		case b.ID:
			found++
			if !e.IsTrashed {
				t.Fatalf("trashed scene b not flagged: %+v", e)
			}
		}
	}
	if found != 2 {
		t.Fatalf("lookup entries missing the test scenes (%d)", found)
	}

	// Permanently deleting a scene removes its copy rows (and nothing else)
	if _, err := sceneRepo.HardDelete(a.ID); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	if list, err := repo.ListBySceneID(a.ID); err != nil || len(list) != 0 {
		t.Fatalf("copies of a deleted scene remain: %+v (%v)", list, err)
	}
	if list, err := repo.ListBySceneID(b.ID); err != nil || len(list) != 1 {
		t.Fatalf("copies of scene b: %+v (%v)", list, err)
	}

	// scan_history.copies_found round trip
	scanRepo := NewScanHistoryRepository(db)
	scan := &ScanHistory{Status: "completed", StartedAt: time.Now(), CreatedAt: time.Now(), CopiesFound: 42}
	if err := scanRepo.Create(scan); err != nil {
		t.Fatalf("create scan: %v", err)
	}
	t.Cleanup(func() { db.Delete(&ScanHistory{}, scan.ID) })
	got, err := scanRepo.GetByID(scan.ID)
	if err != nil || got == nil || got.CopiesFound != 42 {
		t.Fatalf("copies_found round trip: %+v (%v)", got, err)
	}
}
