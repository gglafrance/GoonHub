package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
)

type fakeStoragePaths struct {
	paths []data.StoragePath
	def   *data.StoragePath
}

func (f *fakeStoragePaths) List() ([]data.StoragePath, error)      { return f.paths, nil }
func (f *fakeStoragePaths) GetDefault() (*data.StoragePath, error) { return f.def, nil }

func strPtr(s string) *string { return &s }

func newTestShortsSettings(t *testing.T, record *data.AppSettingsRecord, paths *fakeStoragePaths) (*ShortsSettingsService, *mocks.MockAppSettingsRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAppSettingsRepository(ctrl)
	if record != nil {
		repo.EXPECT().Get().Return(record, nil).AnyTimes()
	}
	return NewShortsSettingsService(repo, paths, "shorts", "", zap.NewNop()), repo
}

func TestShortsSettings_ResolveSaveDir(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "media", "videos")
	other := filepath.Join(root, "elsewhere", "clips")

	paths := &fakeStoragePaths{
		paths: []data.StoragePath{{ID: 3, Name: "Videos", Path: media, IsDefault: true}},
		def:   &data.StoragePath{ID: 3, Name: "Videos", Path: media, IsDefault: true},
	}

	tests := []struct {
		name       string
		saveDir    *string
		wantDir    string
		wantPathID *uint
	}{
		{"null uses default", nil, filepath.Join(media, "shorts"), uintPtr(3)},
		{"empty uses default", strPtr(""), filepath.Join(media, "shorts"), uintPtr(3)},
		{"blank uses default", strPtr("   "), filepath.Join(media, "shorts"), uintPtr(3)},
		{"custom inside storage path", strPtr(filepath.Join(media, "cut")), filepath.Join(media, "cut"), uintPtr(3)},
		{"custom outside storage paths", strPtr(other), other, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 60, ShortsSaveDir: tt.saveDir}, paths)
			dir, id, err := svc.ResolveSaveDir()
			if err != nil {
				t.Fatalf("ResolveSaveDir: %v", err)
			}
			if dir != tt.wantDir {
				t.Fatalf("dir = %q, want %q", dir, tt.wantDir)
			}
			if (id == nil) != (tt.wantPathID == nil) || (id != nil && *id != *tt.wantPathID) {
				t.Fatalf("storage path id = %v, want %v", derefUint(id), derefUint(tt.wantPathID))
			}
		})
	}
}

func TestShortsSettings_DefaultFollowsDefaultStoragePath(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	paths := &fakeStoragePaths{
		paths: []data.StoragePath{{ID: 1, Path: a}, {ID: 2, Path: b}},
		def:   &data.StoragePath{ID: 1, Path: a},
	}
	svc, _ := newTestShortsSettings(t, &data.AppSettingsRecord{}, paths)

	dir, _, _ := svc.ResolveSaveDir()
	if dir != filepath.Join(a, "shorts") {
		t.Fatalf("dir = %q, want under a", dir)
	}

	paths.def = &data.StoragePath{ID: 2, Path: b}
	dir, id, _ := svc.ResolveSaveDir()
	if dir != filepath.Join(b, "shorts") || id == nil || *id != 2 {
		t.Fatalf("after changing the default, dir = %q id = %v", dir, derefUint(id))
	}
}

func TestShortsSettings_NoDefaultStoragePath(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAppSettingsRepository(ctrl)
	repo.EXPECT().Get().Return(&data.AppSettingsRecord{}, nil).AnyTimes()

	svc := NewShortsSettingsService(repo, &fakeStoragePaths{}, "shorts", "", zap.NewNop())
	if _, _, err := svc.ResolveSaveDir(); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error without a default storage path, got %v", err)
	}

	upload := t.TempDir()
	svc = NewShortsSettingsService(repo, &fakeStoragePaths{}, "shorts", upload, zap.NewNop())
	dir, id, err := svc.ResolveSaveDir()
	if err != nil || dir != filepath.Join(upload, "shorts") || id != nil {
		t.Fatalf("fallback: dir = %q id = %v err = %v", dir, derefUint(id), err)
	}
}

func TestMatchStoragePath(t *testing.T) {
	sep := string(filepath.Separator)
	videos := sep + filepath.Join("media", "videos")
	videos2 := sep + filepath.Join("media", "videos2")
	nested := sep + filepath.Join("media", "videos", "archive")

	paths := []data.StoragePath{
		{ID: 1, Path: videos},
		{ID: 2, Path: nested},
	}

	tests := []struct {
		name string
		dir  string
		want uint // 0 = none
	}{
		{"exact root", videos, 1},
		{"child", filepath.Join(videos, "shorts"), 1},
		{"trailing separator on root", filepath.Join(videos, "x"), 1},
		{"sibling with same prefix is outside", filepath.Join(videos2, "shorts"), 0},
		{"deepest storage path wins", filepath.Join(nested, "shorts"), 2},
		{"unrelated", sep + filepath.Join("srv", "clips"), 0},
		{"unclean path", videos + sep + "." + sep + "shorts", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchStoragePath(tt.dir, paths)
			if tt.want == 0 {
				if got != nil {
					t.Fatalf("expected no match, got %d", got.ID)
				}
				return
			}
			if got == nil || got.ID != tt.want {
				t.Fatalf("got %v, want %d", got, tt.want)
			}
		})
	}

	// A storage path saved with a trailing separator still matches.
	withSlash := []data.StoragePath{{ID: 9, Path: videos + sep}}
	if got := matchStoragePath(filepath.Join(videos, "shorts"), withSlash); got == nil || got.ID != 9 {
		t.Fatalf("trailing separator storage path did not match")
	}
}

