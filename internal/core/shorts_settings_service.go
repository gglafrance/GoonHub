package core

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

// DefaultShortsMaxDuration is the feed limit used when none is stored.
const DefaultShortsMaxDuration = 60

// DefaultShortsSearchDefault is what the search page's Shorts filter starts on
// when nothing is stored: hide every short.
const DefaultShortsSearchDefault = "hide"

// ShortsSearchModes are the search page's Shorts filter values.
var ShortsSearchModes = []string{"all", "hide", "only", "hide_clips", "only_clips"}

// IsValidShortsSearchMode reports whether v is one of ShortsSearchModes.
func IsValidShortsSearchMode(v string) bool {
	for _, m := range ShortsSearchModes {
		if m == v {
			return true
		}
	}
	return false
}

func searchDefaultOf(record *data.AppSettingsRecord) string {
	if IsValidShortsSearchMode(record.ShortsSearchDefault) {
		return record.ShortsSearchDefault
	}
	return DefaultShortsSearchDefault
}

// MaxShortsMaxDuration is the largest feed limit the INT column can store.
const MaxShortsMaxDuration = math.MaxInt32

// IsValidShortsMaxDuration reports whether d is a usable feed limit: any whole
// number of seconds from 1 up.
func IsValidShortsMaxDuration(d int) bool {
	return d >= 1 && d <= MaxShortsMaxDuration
}

// shortsStoragePaths is the part of the storage path repository the shorts
// settings need. data.StoragePathRepository satisfies it.
type shortsStoragePaths interface {
	List() ([]data.StoragePath, error)
	GetDefault() (*data.StoragePath, error)
}

// ShortsStoragePathRef names the storage path a shorts folder lies inside.
type ShortsStoragePathRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ShortsSettings is the admin view of the shorts settings.
type ShortsSettings struct {
	MaxDuration      int                   `json:"max_duration"`
	SaveDir          *string               `json:"save_dir"`
	SearchDefault    string                `json:"search_default"`
	DefaultSaveDir   string                `json:"default_save_dir"`
	EffectiveSaveDir string                `json:"effective_save_dir"`
	StoragePath      *ShortsStoragePathRef `json:"storage_path"`
	Writable         bool                  `json:"writable"`
	Problem          string                `json:"problem,omitempty"`
}

// ShortsSettingsService stores the shorts feed limit and works out where new
// shorts are written.
type ShortsSettingsService struct {
	appSettingsRepo data.AppSettingsRepository
	storagePaths    shortsStoragePaths
	subdir          string
	fallbackDir     string
	logger          *zap.Logger
}

// NewShortsSettingsService creates a ShortsSettingsService. subdir is the folder
// under the default storage path used when no save folder is set; fallbackDir
// (the upload folder) stands in for the default storage path when there is none.
func NewShortsSettingsService(
	appSettingsRepo data.AppSettingsRepository,
	storagePaths shortsStoragePaths,
	subdir string,
	fallbackDir string,
	logger *zap.Logger,
) *ShortsSettingsService {
	if subdir == "" {
		subdir = "shorts"
	}
	return &ShortsSettingsService{
		appSettingsRepo: appSettingsRepo,
		storagePaths:    storagePaths,
		subdir:          subdir,
		fallbackDir:     fallbackDir,
		logger:          logger.With(zap.String("component", "shorts_settings")),
	}
}

// GetSearchDefault returns what the search page's Shorts filter starts on.
func (s *ShortsSettingsService) GetSearchDefault() (string, error) {
	record, err := s.appSettingsRepo.Get()
	if err != nil {
		return "", apperrors.NewInternalError("failed to load app settings", err)
	}
	return searchDefaultOf(record), nil
}

// GetMaxDuration returns the feed limit in seconds.
func (s *ShortsSettingsService) GetMaxDuration() (int, error) {
	record, err := s.appSettingsRepo.Get()
	if err != nil {
		return 0, apperrors.NewInternalError("failed to load app settings", err)
	}
	if !IsValidShortsMaxDuration(record.ShortsMaxDuration) {
		return DefaultShortsMaxDuration, nil
	}
	return record.ShortsMaxDuration, nil
}

// DefaultSaveDir returns <default storage path>/<subdir>. It is worked out on
// every call, so changing the default storage path moves where new shorts go.
func (s *ShortsSettingsService) DefaultSaveDir() (string, error) {
	base := ""
	if s.storagePaths != nil {
		def, err := s.storagePaths.GetDefault()
		if err != nil {
			return "", apperrors.NewInternalError("failed to load default storage path", err)
		}
		if def != nil {
			base = def.Path
		}
	}
	if base == "" {
		if s.fallbackDir == "" {
			return "", apperrors.NewValidationError("no default storage path is set, so there is nowhere to save shorts")
		}
		abs, err := filepath.Abs(s.fallbackDir)
		if err != nil {
			return "", apperrors.NewInternalError("failed to resolve upload folder", err)
		}
		base = abs
	}
	return filepath.Join(base, s.subdir), nil
}

// ResolveSaveDir returns the folder new shorts are written to and the ID of the
// storage path that contains it (nil when it is outside every storage path).
func (s *ShortsSettingsService) ResolveSaveDir() (string, *uint, error) {
	record, err := s.appSettingsRepo.Get()
	if err != nil {
		return "", nil, apperrors.NewInternalError("failed to load app settings", err)
	}
	return s.resolve(record)
}

func (s *ShortsSettingsService) resolve(record *data.AppSettingsRecord) (string, *uint, error) {
	dir := ""
	if record.ShortsSaveDir != nil && strings.TrimSpace(*record.ShortsSaveDir) != "" {
		dir = filepath.Clean(strings.TrimSpace(*record.ShortsSaveDir))
	} else {
		def, err := s.DefaultSaveDir()
		if err != nil {
			return "", nil, err
		}
		dir = def
	}

	sp, err := s.containingStoragePath(dir)
	if err != nil {
		return "", nil, err
	}
	if sp == nil {
		return dir, nil, nil
	}
	id := sp.ID
	return dir, &id, nil
}

