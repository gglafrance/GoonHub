package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestStoragePathService(t *testing.T) (*StoragePathService, *mocks.MockStoragePathRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStoragePathRepository(ctrl)
	svc := NewStoragePathService(repo, zap.NewNop())
	return svc, repo
}

func TestGetDiskUsage_ValidPath(t *testing.T) {
	svc, _ := newTestStoragePathService(t)
	dir := t.TempDir()

	usage := svc.GetDiskUsage(dir)
	if usage == nil {
		t.Fatal("expected non-nil usage for valid temp dir")
	}
	if usage.TotalBytes == 0 {
		t.Fatal("expected TotalBytes > 0")
	}
	if usage.FreeBytes == 0 {
		t.Fatal("expected FreeBytes > 0")
	}
	if usage.UsedPct < 0 || usage.UsedPct > 100 {
		t.Fatalf("expected UsedPct in [0,100], got %f", usage.UsedPct)
	}
	if usage.UsedBytes+usage.FreeBytes != usage.TotalBytes {
		t.Fatalf("expected UsedBytes(%d) + FreeBytes(%d) == TotalBytes(%d)",
			usage.UsedBytes, usage.FreeBytes, usage.TotalBytes)
	}
}

func TestGetDiskUsage_InvalidPath(t *testing.T) {
	svc, _ := newTestStoragePathService(t)

	usage := svc.GetDiskUsage("/nonexistent/path/that/does/not/exist")
	if usage != nil {
		t.Fatal("expected nil usage for nonexistent path")
	}
}

func TestListWithDiskUsage_Success(t *testing.T) {
	svc, repo := newTestStoragePathService(t)
	dir := t.TempDir()

	paths := []data.StoragePath{
		{ID: 1, Name: "test", Path: dir, IsDefault: true},
	}
	repo.EXPECT().List().Return(paths, nil)

	resultPaths, usageMap, err := svc.ListWithDiskUsage()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(resultPaths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(resultPaths))
	}
	if usageMap[1] == nil {
		t.Fatal("expected non-nil usage for path ID 1")
	}
	if usageMap[1].TotalBytes == 0 {
		t.Fatal("expected TotalBytes > 0")
	}
}

func TestListWithDiskUsage_RepoError(t *testing.T) {
	svc, repo := newTestStoragePathService(t)

	repo.EXPECT().List().Return(nil, fmt.Errorf("db connection failed"))

	_, _, err := svc.ListWithDiskUsage()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListWithDiskUsage_InvalidPathReturnsNilUsage(t *testing.T) {
	svc, repo := newTestStoragePathService(t)

	paths := []data.StoragePath{
		{ID: 1, Name: "bad", Path: "/nonexistent/path/xyz", IsDefault: false},
	}
	repo.EXPECT().List().Return(paths, nil)

	resultPaths, usageMap, err := svc.ListWithDiskUsage()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(resultPaths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(resultPaths))
	}
	if usageMap[1] != nil {
		t.Fatal("expected nil usage for nonexistent path")
	}
}
