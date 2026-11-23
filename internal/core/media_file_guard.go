package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

// MediaFileGuard is the single place that decides whether GoonHub may modify (delete, move,
// rename or write) a media file. Files under a read-only storage path are never modified.
// Generated files (thumbnails, sprites, VTT, previews in the data directory) are not media and
// are not checked here.
//
// A nil guard allows everything (no storage path is known to be read-only).
type MediaFileGuard struct {
	repo   data.StoragePathRepository
	logger *zap.Logger
}

// NewMediaFileGuard creates a MediaFileGuard backed by the storage paths repository.
func NewMediaFileGuard(repo data.StoragePathRepository, logger *zap.Logger) *MediaFileGuard {
	return &MediaFileGuard{
		repo:   repo,
		logger: logger.With(zap.String("component", "media_file_guard")),
	}
}

// ProtectingPath returns the read-only storage path that protects a media file, or nil when
// the file may be modified. A file is protected when storagePathID points to a read-only path,
// or when filePath lies under a read-only path (checked too, for scenes whose storage_path_id
// is missing or stale). An error means the decision could not be made: callers must then
// leave the file alone.
func (g *MediaFileGuard) ProtectingPath(storagePathID *uint, filePath string) (*data.StoragePath, error) {
	if g == nil || g.repo == nil {
		return nil, nil
	}
	paths, err := g.repo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list storage paths: %w", err)
	}
	return protectingPathAmong(paths, storagePathID, filePath), nil
}

// ReadOnlyChecker returns a function telling whether a media file is protected (see
// ProtectingPath), with the storage paths read once: for checking many files at a time.
func (g *MediaFileGuard) ReadOnlyChecker() (func(storagePathID *uint, filePath string) bool, error) {
	if g == nil || g.repo == nil {
		return func(*uint, string) bool { return false }, nil
	}
	paths, err := g.repo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list storage paths: %w", err)
	}
	return func(storagePathID *uint, filePath string) bool {
		return protectingPathAmong(paths, storagePathID, filePath) != nil
	}, nil
}

// protectingPathAmong is ProtectingPath on a list of storage paths.
func protectingPathAmong(paths []data.StoragePath, storagePathID *uint, filePath string) *data.StoragePath {
	for i := range paths {
		p := &paths[i]
		if !p.ReadOnly {
			continue
		}
		if storagePathID != nil && *storagePathID == p.ID {
			return p
		}
		if filePath != "" && pathWithin(filePath, p.Path) {
			return p
		}
	}
	return nil
}

// CheckModifiable returns nil when GoonHub may delete, move, rename or write the file, a
// ValidationError (apperrors.ErrStoragePathReadOnly) when it is under a read-only storage path,
// and an InternalError when the check itself failed.
func (g *MediaFileGuard) CheckModifiable(storagePathID *uint, filePath string) error {
	protecting, err := g.ProtectingPath(storagePathID, filePath)
	if err != nil {
		return apperrors.NewInternalError("failed to check storage path permissions", err)
	}
	if protecting != nil {
		return apperrors.ErrStoragePathReadOnly(protecting.Name)
	}
	return nil
}

// CheckSceneModifiable is CheckModifiable for a scene's media file. Operations whose only
// purpose is to change the file (move, rename, rewrite) must call it and refuse on error.
func (g *MediaFileGuard) CheckSceneModifiable(scene *data.Scene) error {
	return g.CheckModifiable(scene.StoragePathID, scene.StoredPath)
}

// RemoveSceneFile deletes a scene's media file, unless it is under a read-only storage path
// (or the check fails), in which case the file is kept and an info log is written. Returns
// true when the file was removed or did not exist.
func (g *MediaFileGuard) RemoveSceneFile(scene *data.Scene) bool {
	if scene == nil || scene.StoredPath == "" {
		return true
	}
	logger := g.log()
	protecting, err := g.ProtectingPath(scene.StoragePathID, scene.StoredPath)
	if err != nil {
		logger.Warn("Kept media file: could not check whether its storage path is read-only",
			zap.Uint("scene_id", scene.ID),
			zap.String("path", scene.StoredPath),
			zap.Error(err),
		)
		return false
	}
	if protecting != nil {
		logger.Info("Kept media file on read-only storage path",
			zap.Uint("scene_id", scene.ID),
			zap.String("path", scene.StoredPath),
			zap.Uint("storage_path_id", protecting.ID),
			zap.String("storage_path", protecting.Name),
		)
		return false
	}
	if err := os.Remove(scene.StoredPath); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to delete video file",
			zap.Uint("scene_id", scene.ID),
			zap.String("path", scene.StoredPath),
			zap.Error(err),
		)
		return false
	}
	return true
}

// RemoveMediaFile deletes a media file that is not a scene's main file (e.g. a copy of a
// scene's file), unless it is under a read-only storage path: then nothing is deleted and the
// ValidationError of CheckModifiable is returned. A file already gone is not an error.
func (g *MediaFileGuard) RemoveMediaFile(storagePathID *uint, filePath string) error {
	if filePath == "" {
		return apperrors.NewValidationError("no file path")
	}
	if err := g.CheckModifiable(storagePathID, filePath); err != nil {
		return err
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return apperrors.NewInternalError("failed to delete the file", err)
	}
	g.log().Info("Deleted media file", zap.String("path", filePath))
	return nil
}

func (g *MediaFileGuard) log() *zap.Logger {
	if g == nil || g.logger == nil {
		return zap.NewNop()
	}
	return g.logger
}

// pathWithin reports whether file is root itself or lies under root. Both paths are compared
// as given (cleaned), as absolute paths and with symlinks resolved, so that a relative storage
// path, a different spelling or a symlinked mount still matches.
func pathWithin(file, root string) bool {
	if root == "" {
		return false
	}
	for _, r := range pathForms(root) {
		for _, f := range pathForms(file) {
			if isSubPath(f, r) {
				return true
			}
		}
	}
	return false
}

// pathForms returns the cleaned, absolute and symlink-resolved forms of a path. A missing
// file is resolved through its parent directory.
func pathForms(p string) []string {
	forms := []string{filepath.Clean(p)}
	if abs, err := filepath.Abs(p); err == nil {
		forms = append(forms, abs)
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		dir, dirErr := filepath.EvalSymlinks(filepath.Dir(p))
		if dirErr != nil {
			return forms
		}
		resolved = filepath.Join(dir, filepath.Base(p))
	}
	if abs, err := filepath.Abs(resolved); err == nil {
		resolved = abs
	}
	return append(forms, resolved)
}

func isSubPath(file, root string) bool {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
