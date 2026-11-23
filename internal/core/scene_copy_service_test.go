package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// copyTestEnv is a copy service on real temporary files: a storage path "rw" (id 1) and a
// read-only one "ro" (id 2), with mocked repositories.
type copyTestEnv struct {
	svc    *SceneCopyService
	copies *mocks.MockSceneCopyRepository
	rw, ro string
}

func newCopyTestEnv(t *testing.T) *copyTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	copies := mocks.NewMockSceneCopyRepository(ctrl)
	spRepo := mocks.NewMockStoragePathRepository(ctrl)
	root := t.TempDir()
	env := &copyTestEnv{copies: copies, rw: filepath.Join(root, "rw"), ro: filepath.Join(root, "ro")}
	for _, d := range []string{env.rw, env.ro} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	spRepo.EXPECT().List().Return([]data.StoragePath{
		{ID: 1, Name: "rw", Path: env.rw},
		{ID: 2, Name: "ro", Path: env.ro, ReadOnly: true},
	}, nil).AnyTimes()
	guard := NewMediaFileGuard(spRepo, zap.NewNop())
	env.svc = NewSceneCopyService(copies, nil, NewStoragePathService(spRepo, zap.NewNop()), guard, nil, zap.NewNop())
	return env
}

func copyContent(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i%251) ^ seed
	}
	return b
}

func writeCopyFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s changed", path)
	}
}

// expectCopy makes the repository return the copy and its scene's main file.
func (e *copyTestEnv) expectCopy(c data.SceneCopy, mainPath string, mainSP *uint) {
	e.copies.EXPECT().GetByID(c.ID).Return(&c, nil)
	e.copies.EXPECT().GetSceneFile(c.SceneID).Return(&data.SceneFileRow{ID: c.SceneID, StoredPath: mainPath, StoragePathID: mainSP}, nil)
}

func TestSceneCopyDeleteFileDeletesAnIdenticalCopy(t *testing.T) {
	env := newCopyTestEnv(t)
	content := copyContent(200*1024, 1)
	mainPath := filepath.Join(env.rw, "main", "video.mp4")
	copyPath := filepath.Join(env.rw, "copies", "video.mp4")
	writeCopyFile(t, mainPath, content)
	writeCopyFile(t, copyPath, content)

	env.expectCopy(data.SceneCopy{ID: 7, SceneID: 3, StoredPath: copyPath, StoragePathID: uintPtr(1), Size: int64(len(content))}, mainPath, uintPtr(1))
	env.copies.EXPECT().Delete(uint(7)).Return(nil)

	freed, err := env.svc.DeleteFile(7)
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if freed != int64(len(content)) {
		t.Fatalf("freed %d, want %d", freed, len(content))
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Fatalf("copy file still there (%v)", err)
	}
	assertFileContent(t, mainPath, content)
}

