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

	// Password must meet complexity requirements: 12+ chars, upper, lower, digit
	validPassword := "SecurePass123!"

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
		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(validPassword)); err != nil {
			t.Fatal("password not properly hashed")
		}
		return nil
	})

	err := svc.CreateUser("newuser", validPassword, "viewer")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	svc, _, roleRepo := newTestAdminService(t)

	// Use a valid password so we can test the role validation
	validPassword := "SecurePass123!"

	roleRepo.EXPECT().GetByName("superadmin").Return(nil, fmt.Errorf("record not found"))

	err := svc.CreateUser("newuser", validPassword, "superadmin")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Fatalf("expected 'invalid role' error, got: %v", err)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	svc, userRepo, roleRepo := newTestAdminService(t)

	// Use a valid password so we can test the username validation
	validPassword := "SecurePass123!"

	roleRepo.EXPECT().GetByName("viewer").Return(&data.Role{ID: 2, Name: "viewer"}, nil)
	userRepo.EXPECT().Exists("existing").Return(true, nil)

	err := svc.CreateUser("existing", validPassword, "viewer")
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

	// Password must meet complexity requirements: 12+ chars, upper, lower, digit
	validPassword := "NewPass12345!"

	userRepo.EXPECT().UpdatePassword(uint(5), gomock.Any()).DoAndReturn(func(id uint, hash string) error {
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(validPassword)); err != nil {
			t.Fatal("stored hash does not match new password")
		}
		return nil
	})

	err := svc.ResetUserPassword(5, validPassword)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestResetUserPassword_RepoFails(t *testing.T) {
	svc, userRepo, _ := newTestAdminService(t)

	// Password must meet complexity requirements: 12+ chars, upper, lower, digit
	validPassword := "NewPass12345!"

	userRepo.EXPECT().UpdatePassword(uint(5), gomock.Any()).Return(fmt.Errorf("connection reset"))

	err := svc.ResetUserPassword(5, validPassword)
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
	if !strings.Contains(err.Error(), "failed to reset password") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
}

func TestCreateUser_WeakPassword(t *testing.T) {
	svc, _, _ := newTestAdminService(t)

	// Test password too short
	err := svc.CreateUser("newuser", "short", "viewer")
	if err == nil {
		t.Fatal("expected error for short password")
	}
	if !strings.Contains(err.Error(), "at least 12 characters") {
		t.Fatalf("expected password length error, got: %v", err)
	}

	// Test password without uppercase
	err = svc.CreateUser("newuser", "alllowercase123", "viewer")
	if err == nil {
		t.Fatal("expected error for password without uppercase")
	}
	if !strings.Contains(err.Error(), "uppercase") {
		t.Fatalf("expected uppercase error, got: %v", err)
	}

	// Test password without lowercase
	err = svc.CreateUser("newuser", "ALLUPPERCASE123", "viewer")
	if err == nil {
		t.Fatal("expected error for password without lowercase")
	}
	if !strings.Contains(err.Error(), "lowercase") {
		t.Fatalf("expected lowercase error, got: %v", err)
	}

	// Test password without digit
	err = svc.CreateUser("newuser", "NoDigitsHere!", "viewer")
	if err == nil {
		t.Fatal("expected error for password without digit")
	}
	if !strings.Contains(err.Error(), "digit") {
		t.Fatalf("expected digit error, got: %v", err)
	}
}
