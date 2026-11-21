package core

import (
	"fmt"
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

func TestWithPlayerGestures(t *testing.T) {
	on, off := true, false
	tests := []struct {
		name        string
		current     data.UserSettings
		enabled     *bool
		step        int
		wantEnabled bool
		wantStep    int
		wantErr     bool
	}{
		{"turn off", data.UserSettings{PlayerGestures: true, GestureSeekStep: 10}, &off, 10, false, 10, false},
		{"turn on", data.UserSettings{PlayerGestures: false, GestureSeekStep: 10}, &on, 15, true, 15, false},
		{"nil keeps enabled", data.UserSettings{PlayerGestures: true, GestureSeekStep: 30}, nil, 5, true, 5, false},
		{"nil keeps disabled", data.UserSettings{PlayerGestures: false, GestureSeekStep: 30}, nil, 30, false, 30, false},
		{"zero step keeps current", data.UserSettings{PlayerGestures: true, GestureSeekStep: 15}, &on, 0, true, 15, false},
		{"zero step on new row defaults", data.UserSettings{PlayerGestures: true}, nil, 0, true, DefaultGestureSeekStep, false},
		{"step 5", data.UserSettings{GestureSeekStep: 10}, &on, 5, true, 5, false},
		{"step 30", data.UserSettings{GestureSeekStep: 10}, &on, 30, true, 30, false},
		{"step 20 rejected", data.UserSettings{PlayerGestures: true, GestureSeekStep: 10}, &off, 20, true, 10, true},
		{"negative step rejected", data.UserSettings{PlayerGestures: true, GestureSeekStep: 10}, &off, -10, true, 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := tt.current
			err := WithPlayerGestures(tt.enabled, tt.step)(&settings)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// on error the settings must be left untouched
			if settings.PlayerGestures != tt.wantEnabled || settings.GestureSeekStep != tt.wantStep {
				t.Fatalf("got enabled=%v step=%d; want enabled=%v step=%d",
					settings.PlayerGestures, settings.GestureSeekStep, tt.wantEnabled, tt.wantStep)
			}
		})
	}
}

func TestGetSettings_PlayerGesturesDefaults(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, fmt.Errorf("record not found"))

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !settings.PlayerGestures {
		t.Fatal("expected gestures enabled by default")
	}
	if settings.GestureSeekStep != DefaultGestureSeekStep {
		t.Fatalf("expected seek step %d, got %d", DefaultGestureSeekStep, settings.GestureSeekStep)
	}
}

// A user without a settings row who saves settings from a client that does not send the gesture
// fields gets the defaults, and an explicit false is kept.
func TestUpdateAllSettings_PlayerGesturesNewRow(t *testing.T) {
	off := false
	tests := []struct {
		name        string
		enabled     *bool
		wantEnabled bool
	}{
		{"omitted", nil, true},
		{"disabled", &off, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)
			settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, fmt.Errorf("record not found"))
			var saved *data.UserSettings
			settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
				saved = s
				return nil
			})

			autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
			_, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "", scc,
				WithPlayerGestures(tt.enabled, 0))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if saved == nil {
				t.Fatal("settings not saved")
			}
			if saved.PlayerGestures != tt.wantEnabled {
				t.Fatalf("expected gestures %v, got %v", tt.wantEnabled, saved.PlayerGestures)
			}
			if saved.GestureSeekStep != DefaultGestureSeekStep {
				t.Fatalf("expected seek step %d, got %d", DefaultGestureSeekStep, saved.GestureSeekStep)
			}
		})
	}
}
