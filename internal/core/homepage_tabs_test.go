package core

import (
	"strings"
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func homepageTestSection(id, sectionType string) data.HomepageSection {
	return data.HomepageSection{ID: id, Type: sectionType, Title: id, Enabled: true, Limit: 10, Config: map[string]interface{}{}}
}

func TestHomepageConfigNormalize_MigratesLegacySections(t *testing.T) {
	cfg := data.HomepageConfig{Sections: []data.HomepageSection{homepageTestSection("a", "latest")}}
	cfg.Normalize()
	if len(cfg.Tabs) != 1 || cfg.Tabs[0].ID != "home" || len(cfg.Tabs[0].Sections) != 1 {
		t.Fatalf("legacy sections not migrated: %+v", cfg.Tabs)
	}
	if len(cfg.Sections) != 0 || cfg.DefaultTabID != "home" || cfg.OpenOn != "last" {
		t.Fatalf("unexpected normalized config: %+v", cfg)
	}
	if cfg.FindSection("a") == nil {
		t.Fatal("FindSection must look into tabs")
	}
}

func TestValidateHomepageConfig_Tabs(t *testing.T) {
	svc := NewSettingsService(nil, nil, zap.NewNop())
	tests := []struct {
		name string
		cfg  data.HomepageConfig
		want string
	}{
		{"duplicate tab id", data.HomepageConfig{Tabs: []data.HomepageTab{{ID: "x", Name: "A"}, {ID: "x", Name: "B"}}}, "duplicate id"},
		{"tab without name", data.HomepageConfig{Tabs: []data.HomepageTab{{ID: "x"}}}, "name is required"},
		{"section ids unique across tabs", data.HomepageConfig{Tabs: []data.HomepageTab{
			{ID: "a", Name: "A", Sections: []data.HomepageSection{homepageTestSection("s1", "latest")}},
			{ID: "b", Name: "B", Sections: []data.HomepageSection{homepageTestSection("s1", "latest")}},
		}}, "duplicate id 's1'"},
		{"unknown type", data.HomepageConfig{Tabs: []data.HomepageTab{
			{ID: "a", Name: "A", Sections: []data.HomepageSection{homepageTestSection("s1", "nope")}},
		}}, "invalid type"},
		{"too many tabs", data.HomepageConfig{Tabs: func() []data.HomepageTab {
			tabs := make([]data.HomepageTab, maxHomepageTabs+1)
			for i := range tabs {
				tabs[i] = data.HomepageTab{ID: string(rune('a' + i)), Name: "T"}
			}
			return tabs
		}()}, "maximum of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := svc.validateHomepageConfig(&cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	valid := data.HomepageConfig{Tabs: []data.HomepageTab{
		{ID: "a", Name: "A", Sections: []data.HomepageSection{homepageTestSection("s1", "latest")}},
		{ID: "b", Name: "B", Sections: []data.HomepageSection{homepageTestSection("s2", "text_block")}},
	}, DefaultTabID: "b", OpenOn: "default"}
	if err := svc.validateHomepageConfig(&valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestGetHomepageData_SelectsTabAndFetchesWidgets(t *testing.T) {
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	settingsService := NewSettingsService(settingsRepo, nil, zap.NewNop())
	svc := NewHomepageService(settingsService, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())

	text := homepageTestSection("note", "text_block")
	cfg := data.HomepageConfig{
		Tabs: []data.HomepageTab{
			{ID: "home", Name: "Home", Sections: []data.HomepageSection{}},
			{ID: "notes", Name: "Notes", Sections: []data.HomepageSection{text}},
		},
		DefaultTabID: "home",
	}
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{HomepageConfig: cfg}, nil).Times(2)

	resp, err := svc.GetHomepageData(1, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if resp.ActiveTabID != "notes" || len(resp.Sections) != 1 || resp.Sections[0].Section.ID != "note" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	resp, err = svc.GetHomepageData(1, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if resp.ActiveTabID != "home" || len(resp.Sections) != 0 {
		t.Fatalf("unknown tab must fall back to the default tab: %+v", resp)
	}
}
