package core

import (
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestHomepageService_GetHomepageData_EmptySections(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	// Return config with no sections
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{
		UserID: 1,
		HomepageConfig: data.HomepageConfig{
			ShowUpload: true,
			Sections:   []data.HomepageSection{},
		},
	}, nil)

	svc := NewHomepageService(
		settingsService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	response, err := svc.GetHomepageData(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(response.Sections) != 0 {
		t.Fatalf("expected 0 sections, got %d", len(response.Sections))
	}
	if !response.Config.ShowUpload {
		t.Fatal("expected ShowUpload to be true")
	}
}

func TestHomepageService_GetHomepageData_DisabledSectionsSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	// Return config with disabled sections
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{
		UserID: 1,
		HomepageConfig: data.HomepageConfig{
			ShowUpload: true,
			Sections: []data.HomepageSection{
				{ID: "s1", Type: "latest", Title: "Latest", Enabled: false, Limit: 10, Order: 0},
				{ID: "s2", Type: "latest", Title: "Also Latest", Enabled: false, Limit: 10, Order: 1},
			},
		},
	}, nil)

	svc := NewHomepageService(
		settingsService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	response, err := svc.GetHomepageData(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Both sections are disabled, so response should have 0 sections
	if len(response.Sections) != 0 {
		t.Fatalf("expected 0 sections (all disabled), got %d", len(response.Sections))
	}
}

func TestHomepageService_GetSectionData_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{
		UserID: 1,
		HomepageConfig: data.HomepageConfig{
			ShowUpload: true,
			Sections: []data.HomepageSection{
				{ID: "existing", Type: "latest", Title: "Existing", Enabled: true, Limit: 10},
			},
		},
	}, nil)

	svc := NewHomepageService(
		settingsService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	_, err := svc.GetSectionData(1, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent section")
	}
	if err.Error() != "section not found: nonexistent" {
		t.Fatalf("expected 'section not found' error, got: %v", err)
	}
}

func TestHomepageService_ContinueWatching_EmptyHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	watchHistoryRepo := mocks.NewMockWatchHistoryRepository(ctrl)

	svc := NewHomepageService(
		nil, nil, nil, nil,
		watchHistoryRepo,
		nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "continue",
		Type:    "continue_watching",
		Title:   "Continue Watching",
		Enabled: true,
		Limit:   5,
	}

	// Return empty watch history
	watchHistoryRepo.EXPECT().ListUserHistory(uint(1), 1, 15).Return([]data.UserSceneWatch{}, int64(0), nil)

	result, err := svc.fetchContinueWatchingSection(1, section)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result.Scenes) != 0 {
		t.Fatalf("expected 0 videos, got %d", len(result.Scenes))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
}

func TestHomepageService_ContinueWatching_FiltersCompletedVideos(t *testing.T) {
	ctrl := gomock.NewController(t)
	watchHistoryRepo := mocks.NewMockWatchHistoryRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)

	svc := NewHomepageService(
		nil, nil, nil, nil,
		watchHistoryRepo,
		nil,
		sceneRepo,
		nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "continue",
		Type:    "continue_watching",
		Title:   "Continue Watching",
		Enabled: true,
		Limit:   5,
	}

	// Return mix of completed and incomplete watches
	watchHistoryRepo.EXPECT().ListUserHistory(uint(1), 1, 15).Return([]data.UserSceneWatch{
		{SceneID: 1, Completed: true, LastPosition: 100},  // Completed - should be skipped
		{SceneID: 2, Completed: false, LastPosition: 50},  // Incomplete with position
		{SceneID: 3, Completed: false, LastPosition: 0},   // Incomplete but no position - skipped
		{SceneID: 4, Completed: false, LastPosition: 75},  // Incomplete with position
		{SceneID: 5, Completed: true, LastPosition: 200},  // Completed - should be skipped
	}, int64(5), nil)

	// Only videos 2 and 4 should be fetched
	sceneRepo.EXPECT().GetByIDs([]uint{2, 4}).Return([]data.Scene{
		{ID: 2, Title: "Video 2"},
		{ID: 4, Title: "Video 4"},
	}, nil)

	result, err := svc.fetchContinueWatchingSection(1, section)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result.Scenes) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(result.Scenes))
	}
	// Total should reflect the filtered count, not the unfiltered total
	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}
}

