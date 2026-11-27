package core

import (
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestWithScrubber(t *testing.T) {
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
			settings := &data.UserSettings{ShowScrubber: tt.current}
			if err := WithScrubber(tt.in)(settings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.ShowScrubber != tt.want {
				t.Fatalf("got %v, want %v", settings.ShowScrubber, tt.want)
			}
		})
	}
}

func TestGetSettings_ScrubberDefaultsOff(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ShowScrubber {
		t.Fatal("expected the scrubber off by default")
	}
}

func TestUpdateAllSettings_Scrubber(t *testing.T) {
	on := true
	tests := []struct {
		name     string
		existing *data.UserSettings
		in       *bool
		want     bool
	}{
		{"new row turned on", nil, &on, true},
		{"new row absent stays off", nil, nil, false},
		{"existing on kept when absent", &data.UserSettings{UserID: 1, ShowScrubber: true}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)
			if tt.existing == nil {
				settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
			} else {
				settingsRepo.EXPECT().GetByUserID(uint(1)).Return(tt.existing, nil)
			}
			var saved *data.UserSettings
			settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
				saved = s
				return nil
			})

			autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
			settings, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "grouped", scc,
				WithScrubber(tt.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.ShowScrubber != tt.want || saved == nil || saved.ShowScrubber != tt.want {
				t.Fatalf("got %v (saved %+v), want %v", settings.ShowScrubber, saved, tt.want)
			}
		})
	}
}
