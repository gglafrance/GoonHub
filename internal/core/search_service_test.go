package core

import (
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestSearchService_Search_RequiresMeilisearch(t *testing.T) {
	logger := zap.NewNop()

	// Create search service without Meilisearch client (nil)
	service := NewSearchService(nil, nil, nil, nil, nil, nil, logger)

	params := data.SceneSearchParams{
		Page:  1,
		Limit: 20,
		Query: "test",
	}

	// Should return error when Meilisearch is not configured
	_, err := service.Search(params)
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
		params   data.SceneSearchParams
		expected bool
	}{
		{
			name:     "no user ID",
			params:   data.SceneSearchParams{UserID: 0},
			expected: false,
		},
		{
			name:     "user ID but no filters",
			params:   data.SceneSearchParams{UserID: 1},
			expected: false,
		},
		{
			name:     "liked filter",
			params:   data.SceneSearchParams{UserID: 1, Liked: boolPtr(true)},
			expected: true,
		},
		{
			name:     "min rating filter",
			params:   data.SceneSearchParams{UserID: 1, MinRating: 3.0},
			expected: true,
		},
		{
			name:     "max rating filter",
			params:   data.SceneSearchParams{UserID: 1, MaxRating: 5.0},
			expected: true,
		},
		{
			name:     "min jizz count filter",
			params:   data.SceneSearchParams{UserID: 1, MinJizzCount: 1},
			expected: true,
		},
		{
			name:     "max jizz count filter",
			params:   data.SceneSearchParams{UserID: 1, MaxJizzCount: 10},
			expected: true,
		},
		{
			name:     "marker labels filter",
			params:   data.SceneSearchParams{UserID: 1, MarkerLabels: []string{"favorite", "watch later"}},
			expected: true,
		},
		{
			name:     "empty marker labels",
			params:   data.SceneSearchParams{UserID: 1, MarkerLabels: []string{}},
			expected: false,
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

func TestHandleRandomSort_SameSeedSameOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSceneRepo := mocks.NewMockSceneRepository(ctrl)

	service := &SearchService{
		sceneRepo: mockSceneRepo,
		logger:    zap.NewNop(),
	}

	allIDs := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	params := data.SceneSearchParams{Page: 1, Limit: 5, Seed: 42}

	// Mock GetByIDs to return scenes in the order requested
	mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).DoAndReturn(func(ids []uint) ([]data.Scene, error) {
		scenes := make([]data.Scene, len(ids))
		for i, id := range ids {
			scenes[i] = data.Scene{}
			scenes[i].ID = id
		}
		return scenes, nil
	}).Times(2)

	// First call
	ids1 := make([]uint, len(allIDs))
	copy(ids1, allIDs)
	result1, err := service.handleRandomSort(ids1, params)
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}

	// Second call with same seed
	ids2 := make([]uint, len(allIDs))
	copy(ids2, allIDs)
	result2, err := service.handleRandomSort(ids2, params)
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}

	// Same seed must produce same order
	if len(result1.Scenes) != len(result2.Scenes) {
		t.Fatalf("different result lengths: %d vs %d", len(result1.Scenes), len(result2.Scenes))
	}
	for i := range result1.Scenes {
		if result1.Scenes[i].ID != result2.Scenes[i].ID {
			t.Errorf("position %d: got ID %d vs %d", i, result1.Scenes[i].ID, result2.Scenes[i].ID)
		}
	}
	if result1.Seed != result2.Seed {
		t.Errorf("seeds differ: %d vs %d", result1.Seed, result2.Seed)
	}
}

func TestHandleRandomSort_DifferentSeedsDifferentOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSceneRepo := mocks.NewMockSceneRepository(ctrl)

	service := &SearchService{
		sceneRepo: mockSceneRepo,
		logger:    zap.NewNop(),
	}

	allIDs := make([]uint, 100)
	for i := range allIDs {
		allIDs[i] = uint(i + 1)
	}

	mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).DoAndReturn(func(ids []uint) ([]data.Scene, error) {
		scenes := make([]data.Scene, len(ids))
		for i, id := range ids {
			scenes[i] = data.Scene{}
			scenes[i].ID = id
		}
		return scenes, nil
	}).Times(2)

	ids1 := make([]uint, len(allIDs))
	copy(ids1, allIDs)
	result1, err := service.handleRandomSort(ids1, data.SceneSearchParams{Page: 1, Limit: 20, Seed: 42})
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}

	ids2 := make([]uint, len(allIDs))
	copy(ids2, allIDs)
	result2, err := service.handleRandomSort(ids2, data.SceneSearchParams{Page: 1, Limit: 20, Seed: 9999})
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}

	// With 100 items and different seeds, the first 20 should almost certainly differ
	sameCount := 0
	for i := range result1.Scenes {
		if result1.Scenes[i].ID == result2.Scenes[i].ID {
			sameCount++
		}
	}
	if sameCount == len(result1.Scenes) {
		t.Error("different seeds produced identical ordering")
	}
}

