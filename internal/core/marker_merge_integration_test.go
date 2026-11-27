package core

import (
	"os"
	"slices"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/persistence/migrator"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Markers moved to the kept scene of a duplicate group and folded there, against PostgreSQL
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a disposable database): times shifted by the offset,
// marker tags following their marker and united on the kept duplicate, a marker outside the
// match left on the trashed scene with its tags.
func TestMergeMarkers_PostgreSQL(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_IMPORT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database) to run marker merge integration tests")
	}
	if err := migrator.Run(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test connection: %v", err)
		}
	})

	suffix := uuid.NewString()[:8]
	user := &data.User{Username: "marker-merge-" + suffix, Password: "x", Role: "user"}
	tags := []data.Tag{{Name: "mm-a-" + suffix}, {Name: "mm-b-" + suffix}}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&tags).Error; err != nil {
		t.Fatalf("create tags: %v", err)
	}
	newScene := func(name string, duration int) *data.Scene {
		s := &data.Scene{Title: name, StoredPath: "/marker-merge-test/" + suffix + "/" + name + ".mp4", Duration: duration, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	kept, trashed, other := newScene("compilation", 600), newScene("clip", 90), newScene("other", 300)
	t.Cleanup(func() {
		for _, res := range []*gorm.DB{
			db.Where("user_id = ?", user.ID).Delete(&data.UserSceneMarker{}),
			db.Unscoped().Where("stored_path LIKE ?", "/marker-merge-test/"+suffix+"/%").Delete(&data.Scene{}),
			db.Delete(&tags),
			db.Delete(user),
		} {
			if res.Error != nil {
				t.Errorf("cleanup: %v", res.Error)
			}
		}
	})

	t0 := time.Now().Add(-time.Hour)
	end := 20
	marker := func(sceneID uint, ts int, end *int, label string, created time.Time, tagIDs ...uint) *data.UserSceneMarker {
		m := &data.UserSceneMarker{UserID: user.ID, SceneID: sceneID, Timestamp: ts, EndTimestamp: end, Label: label, Color: "#FFFFFF", CreatedAt: created}
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
		for _, id := range tagIDs {
			if err := db.Create(&data.MarkerTag{MarkerID: m.ID, TagID: id}).Error; err != nil {
				t.Fatalf("create marker tag: %v", err)
			}
		}
		return m
	}
	// Already on the kept scene, the same moment as the clip's first marker
	existing := marker(kept.ID, 80, nil, "Kiss", t0, tags[0].ID)
	// On the clip (70 s into the compilation)
	kiss := marker(trashed.ID, 10, &end, " kiss", t0.Add(time.Minute), tags[1].ID)
	hug := marker(trashed.ID, 50, nil, "Hug", t0.Add(2*time.Minute), tags[0].ID)
	credits := marker(trashed.ID, 95, nil, "Credits", t0.Add(3*time.Minute), tags[1].ID)
	// Two copies on another scene, for the maintenance listing
	marker(other.ID, 30, nil, "Kiss", t0)
	marker(other.ID, 31, nil, "", t0)

	cfg := &config.Config{}
	cfg.Processing.MarkerThumbnailDir = t.TempDir()
	markerRepo := data.NewMarkerRepository(db)
	markerSvc := NewMarkerService(markerRepo, data.NewMarkerLabelRepository(db), data.NewSceneRepository(db), nil, cfg, zap.NewNop())
	svc := NewDuplicateService(nil, nil, nil, nil, markerRepo, nil, nil, zap.NewNop())
	svc.SetMarkerService(markerSvc)

	ref := kept.ID
	group := &data.DuplicateGroup{ID: 1, Members: []data.DuplicateGroupMember{
		{SceneID: kept.ID},
		{SceneID: trashed.ID, RefSceneID: &ref, MatchStart: ptr[float64](0), MatchEnd: ptr[float64](90),
			RefStart: ptr[float64](70), RefEnd: ptr[float64](160), Relation: data.DuplicateRelationContained},
	}}
	scenes := map[uint]data.Scene{kept.ID: *kept, trashed.ID: *trashed}

	summary, err := svc.mergeMarkers(group, kept.ID, []uint{trashed.ID}, scenes)
	if err != nil {
		t.Fatal(err)
	}
	if summary != (MarkerMergeSummary{Moved: 2, Shifted: 2, Deduplicated: 1, LeftBehind: 1}) {
		t.Fatalf("unexpected summary %+v", summary)
	}

	onKept, err := markerRepo.GetAllByScene(kept.ID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uint]data.UserSceneMarker{}
	for _, m := range onKept {
		byID[m.ID] = m
	}
	if len(onKept) != 2 {
		t.Fatalf("kept scene markers = %+v, want the moved kiss and hug", onKept)
	}
	// The moved kiss has an end: kept over the existing one, which is deleted
	if m, ok := byID[kiss.ID]; !ok || m.Timestamp != 80 || m.EndTimestamp == nil || *m.EndTimestamp != 90 {
		t.Fatalf("kiss not moved with its offset: %+v", byID[kiss.ID])
	}
	if m, ok := byID[hug.ID]; !ok || m.Timestamp != 120 || m.EndTimestamp != nil {
		t.Fatalf("hug not moved with its offset: %+v", byID[hug.ID])
	}
	if _, err := markerRepo.GetByID(existing.ID); err == nil {
		t.Fatal("the duplicate of the moved kiss should be deleted")
	}

	tagsOf := func(id uint) []uint {
		t.Helper()
		got, err := markerRepo.GetMarkerTags(id)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]uint, 0, len(got))
		for _, tag := range got {
			ids = append(ids, tag.ID)
		}
		slices.Sort(ids)
		return ids
	}
	if got := tagsOf(kiss.ID); !slices.Equal(got, []uint{tags[0].ID, tags[1].ID}) {
		t.Fatalf("kiss tags = %v, want both (its own and the deleted duplicate's)", got)
	}
	if got := tagsOf(hug.ID); !slices.Equal(got, []uint{tags[0].ID}) {
		t.Fatalf("hug tags = %v, want its own", got)
	}
	var orphanTags int64
	if err := db.Model(&data.MarkerTag{}).Where("marker_id = ?", existing.ID).Count(&orphanTags).Error; err != nil || orphanTags != 0 {
		t.Fatalf("tags of the deleted duplicate should go with it: %d, %v", orphanTags, err)
	}

	// Outside the match: left on the trashed scene, with its tag
	left, err := markerRepo.GetAllByScene(trashed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].ID != credits.ID || left[0].Timestamp != 95 {
		t.Fatalf("trashed scene markers = %+v, want the credits only", left)
	}
	if got := tagsOf(credits.ID); !slices.Equal(got, []uint{tags[1].ID}) {
		t.Fatalf("credits tags = %v", got)
	}

	// Maintenance: the other scene is a candidate, the kept one no longer is
	candidates, err := markerRepo.ListSceneIDsWithNearbyMarkers(markerDuplicateToleranceSeconds)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(candidates, other.ID) || slices.Contains(candidates, kept.ID) {
		t.Fatalf("candidates %v should hold scene %d and not %d", candidates, other.ID, kept.ID)
	}
	dry, err := markerSvc.DeduplicateMarkers([]uint{other.ID}, true)
	if err != nil || dry.Removed != 1 {
		t.Fatalf("dry run = %+v, %v", dry, err)
	}
	if n, _ := markerRepo.GetAllByScene(other.ID); len(n) != 2 {
		t.Fatalf("a dry run must not delete anything, %d markers left", len(n))
	}
	applied, err := markerSvc.DeduplicateMarkers([]uint{other.ID}, false)
	if err != nil || applied.Removed != 1 {
		t.Fatalf("apply = %+v, %v", applied, err)
	}
	if n, _ := markerRepo.GetAllByScene(other.ID); len(n) != 1 || n[0].Label != "Kiss" {
		t.Fatalf("expected one labelled marker left, got %+v", n)
	}
}
