package core

import (
	"testing"

	"goonhub/internal/data"
)

func TestMarkerWidgetsValidate(t *testing.T) {
	manyLabels := make([]interface{}, markerWidgetMaxLabels+1)
	for i := range manyLabels {
		manyLabels[i] = "l"
	}
	tests := []struct {
		name    string
		widget  string
		cfg     map[string]interface{}
		wantErr bool
	}{
		{"moments default", "moments", map[string]interface{}{}, false},
		{"moments recent", "moments", map[string]interface{}{"order": "recent", "labels": []interface{}{"Cum"}}, false},
		{"moments bad order", "moments", map[string]interface{}{"order": "oldest"}, true},
		{"moments too many labels", "moments", map[string]interface{}{"labels": manyLabels}, true},
		{"compilation clip ok", "marker_compilation", map[string]interface{}{"clip_seconds": float64(20)}, false},
		{"compilation clip too short", "marker_compilation", map[string]interface{}{"clip_seconds": float64(1)}, true},
		{"compilation clip too long", "marker_compilation", map[string]interface{}{"clip_seconds": float64(500)}, true},
		{"recent markers", "recent_markers", map[string]interface{}{"labels": "Kiss,Cum"}, false},
		{"browse tags", "browse_tags", map[string]interface{}{"source": "tags", "items": []interface{}{"Pup"}}, false},
		{"browse labels default", "browse_tags", map[string]interface{}{}, false},
		{"browse bad source", "browse_tags", map[string]interface{}{"source": "studios"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := data.HomepageSection{Type: tt.widget, Config: tt.cfg}
			err := validateHomepageWidgetConfig(&section)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarkerWidgetsRegistered(t *testing.T) {
	for _, widget := range []string{"moments", "marker_compilation", "recent_markers", "browse_tags"} {
		if !IsHomepageWidgetType(widget) {
			t.Fatalf("widget %q not registered", widget)
		}
	}
}

func TestMarkerWidgetsWithoutDB(t *testing.T) {
	// Without a database the widgets return empty payloads instead of failing
	svc := &HomepageService{}
	for _, widget := range []string{"moments", "marker_compilation", "recent_markers", "browse_tags"} {
		section := data.HomepageSection{ID: widget, Type: widget, Limit: 5, Config: map[string]interface{}{}}
		got, err := homepageWidgets[widget].Fetch(svc, 1, section)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", widget, err)
		}
		if got.Widget == nil || got.Total != 0 {
			t.Fatalf("%s: unexpected data %+v", widget, got)
		}
	}
}

func TestMarkerCompilationClampsClip(t *testing.T) {
	svc := &HomepageService{}
	section := data.HomepageSection{Type: "marker_compilation", Limit: 5, Config: map[string]interface{}{"clip_seconds": float64(999)}}
	got, err := fetchMarkerCompilationWidget(svc, 1, section)
	if err != nil {
		t.Fatal(err)
	}
	payload := got.Widget.(MarkerWidgetPayload)
	if payload.ClipSeconds != compilationMaxClipSeconds {
		t.Fatalf("clip not clamped: %d", payload.ClipSeconds)
	}
}
