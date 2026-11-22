package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newSettingsTestRouter(t *testing.T) (*gin.Engine, *mocks.MockUserSettingsRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	h := NewSettingsHandler(core.NewSettingsService(settingsRepo, userRepo, zap.NewNop()), 100)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1})
		c.Next()
	})
	r.PUT("/settings", h.UpdateAllSettings)
	r.PATCH("/settings", h.PatchSettings)
	return r, settingsRepo
}

func doSettingsRequest(r *gin.Engine, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// existingSettings is a stored row with non-default values.
func existingSettings() *data.UserSettings {
	return &data.UserSettings{
		ID:                       3,
		UserID:                   1,
		Autoplay:                 true,
		DefaultVolume:            35,
		VideosPerPage:            48,
		DefaultSortOrder:         "title_asc",
		DefaultTagSort:           "most",
		MarkerThumbnailCycling:   true,
		HomepageConfig:           data.DefaultHomepageConfig(),
		ParsingRules:             data.DefaultParsingRulesSettings(),
		SortPreferences:          data.DefaultSortPreferences(),
		PlaylistAutoAdvance:      "manual",
		PlaylistCountdownSeconds: 9,
		DefaultMarkerView:        "all",
		PaginationPosition:       "both",
		DateFormat:               "YYYY-MM-DD",
		PlayerGestures:           true,
		GestureSeekStep:          15,
		ShowReplayHeatmap:        true,
		RecordReplayHeatmap:      true,
		SceneCardConfig:          data.DefaultSceneCardConfig(),
		GridDensity:              "large",
		InfiniteScroll:           true,
		SceneListView:            data.DefaultSceneListView(),
		ShowScrubber:             true,
		ScrubberMode:             "free",
		ContentWidth:             "wide",
	}
}

func TestPatchSettings_UpdatesOnlyPresentFields(t *testing.T) {
	r, settingsRepo := newSettingsTestRouter(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(existingSettings(), nil)
	var saved *data.UserSettings
	settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
		saved = s
		return nil
	})

	w := doSettingsRequest(r, http.MethodPatch, `{"show_scrubber": false, "grid_density": "compact"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	want := existingSettings()
	want.ShowScrubber = false
	want.GridDensity = "compact"
	want.MaxItemsPerPage = 100 // set on the returned settings
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved settings differ:\n got %+v\nwant %+v", saved, want)
	}

	// The full settings are returned, like PUT
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["show_scrubber"] != false || got["grid_density"] != "compact" || got["default_volume"] != float64(35) ||
		got["date_format"] != "YYYY-MM-DD" || got["max_items_per_page"] != float64(100) || got["homepage_config"] == nil {
		t.Fatalf("unexpected response: %v", got)
	}
}

func TestPatchSettings_EveryField(t *testing.T) {
	tests := []struct {
		body  string
		check func(s *data.UserSettings) bool
	}{
		{`{"autoplay": false}`, func(s *data.UserSettings) bool { return !s.Autoplay }},
		{`{"default_volume": 0}`, func(s *data.UserSettings) bool { return s.DefaultVolume == 0 }},
		{`{"loop": true}`, func(s *data.UserSettings) bool { return s.Loop }},
		{`{"ab_loop_controls": true}`, func(s *data.UserSettings) bool { return s.AbLoopControls }},
		{`{"marker_loop": true}`, func(s *data.UserSettings) bool { return s.MarkerLoop }},
		{`{"videos_per_page": 30}`, func(s *data.UserSettings) bool { return s.VideosPerPage == 30 }},
		{`{"videos_per_page": 5000}`, func(s *data.UserSettings) bool { return s.VideosPerPage == 100 }},
		{`{"default_sort_order": "random"}`, func(s *data.UserSettings) bool { return s.DefaultSortOrder == "random" }},
		{`{"default_tag_sort": "za"}`, func(s *data.UserSettings) bool { return s.DefaultTagSort == "za" }},
		{`{"marker_thumbnail_cycling": false}`, func(s *data.UserSettings) bool { return !s.MarkerThumbnailCycling }},
		{`{"homepage_config": {"show_upload": true, "tabs": [{"id": "a", "name": "A", "sections": []}], "default_tab_id": "a", "open_on": "default"}}`,
			func(s *data.UserSettings) bool {
				return s.HomepageConfig.ShowUpload && len(s.HomepageConfig.Tabs) == 1 && s.HomepageConfig.OpenOn == "default"
			}},
		{`{"parsing_rules": {"presets": [{"id": "p", "name": "P", "rules": []}], "activePresetId": "p"}}`,
			func(s *data.UserSettings) bool {
				return len(s.ParsingRules.Presets) == 1 && *s.ParsingRules.ActivePresetID == "p"
			}},
		{`{"sort_preferences": {"actors": "name_desc", "studios": "name_asc", "markers": "recent", "actor_scenes": "random", "studio_scenes": ""}}`,
			func(s *data.UserSettings) bool {
				return s.SortPreferences.Actors == "name_desc" && s.SortPreferences.ActorScenes == "random"
			}},
		{`{"playlist_auto_advance": "instant"}`, func(s *data.UserSettings) bool { return s.PlaylistAutoAdvance == "instant" }},
		{`{"playlist_countdown_seconds": 15}`, func(s *data.UserSettings) bool { return s.PlaylistCountdownSeconds == 15 }},
		{`{"show_page_size_selector": true}`, func(s *data.UserSettings) bool { return s.ShowPageSizeSelector }},
		{`{"advanced_marker_filters": true}`, func(s *data.UserSettings) bool { return s.AdvancedMarkerFilters }},
		{`{"default_marker_view": "grouped"}`, func(s *data.UserSettings) bool { return s.DefaultMarkerView == "grouped" }},
		{`{"pagination_position": "top"}`, func(s *data.UserSettings) bool { return s.PaginationPosition == "top" }},
		{`{"date_format": ""}`, func(s *data.UserSettings) bool { return s.DateFormat == "auto" }},
		{`{"player_gestures": false}`, func(s *data.UserSettings) bool { return !s.PlayerGestures && s.GestureSeekStep == 15 }},
		{`{"gesture_seek_step": 30}`, func(s *data.UserSettings) bool { return s.GestureSeekStep == 30 && s.PlayerGestures }},
		{`{"show_replay_heatmap": false}`, func(s *data.UserSettings) bool { return !s.ShowReplayHeatmap && s.RecordReplayHeatmap }},
		{`{"record_replay_heatmap": false}`, func(s *data.UserSettings) bool { return s.ShowReplayHeatmap && !s.RecordReplayHeatmap }},
		{`{"scene_card_config": {"badges": {"top_left": {"items": ["liked"], "direction": "vertical"}, "top_right": {"items": [], "direction": ""}, "bottom_left": {"items": [], "direction": "vertical"}, "bottom_right": {"items": [], "direction": "horizontal"}}, "content_rows": [{"type": "full", "field": "tags", "mode": "short"}]}}`,
			func(s *data.UserSettings) bool {
				return s.SceneCardConfig.Badges.TopLeft.Items[0] == "liked" && s.SceneCardConfig.Badges.TopRight.Direction == "vertical" &&
					len(s.SceneCardConfig.ContentRows) == 1
			}},
		{`{"grid_density": "compact"}`, func(s *data.UserSettings) bool { return s.GridDensity == "compact" }},
		{`{"infinite_scroll": false}`, func(s *data.UserSettings) bool { return !s.InfiniteScroll }},
		{`{"scene_list_view": {"pages": {"actor": "table"}, "columns": ["title", "duration"]}}`,
			func(s *data.UserSettings) bool {
				return s.SceneListView.Pages["actor"] == "table" && len(s.SceneListView.Columns) == 2
			}},
		{`{"show_scrubber": false}`, func(s *data.UserSettings) bool { return !s.ShowScrubber }},
		{`{"scrubber_mode": "centered"}`, func(s *data.UserSettings) bool { return s.ScrubberMode == "centered" && s.ShowScrubber }},
		{`{"content_width": "full"}`, func(s *data.UserSettings) bool { return s.ContentWidth == "full" && s.GridDensity == "large" }},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			r, settingsRepo := newSettingsTestRouter(t)
			settingsRepo.EXPECT().GetByUserID(uint(1)).Return(existingSettings(), nil)
			var saved *data.UserSettings
			settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
				saved = s
				return nil
			})
			w := doSettingsRequest(r, http.MethodPatch, tt.body)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			if !tt.check(saved) {
				t.Fatalf("setting not applied: %+v", saved)
			}
		})
	}
}

func TestPatchSettings_Rejected(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty body", ``},
		{"empty object", `{}`},
		{"unknown field", `{"show_scrubber": true, "colour": "red"}`},
		{"null", `{"infinite_scroll": null}`},
		{"wrong type", `{"default_volume": "loud"}`},
		{"volume range", `{"default_volume": 101}`},
		{"videos per page", `{"videos_per_page": 0}`},
		{"sort order", `{"default_sort_order": ""}`},
		{"tag sort", `{"default_tag_sort": "zz"}`},
		{"countdown", `{"playlist_countdown_seconds": 20}`},
		{"auto advance", `{"playlist_auto_advance": "never"}`},
		{"marker view", `{"default_marker_view": "list"}`},
		{"pagination", `{"pagination_position": "side"}`},
		{"date format", `{"date_format": "YYYY"}`},
		{"gesture step", `{"gesture_seek_step": 0}`},
		{"grid density", `{"grid_density": "tiny"}`},
		{"scrubber mode", `{"scrubber_mode": "stash"}`},
		{"empty scrubber mode", `{"scrubber_mode": ""}`},
		{"scrubber mode as bool", `{"scrubber_mode": true}`},
		{"content width", `{"content_width": "huge"}`},
		{"empty content width", `{"content_width": ""}`},
		{"scene list view", `{"scene_list_view": {"pages": {"search": "mosaic"}, "columns": []}}`},
		{"incomplete nested", `{"sort_preferences": {"actors": "name_desc"}}`},
		{"homepage section type", `{"homepage_config": {"show_upload": true, "tabs": [{"id": "a", "name": "A", "sections": [{"id": "s", "type": "bogus", "title": "T", "limit": 5}]}], "default_tab_id": "a", "open_on": "last"}}`},
		{"card config badge", `{"scene_card_config": {"badges": {"top_left": {"items": ["bogus"], "direction": "vertical"}, "top_right": {"items": [], "direction": "vertical"}, "bottom_left": {"items": [], "direction": "vertical"}, "bottom_right": {"items": [], "direction": "vertical"}}, "content_rows": []}}`},
		{"one invalid among valid", `{"show_scrubber": false, "grid_density": "tiny"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No repository call: nothing is read or written for an invalid body
			r, _ := newSettingsTestRouter(t)
			w := doSettingsRequest(r, http.MethodPatch, tt.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp["error"] == "" {
				t.Fatalf("expected an error message, got %s", w.Body.String())
			}
		})
	}
}

