package core

import (
	"fmt"
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

func TestWithGridDensity(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "comfortable", false},
		{"compact", "compact", false},
		{"comfortable", "comfortable", false},
		{"large", "large", false},
		{"huge", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			settings := &data.UserSettings{}
			err := WithGridDensity(tt.in)(settings)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || settings.GridDensity != tt.want {
				t.Fatalf("got %q, %v; want %q", settings.GridDensity, err, tt.want)
			}
		})
	}
}

func TestWithInfiniteScroll(t *testing.T) {
	on, off := true, false
	tests := []struct {
		name    string
		current bool
		in      *bool
		want    bool
	}{
		{"nil keeps off", false, nil, false},
		{"nil keeps on", true, nil, true},
		{"turn on", false, &on, true},
		{"turn off", true, &off, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &data.UserSettings{InfiniteScroll: tt.current}
			if err := WithInfiniteScroll(tt.in)(settings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.InfiniteScroll != tt.want {
				t.Fatalf("got %v, want %v", settings.InfiniteScroll, tt.want)
			}
		})
	}
}

func TestGetSettings_DisplayDefaults(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, fmt.Errorf("record not found"))

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if settings.GridDensity != "comfortable" || settings.InfiniteScroll {
		t.Fatalf("got density %q, infinite scroll %v", settings.GridDensity, settings.InfiniteScroll)
	}
}

func TestUpdateAllSettings_DisplaySettings(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{
		UserID:          1,
		HomepageConfig:  data.DefaultHomepageConfig(),
		ParsingRules:    data.DefaultParsingRulesSettings(),
		SortPreferences: data.DefaultSortPreferences(),
		GridDensity:     "comfortable",
	}, nil)
	settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)

	autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
	on := true
	settings, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "grouped", scc,
		WithGridDensity("compact"), WithInfiniteScroll(&on))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if settings.GridDensity != "compact" || !settings.InfiniteScroll {
		t.Fatalf("got density %q, infinite scroll %v", settings.GridDensity, settings.InfiniteScroll)
	}
}

func TestUpdateAllSettings_InvalidGridDensity(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{UserID: 1}, nil)

	autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
	if _, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "grouped", scc, WithGridDensity("tiny")); err == nil {
		t.Fatal("expected an error for an invalid grid density")
	}
}
