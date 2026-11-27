package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Integration test of the trash page queries (ListTrash, TrashTotals, ListTrashIDs,
// TrashFacets): filters, sorts, totals, purge date, duplicate source and missing file. Runs on
// the disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN). The scenes live
// under a storage path of their own and every query is scoped to it: other tests share the
// database.
func TestSceneRepositoryTrash(t *testing.T) {
	db := importTestDB(t)
	repo := NewSceneRepository(db)
	groups := NewDuplicateGroupRepository(db)
	suffix := uuid.NewString()[:8]
	root := "/trash-test/" + suffix

	sp := &StoragePath{Name: "Trash " + suffix, Path: root}
	if err := db.Create(sp).Error; err != nil {
		t.Fatalf("create storage path: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("stored_path LIKE ?", root+"/%").Delete(&Scene{})
		db.Delete(sp)
	})

	now := time.Now().UTC().Truncate(time.Second)
	day := 24 * time.Hour
	newScene := func(title, file string, size int64, duration, height int, codec string, trashedAt *time.Time) *Scene {
		t.Helper()
		s := &Scene{
			Title: title, StoredPath: root + "/" + file, Size: size, Duration: duration,
			Width: height * 16 / 9, Height: height, VideoCodec: codec, StoragePathID: &sp.ID,
			TrashedAt: trashedAt, Tags: pq.StringArray{}, Actors: pq.StringArray{},
		}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	a := newScene("Alpha Beach", "beach/alpha.mp4", 1000, 600, 1080, "h264", at(1*day))
	b := newScene("bravo night", "night/Bravo_50%.mkv", 5000, 120, 2160, "hevc", at(3*day))
	c := newScene("Charlie", "charlie.mp4", 3000, 1800, 720, "H264", at(5*day))
	newScene("Live", "live.mp4", 9999, 60, 1080, "h264", nil)
	deleted := newScene("Deleted", "deleted.mp4", 7777, 60, 1080, "h264", at(2*day))
	if err := db.Delete(deleted).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// c was not kept when its duplicate group was resolved, its deletion is scheduled sooner
	// than the retention
	keeper := newScene("Keeper", "keeper.mp4", 10, 10, 480, "h264", nil)
	group := &DuplicateGroup{Status: "resolved", SceneCount: 2, BestSceneID: &keeper.ID}
	if err := groups.Create(group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	t.Cleanup(func() { db.Delete(&DuplicateGroup{}, group.ID) })
	deleteAfter := now.Add(1 * day)
	for _, m := range []*DuplicateGroupMember{
		{GroupID: group.ID, SceneID: keeper.ID, IsBest: true, MatchType: "audio"},
		{GroupID: group.ID, SceneID: c.ID, MatchType: "audio", DeleteAfter: &deleteAfter},
	} {
		if err := groups.AddMember(m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	// The last completed missing-file check found b's file missing
	completedAt := now
	check := &LibraryHealthCheck{Kind: HealthCheckKindMissingFiles, Status: HealthCheckStatusCompleted, StartedAt: now, CompletedAt: &completedAt}
	if err := db.Create(check).Error; err != nil {
		t.Fatalf("create check: %v", err)
	}
	t.Cleanup(func() {
		db.Where("check_id = ?", check.ID).Delete(&LibraryHealthCheckItem{})
		db.Delete(&LibraryHealthCheck{}, check.ID)
	})
	if err := db.Create(&LibraryHealthCheckItem{CheckID: check.ID, SceneID: b.ID, StoredPath: b.StoredPath, Reason: HealthItemReasonMissing}).Error; err != nil {
		t.Fatalf("create check item: %v", err)
	}

	scope := TrashFilter{StoragePathID: &sp.ID}
	list := func(f TrashFilter, sort string, desc bool) ([]TrashedSceneRow, TrashTotals) {
		t.Helper()
		rows, totals, err := repo.ListTrash(TrashQuery{Filter: f, Sort: sort, Desc: desc, RetentionDays: 7, Page: 1, Limit: 50})
		if err != nil {
			t.Fatalf("list trash: %v", err)
		}
		if int64(len(rows)) != totals.Count {
			t.Fatalf("%d rows for a total of %d", len(rows), totals.Count)
		}
		return rows, totals
	}
	ids := func(rows []TrashedSceneRow) []uint {
		out := make([]uint, len(rows))
		for i, r := range rows {
			out[i] = r.ID
		}
		return out
	}
	expectIDs := func(name string, rows []TrashedSceneRow, want ...uint) {
		t.Helper()
		got := ids(rows)
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}

	// Only trashed, not soft-deleted scenes; newest trashed first; totals of the filter
	rows, totals := list(scope, TrashSortTrashedAt, true)
	expectIDs("default", rows, a.ID, b.ID, c.ID)
	if totals.Count != 3 || totals.Size != 9000 {
		t.Fatalf("totals = %+v, want 3 scenes, 9000 bytes", totals)
	}

	// Row details: storage path, purge date, duplicate source, missing file
	byID := map[uint]TrashedSceneRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	ra, rb, rc := byID[a.ID], byID[b.ID], byID[c.ID]
	if ra.StoragePathName == nil || *ra.StoragePathName != sp.Name || ra.StoragePathRoot == nil || *ra.StoragePathRoot != root {
		t.Fatalf("storage path not joined: %+v", ra)
	}
	if want := a.TrashedAt.Add(7 * day); !ra.PurgeAt.Equal(want) {
		t.Fatalf("purge of a = %v, want the retention %v", ra.PurgeAt, want)
	}
	if ra.DuplicateGroupID != nil || ra.FileMissing {
		t.Fatalf("a: unexpected duplicate group or missing file: %+v", ra)
	}
	if rc.DuplicateGroupID == nil || *rc.DuplicateGroupID != group.ID || rc.DeleteAfter == nil {
		t.Fatalf("c: duplicate group not reported: %+v", rc)
	}
	if !rc.PurgeAt.Equal(deleteAfter) {
		t.Fatalf("purge of c = %v, want the scheduled deletion %v", rc.PurgeAt, deleteAfter)
	}
	if !rb.FileMissing {
		t.Fatalf("b: missing file not reported: %+v", rb)
	}

	// Sorts
	rows, _ = list(scope, TrashSortSize, true)
	expectIDs("size desc", rows, b.ID, c.ID, a.ID)
	rows, _ = list(scope, TrashSortDuration, false)
	expectIDs("duration asc", rows, b.ID, a.ID, c.ID)
	rows, _ = list(scope, TrashSortTitle, false)
	expectIDs("title asc (case-insensitive)", rows, a.ID, b.ID, c.ID)
	// a: now-1d+7d, b: now-3d+7d, c: now+1d (scheduled, before its retention at now+2d)
	rows, _ = list(scope, TrashSortPurgeAt, false)
	expectIDs("purge asc", rows, c.ID, b.ID, a.ID)

	// Filters
	with := func(edit func(f *TrashFilter)) TrashFilter {
		f := scope
		edit(&f)
		return f
	}
	cases := []struct {
		name   string
		filter TrashFilter
		want   []uint
		size   int64
	}{
		{"query on title", with(func(f *TrashFilter) { f.Query = "ALPHA" }), []uint{a.ID}, 1000},
		{"query on path", with(func(f *TrashFilter) { f.Query = "night/" }), []uint{b.ID}, 5000},
		{"query with a wildcard", with(func(f *TrashFilter) { f.Query = "_50%" }), []uint{b.ID}, 5000},
		{"underscore is literal", with(func(f *TrashFilter) { f.Query = "alpha_" }), nil, 0},
		{"min size", with(func(f *TrashFilter) { f.MinSize = 3000 }), []uint{b.ID, c.ID}, 8000},
		{"size range", with(func(f *TrashFilter) { f.MinSize = 2000; f.MaxSize = 4000 }), []uint{c.ID}, 3000},
		{"duration range", with(func(f *TrashFilter) { f.MinDuration = 300; f.MaxDuration = 900 }), []uint{a.ID}, 1000},
		{"trashed from", with(func(f *TrashFilter) { f.TrashedFrom = at(4 * day) }), []uint{a.ID, b.ID}, 6000},
		{"trashed before", with(func(f *TrashFilter) { f.TrashedBefore = at(2 * day) }), []uint{b.ID, c.ID}, 8000},
		{"resolution 4k", with(func(f *TrashFilter) { f.MinHeight = 2160 }), []uint{b.ID}, 5000},
		{"resolution 720p", with(func(f *TrashFilter) { f.MinHeight = 720; f.MaxHeight = 1079 }), []uint{c.ID}, 3000},
		{"codec case-insensitive", with(func(f *TrashFilter) { f.Codec = "h264" }), []uint{a.ID, c.ID}, 4000},
		{"from the duplicates", with(func(f *TrashFilter) { f.Source = TrashSourceDuplicates }), []uint{c.ID}, 3000},
		{"other sources", with(func(f *TrashFilter) { f.Source = TrashSourceOther }), []uint{a.ID, b.ID}, 6000},
	}
	for _, tc := range cases {
		rows, totals := list(tc.filter, TrashSortTrashedAt, true)
		expectIDs(tc.name, rows, tc.want...)
		if totals.Size != tc.size {
			t.Fatalf("%s: size = %d, want %d", tc.name, totals.Size, tc.size)
		}
	}

	// Pagination keeps the totals of the whole filter
	page2, totals, err := repo.ListTrash(TrashQuery{Filter: scope, Sort: TrashSortSize, Desc: true, RetentionDays: 7, Page: 2, Limit: 2})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != a.ID || totals.Count != 3 || totals.Size != 9000 {
		t.Fatalf("page 2 = %v, totals %+v", ids(page2), totals)
	}

	// The whole trash holds at least this storage path
	all, err := repo.TrashTotals(TrashFilter{})
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if all.Count < 3 || all.Size < 9000 {
		t.Fatalf("whole trash totals = %+v", all)
	}

	// Select all: sorted, capped, with the total
	idList, total, err := repo.ListTrashIDs(TrashQuery{Filter: scope, Sort: TrashSortPurgeAt, RetentionDays: 7}, 2)
	if err != nil {
		t.Fatalf("ids: %v", err)
	}
	if total != 3 || len(idList) != 2 || idList[0].ID != c.ID || idList[1].ID != b.ID || idList[0].Size != 3000 {
		t.Fatalf("ids = %v (total %d), want [c b] of 3", idList, total)
	}

	// Facets of the whole trash include this storage path and its codecs
	facets, err := repo.TrashFacets()
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	var found bool
	for _, p := range facets.StoragePaths {
		if p.StoragePathID != nil && *p.StoragePathID == sp.ID {
			found = p.Count == 3 && p.Size == 9000
		}
	}
	if !found {
		t.Fatalf("storage path missing or wrong in facets: %+v", facets.StoragePaths)
	}
	codecs := map[string]bool{}
	for _, c := range facets.Codecs {
		codecs[c.Codec] = true
	}
	if !codecs["hevc"] || facets.Duplicates < 1 {
		t.Fatalf("facets = %+v", facets)
	}
}
