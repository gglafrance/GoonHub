package request

import (
	"encoding/json"
	"testing"
)

// show_scrubber is optional: a client that does not send it keeps the current value (nil).
func TestUpdateAllSettingsRequest_ShowScrubberOptional(t *testing.T) {
	on, off := true, false
	tests := []struct {
		body string
		want *bool
	}{
		{`{}`, nil},
		{`{"show_scrubber": true}`, &on},
		{`{"show_scrubber": false}`, &off},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			var req UpdateAllSettingsRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if (req.ShowScrubber == nil) != (tt.want == nil) || (tt.want != nil && *req.ShowScrubber != *tt.want) {
				t.Fatalf("got %v, want %v", req.ShowScrubber, tt.want)
			}
		})
	}
}