func TestHandleRandomSort_PaginationNoOverlap(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSceneRepo := mocks.NewMockSceneRepository(ctrl)

	service := &SearchService{
		sceneRepo: mockSceneRepo,
		logger:    zap.NewNop(),
	}

	allIDs := make([]uint, 20)
	for i := range allIDs {
		allIDs[i] = uint(i + 1)
	}

	mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).DoAndReturn(func(ids []uint) ([]data.Scene, error) {
		scenes := make([]data.Scene, len(ids))
		for i, id := range ids {
			scenes[i] = data.Scene{}
			scenes[i].ID = id
		}
		return scenes, nil
	}).Times(2)

	seed := int64(12345)

	// Page 1
	ids1 := make([]uint, len(allIDs))
	copy(ids1, allIDs)
	result1, err := service.handleRandomSort(ids1, data.SceneSearchParams{Page: 1, Limit: 10, Seed: seed})
	if err != nil {
		t.Fatalf("page 1 error: %v", err)
	}

	// Page 2
	ids2 := make([]uint, len(allIDs))
	copy(ids2, allIDs)
	result2, err := service.handleRandomSort(ids2, data.SceneSearchParams{Page: 2, Limit: 10, Seed: seed})
	if err != nil {
		t.Fatalf("page 2 error: %v", err)
	}

	if len(result1.Scenes) != 10 || len(result2.Scenes) != 10 {
		t.Fatalf("expected 10 scenes per page, got %d and %d", len(result1.Scenes), len(result2.Scenes))
	}

	// Verify no overlap
	page1IDs := make(map[uint]bool)
	for _, s := range result1.Scenes {
		page1IDs[s.ID] = true
	}
	for _, s := range result2.Scenes {
		if page1IDs[s.ID] {
			t.Errorf("scene ID %d appears on both page 1 and page 2", s.ID)
		}
	}

	// Total should be 20 for both
	if result1.Total != 20 || result2.Total != 20 {
		t.Errorf("expected total=20, got %d and %d", result1.Total, result2.Total)
	}
}

func TestHandleRandomSort_EmptyIDs(t *testing.T) {
	service := &SearchService{
		logger: zap.NewNop(),
	}

	result, err := service.handleRandomSort([]uint{}, data.SceneSearchParams{Page: 1, Limit: 10, Seed: 42})
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}
	if len(result.Scenes) != 0 {
		t.Errorf("expected 0 scenes, got %d", len(result.Scenes))
	}
	if result.Total != 0 {
		t.Errorf("expected total=0, got %d", result.Total)
	}
}

func TestHandleRandomSort_PageBeyondTotal(t *testing.T) {
	service := &SearchService{
		logger: zap.NewNop(),
	}

	allIDs := []uint{1, 2, 3, 4, 5}
	result, err := service.handleRandomSort(allIDs, data.SceneSearchParams{Page: 10, Limit: 10, Seed: 42})
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}
	if len(result.Scenes) != 0 {
		t.Errorf("expected 0 scenes for out-of-bounds page, got %d", len(result.Scenes))
	}
	if result.Total != 5 {
		t.Errorf("expected total=5, got %d", result.Total)
	}
}

func TestHandleRandomSort_AutoGenerateSeed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSceneRepo := mocks.NewMockSceneRepository(ctrl)

	service := &SearchService{
		sceneRepo: mockSceneRepo,
		logger:    zap.NewNop(),
	}

	mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).DoAndReturn(func(ids []uint) ([]data.Scene, error) {
		scenes := make([]data.Scene, len(ids))
		for i, id := range ids {
			scenes[i] = data.Scene{}
			scenes[i].ID = id
		}
		return scenes, nil
	}).Times(1)

	allIDs := []uint{1, 2, 3, 4, 5}
	result, err := service.handleRandomSort(allIDs, data.SceneSearchParams{Page: 1, Limit: 10, Seed: 0})
	if err != nil {
		t.Fatalf("handleRandomSort() error: %v", err)
	}
	if result.Seed == 0 {
		t.Error("expected auto-generated seed to be non-zero")
	}
	// Must be within JavaScript Number.MAX_SAFE_INTEGER to avoid precision loss in JSON
	const maxSafeSeed int64 = 9007199254740991
	if result.Seed > maxSafeSeed {
		t.Errorf("auto-generated seed %d exceeds JS Number.MAX_SAFE_INTEGER (%d)", result.Seed, maxSafeSeed)
	}
}

func boolPtr(b bool) *bool {
	return &b
}
