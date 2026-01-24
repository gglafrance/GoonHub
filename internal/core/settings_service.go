package core

import (
	"fmt"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var allowedTagSorts = map[string]bool{
	"az":    true,
	"za":    true,
	"most":  true,
	"least": true,
}

var allowedSortOrders = map[string]bool{
	"created_at_desc": true,
	"created_at_asc":  true,
	"title_asc":       true,
	"title_desc":      true,
	"duration_asc":    true,
	"duration_desc":   true,
	"size_asc":        true,
	"size_desc":       true,
}

type SettingsService struct {
	settingsRepo data.UserSettingsRepository
	userRepo     data.UserRepository
	logger       *zap.Logger
}

func NewSettingsService(settingsRepo data.UserSettingsRepository, userRepo data.UserRepository, logger *zap.Logger) *SettingsService {
	return &SettingsService{
		settingsRepo: settingsRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

func (s *SettingsService) GetSettings(userID uint) (*data.UserSettings, error) {
	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		// Return defaults if no row exists
		return &data.UserSettings{
			UserID:           userID,
			Autoplay:         false,
			DefaultVolume:    100,
			Loop:             false,
			VideosPerPage:    20,
			DefaultSortOrder: "created_at_desc",
			DefaultTagSort:   "az",
		}, nil
	}
	return settings, nil
}

func (s *SettingsService) UpdatePlayerSettings(userID uint, autoplay bool, volume int, loop bool) (*data.UserSettings, error) {
	if volume < 0 || volume > 100 {
		return nil, fmt.Errorf("volume must be between 0 and 100")
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{UserID: userID}
	}

	settings.Autoplay = autoplay
	settings.DefaultVolume = volume
	settings.Loop = loop

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update player settings: %w", err)
	}

	return settings, nil
}

func (s *SettingsService) UpdateAppSettings(userID uint, videosPerPage int, sortOrder string) (*data.UserSettings, error) {
	if videosPerPage < 1 || videosPerPage > 100 {
		return nil, fmt.Errorf("videos per page must be between 1 and 100")
	}

	if !allowedSortOrders[sortOrder] {
		return nil, fmt.Errorf("invalid sort order: %s", sortOrder)
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{UserID: userID}
	}

	settings.VideosPerPage = videosPerPage
	settings.DefaultSortOrder = sortOrder

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update app settings: %w", err)
	}

	return settings, nil
}

func (s *SettingsService) UpdateTagSettings(userID uint, defaultTagSort string) (*data.UserSettings, error) {
	if !allowedTagSorts[defaultTagSort] {
		return nil, fmt.Errorf("invalid tag sort: %s", defaultTagSort)
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{UserID: userID}
	}

	settings.DefaultTagSort = defaultTagSort

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update tag settings: %w", err)
	}

	return settings, nil
}

func (s *SettingsService) ChangePassword(userID uint, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(userID, string(hashedPassword)); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	s.logger.Info("Password changed", zap.Uint("user_id", userID))
	return nil
}

func (s *SettingsService) ChangeUsername(userID uint, newUsername string) error {
	if len(newUsername) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}

	exists, err := s.userRepo.Exists(newUsername)
	if err != nil {
		return fmt.Errorf("failed to check username availability: %w", err)
	}
	if exists {
		return fmt.Errorf("username already taken")
	}

	if err := s.userRepo.UpdateUsername(userID, newUsername); err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}

	s.logger.Info("Username changed", zap.Uint("user_id", userID), zap.String("new_username", newUsername))
	return nil
}
