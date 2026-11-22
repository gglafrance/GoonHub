package request

import (
	"errors"
	"strings"
	"testing"
)

const validSortPreferences = `{"actors":"name_asc","studios":"name_asc","markers":"label_asc","actor_scenes":"","studio_scenes":""}`

const validCardConfig = `{"badges":{"top_left":{"items":["rating"],"direction":"vertical"},"top_right":{"items":[],"direction":"vertical"},` +
	`"bottom_left":{"items":[],"direction":"vertical"},"bottom_right":{"items":["duration"],"direction":"horizontal"}},"content_rows":[]}`

const validHomepage = `{"show_upload":true,"tabs":[{"id":"home","name":"Home","icon":"heroicons:home","sections":[` +
	`{"id":"s1","type":"latest","title":"Latest","enabled":true,"limit":12,"order":0}]}],"default_tab_id":"home","open_on":"last"}`

func TestDecodePatchSettingsRequest_Valid(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(r *PatchSettingsRequest) bool
	}{
		{"bool false", `{"show_scrubber": false}`, func(r *PatchSettingsRequest) bool {
			return r.ShowScrubber != nil && !*r.ShowScrubber && r.Autoplay == nil && r.InfiniteScroll == nil
		}},
		{"bool true", `{"autoplay": true}`, func(r *PatchSettingsRequest) bool { return r.Autoplay != nil && *r.Autoplay }},
		{"int zero", `{"default_volume": 0}`, func(r *PatchSettingsRequest) bool { return r.DefaultVolume != nil && *r.DefaultVolume == 0 }},
		{"int", `{"videos_per_page": 40}`, func(r *PatchSettingsRequest) bool { return r.VideosPerPage != nil && *r.VideosPerPage == 40 }},
		{"empty string", `{"date_format": ""}`, func(r *PatchSettingsRequest) bool { return r.DateFormat != nil && *r.DateFormat == "" }},
		{"string", `{"grid_density": "large"}`, func(r *PatchSettingsRequest) bool { return r.GridDensity != nil && *r.GridDensity == "large" }},
		{"several", `{"loop": true, "gesture_seek_step": 15, "pagination_position": "top"}`, func(r *PatchSettingsRequest) bool {
			return *r.Loop && *r.GestureSeekStep == 15 && *r.PaginationPosition == "top" && r.MarkerLoop == nil
		}},
		{"sort preferences", `{"sort_preferences": ` + validSortPreferences + `}`, func(r *PatchSettingsRequest) bool {
			return r.SortPreferences != nil && r.SortPreferences.Actors == "name_asc"
		}},
		{"card config", `{"scene_card_config": ` + validCardConfig + `}`, func(r *PatchSettingsRequest) bool {
			return r.SceneCardConfig != nil && r.SceneCardConfig.Badges.BottomRight.Direction == "horizontal"
		}},
		{"homepage", `{"homepage_config": ` + validHomepage + `}`, func(r *PatchSettingsRequest) bool {
			return r.HomepageConfig != nil && len(r.HomepageConfig.Tabs) == 1 && r.HomepageConfig.Tabs[0].Sections[0].Limit == 12
		}},
		{"parsing rules with null active preset", `{"parsing_rules": {"presets": [], "activePresetId": null}}`, func(r *PatchSettingsRequest) bool {
			return r.ParsingRules != nil && r.ParsingRules.ActivePresetID == nil
		}},
		{"scene list view", `{"scene_list_view": {"pages": {"search": "table"}, "columns": ["title"]}}`, func(r *PatchSettingsRequest) bool {
			return r.SceneListView != nil && r.SceneListView.Pages["search"] == "table"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := DecodePatchSettingsRequest(strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.check(req) {
				t.Fatalf("unexpected request: %+v", req)
			}
		})
	}
}

func TestDecodePatchSettingsRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"empty body", ``, "must be a JSON object"},
		{"null body", `null`, "must be a JSON object"},
		{"array body", `[]`, "must be a JSON object"},
		{"not json", `{"autoplay": tru`, "must be a JSON object"},
		{"empty object", `{}`, "no settings to update"},
		{"unknown field", `{"show_scrubber": true, "show_scrubbr": true}`, `unknown field "show_scrubbr"`},
		{"read-only field", `{"user_id": 2}`, `unknown field "user_id"`},
		{"computed field", `{"max_items_per_page": 500}`, `unknown field "max_items_per_page"`},
		{"null bool", `{"show_scrubber": null}`, `"show_scrubber" cannot be null`},
		{"null nested", `{"scene_list_view": null}`, `"scene_list_view" cannot be null`},
		{"bool as string", `{"autoplay": "yes"}`, `invalid value for "autoplay": expected bool`},
		{"int as string", `{"default_volume": "50"}`, `invalid value for "default_volume": expected integer`},
		{"float int", `{"videos_per_page": 1.5}`, `invalid value for "videos_per_page"`},
		{"string as number", `{"grid_density": 2}`, `invalid value for "grid_density": expected string`},
		{"nested as string", `{"sort_preferences": "name_asc"}`, `invalid value for "sort_preferences": expected object`},
		{"incomplete sort preferences", `{"sort_preferences": {"actors": "name_desc"}}`, `sort_preferences: missing field "studios"`},
		{"sort preferences without entity scenes", `{"sort_preferences": {"actors":"name_asc","studios":"name_asc","markers":"label_asc"}}`, `missing field "actor_scenes"`},
		{"empty required sort", `{"sort_preferences": {"actors":"","studios":"name_asc","markers":"label_asc","actor_scenes":"","studio_scenes":""}}`, "invalid request body"},
		{"incomplete card config", `{"scene_card_config": {"badges": {}, "content_rows": []}}`, `scene_card_config.badges: missing field "top_left"`},
		{"card config without rows", `{"scene_card_config": {"badges": {"top_left":{},"top_right":{},"bottom_left":{},"bottom_right":{}}}}`, `missing field "content_rows"`},
		{"null badges", `{"scene_card_config": {"badges": null, "content_rows": []}}`, `"badges" cannot be null`},
		{"incomplete homepage", `{"homepage_config": {"tabs": []}}`, `homepage_config: missing field "show_upload"`},
		{"homepage tab name too long", `{"homepage_config": {"show_upload":true,"tabs":[{"id":"t","name":"` + strings.Repeat("x", 41) + `","sections":[]}],"default_tab_id":"t","open_on":"last"}}`, "invalid request body"},
		{"homepage section limit", `{"homepage_config": {"show_upload":true,"tabs":[{"id":"t","name":"T","sections":[{"id":"s","type":"latest","title":"L","limit":0}]}],"default_tab_id":"t","open_on":"last"}}`, "invalid request body"},
		{"homepage open on", `{"homepage_config": {"show_upload":true,"tabs":[],"default_tab_id":"","open_on":"first"}}`, "invalid request body"},
		{"incomplete parsing rules", `{"parsing_rules": {"presets": []}}`, `parsing_rules: missing field "activePresetId"`},
		{"parsing preset without name", `{"parsing_rules": {"presets": [{"id":"p","name":"","rules":[]}], "activePresetId": null}}`, "invalid request body"},
		{"incomplete scene list view", `{"scene_list_view": {"pages": {}}}`, `scene_list_view: missing field "columns"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePatchSettingsRequest(strings.NewReader(tt.body))
			if err == nil {
				t.Fatal("expected an error")
			}
			var patchErr *PatchSettingsError
			if !errors.As(err, &patchErr) {
				t.Fatalf("expected a PatchSettingsError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// Every settings field of PUT can be patched (a new field must be added to both requests).
func TestPatchSettingsRequest_CoversUpdateAllSettingsFields(t *testing.T) {
	for _, name := range jsonFieldNames(UpdateAllSettingsRequest{}) {
		if !patchSettingsFields[name] {
			t.Fatalf("field %q of PUT /settings is missing from PatchSettingsRequest", name)
		}
	}
	if len(patchSettingsFields) != len(jsonFieldNames(UpdateAllSettingsRequest{})) {
		t.Fatalf("PatchSettingsRequest has %d fields, UpdateAllSettingsRequest %d", len(patchSettingsFields), len(jsonFieldNames(UpdateAllSettingsRequest{})))
	}
}
