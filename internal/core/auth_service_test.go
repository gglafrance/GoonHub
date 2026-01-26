package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuthService(t *testing.T) (*AuthService, *mocks.MockUserRepository, *mocks.MockRevokedTokenRepository) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	// 32-byte key for PASETO v2 symmetric encryption
	key := "01234567890123456789012345678901"
	// Lockout: 5 attempts, 15 minute duration
	svc, err := NewAuthService(userRepo, revokedRepo, key, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}
	return svc, userRepo, revokedRepo
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(h)
}

func TestLogin_Success(t *testing.T) {
	svc, userRepo, _ := newTestAuthService(t)

	hashed := hashPassword(t, "correctpass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, returnedUser, err := svc.Login("alice", "correctpass")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if returnedUser.ID != 1 {
		t.Fatalf("expected user ID 1, got %d", returnedUser.ID)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, userRepo, _ := newTestAuthService(t)

	userRepo.EXPECT().GetByUsername("nobody").Return(nil, fmt.Errorf("record not found"))

	_, _, err := svc.Login("nobody", "pass")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid credentials" {
		t.Fatalf("expected generic error, got: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, userRepo, _ := newTestAuthService(t)

	hashed := hashPassword(t, "correctpass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "user"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)

	_, _, err := svc.Login("alice", "wrongpass")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid credentials" {
		t.Fatalf("expected generic error, got: %v", err)
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	svc, userRepo, _ := newTestAuthService(t)

	userRepo.EXPECT().GetByUsername("").Return(nil, fmt.Errorf("record not found"))

	_, _, err := svc.Login("", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	svc, userRepo, revokedRepo := newTestAuthService(t)

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 42, Username: "bob", Password: hashed, Role: "viewer"}

	userRepo.EXPECT().GetByUsername("bob").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(42)).Return(nil)

	token, _, err := svc.Login("bob", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	tokenHash := sha256Hash(token)
	revokedRepo.EXPECT().IsRevoked(tokenHash).Return(false, nil)

	payload, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if payload.UserID != 42 {
		t.Fatalf("expected UserID 42, got %d", payload.UserID)
	}
	if payload.Username != "bob" {
		t.Fatalf("expected Username bob, got %s", payload.Username)
	}
	if payload.Role != "viewer" {
		t.Fatalf("expected Role viewer, got %s", payload.Role)
	}
}

func TestValidateToken_Revoked(t *testing.T) {
	svc, userRepo, revokedRepo := newTestAuthService(t)

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, _, err := svc.Login("alice", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	tokenHash := sha256Hash(token)
	revokedRepo.EXPECT().IsRevoked(tokenHash).Return(true, nil)

	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error for revoked token")
	}
	if !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("expected revoked error, got: %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	key := "01234567890123456789012345678901"
	// TTL of -1 hour means token is already expired
	svc, err := NewAuthService(userRepo, revokedRepo, key, -1*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, _, err := svc.Login("alice", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	tokenHash := sha256Hash(token)
	revokedRepo.EXPECT().IsRevoked(tokenHash).Return(false, nil)

	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", err)
	}
}

func TestValidateToken_InvalidFormat(t *testing.T) {
	svc, _, revokedRepo := newTestAuthService(t)

	garbled := "not-a-real-token-at-all"
	tokenHash := sha256Hash(garbled)
	revokedRepo.EXPECT().IsRevoked(tokenHash).Return(false, nil)

	_, err := svc.ValidateToken(garbled)
	if err == nil {
		t.Fatal("expected error for garbled token")
	}
}

func TestValidateToken_WrongKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	key1 := "01234567890123456789012345678901"
	key2 := "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"

	svc1, err := NewAuthService(userRepo, revokedRepo, key1, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create auth service 1: %v", err)
	}
	svc2, err := NewAuthService(userRepo, revokedRepo, key2, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create auth service 2: %v", err)
	}

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, _, err := svc1.Login("alice", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	tokenHash := sha256Hash(token)
	revokedRepo.EXPECT().IsRevoked(tokenHash).Return(false, nil)

	_, err = svc2.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestValidateToken_DBErrorOnRevocationCheck(t *testing.T) {
	svc, userRepo, revokedRepo := newTestAuthService(t)

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, _, err := svc.Login("alice", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	tokenHash := sha256Hash(token)
	revokedRepo.EXPECT().IsRevoked(tokenHash).Return(false, fmt.Errorf("database connection lost"))

	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error when DB fails on revocation check")
	}
	if !strings.Contains(err.Error(), "failed to validate token") {
		t.Fatalf("expected propagated error, got: %v", err)
	}
}

func TestRevokeToken_Success(t *testing.T) {
	svc, userRepo, revokedRepo := newTestAuthService(t)

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, _, err := svc.Login("alice", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	expectedHash := sha256Hash(token)
	revokedRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(rt *data.RevokedToken) error {
		if rt.TokenHash != expectedHash {
			t.Fatalf("expected hash %s, got %s", expectedHash, rt.TokenHash)
		}
		if rt.Reason != "logout" {
			t.Fatalf("expected reason 'logout', got %s", rt.Reason)
		}
		if rt.ExpiresAt.IsZero() {
			t.Fatal("expected non-zero expiry")
		}
		return nil
	})

	err = svc.RevokeToken(token, "logout")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRevokeToken_DBError(t *testing.T) {
	svc, userRepo, revokedRepo := newTestAuthService(t)

	hashed := hashPassword(t, "pass")
	user := &data.User{ID: 1, Username: "alice", Password: hashed, Role: "admin"}

	userRepo.EXPECT().GetByUsername("alice").Return(user, nil)
	userRepo.EXPECT().UpdateLastLogin(uint(1)).Return(nil)

	token, _, err := svc.Login("alice", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	revokedRepo.EXPECT().Create(gomock.Any()).Return(fmt.Errorf("disk full"))

	err = svc.RevokeToken(token, "logout")
	if err == nil {
		t.Fatal("expected error when DB fails")
	}
	if !strings.Contains(err.Error(), "failed to revoke token") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
}

func sha256Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func TestNewAuthService_ShortKeyRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	// Key shorter than 32 bytes should be rejected
	shortKey := "tooshort"
	_, err := NewAuthService(userRepo, revokedRepo, shortKey, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err == nil {
		t.Fatal("expected error for short PASETO key")
	}
	if err != ErrPasetoKeyTooShort {
		t.Fatalf("expected ErrPasetoKeyTooShort, got: %v", err)
	}
}

func TestNewAuthService_ValidKeyAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	// Exactly 32 bytes should work
	validKey := "01234567890123456789012345678901"
	svc, err := NewAuthService(userRepo, revokedRepo, validKey, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("expected no error for valid key, got: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewAuthService_HexKeyAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	revokedRepo := mocks.NewMockRevokedTokenRepository(ctrl)

	// 64-character hex string (32 bytes when decoded) should work
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	svc, err := NewAuthService(userRepo, revokedRepo, hexKey, 24*time.Hour, 5, 15*time.Minute, zap.NewNop())
	if err != nil {
		t.Fatalf("expected no error for hex key, got: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
