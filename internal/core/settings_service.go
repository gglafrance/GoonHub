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
	"view_count_desc": true,
	"view_count_asc":  true,
}

var allowedSectionTypes = map[string]bool{
	"latest":            true,
	"actor":             true,
	"studio":            true,
	"tag":               true,
	"saved_search":      true,
	"continue_watching": true,
	"most_viewed":       true,
	"liked":             true,
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
			UserID:                 userID,
			Autoplay:               false,
			DefaultVolume:          100,
			Loop:                   false,
			VideosPerPage:          20,
			DefaultSortOrder:       "created_at_desc",
			DefaultTagSort:         "az",
			MarkerThumbnailCycling: true,
			HomepageConfig:         data.DefaultHomepageConfig(),
		}, nil
	}
	return settings, nil
}

func (s *SettingsService) GetHomepageConfig(userID uint) (*data.HomepageConfig, error) {
	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		config := data.DefaultHomepageConfig()
		return &config, nil
	}
	return &settings.HomepageConfig, nil
}

func (s *SettingsService) UpdateHomepageConfig(userID uint, config data.HomepageConfig) (*data.UserSettings, error) {
	if err := s.validateHomepageConfig(&config); err != nil {
		return nil, err
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{UserID: userID, HomepageConfig: data.DefaultHomepageConfig()}
	}

	settings.HomepageConfig = config

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update homepage config: %w", err)
	}

	return settings, nil
}

func (s *SettingsService) validateHomepageConfig(config *data.HomepageConfig) error {
	if len(config.Sections) > 20 {
		return fmt.Errorf("maximum of 20 sections allowed")
	}

	seenIDs := make(map[string]bool)
	for i, section := range config.Sections {
		if section.ID == "" {
			return fmt.Errorf("section %d: id is required", i)
		}
		if seenIDs[section.ID] {
			return fmt.Errorf("section %d: duplicate id '%s'", i, section.ID)
		}
		seenIDs[section.ID] = true

		if !allowedSectionTypes[section.Type] {
			return fmt.Errorf("section %d: invalid type '%s'", i, section.Type)
		}

		if section.Title == "" {
			return fmt.Errorf("section %d: title is required", i)
		}
		if len(section.Title) > 100 {
			return fmt.Errorf("section %d: title must be 100 characters or less", i)
		}

		if section.Limit < 1 || section.Limit > 50 {
			return fmt.Errorf("section %d: limit must be between 1 and 50", i)
		}

		if section.Sort != "" && !allowedSortOrders[section.Sort] {
			return fmt.Errorf("section %d: invalid sort order '%s'", i, section.Sort)
		}

		// Validate type-specific config
		if err := s.validateSectionConfig(&section); err != nil {
			return fmt.Errorf("section %d: %w", i, err)
		}
	}

	return nil
}

func (s *SettingsService) validateSectionConfig(section *data.HomepageSection) error {
	switch section.Type {
	case "actor":
		if _, ok := section.Config["actor_uuid"]; !ok {
			return fmt.Errorf("actor section requires actor_uuid in config")
		}
	case "studio":
		if _, ok := section.Config["studio_uuid"]; !ok {
			return fmt.Errorf("studio section requires studio_uuid in config")
		}
	case "tag":
		if _, ok := section.Config["tag_id"]; !ok {
			return fmt.Errorf("tag section requires tag_id in config")
		}
	case "saved_search":
		if _, ok := section.Config["saved_search_uuid"]; !ok {
			return fmt.Errorf("saved_search section requires saved_search_uuid in config")
		}
	}
	return nil
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

func (s *SettingsService) UpdateAppSettings(userID uint, videosPerPage int, sortOrder string, markerThumbnailCycling bool) (*data.UserSettings, error) {
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
	settings.MarkerThumbnailCycling = markerThumbnailCycling

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
