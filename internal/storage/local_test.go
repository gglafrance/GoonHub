package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage_Save(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	content := []byte("test content")
	err := storage.Save("test.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, "test.txt")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("Content mismatch: got %q, want %q", data, content)
	}
}

func TestLocalStorage_SaveCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	content := []byte("nested content")
	err := storage.Save("a/b/c/test.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, "a/b/c/test.txt")
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}
}

func TestLocalStorage_Read(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	// Create a file first
	content := []byte("read test content")
	fullPath := filepath.Join(tmpDir, "read.txt")
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader, err := storage.Read("read.txt")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read content: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("Content mismatch: got %q, want %q", data, content)
	}
}

func TestLocalStorage_ReadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	_, err := storage.Read("nonexistent.txt")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("Expected IsNotExist error, got: %v", err)
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	// Create a file first
	fullPath := filepath.Join(tmpDir, "delete.txt")
	if err := os.WriteFile(fullPath, []byte("delete me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := storage.Delete("delete.txt")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatal("File was not deleted")
	}
}

func TestLocalStorage_DeleteNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	// Deleting a nonexistent file should not return an error
	err := storage.Delete("nonexistent.txt")
	if err != nil {
		t.Fatalf("Delete of nonexistent file should not error: %v", err)
	}
}

func TestLocalStorage_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	// Test nonexistent file
	exists, err := storage.Exists("nonexistent.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("Expected false for nonexistent file")
	}

	// Create a file
	fullPath := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(fullPath, []byte("I exist"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test existing file
	exists, err = storage.Exists("exists.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected true for existing file")
	}
}

func TestLocalStorage_MkdirAll(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	err := storage.MkdirAll("x/y/z")
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	fullPath := filepath.Join(tmpDir, "x/y/z")
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Created path is not a directory")
	}
}

func TestLocalStorage_Stat(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	content := []byte("stat test content")
	fullPath := filepath.Join(tmpDir, "stat.txt")
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	info, err := storage.Stat("stat.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Size() != int64(len(content)) {
		t.Fatalf("Size mismatch: got %d, want %d", info.Size(), len(content))
	}

	if info.IsDir() {
		t.Fatal("Expected file, not directory")
	}
}

func TestLocalStorage_Glob(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage(tmpDir)

	// Create some files
	files := []string{"test1.txt", "test2.txt", "other.log"}
	for _, f := range files {
		fullPath := filepath.Join(tmpDir, f)
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	matches, err := storage.Glob("test*.txt")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("Expected 2 matches, got %d: %v", len(matches), matches)
	}
}

func TestLocalStorage_Join(t *testing.T) {
	storage := NewLocalStorage("")

	path := storage.Join("a", "b", "c.txt")
	expected := filepath.Join("a", "b", "c.txt")

	if path != expected {
		t.Fatalf("Join mismatch: got %q, want %q", path, expected)
	}
}

func TestLocalStorage_NoBasePath(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewLocalStorage("")

	// Use absolute path when no base path
	content := []byte("no base path test")
	fullPath := filepath.Join(tmpDir, "nobase.txt")

	err := storage.Save(fullPath, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	exists, err := storage.Exists(fullPath)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected file to exist")
	}
}
