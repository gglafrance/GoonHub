package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// newTestMediaFileGuard returns a guard over the given storage paths.
func newTestMediaFileGuard(t *testing.T, paths ...data.StoragePath) *MediaFileGuard {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStoragePathRepository(ctrl)
	repo.EXPECT().List().Return(paths, nil).AnyTimes()
	return NewMediaFileGuard(repo, zap.NewNop())
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("video"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func assertFileExists(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Stat(path)
	if exists := err == nil; exists != want {
		t.Fatalf("file %s: exists=%v, want %v (err %v)", path, exists, want, err)
	}
}

func uintPtr(v uint) *uint { return &v }

func TestPathWithin(t *testing.T) {
	tests := []struct {
		name string
		file string
		root string
		want bool
	}{
		{"file directly under root", "/media/stash/a.mp4", "/media/stash", true},
		{"file deep under root", "/media/stash/x/y/a.mp4", "/media/stash", true},
		{"root with trailing slash", "/media/stash/a.mp4", "/media/stash/", true},
		{"root itself", "/media/stash", "/media/stash", true},
		{"sibling with same prefix", "/media/stash2/a.mp4", "/media/stash", false},
		{"parent directory", "/media/a.mp4", "/media/stash", false},
		{"dot-dot escape", "/media/stash/../other/a.mp4", "/media/stash", false},
		{"unclean path inside", "/media/./stash//x/../a.mp4", "/media/stash", true},
		{"relative root and file", "data/videos/a.mp4", "./data/videos", true},
		{"relative root, other relative file", "data/other/a.mp4", "./data/videos", false},
		{"empty root", "/media/stash/a.mp4", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathWithin(tt.file, tt.root); got != tt.want {
				t.Fatalf("pathWithin(%q, %q) = %v, want %v", tt.file, tt.root, got, tt.want)
			}
		})
	}
}

func TestPathWithin_RelativeRootAbsoluteFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !pathWithin(filepath.Join(wd, "data", "videos", "a.mp4"), "./data/videos") {
		t.Fatal("absolute file under a relative storage path must match")
	}
}

func TestPathWithin_Symlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	// Storage path configured through the symlink, scene path through the real directory
	if !pathWithin(filepath.Join(real, "a.mp4"), link) {
		t.Fatal("file under the real directory must match a storage path given through a symlink")
	}
	// And the other way around, file missing (resolved through its directory)
	if !pathWithin(filepath.Join(link, "missing.mp4"), real) {
		t.Fatal("missing file under a symlinked directory must match")
	}
}

func TestProtectingPath(t *testing.T) {
	paths := []data.StoragePath{
		{ID: 1, Name: "Library", Path: "/media/library"},
		{ID: 2, Name: "Stash", Path: "/media/stash", ReadOnly: true},
		{ID: 3, Name: "Nested", Path: "/media/library/stash-mirror", ReadOnly: true},
	}
	guard := newTestMediaFileGuard(t, paths...)

	tests := []struct {
		name          string
		storagePathID *uint
		file          string
		wantID        uint // 0 = not protected
	}{
		{"writable path by id and prefix", uintPtr(1), "/media/library/a.mp4", 0},
		{"read-only by id", uintPtr(2), "/media/stash/a.mp4", 2},
		{"read-only by id, path elsewhere", uintPtr(2), "/somewhere/a.mp4", 2},
		{"read-only by prefix, no id", nil, "/media/stash/x/a.mp4", 2},
		{"read-only by prefix, stale writable id", uintPtr(1), "/media/stash/a.mp4", 2},
		{"nested read-only inside writable", uintPtr(1), "/media/library/stash-mirror/a.mp4", 3},
		{"unknown id, unknown path", uintPtr(99), "/elsewhere/a.mp4", 0},
		{"no id, no path", nil, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := guard.ProtectingPath(tt.storagePathID, tt.file)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantID == 0 {
				if got != nil {
					t.Fatalf("expected not protected, got storage path %d", got.ID)
				}
				return
			}
			if got == nil || got.ID != tt.wantID {
				t.Fatalf("expected protected by %d, got %+v", tt.wantID, got)
			}
		})
	}
}

