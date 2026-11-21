package core

import (
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestWithReplayHeatmap(t *testing.T) {
	on, off := true, false
	tests := []struct {
		name         string
		show, record *bool
		wantShow     bool
		wantRecord   bool
	}{
		{"absent keeps current values", nil, nil, true, false},
		{"turn display off", &off, nil, false, false},
		{"turn recording on", nil, &on, true, true},
		{"both", &off, &on, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &data.UserSettings{ShowReplayHeatmap: true, RecordReplayHeatmap: false}
			if err := WithReplayHeatmap(tt.show, tt.record)(settings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.ShowReplayHeatmap != tt.wantShow || settings.RecordReplayHeatmap != tt.wantRecord {
				t.Fatalf("got show=%v record=%v", settings.ShowReplayHeatmap, settings.RecordReplayHeatmap)
			}
		})
	}
}

func TestGetSettings_ReplayHeatmapDefaultsOn(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !settings.ShowReplayHeatmap || !settings.RecordReplayHeatmap {
		t.Fatalf("expected the replay heatmap on by default, got %+v", settings)
	}
}

func TestUpdateAllSettings_ReplayHeatmapOnNewRow(t *testing.T) {
	off := false
	tests := []struct {
		name        string
		show        *bool
		wantUpserts int
		wantShow    bool
	}{
		// GORM leaves false out of the insert of a column with a default: the row is
		// written a second time so that the setting is really off
		{"turned off", &off, 2, false},
		{"absent stays on", nil, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)
			settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
			settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil).Times(tt.wantUpserts)

			autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
			settings, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "", scc,
				WithReplayHeatmap(tt.show, nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.ShowReplayHeatmap != tt.wantShow || !settings.RecordReplayHeatmap {
				t.Fatalf("got show=%v record=%v", settings.ShowReplayHeatmap, settings.RecordReplayHeatmap)
			}
		})
	}
}