func TestHomepageService_ContinueWatching_RespectsLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	watchHistoryRepo := mocks.NewMockWatchHistoryRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)

	svc := NewHomepageService(
		nil, nil, nil, nil,
		watchHistoryRepo,
		nil,
		sceneRepo,
		nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "continue",
		Type:    "continue_watching",
		Title:   "Continue Watching",
		Enabled: true,
		Limit:   2, // Only want 2 videos
	}

	// Return more incomplete watches than the limit
	watchHistoryRepo.EXPECT().ListUserHistory(uint(1), 1, 6).Return([]data.UserSceneWatch{
		{SceneID: 1, Completed: false, LastPosition: 50},
		{SceneID: 2, Completed: false, LastPosition: 60},
		{SceneID: 3, Completed: false, LastPosition: 70},
		{SceneID: 4, Completed: false, LastPosition: 80},
	}, int64(4), nil)

	// Only first 2 should be fetched
	sceneRepo.EXPECT().GetByIDs([]uint{1, 2}).Return([]data.Scene{
		{ID: 1, Title: "Video 1"},
		{ID: 2, Title: "Video 2"},
	}, nil)

	result, err := svc.fetchContinueWatchingSection(1, section)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result.Scenes) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(result.Scenes))
	}
}

func TestHomepageService_FetchSectionData_UnknownType(t *testing.T) {
	svc := NewHomepageService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "unknown",
		Type:    "invalid_type",
		Title:   "Unknown",
		Enabled: true,
		Limit:   10,
	}

	_, err := svc.fetchSectionData(1, section)
	if err == nil {
		t.Fatal("expected error for unknown section type")
	}
	if err.Error() != "unknown section type: invalid_type" {
		t.Fatalf("expected 'unknown section type' error, got: %v", err)
	}
}

func TestHomepageService_ActorSection_MissingUUID(t *testing.T) {
	svc := NewHomepageService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "actor",
		Type:    "actor",
		Title:   "Actor Section",
		Enabled: true,
		Limit:   10,
		Config:  map[string]interface{}{}, // No actor_uuid
	}

	_, err := svc.fetchActorSection(1, section)
	if err == nil {
		t.Fatal("expected error for missing actor_uuid")
	}
	if err.Error() != "actor_uuid not found in config" {
		t.Fatalf("expected 'actor_uuid not found in config' error, got: %v", err)
	}
}

func TestHomepageService_StudioSection_MissingUUID(t *testing.T) {
	svc := NewHomepageService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "studio",
		Type:    "studio",
		Title:   "Studio Section",
		Enabled: true,
		Limit:   10,
		Config:  map[string]interface{}{}, // No studio_uuid
	}

	_, err := svc.fetchStudioSection(1, section)
	if err == nil {
		t.Fatal("expected error for missing studio_uuid")
	}
	if err.Error() != "studio_uuid not found in config" {
		t.Fatalf("expected 'studio_uuid not found in config' error, got: %v", err)
	}
}

func TestHomepageService_TagSection_MissingID(t *testing.T) {
	svc := NewHomepageService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "tag",
		Type:    "tag",
		Title:   "Tag Section",
		Enabled: true,
		Limit:   10,
		Config:  map[string]interface{}{}, // No tag_id
	}

	_, err := svc.fetchTagSection(1, section)
	if err == nil {
		t.Fatal("expected error for missing tag_id")
	}
	if err.Error() != "tag_id not found in config" {
		t.Fatalf("expected 'tag_id not found in config' error, got: %v", err)
	}
}

