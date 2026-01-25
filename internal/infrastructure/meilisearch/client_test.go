package meilisearch

import (
	"testing"

	"go.uber.org/zap"
)

func TestClient_buildFilters(t *testing.T) {
	client := &Client{logger: zap.NewNop()}

	tests := []struct {
		name           string
		params         SearchParams
		expectedLen    int
		expectContains []string
	}{
		{
			name:        "empty params",
			params:      SearchParams{},
			expectedLen: 0,
		},
		{
			name: "tag IDs filter",
			params: SearchParams{
				TagIDs: []uint{1, 2, 3},
			},
			expectedLen:    3,
			expectContains: []string{"tag_ids = 1", "tag_ids = 2", "tag_ids = 3"},
		},
		{
			name: "studio filter",
			params: SearchParams{
				Studio: "Test Studio",
			},
			expectedLen:    1,
			expectContains: []string{`studio = "Test Studio"`},
		},
		{
			name: "duration range",
			params: SearchParams{
				MinDuration: floatPtr(60),
				MaxDuration: floatPtr(3600),
			},
			expectedLen:    2,
			expectContains: []string{"duration >= 60.000000", "duration <= 3600.000000"},
		},
		{
			name: "height range",
			params: SearchParams{
				MinHeight: intPtr(720),
				MaxHeight: intPtr(1080),
			},
			expectedLen:    2,
			expectContains: []string{"height >= 720", "height <= 1080"},
		},
		{
			name: "video IDs filter",
			params: SearchParams{
				VideoIDs: []uint{1, 2, 3},
			},
			expectedLen:    1,
			expectContains: []string{"(id = 1 OR id = 2 OR id = 3)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := client.buildFilters(tt.params)

			if len(filters) != tt.expectedLen {
				t.Errorf("buildFilters() returned %d filters, want %d", len(filters), tt.expectedLen)
			}

			for _, expected := range tt.expectContains {
				found := false
				for _, f := range filters {
					if f == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildFilters() missing expected filter %q", expected)
				}
			}
		})
	}
}

func TestClient_buildSort(t *testing.T) {
	client := &Client{logger: zap.NewNop()}

	tests := []struct {
		name     string
		params   SearchParams
		expected []string
	}{
		{
			name:     "empty sort",
			params:   SearchParams{},
			expected: nil,
		},
		{
			name:     "relevance sort",
			params:   SearchParams{Sort: "relevance"},
			expected: nil,
		},
		{
			name:     "date sort desc",
			params:   SearchParams{Sort: "date", SortDir: "desc"},
			expected: []string{"created_at:desc"},
		},
		{
			name:     "date sort asc",
			params:   SearchParams{Sort: "created_at", SortDir: "asc"},
			expected: []string{"created_at:asc"},
		},
		{
			name:     "title sort",
			params:   SearchParams{Sort: "title", SortDir: "asc"},
			expected: []string{"title:asc"},
		},
		{
			name:     "duration sort",
			params:   SearchParams{Sort: "duration", SortDir: "desc"},
			expected: []string{"duration:desc"},
		},
		{
			name:     "unknown sort field",
			params:   SearchParams{Sort: "unknown"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildSort(tt.params)

			if len(result) != len(tt.expected) {
				t.Errorf("buildSort() returned %d elements, want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("buildSort()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestEscapeFilterValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with "quotes"`, `with \"quotes\"`},
		{`with \backslash`, `with \\backslash`},
		{`both "quotes" and \backslash`, `both \"quotes\" and \\backslash`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeFilterValue(tt.input)
			if result != tt.expected {
				t.Errorf("escapeFilterValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
