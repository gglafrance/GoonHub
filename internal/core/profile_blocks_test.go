package core

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

func TestProfileBlockRegistry(t *testing.T) {
	specs := ProfileBlockSpecs()
	want := []string{"favorite_scenes", "favorite_markers", "recent_watches", "most_watched", "recent_markers", "liked_scenes", "stats", "activity_calendar", "playlists", "friends"}
	if len(specs) != len(want) {
		t.Fatalf("%d block types, want %d", len(specs), len(want))
	}
	for i, spec := range specs {
		if spec.Type != want[i] {
			t.Fatalf("block %d = %s, want %s", i, spec.Type, want[i])
		}
		if _, ok := privacyItemByKey(spec.PrivacyItem); !ok {
			t.Fatalf("%s: unknown privacy item %s", spec.Type, spec.PrivacyItem)
		}
	}
	layout := DefaultProfileLayout()
	if len(layout) != len(want) || !layout[0].Visible || layout[3].Visible {
		t.Fatalf("default layout: %+v", layout)
	}
}

func TestValidateProfileLayout(t *testing.T) {
	cases := []struct {
		name   string
		layout data.ProfileLayout
		ok     bool
	}{
		{"unknown type", data.ProfileLayout{{Type: "nope"}}, false},
		{"duplicate", data.ProfileLayout{{Type: "stats"}, {Type: "stats"}}, false},
		{"unknown option", data.ProfileLayout{{Type: "stats", Config: map[string]any{"count": 3.0}}}, false},
		{"count out of range", data.ProfileLayout{{Type: "recent_watches", Config: map[string]any{"count": 99.0}}}, false},
		{"count not whole", data.ProfileLayout{{Type: "recent_watches", Config: map[string]any{"count": 4.5}}}, false},
		{"count as a string", data.ProfileLayout{{Type: "recent_watches", Config: map[string]any{"count": "4"}}}, false},
		{"unknown period", data.ProfileLayout{{Type: "most_watched", Config: map[string]any{"period": "decade"}}}, false},
		{"valid", data.ProfileLayout{{Type: "most_watched", Visible: true, Config: map[string]any{"count": 12.0, "period": "year"}}}, true},
		{"empty", data.ProfileLayout{}, true},
	}
	for _, tc := range cases {
		out, err := ValidateProfileLayout(tc.layout)
		if tc.ok != (err == nil) {
			t.Fatalf("%s: err = %v", tc.name, err)
		}
		if err != nil {
			if !apperrors.IsValidation(err) {
				t.Fatalf("%s: not a validation error: %v", tc.name, err)
			}
			continue
		}
		if len(out) != len(ProfileBlockSpecs()) {
			t.Fatalf("%s: missing types not appended: %d", tc.name, len(out))
		}
	}
	out, _ := ValidateProfileLayout(data.ProfileLayout{{Type: "most_watched", Visible: true, Config: map[string]any{"count": 12.0}}})
	if out[0].Type != "most_watched" || out[0].Config["count"] != 12 || out[1].Visible {
		t.Fatalf("normalized: %+v", out[:2])
	}
}

func TestNormalizeProfileLayout(t *testing.T) {
	if got := NormalizeProfileLayout(nil); len(got) != len(DefaultProfileLayout()) || !got[0].Visible {
		t.Fatalf("nil layout: %+v", got)
	}
	// A stored layout with an old type and a bad value: both dropped, the rest kept
	got := NormalizeProfileLayout(data.ProfileLayout{
		{Type: "removed_block", Visible: true},
		{Type: "friends", Visible: true, LockedPlaceholder: true, Config: map[string]any{"count": 500.0}},
		{Type: "friends", Visible: false},
	})
	if got[0].Type != "friends" || !got[0].Visible || !got[0].LockedPlaceholder || len(got[0].Config) != 0 {
		t.Fatalf("normalized: %+v", got[0])
	}
	for _, b := range got[1:] {
		if b.Visible {
			t.Fatalf("appended block %s visible", b.Type)
		}
	}
	spec, _ := ProfileBlockSpecFor("most_watched")
	values := spec.resolve(map[string]any{"count": 3})
	if values.Int("count") != 3 || values.String("period") != profilePeriodMonth {
		t.Fatalf("resolved: %v", values)
	}
}
