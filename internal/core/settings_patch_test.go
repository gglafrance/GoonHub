package core

import (
	"reflect"
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// storedSettings is a settings row where every setting differs from its default, so that a
// setting reset by an update shows up.
func storedSettings() *data.UserSettings {
	presetID := "p1"
	return &data.UserSettings{
		ID:                       7,
		UserID:                   1,
		Autoplay:                 true,
		DefaultVolume:            40,
		Loop:                     true,
		AbLoopControls:           true,
		MarkerLoop:               true,
		VideosPerPage:            48,
		DefaultSortOrder:         "title_asc",
		DefaultTagSort:           "most",
		MarkerThumbnailCycling:   false,
		HomepageConfig:           data.DefaultHomepageConfig(),
		ParsingRules:             data.ParsingRulesSettings{Presets: []data.ParsingPreset{{ID: "p1", Name: "Mine", Rules: []data.ParsingRule{}}}, ActivePresetID: &presetID},
		SortPreferences:          data.SortPreferences{Actors: "name_desc", Studios: "scene_count_desc", Markers: "recent", ActorScenes: "title_asc", StudioScenes: "random"},
		PlaylistAutoAdvance:      "manual",
		PlaylistCountdownSeconds: 12,
		ShowPageSizeSelector:     true,
		AdvancedMarkerFilters:    true,
		DefaultMarkerView:        "all",
		PaginationPosition:       "both",
		DateFormat:               "YYYY-MM-DD",
		PlayerGestures:           false,
		GestureSeekStep:          30,
		ShowReplayHeatmap:        false,
		RecordReplayHeatmap:      false,
		SceneCardConfig:          data.DefaultSceneCardConfig(),
		GridDensity:              "compact",
		InfiniteScroll:           true,
		SceneListView:            data.SceneListView{Pages: map[string]string{"search": "table"}, Columns: []string{"title", "duration"}},
		ShowScrubber:             true,
		ScrubberMode:             "centered",
		ContentWidth:             "wide",
	}
}

func TestUpdateSettings_AbsentFieldsUntouched(t *testing.T) {
	off, on := false, true
	free := ScrubberModeFree
	tests := []struct {
		name   string
		opt    SettingsOption
		change func(s *data.UserSettings)
	}{
		{"scrubber", WithScrubber(&off), func(s *data.UserSettings) { s.ShowScrubber = false }},
		{"scrubber mode", WithScrubberMode(&free), func(s *data.UserSettings) { s.ScrubberMode = "free" }},
		{"content width", WithContentWidth(ptr(ContentWidthFull)), func(s *data.UserSettings) { s.ContentWidth = "full" }},
		{"infinite scroll", WithInfiniteScroll(&off), func(s *data.UserSettings) { s.InfiniteScroll = false }},
		{"grid density", WithGridDensity("large"), func(s *data.UserSettings) { s.GridDensity = "large" }},
		{"volume to zero", WithDefaultVolume(0), func(s *data.UserSettings) { s.DefaultVolume = 0 }},
		{"autoplay", WithAutoplay(false), func(s *data.UserSettings) { s.Autoplay = false }},
		{"marker cycling", WithMarkerThumbnailCycling(true), func(s *data.UserSettings) { s.MarkerThumbnailCycling = true }},
		{"sort order", WithDefaultSortOrder("random"), func(s *data.UserSettings) { s.DefaultSortOrder = "random" }},
		{"tag sort", WithDefaultTagSort("za"), func(s *data.UserSettings) { s.DefaultTagSort = "za" }},
		{"videos per page", WithVideosPerPage(10), func(s *data.UserSettings) { s.VideosPerPage = 10 }},
		{"countdown", WithPlaylistCountdownSeconds(3), func(s *data.UserSettings) { s.PlaylistCountdownSeconds = 3 }},
		{"gesture step", WithGestureSeekStep(5), func(s *data.UserSettings) { s.GestureSeekStep = 5 }},
		{"gestures only", WithPlayerGestures(&on, 0), func(s *data.UserSettings) { s.PlayerGestures = true }},
		{"heatmap show only", WithReplayHeatmap(&on, nil), func(s *data.UserSettings) { s.ShowReplayHeatmap = true }},
		{"date format", WithDateFormat("DD.MM.YYYY"), func(s *data.UserSettings) { s.DateFormat = "DD.MM.YYYY" }},
		{"sort preferences", WithSortPreferences(data.DefaultSortPreferences()), func(s *data.UserSettings) { s.SortPreferences = data.DefaultSortPreferences() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)
			settingsRepo.EXPECT().GetByUserID(uint(1)).Return(storedSettings(), nil)
			var saved *data.UserSettings
			settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
				saved = s
				return nil
			})

			got, err := svc.UpdateSettings(1, tt.opt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := storedSettings()
			tt.change(want)
			if !reflect.DeepEqual(saved, want) {
				t.Fatalf("saved settings differ:\n got %+v\nwant %+v", saved, want)
			}
			if got != saved {
				t.Fatal("expected the saved settings to be returned")
			}
		})
	}
}