func TestPatchSettings_UnknownFieldMessage(t *testing.T) {
	r, _ := newSettingsTestRouter(t)
	w := doSettingsRequest(r, http.MethodPatch, `{"show_scrubbr": true}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `unknown field \"show_scrubbr\"`) {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestPatchSettings_NewRow(t *testing.T) {
	r, settingsRepo := newSettingsTestRouter(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	var saved *data.UserSettings
	settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
		saved = s
		return nil
	})

	w := doSettingsRequest(r, http.MethodPatch, `{"infinite_scroll": true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !saved.InfiniteScroll || saved.DefaultVolume != 100 || saved.VideosPerPage != 20 || !saved.ShowReplayHeatmap || saved.DateFormat != "auto" {
		t.Fatalf("new row should start from the defaults: %+v", saved)
	}
}

const fullPutBody = `{
	"autoplay": true, "default_volume": 60, "loop": false, "ab_loop_controls": false, "marker_loop": false,
	"videos_per_page": 24, "default_sort_order": "created_at_desc", "default_tag_sort": "az",
	"marker_thumbnail_cycling": true,
	"homepage_config": {"show_upload": true, "sections": [], "tabs": [{"id": "home", "name": "Home", "sections": []}], "default_tab_id": "home", "open_on": "last"},
	"parsing_rules": {"presets": [], "activePresetId": null},
	"sort_preferences": {"actors": "name_asc", "studios": "name_asc", "markers": "label_asc"},
	"playlist_auto_advance": "countdown", "playlist_countdown_seconds": 5,
	"show_page_size_selector": false, "advanced_marker_filters": false, "default_marker_view": "grouped",
	"pagination_position": "bottom", "date_format": "auto",
	"scene_card_config": {"badges": {"top_left": {"items": [], "direction": "vertical"}, "top_right": {"items": [], "direction": "vertical"}, "bottom_left": {"items": [], "direction": "vertical"}, "bottom_right": {"items": [], "direction": "vertical"}}, "content_rows": []},
	"grid_density": "comfortable"
}`

// PUT keeps its behaviour: a full body replaces the settings, the optional fields left out keep
// their value, and a partial body is still rejected.
func TestUpdateAllSettings_PutUnchanged(t *testing.T) {
	t.Run("full body", func(t *testing.T) {
		r, settingsRepo := newSettingsTestRouter(t)
		settingsRepo.EXPECT().GetByUserID(uint(1)).Return(existingSettings(), nil)
		var saved *data.UserSettings
		settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
			saved = s
			return nil
		})
		w := doSettingsRequest(r, http.MethodPut, fullPutBody)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if saved.DefaultVolume != 60 || saved.VideosPerPage != 24 || saved.DefaultSortOrder != "created_at_desc" ||
			saved.PlaylistAutoAdvance != "countdown" || saved.DefaultMarkerView != "grouped" || saved.GridDensity != "comfortable" {
			t.Fatalf("full body not applied: %+v", saved)
		}
		// Optional fields of PUT left out keep their value
		if !saved.ShowScrubber || !saved.InfiniteScroll || saved.GestureSeekStep != 15 || !saved.PlayerGestures || saved.ScrubberMode != "free" || saved.ContentWidth != "wide" {
			t.Fatalf("optional fields reset: %+v", saved)
		}
	})
	t.Run("partial body rejected", func(t *testing.T) {
		r, _ := newSettingsTestRouter(t)
		w := doSettingsRequest(r, http.MethodPut, `{"show_scrubber": false}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("invalid value rejected", func(t *testing.T) {
		r, _ := newSettingsTestRouter(t)
		w := doSettingsRequest(r, http.MethodPut, strings.Replace(fullPutBody, `"grid_density": "comfortable"`, `"grid_density": "tiny"`, 1))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("unknown field ignored", func(t *testing.T) {
		r, settingsRepo := newSettingsTestRouter(t)
		settingsRepo.EXPECT().GetByUserID(uint(1)).Return(existingSettings(), nil)
		settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)
		w := doSettingsRequest(r, http.MethodPut, strings.Replace(fullPutBody, `"autoplay": true`, `"autoplay": true, "colour": "red"`, 1))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// scrubber_mode through PUT: set when sent, kept when left out (fullPutBody), validated.
func TestUpdateAllSettings_ScrubberMode(t *testing.T) {
	withMode := func(mode string) string {
		return strings.Replace(fullPutBody, `"grid_density": "comfortable"`, `"grid_density": "comfortable", "scrubber_mode": `+mode, 1)
	}
	t.Run("set", func(t *testing.T) {
		r, settingsRepo := newSettingsTestRouter(t)
		settingsRepo.EXPECT().GetByUserID(uint(1)).Return(existingSettings(), nil)
		var saved *data.UserSettings
		settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
			saved = s
			return nil
		})
		w := doSettingsRequest(r, http.MethodPut, withMode(`"centered"`))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if saved.ScrubberMode != "centered" || !strings.Contains(w.Body.String(), `"scrubber_mode":"centered"`) {
			t.Fatalf("mode not applied: %+v / %s", saved, w.Body.String())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		r, _ := newSettingsTestRouter(t)
		w := doSettingsRequest(r, http.MethodPut, withMode(`"stash"`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// content_width through PUT: set when sent, kept when left out (fullPutBody), validated.
func TestUpdateAllSettings_ContentWidth(t *testing.T) {
	withWidth := func(width string) string {
		return strings.Replace(fullPutBody, `"grid_density": "comfortable"`, `"grid_density": "comfortable", "content_width": `+width, 1)
	}
	t.Run("set", func(t *testing.T) {
		r, settingsRepo := newSettingsTestRouter(t)
		settingsRepo.EXPECT().GetByUserID(uint(1)).Return(existingSettings(), nil)
		var saved *data.UserSettings
		settingsRepo.EXPECT().Upsert(gomock.Any()).DoAndReturn(func(s *data.UserSettings) error {
			saved = s
			return nil
		})
		w := doSettingsRequest(r, http.MethodPut, withWidth(`"full"`))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if saved.ContentWidth != "full" || !strings.Contains(w.Body.String(), `"content_width":"full"`) {
			t.Fatalf("width not applied: %+v / %s", saved, w.Body.String())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		r, _ := newSettingsTestRouter(t)
		w := doSettingsRequest(r, http.MethodPut, withWidth(`"huge"`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}
