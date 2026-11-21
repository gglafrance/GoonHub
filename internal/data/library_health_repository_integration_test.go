package data

import (
	"fmt"
	"testing"
	"time"

	"goonhub/internal/playability"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration tests of the library health queries, on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN). The database may hold other rows, so the counts are
// compared to a baseline taken after the fixture's storage path exists.

type healthFixture struct {
	db      *gorm.DB
	repo    *LibraryHealthRepositoryImpl
	base    string
	storage StoragePath
	user    User
}

func newHealthFixture(t *testing.T) *healthFixture {
	t.Helper()
	db := importTestDB(t)
	suffix := uuid.NewString()[:8]
	f := &healthFixture{
		db:      db,
		repo:    NewLibraryHealthRepository(db),
		base:    "/health-test/" + suffix,
		storage: StoragePath{Name: "health " + suffix, Path: "/health-test/" + suffix + "/lib"},
		user:    User{Username: "health-test-" + suffix, Password: "x", Role: "admin"},
	}
	if err := db.Create(&f.storage).Error; err != nil {
		t.Fatalf("create storage path: %v", err)
	}
	if err := db.Create(&f.user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		var ids []uint
		db.Unscoped().Model(&Scene{}).Where("stored_path LIKE ?", f.base+"/%").Pluck("id", &ids)
		for _, res := range []*gorm.DB{
			db.Where("scene_id IN ?", append(ids, 0)).Delete(&JobHistory{}),
			db.Where("user_id = ?", f.user.ID).Delete(&UserSceneMarker{}),
			db.Where("scene_id IN ?", append(ids, 0)).Delete(&LibraryHealthCheckItem{}),
			db.Unscoped().Where("stored_path LIKE ?", f.base+"/%").Delete(&Scene{}),
			db.Delete(&f.storage),
			db.Delete(&f.user),
		} {
			if res.Error != nil {
				t.Errorf("cleanup: %v", res.Error)
			}
		}
	})
	return f
}

func (f *healthFixture) scene(t *testing.T, s Scene) *Scene {
	t.Helper()
	s.Tags = pq.StringArray{}
	s.Actors = pq.StringArray{}
	if s.Title == "" {
		s.Title = s.StoredPath
	}
	if err := f.db.Create(&s).Error; err != nil {
		t.Fatalf("create scene: %v", err)
	}
	return &s
}

func (f *healthFixture) checkCleanup(t *testing.T, check *LibraryHealthCheck) {
	t.Cleanup(func() {
		if err := f.db.Delete(&LibraryHealthCheck{}, check.ID).Error; err != nil {
			t.Errorf("cleanup check: %v", err)
		}
	})
}

func categoryIDs(t *testing.T, repo *LibraryHealthRepositoryImpl, category string, checkID uint) map[uint]HealthSceneRow {
	t.Helper()
	rows, total, err := repo.ListCategoryScenes(category, checkID, 1, 100000)
	if err != nil {
		t.Fatalf("list %s: %v", category, err)
	}
	if int64(len(rows)) != total {
		t.Fatalf("list %s: %d rows but total %d", category, len(rows), total)
	}
	ids := make(map[uint]HealthSceneRow, len(rows))
	for _, row := range rows {
		ids[row.ID] = row
	}
	return ids
}