func TestShortsSettings_ValidateSaveDir(t *testing.T) {
	svc, _ := newTestShortsSettings(t, nil, &fakeStoragePaths{})

	t.Run("relative path rejected", func(t *testing.T) {
		if err := svc.ValidateSaveDir(filepath.Join("data", "shorts")); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("missing folder is created", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "new", "shorts")
		if err := svc.ValidateSaveDir(dir); err != nil {
			t.Fatalf("ValidateSaveDir: %v", err)
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Fatalf("folder was not created: %v", err)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Fatalf("test file was left behind: %v", entries)
		}
	})

	t.Run("unwritable folder rejected", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("permission bits are not enforced on Windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("root can write anywhere")
		}
		dir := filepath.Join(t.TempDir(), "ro")
		if err := os.Mkdir(dir, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
		if err := svc.ValidateSaveDir(dir); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("path under a file rejected", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := svc.ValidateSaveDir(filepath.Join(file, "shorts")); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
}

func TestShortsSettings_UpdateSettings(t *testing.T) {
	root := t.TempDir()
	paths := &fakeStoragePaths{def: &data.StoragePath{ID: 1, Path: root}, paths: []data.StoragePath{{ID: 1, Path: root}}}

	for _, d := range []int{-5, 0, MaxShortsMaxDuration + 1} {
		t.Run("rejects max_duration", func(t *testing.T) {
			svc, _ := newTestShortsSettings(t, nil, paths)
			if _, err := svc.UpdateSettings(d, nil, ""); !apperrors.IsValidation(err) {
				t.Fatalf("max_duration %d: expected validation error, got %v", d, err)
			}
		})
	}

	for _, d := range []int{1, 45, 60, 90, 120, 600, MaxShortsMaxDuration} {
		t.Run("accepts max_duration", func(t *testing.T) {
			svc, repo := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: d}, paths)
			repo.EXPECT().UpdateShortsSettings(d, (*string)(nil), "hide").Return(nil)
			got, err := svc.UpdateSettings(d, strPtr(""), "")
			if err != nil {
				t.Fatalf("UpdateSettings(%d): %v", d, err)
			}
			if got.MaxDuration != d || !got.Writable {
				t.Fatalf("got %+v", got)
			}
		})
	}

	t.Run("relative save_dir rejected before saving", func(t *testing.T) {
		svc, _ := newTestShortsSettings(t, nil, paths)
		if _, err := svc.UpdateSettings(60, strPtr("relative/dir"), ""); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("custom save_dir stored cleaned", func(t *testing.T) {
		custom := filepath.Join(t.TempDir(), "clips")
		svc, repo := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 120, ShortsSaveDir: &custom}, paths)
		repo.EXPECT().UpdateShortsSettings(120, gomock.Any(), "hide").DoAndReturn(func(_ int, dir *string, _ string) error {
			if dir == nil || *dir != custom {
				t.Fatalf("stored dir = %v, want %q", dir, custom)
			}
			return nil
		})
		got, err := svc.UpdateSettings(120, strPtr(custom+string(filepath.Separator)), "")
		if err != nil {
			t.Fatalf("UpdateSettings: %v", err)
		}
		if got.EffectiveSaveDir != custom || got.StoragePath != nil {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestShortsSettings_GetMaxDurationFallsBack(t *testing.T) {
	svc, _ := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 0}, &fakeStoragePaths{})
	if d, err := svc.GetMaxDuration(); err != nil || d != DefaultShortsMaxDuration {
		t.Fatalf("GetMaxDuration = %d, %v", d, err)
	}
}

func uintPtr(u uint) *uint { return &u }

func derefUint(u *uint) any {
	if u == nil {
		return nil
	}
	return *u
}

func TestShortsSettings_SearchDefault(t *testing.T) {
	root := t.TempDir()
	paths := &fakeStoragePaths{def: &data.StoragePath{ID: 1, Path: root}, paths: []data.StoragePath{{ID: 1, Path: root}}}

	t.Run("falls back to hide", func(t *testing.T) {
		svc, _ := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 60}, paths)
		if got, err := svc.GetSearchDefault(); err != nil || got != "hide" {
			t.Fatalf("GetSearchDefault = %q, %v", got, err)
		}
	})

	t.Run("empty keeps the stored value", func(t *testing.T) {
		svc, repo := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 60, ShortsSearchDefault: "only_clips"}, paths)
		repo.EXPECT().UpdateShortsSettings(90, (*string)(nil), "only_clips").Return(nil)
		if _, err := svc.UpdateSettings(90, nil, ""); err != nil {
			t.Fatal(err)
		}
	})

	for _, m := range ShortsSearchModes {
		t.Run("stores "+m, func(t *testing.T) {
			svc, repo := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 60, ShortsSearchDefault: m}, paths)
			repo.EXPECT().UpdateShortsSettings(60, (*string)(nil), m).Return(nil)
			got, err := svc.UpdateSettings(60, nil, m)
			if err != nil || got.SearchDefault != m {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}

	t.Run("rejects unknown", func(t *testing.T) {
		svc, _ := newTestShortsSettings(t, &data.AppSettingsRecord{ShortsMaxDuration: 60}, paths)
		if _, err := svc.UpdateSettings(60, nil, "sometimes"); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
}
