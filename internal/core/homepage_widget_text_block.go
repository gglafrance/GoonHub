package core

import (
	"fmt"
	"unicode/utf8"

	"goonhub/internal/data"
)

// text_block: free text (markdown) and shortcut links, entirely stored in the section config:
//
//	{"markdown": "...", "links": [{"label": "...", "url": "/search?..."}]}
//
// It needs no data: the frontend renders the config.

const textBlockMaxMarkdown = 10000

func init() {
	registerHomepageWidget("text_block", HomepageWidgetSpec{
		Fetch: func(_ *HomepageService, _ uint, section data.HomepageSection) (*HomepageSectionData, error) {
			return emptyWidgetData(section, nil), nil
		},
		Validate: func(cfg map[string]interface{}) error {
			if md, ok := cfg["markdown"].(string); ok && utf8.RuneCountInString(md) > textBlockMaxMarkdown {
				return fmt.Errorf("text block: markdown must be at most %d characters", textBlockMaxMarkdown)
			}
			if links, ok := cfg["links"].([]interface{}); ok && len(links) > 30 {
				return fmt.Errorf("text block: at most 30 links")
			}
			return nil
		},
	})
}
