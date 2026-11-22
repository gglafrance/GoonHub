package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/gin-gonic/gin/binding"
)

// PatchSettingsRequest is the body of PATCH /settings: only the fields present in the JSON are
// updated, the others keep their stored value. Every field is a pointer so that an absent field
// (nil) can be told apart from a zero value.
//
// Nested objects (homepage_config, parsing_rules, sort_preferences, scene_card_config,
// scene_list_view) replace the stored object as a whole and must be complete (see
// patchRequiredKeys): they are never merged with the stored value.
type PatchSettingsRequest struct {
	Autoplay                 *bool                         `json:"autoplay"`
	DefaultVolume            *int                          `json:"default_volume"`
	Loop                     *bool                         `json:"loop"`
	AbLoopControls           *bool                         `json:"ab_loop_controls"`
	MarkerLoop               *bool                         `json:"marker_loop"`
	VideosPerPage            *int                          `json:"videos_per_page"`
	DefaultSortOrder         *string                       `json:"default_sort_order"`
	DefaultTagSort           *string                       `json:"default_tag_sort"`
	MarkerThumbnailCycling   *bool                         `json:"marker_thumbnail_cycling"`
	HomepageConfig           *UpdateHomepageConfigRequest  `json:"homepage_config"`
	ParsingRules             *UpdateParsingRulesRequest    `json:"parsing_rules"`
	SortPreferences          *UpdateSortPreferencesRequest `json:"sort_preferences"`
	PlaylistAutoAdvance      *string                       `json:"playlist_auto_advance"`
	PlaylistCountdownSeconds *int                          `json:"playlist_countdown_seconds"`
	ShowPageSizeSelector     *bool                         `json:"show_page_size_selector"`
	AdvancedMarkerFilters    *bool                         `json:"advanced_marker_filters"`
	DefaultMarkerView        *string                       `json:"default_marker_view"`
	PaginationPosition       *string                       `json:"pagination_position"`
	DateFormat               *string                       `json:"date_format"`
	PlayerGestures           *bool                         `json:"player_gestures"`
	GestureSeekStep          *int                          `json:"gesture_seek_step"`
	ShowReplayHeatmap        *bool                         `json:"show_replay_heatmap"`
	RecordReplayHeatmap      *bool                         `json:"record_replay_heatmap"`
	SceneCardConfig          *UpdateSceneCardConfigRequest `json:"scene_card_config"`
	GridDensity              *string                       `json:"grid_density"`
	InfiniteScroll           *bool                         `json:"infinite_scroll"`
	SceneListView            *SceneListViewRequest         `json:"scene_list_view"`
	ShowScrubber             *bool                         `json:"show_scrubber"`
}

// requiredKeys lists the keys a nested object must contain, and the requirements of its own
// nested objects.
type requiredKeys struct {
	keys   []string
	nested map[string]requiredKeys
}

var badgeZoneKeys = requiredKeys{keys: []string{"top_left", "top_right", "bottom_left", "bottom_right"}}

// patchRequiredKeys are the keys of the nested objects that a PATCH must send: a nested object
// replaces the stored one, so a missing key would silently reset that part of it.
var patchRequiredKeys = map[string]requiredKeys{
	// sections is the legacy single-page layout and may be left out
	"homepage_config":  {keys: []string{"show_upload", "tabs", "default_tab_id", "open_on"}},
	"parsing_rules":    {keys: []string{"presets", "activePresetId"}},
	"sort_preferences": {keys: []string{"actors", "studios", "markers", "actor_scenes", "studio_scenes"}},
	"scene_card_config": {
		keys:   []string{"badges", "content_rows"},
		nested: map[string]requiredKeys{"badges": badgeZoneKeys},
	},
	"scene_list_view": {keys: []string{"pages", "columns"}},
}

// patchSettingsFields are the JSON names of the fields of PatchSettingsRequest.
var patchSettingsFields = func() map[string]bool {
	fields := map[string]bool{}
	for _, name := range jsonFieldNames(PatchSettingsRequest{}) {
		fields[name] = true
	}
	return fields
}()

// jsonFieldNames returns the JSON names of the fields of a struct.
func jsonFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		names = append(names, name)
	}
	return names
}

// jsonTypeName names the JSON type expected for a Go type, for error messages.
func jsonTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	default:
		return t.String()
	}
}

// PatchSettingsError is a client error in a PATCH /settings body (answered with a 400).
type PatchSettingsError struct{ msg string }

func (e *PatchSettingsError) Error() string { return e.msg }

func patchError(format string, args ...any) error {
	return &PatchSettingsError{msg: fmt.Sprintf(format, args...)}
}

// DecodePatchSettingsRequest reads and validates a PATCH /settings body. The body must be a
// non-empty JSON object of known settings; null is not accepted for a setting (leave the field
// out to keep its value); nested objects must be complete. The binding rules of the nested
// request types are checked like for PUT; the value rules (ranges, enums) are checked by the
// settings service options.
func DecodePatchSettingsRequest(body io.Reader) (*PatchSettingsRequest, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, patchError("request body must be a JSON object")
	}
	if len(fields) == 0 {
		return nil, patchError("no settings to update")
	}

	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !patchSettingsFields[name] {
			return nil, patchError("unknown field %q", name)
		}
		value := fields[name]
		if isJSONNull(value) {
			return nil, patchError("field %q cannot be null (leave it out to keep its value)", name)
		}
		if spec, ok := patchRequiredKeys[name]; ok {
			if err := checkRequiredKeys(name, value, spec); err != nil {
				return nil, err
			}
		}
	}

	var req PatchSettingsRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return nil, patchError("invalid value for %q: expected %s", typeErr.Field, jsonTypeName(typeErr.Type))
		}
		return nil, patchError("invalid request body: %v", err)
	}
	if err := binding.Validator.ValidateStruct(&req); err != nil {
		return nil, patchError("invalid request body: %v", err)
	}
	return &req, nil
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

// checkRequiredKeys checks that a nested object has all the keys of spec. A value that is not an
// object is left to the decoding, which reports the type error.
func checkRequiredKeys(path string, value json.RawMessage, spec requiredKeys) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(value, &obj); err != nil || obj == nil {
		return nil
	}
	for _, key := range spec.keys {
		child, ok := obj[key]
		if !ok {
			return patchError("%s: missing field %q (a nested object replaces the stored one and must be complete)", path, key)
		}
		if nested, ok := spec.nested[key]; ok {
			if isJSONNull(child) {
				return patchError("%s: field %q cannot be null", path, key)
			}
			if err := checkRequiredKeys(path+"."+key, child, nested); err != nil {
				return err
			}
		}
	}
	return nil
}
