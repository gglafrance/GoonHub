package data

import (
	"math"
	"reflect"
	"testing"

	"github.com/lib/pq"
)

// Integration test of the replay heatmap upsert. It reuses the import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN, disposable) and fixtures.
func TestReplayHeatmapRepository(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewReplayHeatmapRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	scene := &Scene{Title: "Heatmap", StoredPath: "/import-test/" + suffix + "/heatmap.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	other := &Scene{Title: "Other", StoredPath: "/import-test/" + suffix + "/other.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{scene, other} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}

	got, err := repo.GetBuckets(f.user.ID, scene.ID)
	if err != nil || got != nil {
		t.Fatalf("expected no heatmap, got %v, %v", got, err)
	}

	first := []int32{1000, 0, 500, 0}
	got, err = repo.AddBuckets(f.user.ID, scene.ID, first)
	if err != nil {
		t.Fatalf("first add: %v", err)
	}
	if !reflect.DeepEqual(got, first) {
		t.Fatalf("first add returned %v", got)
	}

	// Rewatching adds up element-wise and saturates at the INTEGER maximum
	got, err = repo.AddBuckets(f.user.ID, scene.ID, []int32{250, 0, math.MaxInt32, 7})
	if err != nil {
		t.Fatalf("second add: %v", err)
	}
	want := []int32{1250, 0, math.MaxInt32, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("second add returned %v, want %v", got, want)
	}
	if stored, err := repo.GetBuckets(f.user.ID, scene.ID); err != nil || !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored %v, %v; want %v", stored, err, want)
	}

	// Another bucket count keeps the longer histogram's extra elements
	got, err = repo.AddBuckets(f.user.ID, scene.ID, []int32{1, 1, 1, 1, 9, 9})
	if err != nil {
		t.Fatalf("longer add: %v", err)
	}
	if want := []int32{1251, 1, math.MaxInt32, 8, 9, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("longer add returned %v, want %v", got, want)
	}

	// Histograms are per scene
	if _, err := repo.AddBuckets(f.user.ID, other.ID, []int32{5}); err != nil {
		t.Fatalf("other scene add: %v", err)
	}
	if stored, _ := repo.GetBuckets(f.user.ID, other.ID); !reflect.DeepEqual(stored, []int32{5}) {
		t.Fatalf("other scene stored %v", stored)
	}

	// Deleting a scene for good removes its histograms
	if err := db.Unscoped().Delete(other).Error; err != nil {
		t.Fatalf("delete scene: %v", err)
	}
	var count int64
	if err := db.Model(&UserSceneHeatmap{}).Where("scene_id = ?", other.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("expected the histogram to be deleted with the scene, count %d, %v", count, err)
	}
	// The fixture cleanup removes the remaining scene and the user (cascading to the rest)
}
