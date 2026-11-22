package request

import (
	"encoding/json"
	"strings"
	"testing"
)

// scrubber_mode is optional in PUT: a client that does not send it keeps the current value (nil).
func TestUpdateAllSettingsRequest_ScrubberModeOptional(t *testing.T) {
	tests := []struct {
		body string
		want *string
	}{
		{`{}`, nil},
		{`{"scrubber_mode": "centered"}`, ptrTo("centered")},
		{`{"scrubber_mode": "free"}`, ptrTo("free")},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			var req UpdateAllSettingsRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if (req.ScrubberMode == nil) != (tt.want == nil) || (tt.want != nil && *req.ScrubberMode != *tt.want) {
				t.Fatalf("got %v, want %v", req.ScrubberMode, tt.want)
			}
		})
	}
}

func TestDecodePatchSettingsRequest_ScrubberMode(t *testing.T) {
	req, err := DecodePatchSettingsRequest(strings.NewReader(`{"scrubber_mode": "centered"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ScrubberMode == nil || *req.ScrubberMode != "centered" || req.ShowScrubber != nil {
		t.Fatalf("unexpected request: %+v", req)
	}

	tests := []struct {
		body string
		want string
	}{
		{`{"scrubber_mode": null}`, `"scrubber_mode" cannot be null`},
		{`{"scrubber_mode": 1}`, `invalid value for "scrubber_mode": expected string`},
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

func ptrTo(s string) *string { return &s }
