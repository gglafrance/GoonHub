package core

import (
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestWithContentWidth(t *testing.T) {
	tests := []struct {
		name    string
		current string
		in      *string
		want    string
		wantErr bool
	}{
		{"nil keeps default", "default", nil, "default", false},
		{"nil keeps full", "full", nil, "full", false},
		{"to wide", "default", ptr("wide"), "wide", false},
		{"to full", "wide", ptr("full"), "full", false},
		{"back to default", "full", ptr("default"), "default", false},
		{"unknown width", "wide", ptr("huge"), "wide", true},
		{"empty", "wide", ptr(""), "wide", true},
		{"case sensitive", "default", ptr("Full"), "default", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &data.UserSettings{ContentWidth: tt.current}
			err := WithContentWidth(tt.in)(settings)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want error %v", err, tt.wantErr)
			}
			if settings.ContentWidth != tt.want {
				t.Fatalf("got %q, want %q", settings.ContentWidth, tt.want)
			}
		})
	}
}

func TestGetSettings_ContentWidthDefault(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ContentWidth != ContentWidthDefault {
		t.Fatalf("expected the default width, got %q", settings.ContentWidth)
	}
}

// A first update of another setting creates the row with the default width (the CHECK
// constraint rejects an empty width).
func TestUpdateSettings_NewRowContentWidthDefault(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	var saved *data.UserSettings
	settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
		saved = s
		return nil
	})
	if _, err := svc.UpdateSettings(1, WithGridDensity("large")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved == nil || saved.ContentWidth != ContentWidthDefault || saved.GridDensity != "large" {
		t.Fatalf("unexpected new row: %+v", saved)
	}
}
