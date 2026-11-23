package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func strPtr(s string) *string { return &s }

func TestListTrash_MapsRowsAndTotals(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSceneRepository(ctrl)
	paths := mocks.NewMockStoragePathRepository(ctrl)
	svc := &SceneService{Repo: repo, logger: zap.NewNop(), mediaGuard: NewMediaFileGuard(paths, zap.NewNop())}

	spID, roID := uint(1), uint(2)
	groupID := uint(9)
	trashedAt := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	purge := trashedAt.AddDate(0, 0, 7)
	filter := data.TrashFilter{Query: "beach"}

	repo.EXPECT().ListTrash(gomock.Any()).DoAndReturn(func(q data.TrashQuery) ([]data.TrashedSceneRow, data.TrashTotals, error) {
		if q.Sort != data.TrashSortTrashedAt || q.RetentionDays != 7 || q.Page != 1 || q.Limit != 20 || q.Filter != filter {
			t.Fatalf("unexpected query %+v", q)
		}
		return []data.TrashedSceneRow{
			{ID: 1, Title: "a", StoredPath: "/media/main/beach/a.mp4", StoragePathID: &spID, StoragePathName: strPtr("Main"),
				StoragePathRoot: strPtr("/media/main/"), StoragePathReadOnly: boolPtr(false), Size: 100, TrashedAt: trashedAt, PurgeAt: purge},
			{ID: 2, Title: "b", StoredPath: "/media/ro/b.mp4", StoragePathID: &roID, StoragePathName: strPtr("Archive"),
				StoragePathRoot: strPtr("/media/ro"), Size: 200, TrashedAt: trashedAt, PurgeAt: purge, DuplicateGroupID: &groupID, FileMissing: true},
			{ID: 3, Title: "c", StoredPath: "/elsewhere/c.mp4", Size: 300, TrashedAt: trashedAt, PurgeAt: purge},
		}, data.TrashTotals{Count: 3, Size: 600}, nil
	})
	repo.EXPECT().TrashTotals(data.TrashFilter{}).Return(data.TrashTotals{Count: 10, Size: 5000}, nil)
	paths.EXPECT().List().Return([]data.StoragePath{{ID: 1, Path: "/media/main"}, {ID: 2, Path: "/media/ro", ReadOnly: true}}, nil)

	list, err := svc.ListTrash(TrashListParams{Filter: filter, Sort: "bogus"})
	if err != nil {
		t.Fatal(err)
	}
	if list.Filtered.Count != 3 || list.Filtered.Size != 600 || list.All.Count != 10 || list.All.Size != 5000 {
		t.Fatalf("totals: %+v %+v", list.Filtered, list.All)
	}
	if list.RetentionDays != 7 || len(list.Items) != 3 {
		t.Fatalf("unexpected list: %+v", list)
	}
	a, b, c := list.Items[0], list.Items[1], list.Items[2]
	if a.StoragePathName != "Main" || a.RelativePath != "beach/a.mp4" || a.KeptOnDisk || a.Source != data.TrashSourceOther {
		t.Fatalf("a: %+v", a)
	}
	if !b.KeptOnDisk || b.Source != data.TrashSourceDuplicates || !b.FileMissing || b.RelativePath != "b.mp4" {
		t.Fatalf("b: %+v", b)
	}
	if c.RelativePath != "/elsewhere/c.mp4" || c.StoragePathName != "" || !c.ExpiresAt.Equal(purge) {
		t.Fatalf("c: %+v", c)
	}
}

