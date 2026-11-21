package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of GetWatchedBySceneIDs (the "Watched" card badge), on the disposable
// import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestInteractionRepositoryGetWatchedBySceneIDs(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewInteractionRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	mk := func(name string) *Scene {
		s := &Scene{Title: name, StoredPath: "/import-test/" + suffix + "/" + name + ".mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	finished, partly, twice, never := mk("finished"), mk("partly"), mk("twice"), mk("never")
	watch := func(s *Scene, completed bool) {
		w := &UserSceneWatch{UserID: f.user.ID, SceneID: s.ID, WatchedAt: time.Now(), LastPosition: 10, Completed: completed}
		if err := db.Create(w).Error; err != nil {
			t.Fatalf("create watch: %v", err)
		}
	}
	watch(finished, true)
	watch(partly, false)
	watch(twice, false)
	watch(twice, true) // finished once out of two watches

	other := &User{Username: "import-test-other-" + suffix, Password: "x", Role: "user"}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(other) })
	if err := db.Create(&UserSceneWatch{UserID: other.ID, SceneID: never.ID, WatchedAt: time.Now(), Completed: true}).Error; err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetWatchedBySceneIDs(f.user.ID, []uint{finished.ID, partly.ID, twice.ID, never.ID})
	if err != nil {
		t.Fatalf("GetWatchedBySceneIDs: %v", err)
	}
	if len(got) != 2 || !got[finished.ID] || !got[twice.ID] {
		t.Fatalf("expected finished and twice, got %v", got)
	}
	if got[never.ID] {
		t.Fatal("another user's watch must not count")
	}

	empty, err := repo.GetWatchedBySceneIDs(f.user.ID, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("no scene IDs: got %v, %v", empty, err)
	}
}
