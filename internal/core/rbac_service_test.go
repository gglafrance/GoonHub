package core

import (
	"fmt"
	"goonhub/internal/mocks"
	"sync"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestRBACService(t *testing.T, rolePerms map[string][]string) (*RBACService, *mocks.MockRoleRepository, *mocks.MockPermissionRepository) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	roleRepo.EXPECT().GetAllRolePermissions().Return(rolePerms, nil)

	svc, err := NewRBACService(roleRepo, permRepo, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create RBAC service: %v", err)
	}
	return svc, roleRepo, permRepo
}

func TestHasPermission_Exists(t *testing.T) {
	perms := map[string][]string{
		"admin": {"videos.upload", "videos.delete", "users.manage"},
	}
	svc, _, _ := newTestRBACService(t, perms)

	if !svc.HasPermission("admin", "videos.upload") {
		t.Fatal("expected admin to have videos.upload permission")
	}
	if !svc.HasPermission("admin", "videos.delete") {
		t.Fatal("expected admin to have videos.delete permission")
	}
}

func TestHasPermission_UnknownRole(t *testing.T) {
	perms := map[string][]string{
		"admin": {"videos.upload"},
	}
	svc, _, _ := newTestRBACService(t, perms)

	if svc.HasPermission("nonexistent_role", "videos.upload") {
		t.Fatal("expected false for unknown role")
	}
}

func TestHasPermission_UnknownPermission(t *testing.T) {
	perms := map[string][]string{
		"viewer": {"videos.view"},
	}
	svc, _, _ := newTestRBACService(t, perms)

	if svc.HasPermission("viewer", "videos.delete") {
		t.Fatal("expected false for unknown permission on known role")
	}
}

func TestHasPermission_EmptyCache(t *testing.T) {
	perms := map[string][]string{}
	svc, _, _ := newTestRBACService(t, perms)

	if svc.HasPermission("admin", "anything") {
		t.Fatal("expected false when cache is empty")
	}
}

func TestRefreshCache_PopulatesCorrectly(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	initialPerms := map[string][]string{
		"admin":  {"videos.upload", "videos.delete"},
		"viewer": {"videos.view"},
	}
	roleRepo.EXPECT().GetAllRolePermissions().Return(initialPerms, nil)

	svc, err := NewRBACService(roleRepo, permRepo, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create RBAC service: %v", err)
	}

	if !svc.HasPermission("admin", "videos.upload") {
		t.Fatal("admin should have videos.upload")
	}
	if !svc.HasPermission("admin", "videos.delete") {
		t.Fatal("admin should have videos.delete")
	}
	if !svc.HasPermission("viewer", "videos.view") {
		t.Fatal("viewer should have videos.view")
	}
	if svc.HasPermission("viewer", "videos.upload") {
		t.Fatal("viewer should not have videos.upload")
	}
}

func TestRefreshCache_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	roleRepo.EXPECT().GetAllRolePermissions().Return(nil, fmt.Errorf("connection refused"))

	_, err := NewRBACService(roleRepo, permRepo, zap.NewNop())
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

func TestRefreshCache_OverwritesPrevious(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	// First call during init
	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"admin": {"videos.upload"},
	}, nil)

	svc, err := NewRBACService(roleRepo, permRepo, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create RBAC service: %v", err)
	}

	if !svc.HasPermission("admin", "videos.upload") {
		t.Fatal("admin should have videos.upload initially")
	}

	// Second call replaces the cache
	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"admin": {"users.manage"},
	}, nil)

	if err := svc.RefreshCache(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if svc.HasPermission("admin", "videos.upload") {
		t.Fatal("admin should no longer have videos.upload after refresh")
	}
	if !svc.HasPermission("admin", "users.manage") {
		t.Fatal("admin should have users.manage after refresh")
	}
}

func TestHasPermission_ConcurrentReads(t *testing.T) {
	perms := map[string][]string{
		"admin":  {"videos.upload", "videos.delete"},
		"viewer": {"videos.view"},
	}
	svc, _, _ := newTestRBACService(t, perms)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.HasPermission("admin", "videos.upload")
			svc.HasPermission("viewer", "videos.view")
			svc.HasPermission("unknown", "anything")
		}()
	}
	wg.Wait()
}

func TestHasPermission_ConcurrentReadsDuringRefresh(t *testing.T) {
	ctrl := gomock.NewController(t)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	permRepo := mocks.NewMockPermissionRepository(ctrl)

	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"admin": {"videos.upload"},
	}, nil)

	svc, err := NewRBACService(roleRepo, permRepo, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create RBAC service: %v", err)
	}

	// Allow multiple refresh calls during concurrent test
	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{
		"admin": {"videos.upload", "users.manage"},
	}, nil).AnyTimes()

	var wg sync.WaitGroup
	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				svc.HasPermission("admin", "videos.upload")
			}
		}()
	}
	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				svc.RefreshCache()
			}
		}()
	}
	wg.Wait()
}