func TestHomepageService_SavedSearchSection_MissingUUID(t *testing.T) {
	svc := NewHomepageService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zap.NewNop(),
	)

	section := data.HomepageSection{
		ID:      "saved",
		Type:    "saved_search",
		Title:   "Saved Search",
		Enabled: true,
		Limit:   10,
		Config:  map[string]interface{}{}, // No saved_search_uuid
	}

	_, err := svc.fetchSavedSearchSection(1, section)
	if err == nil {
		t.Fatal("expected error for missing saved_search_uuid")
	}
	if err.Error() != "saved_search_uuid not found in config" {
		t.Fatalf("expected 'saved_search_uuid not found in config' error, got: %v", err)
	}
}

func TestHomepageService_SettingsService_GetHomepageConfig_Defaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)

	// When settings don't exist, the repository returns gorm.ErrRecordNotFound
	// and GetHomepageConfig should return default config
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)

	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())
	config, err := settingsService.GetHomepageConfig(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if config == nil {
		t.Fatal("expected config, got nil")
	}
	// Should return default config
	if !config.ShowUpload {
		t.Fatal("expected ShowUpload to be true in default config")
	}
	if len(config.Sections) != 1 {
		t.Fatalf("expected 1 default section, got %d", len(config.Sections))
	}
	if config.Sections[0].Type != "latest" {
		t.Fatalf("expected default section type 'latest', got '%s'", config.Sections[0].Type)
	}
}

func TestHomepageService_ValidateHomepageConfig_MaxSections(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	// Create config with 21 sections (exceeds max of 20)
	sections := make([]data.HomepageSection, 21)
	for i := 0; i < 21; i++ {
		sections[i] = data.HomepageSection{
			ID:      "s" + string(rune('a'+i)),
			Type:    "latest",
			Title:   "Section",
			Enabled: true,
			Limit:   10,
			Order:   i,
		}
	}

	config := data.HomepageConfig{
		ShowUpload: true,
		Sections:   sections,
	}

	_, err := settingsService.UpdateHomepageConfig(1, config)
	if err == nil {
		t.Fatal("expected error for too many sections")
	}
	if err.Error() != "maximum of 20 sections allowed" {
		t.Fatalf("expected 'maximum of 20 sections allowed' error, got: %v", err)
	}
}

func TestHomepageService_ValidateHomepageConfig_DuplicateIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	config := data.HomepageConfig{
		ShowUpload: true,
		Sections: []data.HomepageSection{
			{ID: "same-id", Type: "latest", Title: "First", Enabled: true, Limit: 10, Order: 0},
			{ID: "same-id", Type: "latest", Title: "Second", Enabled: true, Limit: 10, Order: 1},
		},
	}

	_, err := settingsService.UpdateHomepageConfig(1, config)
	if err == nil {
		t.Fatal("expected error for duplicate section IDs")
	}
}

func TestHomepageService_ValidateHomepageConfig_InvalidSectionType(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	config := data.HomepageConfig{
		ShowUpload: true,
		Sections: []data.HomepageSection{
			{ID: "s1", Type: "invalid_type", Title: "Invalid", Enabled: true, Limit: 10, Order: 0},
		},
	}

	_, err := settingsService.UpdateHomepageConfig(1, config)
	if err == nil {
		t.Fatal("expected error for invalid section type")
	}
}

func TestHomepageService_ValidateHomepageConfig_LimitBoundaries(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, userRepo, zap.NewNop())

	tests := []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{"limit 0 invalid", 0, true},
		{"limit 1 valid", 1, false},
		{"limit 50 valid", 50, false},
		{"limit 51 invalid", 51, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := data.HomepageConfig{
				ShowUpload: true,
				Sections: []data.HomepageSection{
					{ID: "s1", Type: "latest", Title: "Test", Enabled: true, Limit: tt.limit, Order: 0},
				},
			}

			if !tt.wantErr {
				settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{UserID: 1}, nil)
				settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)
			}

			_, err := settingsService.UpdateHomepageConfig(1, config)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