func TestLibraryHealthRepository_LiveCategories(t *testing.T) {
	f := newHealthFixture(t)
	baseline, baseTotal, err := f.repo.CountLiveCategories()
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}

	fp := "audio"
	spID := f.storage.ID
	healthy := f.scene(t, Scene{
		StoredPath: f.base + "/lib/healthy.mp4", Size: 100, Duration: 60,
		ThumbnailPath: "t.webp", SpriteSheetPath: "s.webp", PreviewVideoPath: "p.mp4",
		FingerprintType: &fp, VideoCodec: "h264", AudioCodec: "aac", StoragePathID: &spID,
	})
	broken := f.scene(t, Scene{
		StoredPath: f.base + "/elsewhere/broken.avi", Size: 0, Duration: 0,
	})
	unplayable := f.scene(t, Scene{
		StoredPath: f.base + "/lib/unplayable.mkv", Size: 50, Duration: 10, IsCorrupted: true,
		SpriteSheetPath: "s.webp", PreviewVideoPath: "p.mp4", FingerprintType: &fp,
		VideoCodec: "hevc", AudioCodec: "aac", StoragePathID: &spID,
	})
	trashedAt := time.Now()
	f.scene(t, Scene{StoredPath: f.base + "/lib/trashed.avi", TrashedAt: &trashedAt, IsCorrupted: true})
	deleted := f.scene(t, Scene{StoredPath: f.base + "/lib/deleted.avi", IsCorrupted: true})
	if err := f.db.Delete(deleted).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	counts, total, err := f.repo.CountLiveCategories()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total-baseTotal != 3 {
		t.Fatalf("total: got +%d, want +3 (trashed and deleted scenes excluded)", total-baseTotal)
	}
	want := map[string]int64{
		HealthCategoryCorrupted:          1, // unplayable
		HealthCategorySizeZero:           1, // broken
		HealthCategoryDurationZero:       1, // broken
		HealthCategoryUnplayable:         1, // unplayable (broken has no codec yet: unknown)
		HealthCategoryNoThumbnail:        2, // broken, unplayable
		HealthCategoryNoSprites:          1, // broken
		HealthCategoryNoPreview:          1, // broken
		HealthCategoryNoFingerprint:      1, // broken
		HealthCategoryNoStoragePath:      1, // broken
		HealthCategoryOutsideStoragePath: 1, // broken
	}
	for category, delta := range want {
		if got := counts[category] - baseline[category]; got != delta {
			t.Errorf("%s: got +%d, want +%d", category, got, delta)
		}
	}

	// The lists use the same conditions as the counts
	members := map[string][]*Scene{
		HealthCategoryCorrupted:          {unplayable},
		HealthCategorySizeZero:           {broken},
		HealthCategoryUnplayable:         {unplayable},
		HealthCategoryNoThumbnail:        {broken, unplayable},
		HealthCategoryOutsideStoragePath: {broken},
		HealthCategoryNoStoragePath:      {broken},
	}
	for category, scenes := range members {
		ids := categoryIDs(t, f.repo, category, 0)
		for _, s := range []*Scene{healthy, broken, unplayable} {
			_, in := ids[s.ID]
			expected := false
			for _, m := range scenes {
				expected = expected || m.ID == s.ID
			}
			if in != expected {
				t.Errorf("%s: scene %s listed=%v, want %v", category, s.StoredPath, in, expected)
			}
		}
		if _, in := ids[deleted.ID]; in {
			t.Errorf("%s: soft-deleted scene listed", category)
		}
	}

	row := categoryIDs(t, f.repo, HealthCategoryUnplayable, 0)[unplayable.ID]
	if row.VideoCodec != "hevc" || row.StoredPath != unplayable.StoredPath || row.StoragePathID == nil || *row.StoragePathID != spID {
		t.Fatalf("unexpected row: %+v", row)
	}

	// Pagination
	page, total2, err := f.repo.ListCategoryScenes(HealthCategoryNoThumbnail, 0, 1, 1)
	if err != nil || len(page) != 1 || total2 < 2 {
		t.Fatalf("page of 1: rows=%d total=%d err=%v", len(page), total2, err)
	}
	if _, _, err := f.repo.ListCategoryScenes("nope", 0, 1, 10); err == nil {
		t.Fatal("expected an error for an unknown category")
	}
}

func TestLibraryHealthRepository_RelativeStoragePath(t *testing.T) {
	f := newHealthFixture(t)
	rel := StoragePath{Name: "relative", Path: "." + f.base + "-rel"}
	if err := f.db.Create(&rel).Error; err != nil {
		t.Fatalf("create storage path: %v", err)
	}
	t.Cleanup(func() { f.db.Delete(&rel) })
	// A scan of "./x" stores "x/file" (filepath.Join cleans the path)
	inside := f.scene(t, Scene{StoredPath: f.base[1:] + "-rel/a.mp4"})
	t.Cleanup(func() { f.db.Unscoped().Delete(inside) })

	if _, in := categoryIDs(t, f.repo, HealthCategoryOutsideStoragePath, 0)[inside.ID]; in {
		t.Fatal("a scene under a relative storage path is reported outside every storage path")
	}
}

