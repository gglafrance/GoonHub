// Package storage provides a file storage abstraction layer.
// This allows for easy testing and future extension to other storage backends (S3, etc.).
package storage

import (
	"io"
)

// Storage defines the interface for file storage operations.
// Implementations should be safe for concurrent use.
type Storage interface {
	// Save writes the content from reader to the specified path.
	// Creates parent directories as needed.
	Save(path string, reader io.Reader) error

	// Read returns a reader for the file at the specified path.
	// The caller is responsible for closing the reader.
	Read(path string) (io.ReadCloser, error)

	// Delete removes the file at the specified path.
	// Returns nil if the file doesn't exist.
	Delete(path string) error

	// Exists checks if a file exists at the specified path.
	Exists(path string) (bool, error)

	// MkdirAll creates a directory at the specified path along with any necessary parents.
	MkdirAll(path string) error

	// Stat returns file info for the specified path.
	Stat(path string) (FileInfo, error)

	// Glob returns all paths matching the pattern.
	Glob(pattern string) ([]string, error)

	// Join joins path elements together.
	// This allows storage implementations to use their own path separator.
	Join(elem ...string) string
}

// FileInfo contains information about a file.
type FileInfo interface {
	// Size returns the file size in bytes.
	Size() int64
	// ModTime returns the modification time.
	ModTime() int64
	// IsDir returns true if the path is a directory.
	IsDir() bool
}
