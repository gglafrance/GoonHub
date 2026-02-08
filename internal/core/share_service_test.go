package core

import (
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestShareService(t *testing.T) (*ShareService, *mocks.MockShareLinkRepository, *mocks.MockSceneRepository) {
	ctrl := gomock.NewController(t)
	shareLinkRepo := mocks.NewMockShareLinkRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)

	svc := NewShareService(shareLinkRepo, sceneRepo, zap.NewNop())
	return svc, shareLinkRepo, sceneRepo
}

func TestCreateShareLink_Success(t *testing.T) {
	svc, shareLinkRepo, sceneRepo := newTestShareService(t)

	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1, Title: "Test"}, nil)
	shareLinkRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(link *data.ShareLink) error {
		if link.SceneID != 1 {
			t.Fatalf("expected scene_id 1, got %d", link.SceneID)
		}
		if link.UserID != 10 {
			t.Fatalf("expected user_id 10, got %d", link.UserID)
		}
		if link.ShareType != data.ShareTypePublic {
			t.Fatalf("expected share_type public, got %q", link.ShareType)
		}
		if link.Token == "" {
			t.Fatal("expected non-empty token")
		}
		if link.ExpiresAt == nil {
			t.Fatal("expected non-nil expires_at for 24h expiry")
		}
		link.ID = 1
		return nil
	})

	link, err := svc.CreateShareLink(10, 1, "public", "24h")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if link.ID != 1 {
		t.Fatalf("expected link ID 1, got %d", link.ID)
	}
}

func TestCreateShareLink_NeverExpires(t *testing.T) {
	svc, shareLinkRepo, sceneRepo := newTestShareService(t)

	sceneRepo.EXPECT().GetByID(uint(5)).Return(&data.Scene{ID: 5}, nil)
	shareLinkRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(link *data.ShareLink) error {
		if link.ExpiresAt != nil {
			t.Fatal("expected nil expires_at for 'never' expiry")
		}
		link.ID = 2
		return nil
	})

	link, err := svc.CreateShareLink(10, 5, "auth_required", "never")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if link.ID != 2 {
		t.Fatalf("expected link ID 2, got %d", link.ID)
	}
}

func TestCreateShareLink_InvalidShareType(t *testing.T) {
	svc, _, _ := newTestShareService(t)

	_, err := svc.CreateShareLink(10, 1, "invalid", "24h")
	if err == nil {
		t.Fatal("expected error for invalid share type")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestCreateShareLink_InvalidExpiration(t *testing.T) {
	svc, _, _ := newTestShareService(t)

	_, err := svc.CreateShareLink(10, 1, "public", "99d")
	if err == nil {
		t.Fatal("expected error for invalid expiration")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestCreateShareLink_SceneNotFound(t *testing.T) {
	svc, _, sceneRepo := newTestShareService(t)

	sceneRepo.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.CreateShareLink(10, 999, "public", "24h")
	if err == nil {
		t.Fatal("expected error for non-existent scene")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not-found error, got: %v", err)
	}
}

func TestListShareLinks_Success(t *testing.T) {
	svc, shareLinkRepo, _ := newTestShareService(t)

	expected := []data.ShareLink{
		{ID: 1, Token: "abc", SceneID: 1, UserID: 10, ShareType: "public"},
		{ID: 2, Token: "def", SceneID: 1, UserID: 10, ShareType: "auth_required"},
	}
	shareLinkRepo.EXPECT().ListBySceneAndUser(uint(1), uint(10)).Return(expected, nil)

	links, err := svc.ListShareLinks(10, 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
}

func TestDeleteShareLink_Success(t *testing.T) {
	svc, shareLinkRepo, _ := newTestShareService(t)

	shareLinkRepo.EXPECT().Delete(uint(1), uint(10)).Return(nil)

	err := svc.DeleteShareLink(1, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteShareLink_NotFound(t *testing.T) {
	svc, shareLinkRepo, _ := newTestShareService(t)

	shareLinkRepo.EXPECT().Delete(uint(999), uint(10)).Return(gorm.ErrRecordNotFound)

	err := svc.DeleteShareLink(999, 10)
	if err == nil {
		t.Fatal("expected error for non-existent link")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not-found error, got: %v", err)
	}
}

func TestResolveShareLink_Success(t *testing.T) {
	svc, shareLinkRepo, sceneRepo := newTestShareService(t)

	link := &data.ShareLink{
		ID:        1,
		Token:     "test-token",
		SceneID:   1,
		UserID:    10,
		ShareType: "public",
	}
	shareLinkRepo.EXPECT().GetByToken("test-token").Return(link, nil)
	shareLinkRepo.EXPECT().IncrementViewCount(uint(1)).Return(nil)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:          1,
		Title:       "Test Scene",
		Description: "A test scene",
		Duration:    120,
		Studio:      "Test Studio",
	}, nil)

	resolved, err := svc.ResolveShareLink("test-token", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resolved.Scene.Title != "Test Scene" {
		t.Fatalf("expected title 'Test Scene', got %q", resolved.Scene.Title)
	}
	if resolved.Scene.Duration != 120 {
		t.Fatalf("expected duration 120, got %d", resolved.Scene.Duration)
	}
}

func TestResolveShareLink_NotFound(t *testing.T) {
	svc, shareLinkRepo, _ := newTestShareService(t)

	shareLinkRepo.EXPECT().GetByToken("missing").Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.ResolveShareLink("missing", false)
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not-found error, got: %v", err)
	}
}

func TestResolveShareLink_Expired(t *testing.T) {
	svc, shareLinkRepo, _ := newTestShareService(t)

	expired := time.Now().Add(-time.Hour)
	link := &data.ShareLink{
		ID:        1,
		Token:     "expired-token",
		SceneID:   1,
		UserID:    10,
		ShareType: "public",
		ExpiresAt: &expired,
	}
	shareLinkRepo.EXPECT().GetByToken("expired-token").Return(link, nil)

	_, err := svc.ResolveShareLink("expired-token", false)
	if err == nil {
		t.Fatal("expected error for expired link")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error (expired), got: %v", err)
	}
}

func TestResolveShareLink_AuthRequired_NotAuthenticated(t *testing.T) {
	svc, shareLinkRepo, _ := newTestShareService(t)

	link := &data.ShareLink{
		ID:        1,
		Token:     "auth-token",
		SceneID:   1,
		UserID:    10,
		ShareType: "auth_required",
	}
	shareLinkRepo.EXPECT().GetByToken("auth-token").Return(link, nil)

	_, err := svc.ResolveShareLink("auth-token", false)
	if err == nil {
		t.Fatal("expected error for unauthenticated access to auth_required link")
	}
	if !apperrors.IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error, got: %v", err)
	}
}

func TestResolveShareLink_AuthRequired_Authenticated(t *testing.T) {
	svc, shareLinkRepo, sceneRepo := newTestShareService(t)

	link := &data.ShareLink{
		ID:        1,
		Token:     "auth-token",
		SceneID:   1,
		UserID:    10,
		ShareType: "auth_required",
	}
	shareLinkRepo.EXPECT().GetByToken("auth-token").Return(link, nil)
	shareLinkRepo.EXPECT().IncrementViewCount(uint(1)).Return(nil)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:    1,
		Title: "Auth Scene",
	}, nil)

	resolved, err := svc.ResolveShareLink("auth-token", true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resolved.Scene.Title != "Auth Scene" {
		t.Fatalf("expected title 'Auth Scene', got %q", resolved.Scene.Title)
	}
}
