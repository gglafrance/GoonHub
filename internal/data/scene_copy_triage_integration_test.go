package data

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration test of the copy triage queries (Duplicates page > Copies): filters, sorts,
// totals and select-all IDs; the swap of a copy with the main file (scene ID, size and attached
// rows kept, missing scene restored); turning a copy into a scene; forgetting copies.
// Runs on the disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestSceneCopyTriage(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	suffix := f.tags[0].Name[len("import-a-"):]
	repo := NewSceneCopyRepository(db)
	base := "/import-test/" + suffix

	var sps [2]StoragePath
	for i := range sps {
		sps[i] = StoragePath{Name: fmt.Sprintf("triage-%s-%d", suffix, i), Path: fmt.Sprintf("%s/sp%d", base, i)}
		if err := db.Create(&sps[i]).Error; err != nil {
			t.Fatalf("create storage path: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Where("stored_path LIKE ?", base+"/%").Delete(&SceneCopy{})
		db.Unscoped().Where("stored_path LIKE ?", base+"/%").Delete(&Scene{})
		db.Delete(&sps)
	})
	sp0, sp1 := sps[0].ID, sps[1].ID

	mkScene := func(name string, size int64) *Scene {
		s := &Scene{Title: "triage " + name, Size: size, StoredPath: fmt.Sprintf("%s/sp0/%s.mp4", base, name), StoragePathID: &sp0, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	big := mkScene("big", 5000)
	small := mkScene("small", 100)
	mkScene("alone", 7000) // no copies: never listed

	now := time.Now()
	copies := []*SceneCopy{
		{SceneID: big.ID, StoredPath: base + "/sp1/big-a.mp4", StoragePathID: &sp1, Size: 5000, Oshash: "000000000000000a", CreatedAt: now.Add(-3 * time.Hour)},
		{SceneID: big.ID, StoredPath: base + "/sp0/big-b.mp4", StoragePathID: &sp0, Size: 5000, Oshash: "000000000000000a", CreatedAt: now.Add(-2 * time.Hour)},
		{SceneID: small.ID, StoredPath: base + "/sp1/small-a.mp4", StoragePathID: &sp1, Size: 100, Oshash: "000000000000000b", CreatedAt: now.Add(-time.Hour)},
	}
	if err := repo.CreateBatch(copies); err != nil {
		t.Fatalf("create copies: %v", err)
	}
	all := SceneCopyFilter{Search: base}

	// Totals and sorts
	totals, err := repo.CopyTotals(all)
	if err != nil || totals != (SceneCopyTotals{Scenes: 2, Copies: 3, Size: 10100}) {
		t.Fatalf("totals %+v (%v)", totals, err)
	}
	order := func(sort string) []uint {
		rows, err := repo.ListScenesWithCopies(SceneCopyQuery{SceneCopyFilter: all, Sort: sort, Page: 1, Limit: 10})
		if err != nil {
			t.Fatalf("list %s: %v", sort, err)
		}
		ids := make([]uint, len(rows))
		for i, r := range rows {
			ids[i] = r.SceneID
		}
		return ids
	}
	if got := order(SceneCopySortNewest); len(got) != 2 || got[0] != small.ID {
		t.Fatalf("newest first: %v", got)
	}
	if got := order(SceneCopySortSize); len(got) != 2 || got[0] != big.ID {
		t.Fatalf("largest first: %v", got)
	}
	if got := order(SceneCopySortCopies); len(got) != 2 || got[0] != big.ID {
		t.Fatalf("most copies first: %v", got)
	}
	rows, err := repo.ListScenesWithCopies(SceneCopyQuery{SceneCopyFilter: all, Sort: SceneCopySortSize, Page: 1, Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("page of 1: %+v (%v)", rows, err)
	}
	r0 := rows[0]
	if r0.SceneID != big.ID || r0.CopyCount != 2 || r0.CopiesSize != 10000 || r0.StoredPath != big.StoredPath || r0.StoragePathID == nil || *r0.StoragePathID != sp0 || r0.IsMissing || r0.IsTrashed {
		t.Fatalf("row %+v", r0)
	}

	// Filters: storage path, search on the copy path, size range
	if tot, _ := repo.CopyTotals(SceneCopyFilter{Search: base, StoragePathID: &sp1}); tot.Copies != 2 || tot.Scenes != 2 {
		t.Fatalf("storage path filter: %+v", tot)
	}
	if tot, _ := repo.CopyTotals(SceneCopyFilter{Search: base + "/sp0/big-"}); tot.Copies != 1 {
		t.Fatalf("search filter: %+v", tot)
	}
	if tot, _ := repo.CopyTotals(SceneCopyFilter{Search: base, MinSize: 101, MaxSize: 5000}); tot.Copies != 2 || tot.Scenes != 1 {
		t.Fatalf("size filter: %+v", tot)
	}
	if tot, _ := repo.CopyTotals(SceneCopyFilter{Search: base + "%"}); tot.Copies != 0 {
		t.Fatalf("LIKE wildcards must be escaped: %+v", tot)
	}
	listed, err := repo.ListCopiesOfScenes([]uint{big.ID, small.ID}, SceneCopyFilter{Search: base, StoragePathID: &sp1})
	if err != nil || len(listed) != 2 || listed[0].StoredPath != base+"/sp1/big-a.mp4" {
		t.Fatalf("copies of scenes: %+v (%v)", listed, err)
	}
	ids, total, err := repo.ListCopyIDs(all, 2)
	if err != nil || total != 3 || len(ids) != 2 || ids[0] != copies[0].ID {
		t.Fatalf("ids %v total %d (%v)", ids, total, err)
	}

	// Swap: the copy becomes the main file of a scene marked missing; the old main file
	// becomes the copy row; scene ID, size and tags are kept
	if err := db.Exec(`INSERT INTO scene_tags (scene_id, tag_id) VALUES (?, ?)`, big.ID, f.tags[0].ID).Error; err != nil {
		t.Fatalf("tag scene: %v", err)
	}
	if err := db.Delete(&Scene{}, big.ID).Error; err != nil {
		t.Fatalf("mark missing: %v", err)
	}
	if file, err := repo.GetSceneFile(big.ID); err != nil || !file.IsDeleted || file.StoredPath != big.StoredPath {
		t.Fatalf("scene file of a missing scene: %+v (%v)", file, err)
	}
	if err := repo.SwapWithMain(copies[0].ID, &sp1); err != nil {
		t.Fatalf("swap: %v", err)
	}
	var after Scene
	if err := db.First(&after, big.ID).Error; err != nil {
		t.Fatalf("scene not restored: %v", err)
	}
	if after.StoredPath != copies[0].StoredPath || after.StoragePathID == nil || *after.StoragePathID != sp1 || after.Size != 5000 {
		t.Fatalf("scene after swap %+v", after)
	}
	swapped, err := repo.GetByID(copies[0].ID)
	if err != nil || swapped.StoredPath != big.StoredPath || swapped.StoragePathID == nil || *swapped.StoragePathID != sp0 || swapped.Oshash != "000000000000000a" || swapped.SceneID != big.ID {
		t.Fatalf("copy after swap %+v (%v)", swapped, err)
	}
	var tagCount int64
	db.Table("scene_tags").Where("scene_id = ?", big.ID).Count(&tagCount)
	if tagCount != 1 {
		t.Fatalf("tags lost in the swap: %d", tagCount)
	}

	// Promote: the copy becomes its own scene, the row goes
	promoted := &Scene{Title: "small-a", StoredPath: copies[2].StoredPath, Size: 100, StoragePathID: &sp1, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := repo.PromoteToScene(copies[2].ID, promoted); err != nil || promoted.ID == 0 {
		t.Fatalf("promote: %v (id %d)", err, promoted.ID)
	}
	if _, err := repo.GetByID(copies[2].ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("promoted copy row still there: %v", err)
	}
	if err := repo.PromoteToScene(copies[2].ID, &Scene{Title: "again", Tags: pq.StringArray{}, Actors: pq.StringArray{}}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("promoting a gone copy: %v", err)
	}

	// Forget
	if err := repo.Delete(copies[1].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(copies[1].ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleting twice: %v", err)
	}
	if n, err := repo.DeleteByIDs([]uint{copies[0].ID, copies[1].ID}); err != nil || n != 1 {
		t.Fatalf("delete by ids: %d (%v)", n, err)
	}
	if got, err := repo.GetByIDs([]uint{copies[0].ID}); err != nil || len(got) != 0 {
		t.Fatalf("get by ids after delete: %+v (%v)", got, err)
	}

	// Last completed scan
	scanRepo := NewScanHistoryRepository(db)
	started := time.Now().Add(240 * time.Hour).Truncate(time.Second)
	scan := &ScanHistory{Status: "completed", StartedAt: started, CreatedAt: started}
	if err := scanRepo.Create(scan); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&ScanHistory{}, scan.ID) })
	if last, err := repo.LastCompletedScanStart(); err != nil || last == nil || !last.Equal(started) {
		t.Fatalf("last completed scan %v (%v)", last, err)
	}
}