func TestLibraryHealthRepository_UnplayableMatchesGoRule(t *testing.T) {
	f := newHealthFixture(t)
	combos := []struct{ ext, video, audio string }{
		{"mp4", "h264", "aac"}, {"MP4", "H264", "AAC"}, {"webm", "vp9", "opus"}, {"m4v", "av1", ""},
		{"avi", "h264", "mp3"}, {"mkv", "hevc", "aac"}, {"mp4", "mpeg2video", "mp2"}, {"mp4", "hevc", "aac"},
		{"mp4", "h264", "ac3"}, {"", "h264", "aac"}, {"avi", "", ""}, {"mov", "h264", "pcm_s16le"},
		{"wmv", "wmv3", "wmav2"}, {"mp4", " h264 ", "aac"},
	}
	scenes := make([]*Scene, len(combos))
	for i, c := range combos {
		path := fmt.Sprintf("%s/lib/combo%d", f.base, i)
		if c.ext != "" {
			path += "." + c.ext
		}
		scenes[i] = f.scene(t, Scene{StoredPath: path, VideoCodec: c.video, AudioCodec: c.audio})
	}
	ids := categoryIDs(t, f.repo, HealthCategoryUnplayable, 0)
	for i, s := range scenes {
		_, inSQL := ids[s.ID]
		inGo := playability.Check(s.StoredPath, s.VideoCodec, s.AudioCodec).Status == playability.StatusUnplayable
		if inSQL != inGo {
			t.Errorf("%+v: SQL says unplayable=%v, Go says %v", combos[i], inSQL, inGo)
		}
	}
}

func TestLibraryHealthRepository_ScenesNeedingPhase(t *testing.T) {
	f := newHealthFixture(t)
	phases := []string{"metadata", "thumbnail", "sprites", "animated_thumbnails", "fingerprint"}
	baseline := map[string]int64{}
	for _, phase := range phases {
		n, err := f.repo.CountScenesNeedingPhase(phase)
		if err != nil {
			t.Fatalf("count %s: %v", phase, err)
		}
		baseline[phase] = n
	}

	fp := "audio"
	noMeta := f.scene(t, Scene{StoredPath: f.base + "/lib/nometa.mp4"})
	bare := f.scene(t, Scene{StoredPath: f.base + "/lib/bare.mp4", Duration: 30})
	done := f.scene(t, Scene{StoredPath: f.base + "/lib/done.mp4", Duration: 30, ThumbnailPath: "t", SpriteSheetPath: "s", PreviewVideoPath: "p", FingerprintType: &fp})
	marker := UserSceneMarker{UserID: f.user.ID, SceneID: done.ID, Timestamp: 5, Label: "x", Color: "#FFFFFF"}
	if err := f.db.Create(&marker).Error; err != nil {
		t.Fatalf("create marker: %v", err)
	}
	_ = noMeta

	// bare misses everything; done only has a marker without animated thumbnail
	want := map[string]int64{"metadata": 1, "thumbnail": 1, "sprites": 1, "animated_thumbnails": 2, "fingerprint": 1}
	sceneRepo := NewSceneRepository(f.db)
	for _, phase := range phases {
		n, err := f.repo.CountScenesNeedingPhase(phase)
		if err != nil {
			t.Fatalf("count %s: %v", phase, err)
		}
		if n-baseline[phase] != want[phase] {
			t.Errorf("%s: got +%d, want +%d", phase, n-baseline[phase], want[phase])
		}
		// The bulk "missing" run selects exactly what the health page counts
		scenes, err := sceneRepo.GetScenesNeedingPhase(phase)
		if err != nil {
			t.Fatalf("get %s: %v", phase, err)
		}
		if int64(len(scenes)) != n {
			t.Errorf("%s: GetScenesNeedingPhase returned %d scenes, CountScenesNeedingPhase %d", phase, len(scenes), n)
		}
	}

	// A pending job takes the scene out of the count, for every phase
	for _, phase := range []string{"thumbnail", "animated_thumbnails"} {
		job := JobHistory{JobID: uuid.NewString(), SceneID: bare.ID, Phase: phase, Status: "pending"}
		if err := f.db.Create(&job).Error; err != nil {
			t.Fatalf("create job: %v", err)
		}
		n, err := f.repo.CountScenesNeedingPhase(phase)
		if err != nil {
			t.Fatal(err)
		}
		if n-baseline[phase] != want[phase]-1 {
			t.Errorf("%s with a pending job: got +%d, want +%d", phase, n-baseline[phase], want[phase]-1)
		}
	}

	if _, err := f.repo.CountScenesNeedingPhase("scan"); err == nil {
		t.Fatal("expected an error for an unknown phase")
	}
}

