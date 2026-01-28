package storage

import (
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements Storage using the local filesystem.
type LocalStorage struct {
	// BasePath is the root directory for all storage operations.
	// If empty, paths are used as-is.
	BasePath string
}

// NewLocalStorage creates a new LocalStorage instance.
// basePath is the root directory for storage operations.
// Pass empty string to use paths as-is (no base path prefix).
func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{BasePath: basePath}
}

// resolvePath converts a relative path to an absolute path within the storage.
func (s *LocalStorage) resolvePath(path string) string {
	if s.BasePath == "" {
		return path
	}
	// If path is already absolute and starts with BasePath, use it directly
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.BasePath, path)
}

// Save writes the content from reader to the specified path.
func (s *LocalStorage) Save(path string, reader io.Reader) error {
	fullPath := s.resolvePath(path)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

// Read returns a reader for the file at the specified path.
func (s *LocalStorage) Read(path string) (io.ReadCloser, error) {
	fullPath := s.resolvePath(path)
	return os.Open(fullPath)
}

// Delete removes the file at the specified path.
func (s *LocalStorage) Delete(path string) error {
	fullPath := s.resolvePath(path)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Exists checks if a file exists at the specified path.
func (s *LocalStorage) Exists(path string) (bool, error) {
	fullPath := s.resolvePath(path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// MkdirAll creates a directory at the specified path.
func (s *LocalStorage) MkdirAll(path string) error {
	fullPath := s.resolvePath(path)
	return os.MkdirAll(fullPath, 0755)
}

// Stat returns file info for the specified path.
func (s *LocalStorage) Stat(path string) (FileInfo, error) {
	fullPath := s.resolvePath(path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	return &localFileInfo{info: info}, nil
}

// Glob returns all paths matching the pattern.
func (s *LocalStorage) Glob(pattern string) ([]string, error) {
	fullPattern := s.resolvePath(pattern)
	return filepath.Glob(fullPattern)
}

// Join joins path elements together.
func (s *LocalStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// localFileInfo wraps os.FileInfo to implement FileInfo.
type localFileInfo struct {
	info os.FileInfo
}

func (f *localFileInfo) Size() int64 {
	return f.info.Size()
}

func (f *localFileInfo) ModTime() int64 {
	return f.info.ModTime().Unix()
}

func (f *localFileInfo) IsDir() bool {
	return f.info.IsDir()
}