func TestListTrash_NoFilterReusesTotals(t *testing.T) {
	svc, repo := newTestSceneService(t)
	repo.EXPECT().ListTrash(gomock.Any()).Return([]data.TrashedSceneRow{}, data.TrashTotals{Count: 4, Size: 40}, nil)
	// No TrashTotals call: the whole trash is the filter
	list, err := svc.ListTrash(TrashListParams{Page: 2, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if list.All != list.Filtered || list.Page != 2 || list.Limit != 50 {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestRelativeToRoot(t *testing.T) {
	tests := []struct{ file, root, want string }{
		{"/media/a/b.mp4", "/media/a", "b.mp4"},
		{"/media/a/x/b.mp4", "/media/a/", "x/b.mp4"},
		{"/media/ab/b.mp4", "/media/a", "/media/ab/b.mp4"},
		{"/other/b.mp4", "/media/a", "/other/b.mp4"},
		{"/media/a/b.mp4", "", "/media/a/b.mp4"},
	}
	for _, tt := range tests {
		if got := relativeToRoot(tt.file, tt.root); got != tt.want {
			t.Errorf("relativeToRoot(%q, %q) = %q, want %q", tt.file, tt.root, got, tt.want)
		}
	}
}

func TestTrashIDs_UsesSortAndLimit(t *testing.T) {
	svc, repo := newTestSceneService(t)
	repo.EXPECT().ListTrashIDs(gomock.Any(), MaxTrashBulk).DoAndReturn(func(q data.TrashQuery, limit int) ([]data.TrashIDSize, int64, error) {
		if q.Sort != data.TrashSortSize || !q.Desc {
			t.Fatalf("unexpected query %+v", q)
		}
		return []data.TrashIDSize{{ID: 3, Size: 1}, {ID: 1, Size: 2}}, 700, nil
	})
	ids, total, err := svc.TrashIDs(TrashListParams{Sort: data.TrashSortSize, Desc: true}, MaxTrashBulk)
	if err != nil || len(ids) != 2 || total != 700 {
		t.Fatalf("ids=%v total=%d err=%v", ids, total, err)
	}
}

func TestDeleteTrashedScenes_OnlyTrashedScenes(t *testing.T) {
	svc, repo := newTestSceneService(t)
	// The "video" is a temporary test file, deleted by the permanent delete
	video := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	trashed := &data.Scene{ID: 1, StoredPath: video, Size: 1000, TrashedAt: &now}

	repo.EXPECT().GetByIDIncludingTrashed(uint(1)).Return(trashed, nil)
	repo.EXPECT().HardDelete(uint(1)).Return(trashed, nil)
	repo.EXPECT().GetByIDIncludingTrashed(uint(2)).Return(&data.Scene{ID: 2, Size: 50}, nil) // not trashed
	repo.EXPECT().GetByIDIncludingTrashed(uint(3)).Return(nil, gorm.ErrRecordNotFound)

	result := svc.DeleteTrashedScenes([]uint{1, 2, 3, 1})
	if len(result.Succeeded) != 1 || result.Succeeded[0] != 1 || result.Size != 1000 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Failed) != 2 || result.Failed[0].SceneID != 2 || result.Failed[0].Error != "scene is not in trash" ||
		result.Failed[1].SceneID != 3 || result.Failed[1].Error != "scene not found" {
		t.Fatalf("unexpected failures: %+v", result.Failed)
	}
	if _, err := os.Stat(video); !os.IsNotExist(err) {
		t.Fatalf("the test video should be deleted, stat err = %v", err)
	}
}

func TestDeleteTrashedScenes_ReportsHardDeleteFailure(t *testing.T) {
	svc, repo := newTestSceneService(t)
	now := time.Now()
	repo.EXPECT().GetByIDIncludingTrashed(uint(5)).Return(&data.Scene{ID: 5, TrashedAt: &now}, nil)
	repo.EXPECT().HardDelete(uint(5)).Return(nil, gorm.ErrInvalidDB)

	result := svc.DeleteTrashedScenes([]uint{5})
	if len(result.Succeeded) != 0 || len(result.Failed) != 1 || result.Failed[0].SceneID != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRestoreScenesFromTrash(t *testing.T) {
	svc, repo := newTestSceneService(t)
	now := time.Now()
	repo.EXPECT().GetByIDIncludingTrashed(uint(1)).Return(&data.Scene{ID: 1, Size: 70, TrashedAt: &now}, nil).Times(2)
	repo.EXPECT().RestoreFromTrash(uint(1)).Return(nil)
	repo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)
	repo.EXPECT().GetByIDIncludingTrashed(uint(2)).Return(&data.Scene{ID: 2}, nil).Times(2) // not trashed

	result := svc.RestoreScenesFromTrash([]uint{1, 2})
	if len(result.Succeeded) != 1 || result.Succeeded[0] != 1 || result.Size != 70 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Failed) != 1 || result.Failed[0].SceneID != 2 {
		t.Fatalf("unexpected failures: %+v", result.Failed)
	}
}
