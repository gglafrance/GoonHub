package data

import (
	"os"
	"testing"
	"time"

	"goonhub/internal/infrastructure/persistence/migrator"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// importTestDB opens a PostgreSQL database for the import repository integration tests.
// GOONHUB_TEST_IMPORT_POSTGRES_DSN must point to a DISPOSABLE database: all migrations
// are applied to it and test rows are written (and cleaned up) in it.
func importTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GOONHUB_TEST_IMPORT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database) to run import repository integration tests")
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
	return db
}

type importFixture struct {
	user   User
	tags   []Tag
	actor  Actor
	studio Studio
}

func newImportFixture(t *testing.T, db *gorm.DB) *importFixture {
	t.Helper()
	suffix := uuid.NewString()[:8]
	f := &importFixture{
		user:   User{Username: "import-test-" + suffix, Password: "x", Role: "admin"},
		tags:   []Tag{{Name: "import-a-" + suffix}, {Name: "import-b-" + suffix}},
		actor:  Actor{Name: "Import Actor " + suffix},
		studio: Studio{Name: "Import Studio " + suffix},
	}
	for _, v := range []any{&f.user, &f.tags, &f.actor, &f.studio} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, res := range []*gorm.DB{
			db.Unscoped().Where("stored_path LIKE ?", "/import-test/"+suffix+"/%").Delete(&Scene{}),
			db.Delete(&f.tags),
			db.Unscoped().Delete(&f.actor),
			db.Unscoped().Delete(&f.studio),
			db.Delete(&f.user),
		} {
			if res.Error != nil {
				t.Errorf("cleanup: %v", res.Error)
			}
		}
	})
	return f
}

