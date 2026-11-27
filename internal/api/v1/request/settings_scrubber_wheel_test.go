package request

import (
	"encoding/json"
	"strings"
	"testing"
)

// scrubber_wheel is optional in PUT: a client that does not send it keeps the current value (nil).
func TestUpdateAllSettingsRequest_ScrubberWheelOptional(t *testing.T) {
	on, off := true, false
	tests := []struct {
		body string
		want *bool
	}{
		{`{}`, nil},
		{`{"scrubber_wheel": true}`, &on},
		{`{"scrubber_wheel": false}`, &off},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			var req UpdateAllSettingsRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if (req.ScrubberWheel == nil) != (tt.want == nil) || (tt.want != nil && *req.ScrubberWheel != *tt.want) {
				t.Fatalf("got %v, want %v", req.ScrubberWheel, tt.want)
			}
		})
	}
}

func TestDecodePatchSettingsRequest_ScrubberWheel(t *testing.T) {
	req, err := DecodePatchSettingsRequest(strings.NewReader(`{"scrubber_wheel": true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ScrubberWheel == nil || !*req.ScrubberWheel || req.ShowScrubber != nil || req.ScrubberMode != nil {
		t.Fatalf("unexpected request: %+v", req)
	}

	tests := []struct {
		body string
		want string
	}{
		{`{"scrubber_wheel": null}`, `"scrubber_wheel" cannot be null`},
		{`{"scrubber_wheel": "yes"}`, `invalid value for "scrubber_wheel": expected bool`},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			_, err := DecodePatchSettingsRequest(strings.NewReader(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}
