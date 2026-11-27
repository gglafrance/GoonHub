package core

import (
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestWithScrubberWheel(t *testing.T) {
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
			settings := &data.UserSettings{ScrubberWheel: tt.current}
			if err := WithScrubberWheel(tt.in)(settings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.ScrubberWheel != tt.want {
				t.Fatalf("got %v, want %v", settings.ScrubberWheel, tt.want)
			}
		})
	}
}

func TestGetSettings_ScrubberWheelDefaultsOff(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ScrubberWheel {
		t.Fatal("expected the scrubber wheel off by default")
	}
}

// A first update of another setting creates the row with the wheel off.
func TestUpdateSettings_NewRowScrubberWheelOff(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	var saved *data.UserSettings
	settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
		saved = s
		return nil
	})
	on := true
	if _, err := svc.UpdateSettings(1, WithScrubber(&on)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved == nil || saved.ScrubberWheel || !saved.ShowScrubber {
		t.Fatalf("unexpected new row: %+v", saved)
	}
}
