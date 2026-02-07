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
	if settings.DefaultTagSort != "az" {
		t.Fatalf("expected default tag sort 'az', got %s", settings.DefaultTagSort)
	}
	if settings.MarkerThumbnailCycling != true {
		t.Fatal("expected default marker thumbnail cycling true")
	}
}

func validAllSettingsArgs() (bool, int, bool, int, string, string, bool, data.HomepageConfig, data.ParsingRulesSettings, data.SortPreferences, string, int, bool) {
	return false, 50, false, 20, "created_at_desc", "az", true,
		data.DefaultHomepageConfig(),
		data.DefaultParsingRulesSettings(),
		data.DefaultSortPreferences(),
		"countdown", 5, false
}

func TestUpdateAllSettings_Success(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)

	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{
		UserID:          1,
		HomepageConfig:  data.DefaultHomepageConfig(),
		ParsingRules:    data.DefaultParsingRulesSettings(),
		SortPreferences: data.DefaultSortPreferences(),
	}, nil)
	settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)

	autoplay, volume, loop, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss := validAllSettingsArgs()
	settings, err := svc.UpdateAllSettings(1, autoplay, volume, loop, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if settings.DefaultVolume != 50 {
		t.Fatalf("expected volume 50, got %d", settings.DefaultVolume)
	}
}

func TestUpdateAllSettings_InvalidFields(t *testing.T) {
	tests := []struct {
		name       string
		volume     int
		vpp        int
		sort       string
		tagSort    string
		actorSort  string
		wantSubstr string
	}{
		{"volume -1", -1, 20, "created_at_desc", "az", "name_asc", "volume must be between"},
		{"volume 101", 101, 20, "created_at_desc", "az", "name_asc", "volume must be between"},
		{"vpp 0", 50, 0, "created_at_desc", "az", "name_asc", "videos per page must be at least 1"},
		{"bad sort order", 50, 20, "nonsense", "az", "name_asc", "invalid sort order"},
		{"bad tag sort", 50, 20, "created_at_desc", "bad", "name_asc", "invalid tag sort"},
		{"bad actors sort", 50, 20, "created_at_desc", "az", "bad", "invalid actors sort"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestSettingsService(t)

			sp := data.DefaultSortPreferences()
			sp.Actors = tt.actorSort

			_, err := svc.UpdateAllSettings(1, false, tt.volume, false, tt.vpp, tt.sort, tt.tagSort, true,
				data.DefaultHomepageConfig(), data.DefaultParsingRulesSettings(), sp, "countdown", 5, false)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantSubstr, err)
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
