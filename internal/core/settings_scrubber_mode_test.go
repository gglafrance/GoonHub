package core

import (
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestWithScrubberMode(t *testing.T) {
	tests := []struct {
		name    string
		current string
		in      *string
		want    string
		wantErr bool
	}{
		{"nil keeps free", "free", nil, "free", false},
		{"nil keeps centered", "centered", nil, "centered", false},
		{"to centered", "free", ptr("centered"), "centered", false},
		{"to free", "centered", ptr("free"), "free", false},
		{"unknown mode", "centered", ptr("stash"), "centered", true},
		{"empty", "centered", ptr(""), "centered", true},
		{"case sensitive", "free", ptr("Centered"), "free", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &data.UserSettings{ScrubberMode: tt.current}
			err := WithScrubberMode(tt.in)(settings)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want error %v", err, tt.wantErr)
			}
			if settings.ScrubberMode != tt.want {
				t.Fatalf("got %q, want %q", settings.ScrubberMode, tt.want)
			}
		})
	}
}

func TestGetSettings_ScrubberModeDefaultsFree(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ScrubberMode != ScrubberModeFree {
		t.Fatalf("expected the free mode by default, got %q", settings.ScrubberMode)
	}
}

// A first update of another setting creates the row with the free mode (the CHECK constraint
// rejects an empty mode).
func TestUpdateSettings_NewRowScrubberModeFree(t *testing.T) {
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
	if saved == nil || saved.ScrubberMode != ScrubberModeFree || !saved.ShowScrubber {
		t.Fatalf("unexpected new row: %+v", saved)
	}
}