// Every refusal keeps both files and the copy row.
func TestSceneCopyDeleteFileRefusals(t *testing.T) {
	content := copyContent(200*1024, 2)
	otherTail := bytes.Clone(content)
	otherTail[len(otherTail)-50] ^= 0xff // same size, different tail: another oshash

	tests := []struct {
		name string
		// setup writes the files and returns the copy and the main path
		setup func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string)
	}{
		{"content no longer matches", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			mainPath := filepath.Join(env.rw, "main.mp4")
			copyPath := filepath.Join(env.rw, "copy.mp4")
			writeCopyFile(t, mainPath, content)
			writeCopyFile(t, copyPath, otherTail)
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: copyPath, StoragePathID: uintPtr(1)}, mainPath
		}},
		{"read-only storage path", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			mainPath := filepath.Join(env.rw, "main.mp4")
			copyPath := filepath.Join(env.ro, "copy.mp4")
			writeCopyFile(t, mainPath, content)
			writeCopyFile(t, copyPath, content)
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: copyPath, StoragePathID: uintPtr(2)}, mainPath
		}},
		{"under a read-only path without storage path id", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			mainPath := filepath.Join(env.rw, "main.mp4")
			copyPath := filepath.Join(env.ro, "sub", "copy.mp4")
			writeCopyFile(t, mainPath, content)
			writeCopyFile(t, copyPath, content)
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: copyPath}, mainPath
		}},
		{"the copy path is the main file", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			mainPath := filepath.Join(env.rw, "main.mp4")
			writeCopyFile(t, mainPath, content)
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: mainPath + string(filepath.Separator), StoragePathID: uintPtr(1)}, mainPath
		}},
		{"the copy is the same file on disk (hard link)", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			mainPath := filepath.Join(env.rw, "main.mp4")
			copyPath := filepath.Join(env.rw, "link.mp4")
			writeCopyFile(t, mainPath, content)
			if err := os.Link(mainPath, copyPath); err != nil {
				t.Skipf("hard links not supported: %v", err)
			}
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: copyPath, StoragePathID: uintPtr(1)}, mainPath
		}},
		{"the main file is missing", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			copyPath := filepath.Join(env.rw, "copy.mp4")
			writeCopyFile(t, copyPath, content)
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: copyPath, StoragePathID: uintPtr(1)}, filepath.Join(env.rw, "gone.mp4")
		}},
		{"the copy file is missing", func(t *testing.T, env *copyTestEnv) (data.SceneCopy, string) {
			mainPath := filepath.Join(env.rw, "main.mp4")
			writeCopyFile(t, mainPath, content)
			return data.SceneCopy{ID: 1, SceneID: 9, StoredPath: filepath.Join(env.rw, "gone.mp4"), StoragePathID: uintPtr(1)}, mainPath
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newCopyTestEnv(t)
			c, mainPath := tt.setup(t, env)
			before := map[string][]byte{}
			for _, p := range []string{c.StoredPath, mainPath} {
				if b, err := os.ReadFile(p); err == nil {
					before[p] = b
				}
			}
			env.expectCopy(c, mainPath, uintPtr(1))
			// No Delete expectation: removing the row would fail the test

			if _, err := env.svc.DeleteFile(c.ID); err == nil || !apperrors.IsValidation(err) {
				t.Fatalf("expected a validation error, got %v", err)
			}
			for p, b := range before {
				assertFileContent(t, p, b)
			}
		})
	}
}

func TestSceneCopyDeleteFilesBulkReportsEachCopy(t *testing.T) {
	env := newCopyTestEnv(t)
	content := copyContent(100*1024, 3)
	mainPath := filepath.Join(env.rw, "main.mp4")
	good := filepath.Join(env.rw, "good.mp4")
	bad := filepath.Join(env.rw, "bad.mp4")
	writeCopyFile(t, mainPath, content)
	writeCopyFile(t, good, content)
	writeCopyFile(t, bad, copyContent(100*1024, 4))

	env.expectCopy(data.SceneCopy{ID: 1, SceneID: 5, StoredPath: good, StoragePathID: uintPtr(1)}, mainPath, uintPtr(1))
	env.expectCopy(data.SceneCopy{ID: 2, SceneID: 5, StoredPath: bad, StoragePathID: uintPtr(1)}, mainPath, uintPtr(1))
	env.copies.EXPECT().GetByID(uint(3)).Return(nil, gorm.ErrRecordNotFound)
	env.copies.EXPECT().Delete(uint(1)).Return(nil)

	res := env.svc.DeleteFilesBulk([]uint{1, 2, 1, 3})
	if len(res.Done) != 1 || res.Done[0] != 1 || res.FreedBytes != int64(len(content)) {
		t.Fatalf("done %v freed %d", res.Done, res.FreedBytes)
	}
	if len(res.Failed) != 2 || res.Failed[0].CopyID != 2 || res.Failed[1].CopyID != 3 {
		t.Fatalf("failed %+v", res.Failed)
	}
	assertFileContent(t, mainPath, content)
	if _, err := os.Stat(bad); err != nil {
		t.Fatalf("mismatching copy deleted: %v", err)
	}
}