func (s *ShortsSettingsService) containingStoragePath(dir string) (*data.StoragePath, error) {
	if s.storagePaths == nil {
		return nil, nil
	}
	paths, err := s.storagePaths.List()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list storage paths", err)
	}
	return matchStoragePath(dir, paths), nil
}

// matchStoragePath returns the storage path that contains dir, preferring the
// deepest one when storage paths are nested. Matching is on whole path
// segments, so /media/videos2 is not inside /media/videos.
func matchStoragePath(dir string, paths []data.StoragePath) *data.StoragePath {
	dir = filepath.Clean(dir)
	var best *data.StoragePath
	bestLen := -1
	for i := range paths {
		root := filepath.Clean(paths[i].Path)
		if !isWithinDir(dir, root) {
			continue
		}
		if len(root) > bestLen {
			best = &paths[i]
			bestLen = len(root)
		}
	}
	return best
}

func isWithinDir(dir, root string) bool {
	if dir == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(dir, prefix)
}

// ValidateSaveDir checks that dir is absolute and that the server can create it
// and write to it. It creates the folder and a temporary file, then removes the
// file.
func (s *ShortsSettingsService) ValidateSaveDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return apperrors.NewValidationErrorWithField("save_dir", fmt.Sprintf("the shorts folder must be an absolute path, got %q", dir))
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return apperrors.NewValidationErrorWithField("save_dir", fmt.Sprintf("the shorts folder %q cannot be created: %v", dir, err))
	}
	f, err := os.CreateTemp(dir, ".goonhub-write-test-*")
	if err != nil {
		return apperrors.NewValidationErrorWithField("save_dir", fmt.Sprintf("the shorts folder %q is not writable: %v", dir, err))
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return apperrors.NewValidationErrorWithField("save_dir", fmt.Sprintf("the shorts folder %q is not writable: %v", dir, err))
	}
	if err := os.Remove(name); err != nil {
		return apperrors.NewValidationErrorWithField("save_dir", fmt.Sprintf("the shorts folder %q is not writable: %v", dir, err))
	}
	return nil
}

// CanCreate reports whether shorts can be saved right now: the folder resolves
// and is writable.
func (s *ShortsSettingsService) CanCreate() bool {
	dir, _, err := s.ResolveSaveDir()
	if err != nil {
		return false
	}
	return s.ValidateSaveDir(dir) == nil
}

// GetSettings returns the admin view of the shorts settings.
func (s *ShortsSettingsService) GetSettings() (*ShortsSettings, error) {
	record, err := s.appSettingsRepo.Get()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load app settings", err)
	}

	maxDuration := record.ShortsMaxDuration
	if !IsValidShortsMaxDuration(maxDuration) {
		maxDuration = DefaultShortsMaxDuration
	}

	settings := &ShortsSettings{
		MaxDuration:   maxDuration,
		SaveDir:       record.ShortsSaveDir,
		SearchDefault: searchDefaultOf(record),
	}

	if def, err := s.DefaultSaveDir(); err == nil {
		settings.DefaultSaveDir = def
	} else if !apperrors.IsValidation(err) {
		return nil, err
	}

	dir, storagePathID, err := s.resolve(record)
	if err != nil {
		if apperrors.IsValidation(err) {
			settings.Problem = err.Error()
			return settings, nil
		}
		return nil, err
	}
	settings.EffectiveSaveDir = dir

	if storagePathID != nil {
		if sp, err := s.containingStoragePath(dir); err == nil && sp != nil {
			settings.StoragePath = &ShortsStoragePathRef{ID: sp.ID, Name: sp.Name}
		}
	}

	if err := s.ValidateSaveDir(dir); err != nil {
		settings.Problem = err.Error()
	} else {
		settings.Writable = true
	}

	return settings, nil
}

// UpdateSettings stores the feed limit, save folder and search default. A nil
// or empty saveDir resets the folder to the default; any other value must be an
// absolute folder the server can write to. An empty searchDefault keeps the
// stored one.
func (s *ShortsSettingsService) UpdateSettings(maxDuration int, saveDir *string, searchDefault string) (*ShortsSettings, error) {
	if !IsValidShortsMaxDuration(maxDuration) {
		return nil, apperrors.NewValidationErrorWithField("max_duration", fmt.Sprintf("max_duration must be a whole number of seconds from 1 to %d", MaxShortsMaxDuration))
	}
	if searchDefault != "" && !IsValidShortsSearchMode(searchDefault) {
		return nil, apperrors.NewValidationErrorWithField("search_default", fmt.Sprintf("search_default must be one of %v", ShortsSearchModes))
	}

	var stored *string
	if saveDir != nil && strings.TrimSpace(*saveDir) != "" {
		dir := strings.TrimSpace(*saveDir)
		if err := s.ValidateSaveDir(dir); err != nil {
			return nil, err
		}
		cleaned := filepath.Clean(dir)
		stored = &cleaned
	}

	if searchDefault == "" {
		current, err := s.GetSearchDefault()
		if err != nil {
			return nil, err
		}
		searchDefault = current
	}

	if err := s.appSettingsRepo.UpdateShortsSettings(maxDuration, stored, searchDefault); err != nil {
		return nil, apperrors.NewInternalError("failed to save shorts settings", err)
	}

	s.logger.Info("shorts settings updated", zap.Int("max_duration", maxDuration), zap.Bool("custom_folder", stored != nil), zap.String("search_default", searchDefault))
	return s.GetSettings()
}
