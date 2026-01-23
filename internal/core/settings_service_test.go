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

func newTestSettingsService(t *testing.T) (*SettingsService, *mocks.MockUserSettingsRepository, *mocks.MockUserRepository) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)

	svc := NewSettingsService(settingsRepo, userRepo, zap.NewNop())
	return svc, settingsRepo, userRepo
}

func TestGetSettings_Defaults(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)

	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, fmt.Errorf("record not found"))

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if settings.DefaultVolume != 100 {
		t.Fatalf("expected default volume 100, got %d", settings.DefaultVolume)
	}
	if settings.VideosPerPage != 20 {
		t.Fatalf("expected default videos per page 20, got %d", settings.VideosPerPage)
	}
	if settings.DefaultSortOrder != "created_at_desc" {
		t.Fatalf("expected default sort order created_at_desc, got %s", settings.DefaultSortOrder)
	}
	if settings.Autoplay != false {
		t.Fatal("expected default autoplay false")
	}
	if settings.Loop != false {
		t.Fatal("expected default loop false")
	}
}

func TestUpdatePlayerSettings_VolumeBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		volume  int
		wantErr bool
	}{
		{"volume 0 valid", 0, false},
		{"volume 100 valid", 100, false},
		{"volume -1 invalid", -1, true},
		{"volume 101 invalid", 101, true},
		{"volume 50 valid", 50, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)

			if !tt.wantErr {
				settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{UserID: 1}, nil)
				settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)
			}

			_, err := svc.UpdatePlayerSettings(1, false, tt.volume, false)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateAppSettings_ValidSortOrders(t *testing.T) {
	validOrders := []string{
		"created_at_desc", "created_at_asc",
		"title_asc", "title_desc",
		"duration_asc", "duration_desc",
		"size_asc", "size_desc",
	}

	for _, order := range validOrders {
		t.Run(order, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)

			settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{UserID: 1}, nil)
			settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)

			_, err := svc.UpdateAppSettings(1, 20, order)
			if err != nil {
				t.Fatalf("expected valid sort order %q to be accepted, got error: %v", order, err)
			}
		})
	}
}

func TestUpdateAppSettings_InvalidSortOrder(t *testing.T) {
	svc, _, _ := newTestSettingsService(t)

	_, err := svc.UpdateAppSettings(1, 20, "random_order")
	if err == nil {
		t.Fatal("expected error for invalid sort order")
	}
	if !strings.Contains(err.Error(), "invalid sort order") {
		t.Fatalf("expected 'invalid sort order' error, got: %v", err)
	}
}

func TestUpdateAppSettings_VideosPerPageBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		wantErr bool
	}{
		{"1 valid", 1, false},
		{"100 valid", 100, false},
		{"0 invalid", 0, true},
		{"101 invalid", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)

			if !tt.wantErr {
				settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{UserID: 1}, nil)
				settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)
			}

			_, err := svc.UpdateAppSettings(1, tt.count, "created_at_desc")
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestChangePassword_Success(t *testing.T) {
	svc, _, userRepo := newTestSettingsService(t)

	oldHash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.MinCost)
	user := &data.User{ID: 5, Username: "bob", Password: string(oldHash)}

	userRepo.EXPECT().GetByID(uint(5)).Return(user, nil)
	userRepo.EXPECT().UpdatePassword(uint(5), gomock.Any()).DoAndReturn(func(id uint, hash string) error {
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("newpass")); err != nil {
			t.Fatalf("stored hash does not match new password")
		}
		return nil
	})

	err := svc.ChangePassword(5, "oldpass", "newpass")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	svc, _, userRepo := newTestSettingsService(t)

	oldHash, _ := bcrypt.GenerateFromPassword([]byte("realpass"), bcrypt.MinCost)
	user := &data.User{ID: 5, Username: "bob", Password: string(oldHash)}

	userRepo.EXPECT().GetByID(uint(5)).Return(user, nil)

	err := svc.ChangePassword(5, "wrongpass", "newpass")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "current password is incorrect") {
		t.Fatalf("expected 'current password is incorrect', got: %v", err)
	}
}

func TestChangePassword_UserNotFound(t *testing.T) {
	svc, _, userRepo := newTestSettingsService(t)

	userRepo.EXPECT().GetByID(uint(99)).Return(nil, fmt.Errorf("record not found"))

	err := svc.ChangePassword(99, "old", "new")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to find user") {
		t.Fatalf("expected propagated error, got: %v", err)
	}
}

func TestChangeUsername_Success(t *testing.T) {
	svc, _, userRepo := newTestSettingsService(t)

	userRepo.EXPECT().Exists("newname").Return(false, nil)
	userRepo.EXPECT().UpdateUsername(uint(1), "newname").Return(nil)

	err := svc.ChangeUsername(1, "newname")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestChangeUsername_TooShort(t *testing.T) {
	svc, _, _ := newTestSettingsService(t)

	err := svc.ChangeUsername(1, "ab")
	if err == nil {
		t.Fatal("expected error for 2-char username")
	}
	if !strings.Contains(err.Error(), "at least 3 characters") {
		t.Fatalf("expected length error, got: %v", err)
	}
}

func TestChangeUsername_ExactMinLength(t *testing.T) {
	svc, _, userRepo := newTestSettingsService(t)

	userRepo.EXPECT().Exists("abc").Return(false, nil)
	userRepo.EXPECT().UpdateUsername(uint(1), "abc").Return(nil)

	err := svc.ChangeUsername(1, "abc")
	if err != nil {
		t.Fatalf("expected 3-char username to be accepted, got: %v", err)
	}
}

func TestChangeUsername_AlreadyTaken(t *testing.T) {
	svc, _, userRepo := newTestSettingsService(t)

	userRepo.EXPECT().Exists("taken").Return(true, nil)

	err := svc.ChangeUsername(1, "taken")
	if err == nil {
		t.Fatal("expected error for taken username")
	}
	if !strings.Contains(err.Error(), "already taken") {
		t.Fatalf("expected 'already taken' error, got: %v", err)
	}
}