func TestSceneCopyMakeMain(t *testing.T) {
	env := newCopyTestEnv(t)
	content := copyContent(1000, 5)
	mainPath := filepath.Join(env.rw, "main.mp4")
	copyPath := filepath.Join(env.rw, "copy.mp4")
	writeCopyFile(t, mainPath, content)
	writeCopyFile(t, copyPath, content)

	// Storage path unknown on the row: resolved from the path
	env.expectCopy(data.SceneCopy{ID: 4, SceneID: 8, StoredPath: copyPath}, mainPath, uintPtr(1))
	env.copies.EXPECT().SwapWithMain(uint(4), gomock.Any()).DoAndReturn(func(_ uint, sp *uint) error {
		if sp == nil || *sp != 1 {
			t.Fatalf("storage path %v, want 1", sp)
		}
		return nil
	})
	if err := env.svc.MakeMain(4); err != nil {
		t.Fatalf("MakeMain: %v", err)
	}
	assertFileContent(t, mainPath, content)
	assertFileContent(t, copyPath, content)

	// A copy whose file is gone cannot become the main file
	env.expectCopy(data.SceneCopy{ID: 5, SceneID: 8, StoredPath: filepath.Join(env.rw, "gone.mp4")}, mainPath, uintPtr(1))
	if err := env.svc.MakeMain(5); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error, got %v", err)
	}
}

func TestSceneCopySeparate(t *testing.T) {
	env := newCopyTestEnv(t)
	copyPath := filepath.Join(env.rw, "dir", "Some Video.mkv")
	writeCopyFile(t, copyPath, copyContent(3000, 6))

	env.expectCopy(data.SceneCopy{ID: 6, SceneID: 8, StoredPath: copyPath, StoragePathID: uintPtr(1)}, filepath.Join(env.rw, "main.mp4"), uintPtr(1))
	env.copies.EXPECT().PromoteToScene(uint(6), gomock.Any()).DoAndReturn(func(_ uint, sc *data.Scene) error {
		sc.ID = 42
		return nil
	})
	scene, err := env.svc.Separate(6)
	if err != nil {
		t.Fatalf("Separate: %v", err)
	}
	if scene.ID != 42 || scene.StoredPath != copyPath || scene.Size != 3000 || scene.Title != "Some Video" ||
		scene.StoragePathID == nil || *scene.StoragePathID != 1 || scene.ProcessingStatus != "pending" {
		t.Fatalf("scene %+v", scene)
	}
	assertFileContent(t, copyPath, copyContent(3000, 6))
}

func TestSceneCopyForget(t *testing.T) {
	env := newCopyTestEnv(t)
	env.copies.EXPECT().Delete(uint(1)).Return(nil)
	env.copies.EXPECT().Delete(uint(2)).Return(gorm.ErrRecordNotFound)
	if err := env.svc.Forget(1); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if err := env.svc.Forget(2); !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	res := env.svc.ForgetBulk(nil)
	if len(res.Done) != 0 || len(res.Failed) != 0 {
		t.Fatalf("empty bulk: %+v", res)
	}
}

func TestSceneCopyListFlagsCopiesNotSeenInLastScan(t *testing.T) {
	env := newCopyTestEnv(t)
	lastScan := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	q := data.SceneCopyQuery{Sort: data.SceneCopySortSize, Page: 1, Limit: 20}
	env.copies.EXPECT().CopyTotals(q.SceneCopyFilter).Return(data.SceneCopyTotals{Scenes: 2, Copies: 3, Size: 30}, nil)
	env.copies.EXPECT().ListScenesWithCopies(q).Return([]data.SceneWithCopiesRow{{SceneID: 1}, {SceneID: 2}}, nil)
	env.copies.EXPECT().LastCompletedScanStart().Return(&lastScan, nil)
	env.copies.EXPECT().ListCopiesOfScenes([]uint{1, 2}, q.SceneCopyFilter).Return([]data.SceneCopy{
		{ID: 10, SceneID: 1, LastSeenAt: lastScan.Add(time.Minute)},
		{ID: 11, SceneID: 1, LastSeenAt: lastScan.Add(-time.Hour)},
		{ID: 12, SceneID: 2, LastSeenAt: lastScan},
	}, nil)

	list, err := env.svc.List(q)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Totals.Copies != 3 || len(list.Scenes) != 2 || len(list.Scenes[0].Copies) != 2 || len(list.Scenes[1].Copies) != 1 {
		t.Fatalf("list %+v", list)
	}
	if list.Scenes[0].Copies[0].NotSeenInLastScan || !list.Scenes[0].Copies[1].NotSeenInLastScan || list.Scenes[1].Copies[0].NotSeenInLastScan {
		t.Fatalf("not-seen flags wrong: %+v", list.Scenes)
	}
}