func TestImportRepository_SceneTransaction(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewImportRepository(db)
	base := "/import-test/" + f.user.Username[len("import-test-"):] + "/"

	watchedAt := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	var sceneID uint
	err := repo.RunInTransaction(func(tx ImportRepository) error {
		scene := &Scene{
			Title:      "Imported",
			StoredPath: base + "a.mp4",
			StudioID:   &f.studio.ID,
			Studio:     f.studio.Name,
			Tags:       pq.StringArray{f.tags[0].Name, f.tags[1].Name},
			Actors:     pq.StringArray{f.actor.Name},
			CreatedAt:  time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
			ViewCount:  5,
		}
		if err := tx.CreateScene(scene); err != nil {
			return err
		}
		sceneID = scene.ID
		if err := tx.SetSceneTags(sceneID, []uint{f.tags[0].ID, f.tags[1].ID}); err != nil {
			return err
		}
		if err := tx.SetSceneActors(sceneID, []uint{f.actor.ID}); err != nil {
			return err
		}
		if err := tx.UpsertRating(f.user.ID, sceneID, 4.5); err != nil {
			return err
		}
		if err := tx.SetLiked(f.user.ID, sceneID, true); err != nil {
			return err
		}
		if err := tx.SetJizzedCount(f.user.ID, sceneID, 3); err != nil {
			return err
		}
		inserted, err := tx.InsertWatchIfAbsent(&UserSceneWatch{UserID: f.user.ID, SceneID: sceneID, WatchedAt: watchedAt, WatchDuration: 10})
		if err != nil || !inserted {
			t.Fatalf("expected first watch inserted, got inserted=%v err=%v", inserted, err)
		}
		inserted, err = tx.InsertWatchIfAbsent(&UserSceneWatch{UserID: f.user.ID, SceneID: sceneID, WatchedAt: watchedAt.Add(400 * time.Millisecond)})
		if err != nil || inserted {
			t.Fatalf("expected duplicate watch (same second) skipped, got inserted=%v err=%v", inserted, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	var stored Scene
	if err := db.First(&stored, sceneID).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.CreatedAt.Equal(time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("created_at not preserved: %v", stored.CreatedAt)
	}
	if len(stored.Tags) != 2 || len(stored.Actors) != 1 || stored.Studio != f.studio.Name || stored.ViewCount != 5 {
		t.Fatalf("unexpected stored scene: %+v", stored)
	}
	var tagCount, actorCount int64
	db.Model(&SceneTag{}).Where("scene_id = ?", sceneID).Count(&tagCount)
	db.Model(&SceneActor{}).Where("scene_id = ?", sceneID).Count(&actorCount)
	if tagCount != 2 || actorCount != 1 {
		t.Fatalf("expected 2 scene tags and 1 scene actor, got %d / %d", tagCount, actorCount)
	}

	// Update: clear release date, replace tags, exact jizzed count reset, unlike.
	err = repo.RunInTransaction(func(tx ImportRepository) error {
		if err := tx.UpdateSceneFields(sceneID, map[string]any{
			"title": "Updated", "release_date": nil, "tags": pq.StringArray{f.tags[0].Name},
		}); err != nil {
			return err
		}
		if err := tx.SetSceneTags(sceneID, []uint{f.tags[0].ID}); err != nil {
			return err
		}
		if err := tx.SetLiked(f.user.ID, sceneID, false); err != nil {
			return err
		}
		return tx.SetJizzedCount(f.user.ID, sceneID, 0)
	})
	if err != nil {
		t.Fatalf("update transaction failed: %v", err)
	}
	stored = Scene{}
	db.First(&stored, sceneID)
	if stored.Title != "Updated" || stored.ReleaseDate != nil || len(stored.Tags) != 1 {
		t.Fatalf("unexpected updated scene: %+v", stored)
	}
	var likeCount, jizzCount int64
	db.Model(&UserSceneLike{}).Where("scene_id = ?", sceneID).Count(&likeCount)
	db.Model(&UserSceneJizzed{}).Where("scene_id = ?", sceneID).Count(&jizzCount)
	if likeCount != 0 || jizzCount != 0 {
		t.Fatalf("expected like and jizzed rows removed, got %d / %d", likeCount, jizzCount)
	}

	// Summary helpers
	scenes, err := repo.GetScenesForSummary([]uint{sceneID, 0})
	if err != nil || len(scenes) != 1 || scenes[0].StoredPath != base+"a.mp4" {
		t.Fatalf("unexpected summary scenes: %+v err=%v", scenes, err)
	}
	watches, err := repo.CountWatchesByScenes(f.user.ID, []uint{sceneID})
	if err != nil || watches[sceneID] != 1 {
		t.Fatalf("expected 1 watch, got %v err=%v", watches, err)
	}
}

func TestImportRepository_RollbackOnFailure(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewImportRepository(db)
	path := "/import-test/" + f.user.Username[len("import-test-"):] + "/rollback.mp4"

	err := repo.RunInTransaction(func(tx ImportRepository) error {
		if err := tx.CreateScene(&Scene{Title: "Rollback", StoredPath: path, Tags: pq.StringArray{}, Actors: pq.StringArray{}}); err != nil {
			return err
		}
		// Unknown tag: FK violation inside the nested (savepoint) transaction.
		return tx.SetSceneTags(0, []uint{f.tags[0].ID})
	})
	if err == nil {
		t.Fatal("expected the transaction to fail")
	}
	var count int64
	db.Model(&Scene{}).Where("stored_path = ?", path).Count(&count)
	if count != 0 {
		t.Fatalf("expected scene creation to be rolled back, found %d rows", count)
	}
}

func TestImportRepository_MarkerDedupAndCounts(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewImportRepository(db)
	scene := &Scene{Title: "M", StoredPath: "/import-test/" + f.user.Username[len("import-test-"):] + "/m.mp4",
		Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := repo.CreateScene(scene); err != nil {
		t.Fatal(err)
	}

	found, err := repo.FindMarker(f.user.ID, scene.ID, 10, "Intro")
	if err != nil || found != nil {
		t.Fatalf("expected no marker, got %+v err=%v", found, err)
	}
	marker := &UserSceneMarker{UserID: f.user.ID, SceneID: scene.ID, Timestamp: 10, Label: "Intro", Color: "#FFFFFF"}
	if err := repo.CreateMarker(marker); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddMarkerTags(marker.ID, []uint{f.tags[0].ID}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMarkerTags(marker.ID, []uint{f.tags[1].ID}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateMarkerColor(marker.ID, "#FF0000"); err != nil {
		t.Fatal(err)
	}
	found, err = repo.FindMarker(f.user.ID, scene.ID, 10, "Intro")
	if err != nil || found == nil || found.ID != marker.ID || found.Color != "#FF0000" {
		t.Fatalf("expected marker %d with updated color, got %+v err=%v", marker.ID, found, err)
	}
	var tagIDs []uint
	db.Model(&MarkerTag{}).Where("marker_id = ?", marker.ID).Pluck("tag_id", &tagIDs)
	if len(tagIDs) != 1 || tagIDs[0] != f.tags[1].ID {
		t.Fatalf("expected marker tags [%d], got %v", f.tags[1].ID, tagIDs)
	}
	counts, err := repo.CountMarkersByScenes(f.user.ID, []uint{scene.ID})
	if err != nil || counts[scene.ID] != 1 {
		t.Fatalf("expected 1 marker, got %v err=%v", counts, err)
	}

	// Trashed scenes are still reported by the summary, soft-deleted ones are not.
	now := time.Now()
	db.Model(&Scene{}).Where("id = ?", scene.ID).Update("trashed_at", now)
	scenes, err := repo.GetScenesForSummary([]uint{scene.ID})
	if err != nil || len(scenes) != 1 || scenes[0].TrashedAt == nil {
		t.Fatalf("expected trashed scene in summary, got %+v err=%v", scenes, err)
	}
	if err := db.Delete(&Scene{}, scene.ID).Error; err != nil {
		t.Fatal(err)
	}
	scenes, err = repo.GetScenesForSummary([]uint{scene.ID})
	if err != nil || len(scenes) != 0 {
		t.Fatalf("expected soft-deleted scene excluded, got %+v err=%v", scenes, err)
	}
}