func TestCheckModifiable(t *testing.T) {
	guard := newTestMediaFileGuard(t,
		data.StoragePath{ID: 1, Name: "Library", Path: "/media/library"},
		data.StoragePath{ID: 2, Name: "Stash", Path: "/media/stash", ReadOnly: true},
	)

	if err := guard.CheckModifiable(uintPtr(1), "/media/library/a.mp4"); err != nil {
		t.Fatalf("writable path must be modifiable, got %v", err)
	}
	err := guard.CheckModifiable(nil, "/media/stash/a.mp4")
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	if apperrors.GetCode(err) != apperrors.CodeStoragePathReadOnly {
		t.Fatalf("expected code %s, got %s", apperrors.CodeStoragePathReadOnly, apperrors.GetCode(err))
	}
	if err := guard.CheckSceneModifiable(&data.Scene{ID: 1, StoragePathID: uintPtr(2), StoredPath: "/x/a.mp4"}); !apperrors.IsValidation(err) {
		t.Fatalf("scene on read-only path must not be modifiable, got %v", err)
	}
}

func TestCheckModifiable_RepoErrorIsNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStoragePathRepository(ctrl)
	repo.EXPECT().List().Return(nil, errors.New("db down"))
	guard := NewMediaFileGuard(repo, zap.NewNop())

	err := guard.CheckModifiable(nil, "/media/a.mp4")
	if err == nil || !apperrors.IsInternal(err) {
		t.Fatalf("expected an internal error when the check fails, got %v", err)
	}
}

func TestCheckModifiable_NilGuardAllows(t *testing.T) {
	var guard *MediaFileGuard
	if err := guard.CheckModifiable(nil, "/media/a.mp4"); err != nil {
		t.Fatalf("nil guard must allow, got %v", err)
	}
}

func TestRemoveSceneFile(t *testing.T) {
	base := t.TempDir()
	roDir := filepath.Join(base, "stash")
	rwDir := filepath.Join(base, "library")
	guard := newTestMediaFileGuard(t,
		data.StoragePath{ID: 1, Name: "Library", Path: rwDir},
		data.StoragePath{ID: 2, Name: "Stash", Path: roDir, ReadOnly: true},
	)

	t.Run("writable path: removed", func(t *testing.T) {
		file := filepath.Join(rwDir, "a.mp4")
		writeTestFile(t, file)
		if !guard.RemoveSceneFile(&data.Scene{ID: 1, StoragePathID: uintPtr(1), StoredPath: file}) {
			t.Fatal("expected the file to be removed")
		}
		assertFileExists(t, file, false)
	})
	t.Run("read-only by id: kept", func(t *testing.T) {
		file := filepath.Join(base, "elsewhere", "b.mp4")
		writeTestFile(t, file)
		if guard.RemoveSceneFile(&data.Scene{ID: 2, StoragePathID: uintPtr(2), StoredPath: file}) {
			t.Fatal("expected the file to be kept")
		}
		assertFileExists(t, file, true)
	})
	t.Run("read-only by prefix: kept", func(t *testing.T) {
		file := filepath.Join(roDir, "sub", "c.mp4")
		writeTestFile(t, file)
		if guard.RemoveSceneFile(&data.Scene{ID: 3, StoredPath: file}) {
			t.Fatal("expected the file to be kept")
		}
		assertFileExists(t, file, true)
	})
	t.Run("missing file on writable path: ok", func(t *testing.T) {
		if !guard.RemoveSceneFile(&data.Scene{ID: 4, StoredPath: filepath.Join(rwDir, "missing.mp4")}) {
			t.Fatal("a missing file counts as removed")
		}
	})
	t.Run("empty stored path: nothing to do", func(t *testing.T) {
		if !guard.RemoveSceneFile(&data.Scene{ID: 5}) {
			t.Fatal("expected true for a scene without file")
		}
	})
}

func TestRemoveSceneFile_RepoErrorKeepsFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStoragePathRepository(ctrl)
	repo.EXPECT().List().Return(nil, errors.New("db down"))
	guard := NewMediaFileGuard(repo, zap.NewNop())

	file := filepath.Join(t.TempDir(), "a.mp4")
	writeTestFile(t, file)
	if guard.RemoveSceneFile(&data.Scene{ID: 1, StoredPath: file}) {
		t.Fatal("the file must be kept when the read-only check fails")
	}
	assertFileExists(t, file, true)
}