func TestLibraryHealthRepository_StoragePathStatsAndPaths(t *testing.T) {
	f := newHealthFixture(t)
	spID := f.storage.ID
	a := f.scene(t, Scene{StoredPath: f.base + "/lib/a.mp4", Size: 1000, StoragePathID: &spID})
	b := f.scene(t, Scene{StoredPath: f.base + "/lib/b.mp4", Size: 0, StoragePathID: &spID})
	trashedAt := time.Now()
	f.scene(t, Scene{StoredPath: f.base + "/lib/c.mp4", Size: 5000, StoragePathID: &spID, TrashedAt: &trashedAt})

	stats, err := f.repo.StoragePathSceneStats()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, st := range stats {
		if st.StoragePathID != nil && *st.StoragePathID == spID {
			found = true
			if st.SceneCount != 2 || st.TotalSize != 1000 {
				t.Fatalf("storage path stats: %+v, want 2 scenes and 1000 bytes", st)
			}
		}
	}
	if !found {
		t.Fatal("no stats for the fixture storage path")
	}

	// Keyset pagination of the size-0 selection
	var seen []uint
	var after uint
	for {
		batch, err := f.repo.ListScenePaths(HealthScenesSizeZero, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(batch) == 0 {
			break
		}
		for _, p := range batch {
			seen = append(seen, p.ID)
			if p.ID == b.ID && p.StoredPath != b.StoredPath {
				t.Fatalf("wrong path for b: %q", p.StoredPath)
			}
		}
		after = batch[len(batch)-1].ID
	}
	n, err := f.repo.CountScenePaths(HealthScenesSizeZero)
	if err != nil || int64(len(seen)) != n {
		t.Fatalf("paged %d size-0 scenes, count says %d (err %v)", len(seen), n, err)
	}
	hasB, hasA := false, false
	for _, id := range seen {
		hasB = hasB || id == b.ID
		hasA = hasA || id == a.ID
	}
	if !hasB || hasA {
		t.Fatalf("size-0 selection: has b=%v has a=%v", hasB, hasA)
	}

	// Size update only applies to a scene still at size 0 and at the same path
	if ok, err := f.repo.UpdateSceneSize(b.ID, "/elsewhere.mp4", 42); err != nil || ok {
		t.Fatalf("update with another path: ok=%v err=%v", ok, err)
	}
	if ok, err := f.repo.UpdateSceneSize(b.ID, b.StoredPath, 42); err != nil || !ok {
		t.Fatalf("update: ok=%v err=%v", ok, err)
	}
	if ok, err := f.repo.UpdateSceneSize(b.ID, b.StoredPath, 43); err != nil || ok {
		t.Fatalf("second update: ok=%v err=%v", ok, err)
	}
	var stored Scene
	if err := f.db.First(&stored, b.ID).Error; err != nil || stored.Size != 42 {
		t.Fatalf("stored size %d (err %v), want 42", stored.Size, err)
	}
}

func TestLibraryHealthRepository_CheckLifecycle(t *testing.T) {
	f := newHealthFixture(t)
	a := f.scene(t, Scene{StoredPath: f.base + "/lib/a.mp4"})
	b := f.scene(t, Scene{StoredPath: f.base + "/lib/b.mp4"})
	moved := f.scene(t, Scene{StoredPath: f.base + "/lib/moved.mp4"})

	newCheck := func(kind string) *LibraryHealthCheck {
		c := &LibraryHealthCheck{Kind: kind, Status: HealthCheckStatusRunning, StartedAt: time.Now(), Total: 3}
		if err := f.repo.CreateCheck(c); err != nil {
			t.Fatalf("create check: %v", err)
		}
		f.checkCleanup(t, c)
		return c
	}
	finish := func(c *LibraryHealthCheck, status string) {
		now := time.Now()
		c.Status = status
		c.CompletedAt = &now
		if err := f.repo.FinishCheck(c); err != nil {
			t.Fatalf("finish check: %v", err)
		}
	}

	first := newCheck(HealthCheckKindMissingFiles)
	first.Checked = 2
	if err := f.repo.UpdateCheck(first); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.InsertCheckItems([]LibraryHealthCheckItem{
		{CheckID: first.ID, SceneID: a.ID, StoredPath: a.StoredPath, Reason: HealthItemReasonMissing},
		{CheckID: first.ID, SceneID: b.ID, StoredPath: b.StoredPath, Reason: HealthItemReasonUnreadable, Detail: "permission denied"},
		{CheckID: first.ID, SceneID: moved.ID, StoredPath: f.base + "/old/moved.mp4", Reason: HealthItemReasonMissing},
	}); err != nil {
		t.Fatalf("insert items: %v", err)
	}
	finish(first, HealthCheckStatusCompleted)

	counts, err := f.repo.CountCheckItems(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The moved scene is no longer at the checked path: not counted
	if counts[HealthItemReasonMissing] != 1 || counts[HealthItemReasonUnreadable] != 1 {
		t.Fatalf("item counts: %v", counts)
	}
	unreadable := categoryIDs(t, f.repo, HealthCategoryUnreadableFile, first.ID)
	if row, ok := unreadable[b.ID]; !ok || row.Detail != "permission denied" || len(unreadable) != 1 {
		t.Fatalf("unreadable list: %+v", unreadable)
	}
	if missing := categoryIDs(t, f.repo, HealthCategoryMissingFile, first.ID); len(missing) != 1 {
		t.Fatalf("missing list: %+v", missing)
	}
	if rows, total, err := f.repo.ListCategoryScenes(HealthCategoryMissingFile, 0, 1, 10); err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("missing list without a check: %d rows, total %d, err %v", len(rows), total, err)
	}

	latest, err := f.repo.GetLatestCheck(HealthCheckKindMissingFiles, HealthCheckStatusCompleted)
	if err != nil || latest == nil || latest.ID != first.ID || latest.Checked != 2 {
		t.Fatalf("latest completed: %+v (err %v)", latest, err)
	}

	// A cancelled check drops its own partial items and leaves the last completed one alone
	cancelled := newCheck(HealthCheckKindMissingFiles)
	if err := f.repo.InsertCheckItems([]LibraryHealthCheckItem{{CheckID: cancelled.ID, SceneID: a.ID, StoredPath: a.StoredPath, Reason: HealthItemReasonMissing}}); err != nil {
		t.Fatal(err)
	}
	finish(cancelled, HealthCheckStatusCancelled)
	if counts, _ := f.repo.CountCheckItems(cancelled.ID); counts[HealthItemReasonMissing] != 0 {
		t.Fatalf("cancelled check kept its items: %v", counts)
	}
	if counts, _ := f.repo.CountCheckItems(first.ID); counts[HealthItemReasonMissing] != 1 {
		t.Fatalf("cancelled check dropped the completed check items: %v", counts)
	}
	if latest, _ := f.repo.GetLatestCheck(HealthCheckKindMissingFiles, ""); latest == nil || latest.ID != cancelled.ID {
		t.Fatalf("latest of any status: %+v", latest)
	}

	// A new completed check replaces the items of the previous ones
	second := newCheck(HealthCheckKindMissingFiles)
	if err := f.repo.InsertCheckItems([]LibraryHealthCheckItem{{CheckID: second.ID, SceneID: b.ID, StoredPath: b.StoredPath, Reason: HealthItemReasonMissing}}); err != nil {
		t.Fatal(err)
	}
	finish(second, HealthCheckStatusCompleted)
	if counts, _ := f.repo.CountCheckItems(first.ID); counts[HealthItemReasonMissing]+counts[HealthItemReasonUnreadable] != 0 {
		t.Fatalf("items of the previous check kept: %v", counts)
	}
	if counts, _ := f.repo.CountCheckItems(second.ID); counts[HealthItemReasonMissing] != 1 {
		t.Fatalf("items of the new check: %v", counts)
	}

	// Restart recovery
	running := newCheck(HealthCheckKindFixSizes)
	if err := f.repo.MarkInterruptedChecksFailed(); err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.GetLatestCheck(HealthCheckKindFixSizes, "")
	if err != nil || got == nil || got.ID != running.ID || got.Status != HealthCheckStatusFailed || got.CompletedAt == nil || got.ErrorMessage == nil {
		t.Fatalf("interrupted check: %+v (err %v)", got, err)
	}
}
