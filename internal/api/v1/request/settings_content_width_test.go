package request

import (
	"encoding/json"
	"strings"
	"testing"
)

// content_width is optional in PUT: a client that does not send it keeps the current value (nil).
func TestUpdateAllSettingsRequest_ContentWidthOptional(t *testing.T) {
	tests := []struct {
		body string
		want *string
	}{
		{`{}`, nil},
		{`{"content_width": "wide"}`, ptrTo("wide")},
		{`{"content_width": "full"}`, ptrTo("full")},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			var req UpdateAllSettingsRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if (req.ContentWidth == nil) != (tt.want == nil) || (tt.want != nil && *req.ContentWidth != *tt.want) {
				t.Fatalf("got %v, want %v", req.ContentWidth, tt.want)
			}
		})
	}
}

func TestDecodePatchSettingsRequest_ContentWidth(t *testing.T) {
	req, err := DecodePatchSettingsRequest(strings.NewReader(`{"content_width": "full"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ContentWidth == nil || *req.ContentWidth != "full" || req.GridDensity != nil {
		t.Fatalf("unexpected request: %+v", req)
	}

	tests := []struct {
		body string
		want string
	}{
		{`{"content_width": null}`, `"content_width" cannot be null`},
		{`{"content_width": 1}`, `invalid value for "content_width": expected string`},
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
