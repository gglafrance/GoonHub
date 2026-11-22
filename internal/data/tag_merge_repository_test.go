package data

import (
	"encoding/json"
	"testing"
)

func TestRetargetHomepageTagRefs(t *testing.T) {
	raw := `{
		"sections": [{"type": "tag", "config": {"tag_id": 2}}],
		"tabs": [{"id": "home", "sections": [
			{"type": "tag", "config": {"tag_id": "3"}},
			{"type": "tag", "config": {"tag_id": 7}},
			{"type": "discover", "config": {"tags": ["cumshot", "Anal", "Cumshot", "cumshot "]}},
			{"type": "browse_tags", "config": {"source": "tags", "items": ["cumshot", "Anal"]}},
			{"type": "browse_tags", "config": {"source": "labels", "items": ["cumshot"]}},
			{"type": "latest", "config": {}}
		]}]
	}`
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	target := Tag{ID: 1, Name: "Cumshot"}
	srcIDs := map[uint]bool{2: true, 3: true}
	srcNames := map[string]bool{"cumshot": true, "cumshot ": true}
	if !RetargetHomepageTagRefs(cfg, srcIDs, srcNames, target) {
		t.Fatal("expected a change")
	}
	out, _ := json.Marshal(cfg)
	var got struct {
		Sections []HomepageSection `json:"sections"`
		Tabs     []HomepageTab     `json:"tabs"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Sections[0].Config["tag_id"] != float64(1) {
		t.Fatalf("legacy tag section = %v", got.Sections[0].Config)
	}
	s := got.Tabs[0].Sections
	if s[0].Config["tag_id"] != float64(1) {
		t.Fatalf("string tag_id section = %v", s[0].Config)
	}
	if s[1].Config["tag_id"] != float64(7) {
		t.Fatalf("other tag section must not change: %v", s[1].Config)
	}
	if tags := s[2].Config["tags"].([]any); len(tags) != 2 || tags[0] != "Cumshot" || tags[1] != "Anal" {
		t.Fatalf("widget tags = %v", tags)
	}
	if items := s[3].Config["items"].([]any); len(items) != 2 || items[0] != "Cumshot" {
		t.Fatalf("browse_tags items = %v", items)
	}
	if items := s[4].Config["items"].([]any); items[0] != "cumshot" {
		t.Fatalf("label tiles must not change: %v", items)
	}

	if RetargetHomepageTagRefs(cfg, srcIDs, srcNames, target) {
		t.Fatal("second pass must not change anything")
	}
}
