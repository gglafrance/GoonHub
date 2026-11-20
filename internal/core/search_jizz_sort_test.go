package core

import (
	"slices"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestHandleJizzSort(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	last := map[uint]time.Time{
		3: base.Add(3 * time.Hour),
		5: base.Add(1 * time.Hour),
		7: base.Add(2 * time.Hour),
	}
	// Scene 1 has an o count without a dated o (imported count), scene 9 has none
	counts := map[uint]int{3: 2, 5: 4, 7: 2, 1: 1}
	tests := []struct {
		name  string
		sort  string
		page  int
		limit int
		want  []uint
	}{
		// Scenes 1 and 9 have no dated o: always last, by ID
		{"last desc", "last_jizz_desc", 1, 5, []uint{3, 7, 5, 1, 9}},
		{"last asc", "last_jizz_asc", 1, 5, []uint{5, 7, 3, 1, 9}},
		{"last second page", "last_jizz_desc", 2, 3, []uint{1, 9}},
		// Ties (3 and 7) by ID; scene 9 counts as 0
		{"count desc", "jizz_count_desc", 1, 5, []uint{5, 3, 7, 1, 9}},
		{"count asc", "jizz_count_asc", 1, 5, []uint{9, 1, 3, 7, 5}},
		{"count asc second page", "jizz_count_asc", 2, 2, []uint{3, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			interactions := mocks.NewMockInteractionRepository(ctrl)
			scenes := mocks.NewMockSceneRepository(ctrl)
			svc := NewSearchService(nil, scenes, interactions, nil, nil, nil, zap.NewNop())

			ids := []uint{9, 5, 1, 3, 7}
			if tt.sort == "jizz_count_desc" || tt.sort == "jizz_count_asc" {
				interactions.EXPECT().GetJizzCountsBySceneIDs(uint(4), ids).Return(counts, nil)
			} else {
				interactions.EXPECT().GetLastJizzTimes(uint(4), ids).Return(last, nil)
			}
			var got []uint
			scenes.EXPECT().GetByIDs(gomock.Any()).DoAndReturn(func(page []uint) ([]data.Scene, error) {
				got = append([]uint(nil), page...)
				out := make([]data.Scene, len(page))
				for i, id := range page {
					out[i] = data.Scene{ID: id}
				}
				return out, nil
			})

			result, err := svc.handleJizzSort(ids, data.SceneSearchParams{UserID: 4, Sort: tt.sort, Page: tt.page, Limit: tt.limit})
			if err != nil {
				t.Fatal(err)
			}
			if result.Total != 5 {
				t.Fatalf("total = %d, want 5", result.Total)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsJizzSortOrder(t *testing.T) {
	for _, s := range []string{"last_jizz_desc", "last_jizz_asc", "jizz_count_desc", "jizz_count_asc"} {
		if !isJizzSortOrder(s) {
			t.Fatalf("%s must be an o sort", s)
		}
	}
	for _, s := range []string{"random", "view_count_desc", ""} {
		if isJizzSortOrder(s) {
			t.Fatalf("%s must not be an o sort", s)
		}
	}
}
