package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func newTestAdminService(t *testing.T) (*AdminService, *mocks.MockUserRepository, *mocks.MockRoleRepository) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	roleRepo := mocks.NewMockRoleRepository(ctrl)

	// Create a minimal RBAC service with no permissions loaded
	roleRepo.EXPECT().GetAllRolePermissions().Return(map[string][]string{}, nil)
	rbac, err := NewRBACService(roleRepo, mocks.NewMockPermissionRepository(ctrl), zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create RBAC service: %v", err)
	}

	svc := NewAdminService(userRepo, roleRepo, rbac, zap.NewNop())
	return svc, userRepo, roleRepo
}

func TestDeleteUser_Success(t *testing.T) {
	svc, userRepo, _ := newTestAdminService(t)

	userRepo.EXPECT().Delete(uint(5)).Return(nil)

	err := svc.DeleteUser(5, 1) // deleting user 5 as user 1
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteUser_SelfDeletion(t *testing.T) {
	svc, _, _ := newTestAdminService(t)

	err := svc.DeleteUser(1, 1) // trying to delete self
	if err == nil {
		t.Fatal("expected error for self-deletion")
	}
	if !strings.Contains(err.Error(), "cannot delete your own account") {
		t.Fatalf("expected self-deletion error, got: %v", err)
	}
}

func TestCreateUser_Success(t *testing.T) {
	svc, userRepo, roleRepo := newTestAdminService(t)

	roleRepo.EXPECT().GetByName("viewer").Return(&data.Role{ID: 2, Name: "viewer"}, nil)
	userRepo.EXPECT().Exists("newuser").Return(false, nil)
	userRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(u *data.User) error {
		if u.Username != "newuser" {
			t.Fatalf("expected username newuser, got %s", u.Username)
		}
		if u.Role != "viewer" {
			t.Fatalf("expected role viewer, got %s", u.Role)
		}
		// Verify password is hashed (not plaintext)
		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("securepass")); err != nil {
			t.Fatal("password not properly hashed")
		}
		return nil
	})

	err := svc.CreateUser("newuser", "securepass", "viewer")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	svc, _, roleRepo := newTestAdminService(t)

	roleRepo.EXPECT().GetByName("superadmin").Return(nil, fmt.Errorf("record not found"))

	err := svc.CreateUser("newuser", "pass", "superadmin")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Fatalf("expected 'invalid role' error, got: %v", err)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	svc, userRepo, roleRepo := newTestAdminService(t)

	roleRepo.EXPECT().GetByName("viewer").Return(&data.Role{ID: 2, Name: "viewer"}, nil)
	userRepo.EXPECT().Exists("existing").Return(true, nil)

	err := svc.CreateUser("existing", "pass", "viewer")
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got: %v", err)
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	svc, _, roleRepo := newTestAdminService(t)

	roleRepo.EXPECT().GetByName("fakerole").Return(nil, fmt.Errorf("not found"))

	err := svc.UpdateUserRole(1, "fakerole")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Fatalf("expected 'invalid role' error, got: %v", err)
	}
}

func TestUpdateUserRole_Success(t *testing.T) {
	svc, userRepo, roleRepo := newTestAdminService(t)

	roleRepo.EXPECT().GetByName("admin").Return(&data.Role{ID: 1, Name: "admin"}, nil)
	userRepo.EXPECT().UpdateRole(uint(5), "admin").Return(nil)

	err := svc.UpdateUserRole(5, "admin")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestResetUserPassword_Success(t *testing.T) {
	svc, userRepo, _ := newTestAdminService(t)

	userRepo.EXPECT().UpdatePassword(uint(5), gomock.Any()).DoAndReturn(func(id uint, hash string) error {
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("newpass123")); err != nil {
			t.Fatal("stored hash does not match new password")
		}
		return nil
	})

	err := svc.ResetUserPassword(5, "newpass123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestResetUserPassword_RepoFails(t *testing.T) {
	svc, userRepo, _ := newTestAdminService(t)

	userRepo.EXPECT().UpdatePassword(uint(5), gomock.Any()).Return(fmt.Errorf("connection reset"))

	err := svc.ResetUserPassword(5, "newpass123")
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
	if !strings.Contains(err.Error(), "failed to reset password") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
}