func TestUpdateSettings_SeveralFields(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(storedSettings(), nil)
	settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)

	got, err := svc.UpdateSettings(1, WithLoop(false), WithPaginationPosition("top"), svc.WithSceneCardConfig(data.SceneCardConfig{
		Badges:      data.BadgeZones{TopLeft: data.BadgeZone{Items: []string{"liked"}, Direction: "vertical"}, TopRight: data.BadgeZone{Items: []string{}, Direction: "vertical"}, BottomLeft: data.BadgeZone{Items: []string{}, Direction: "vertical"}, BottomRight: data.BadgeZone{Items: []string{}, Direction: "horizontal"}},
		ContentRows: []data.ContentRow{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Loop || got.PaginationPosition != "top" || got.SceneCardConfig.Badges.TopLeft.Items[0] != "liked" {
		t.Fatalf("fields not applied: %+v", got)
	}
	if !got.Autoplay || got.DefaultVolume != 40 || got.GridDensity != "compact" || !got.ShowScrubber {
		t.Fatalf("absent fields changed: %+v", got)
	}
}

func TestUpdateSettings_InvalidValueLeavesSettingsUntouched(t *testing.T) {
	tests := []struct {
		name string
		opt  SettingsOption
	}{
		{"volume above 100", WithDefaultVolume(101)},
		{"negative volume", WithDefaultVolume(-1)},
		{"videos per page 0", WithVideosPerPage(0)},
		{"sort order", WithDefaultSortOrder("nope")},
		{"empty sort order", WithDefaultSortOrder("")},
		{"tag sort", WithDefaultTagSort("nope")},
		{"actors sort", WithSortPreferences(data.SortPreferences{Actors: "x", Studios: "name_asc", Markers: "label_asc"})},
		{"studios sort", WithSortPreferences(data.SortPreferences{Actors: "name_asc", Studios: "x", Markers: "label_asc"})},
		{"markers sort", WithSortPreferences(data.SortPreferences{Actors: "name_asc", Studios: "name_asc", Markers: "x"})},
		{"actor scenes sort", WithSortPreferences(data.SortPreferences{Actors: "name_asc", Studios: "name_asc", Markers: "label_asc", ActorScenes: "x"})},
		{"marker view", WithDefaultMarkerView("list")},
		{"auto advance", WithPlaylistAutoAdvance("later")},
		{"countdown too short", WithPlaylistCountdownSeconds(2)},
		{"countdown too long", WithPlaylistCountdownSeconds(16)},
		{"gesture step", WithGestureSeekStep(7)},
		{"gesture step zero", WithGestureSeekStep(0)},
		{"date format", WithDateFormat("YY")},
		{"grid density", WithGridDensity("huge")},
		{"scrubber mode", WithScrubberMode(ptr("stash"))},
		{"empty scrubber mode", WithScrubberMode(ptr(""))},
		{"content width", WithContentWidth(ptr("huge"))},
		{"pagination", WithPaginationPosition("left")},
		{"scene list view", WithSceneListView(&data.SceneListView{Pages: map[string]string{"search": "mosaic"}})},
		{"card config", (&SettingsService{}).WithSceneCardConfig(data.SceneCardConfig{})},
		{"homepage", (&SettingsService{}).WithHomepageConfig(data.HomepageConfig{Tabs: []data.HomepageTab{{ID: "t", Name: " "}}})},
		{"parsing rules", (&SettingsService{}).WithParsingRules(data.ParsingRulesSettings{Presets: []data.ParsingPreset{{ID: "", Name: "x"}}})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No repository call is expected: validation runs before the settings are read
			svc, _, _ := newTestSettingsService(t)
			if _, err := svc.UpdateSettings(1, WithAutoplay(true), tt.opt); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestUpdateSettings_NoOptions(t *testing.T) {
	svc, _, _ := newTestSettingsService(t)
	if _, err := svc.UpdateSettings(1); err == nil {
		t.Fatal("expected an error without settings to update")
	}
}

func TestUpdateSettings_NewRowStartsFromDefaults(t *testing.T) {
	on := true
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)

	got, err := svc.UpdateSettings(1, WithScrubber(&on))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := newSettingsRow(1)
	want.ShowScrubber = true
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got.DefaultVolume != 100 || got.VideosPerPage != 20 || got.DefaultSortOrder != "created_at_desc" || got.PlaylistCountdownSeconds != 5 {
		t.Fatalf("defaults not applied: %+v", got)
	}
}

func TestUpdateSettings_NewRowWithZeroedDefaultIsWrittenTwice(t *testing.T) {
	tests := []struct {
		name        string
		opt         SettingsOption
		wantUpserts int
	}{
		// GORM leaves these zero values out of the insert (the column default would win)
		{"marker cycling off", WithMarkerThumbnailCycling(false), 2},
		{"volume zero", WithDefaultVolume(0), 2},
		{"autoplay on", WithAutoplay(true), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, settingsRepo, _ := newTestSettingsService(t)
			settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
			settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil).Times(tt.wantUpserts)
			if _, err := svc.UpdateSettings(1, tt.opt); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPlaylistOptions_Defaults(t *testing.T) {
	settings := &data.UserSettings{}
	if err := WithPlaylistAutoAdvance("")(settings); err != nil || settings.PlaylistAutoAdvance != "countdown" {
		t.Fatalf("empty auto advance: %v %q", err, settings.PlaylistAutoAdvance)
	}
	if err := WithPlaylistCountdownSeconds(0)(settings); err != nil || settings.PlaylistCountdownSeconds != 5 {
		t.Fatalf("zero countdown: %v %d", err, settings.PlaylistCountdownSeconds)
	}
	if err := WithDefaultMarkerView("")(settings); err != nil || settings.DefaultMarkerView != "grouped" {
		t.Fatalf("empty marker view: %v %q", err, settings.DefaultMarkerView)
	}
}
