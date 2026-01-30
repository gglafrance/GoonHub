package core

import (
	"testing"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

func TestSearchService_Search_RequiresMeilisearch(t *testing.T) {
	logger := zap.NewNop()

	// Create search service without Meilisearch client (nil)
	service := NewSearchService(nil, nil, nil, nil, nil, logger)

	params := data.VideoSearchParams{
		Page:  1,
		Limit: 20,
		Query: "test",
	}

	// Should return error when Meilisearch is not configured
	_, _, err := service.Search(params)
	if err == nil {
		t.Fatal("expected error when Meilisearch is not configured")
	}

	expectedErr := "meilisearch is not configured"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestSearchService_hasUserFilters(t *testing.T) {
	service := &SearchService{}

	tests := []struct {
		name     string
		params   data.VideoSearchParams
		expected bool
	}{
		{
			name:     "no user ID",
			params:   data.VideoSearchParams{UserID: 0},
			expected: false,
		},
		{
			name:     "user ID but no filters",
			params:   data.VideoSearchParams{UserID: 1},
			expected: false,
		},
		{
			name:     "liked filter",
			params:   data.VideoSearchParams{UserID: 1, Liked: boolPtr(true)},
			expected: true,
		},
		{
			name:     "min rating filter",
			params:   data.VideoSearchParams{UserID: 1, MinRating: 3.0},
			expected: true,
		},
		{
			name:     "max rating filter",
			params:   data.VideoSearchParams{UserID: 1, MaxRating: 5.0},
			expected: true,
		},
		{
			name:     "min jizz count filter",
			params:   data.VideoSearchParams{UserID: 1, MinJizzCount: 1},
			expected: true,
		},
		{
			name:     "max jizz count filter",
			params:   data.VideoSearchParams{UserID: 1, MaxJizzCount: 10},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.hasUserFilters(tt.params)
			if result != tt.expected {
				t.Errorf("hasUserFilters() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIntersect(t *testing.T) {
	tests := []struct {
		name     string
		a        []uint
		b        []uint
		expected []uint
	}{
		{
			name:     "empty slices",
			a:        []uint{},
			b:        []uint{},
			expected: []uint{},
		},
		{
			name:     "first empty",
			a:        []uint{},
			b:        []uint{1, 2, 3},
			expected: []uint{},
		},
		{
			name:     "second empty",
			a:        []uint{1, 2, 3},
			b:        []uint{},
			expected: []uint{},
		},
		{
			name:     "no intersection",
			a:        []uint{1, 2, 3},
			b:        []uint{4, 5, 6},
			expected: []uint{},
		},
		{
			name:     "partial intersection",
			a:        []uint{1, 2, 3, 4},
			b:        []uint{3, 4, 5, 6},
			expected: []uint{3, 4},
		},
		{
			name:     "full intersection",
			a:        []uint{1, 2, 3},
			b:        []uint{1, 2, 3},
			expected: []uint{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intersect(tt.a, tt.b)
			if len(result) != len(tt.expected) {
				t.Errorf("intersect() returned %d elements, want %d", len(result), len(tt.expected))
				return
			}

			expectedSet := make(map[uint]bool)
			for _, v := range tt.expected {
				expectedSet[v] = true
			}

			for _, v := range result {
				if !expectedSet[v] {
					t.Errorf("intersect() returned unexpected element %d", v)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
