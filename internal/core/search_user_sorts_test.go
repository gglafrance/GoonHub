package core

import (
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestOrderByKeys(t *testing.T) {
	keys := map[uint]float64{3: 1, 5: 4, 7: 4}
	if got := orderByKeys([]uint{9, 7, 5, 3, 1}, keys, true); !sameIDs(got, 5, 7, 3, 1, 9) {
		t.Fatalf("desc: %v", got)
	}
	// ascending, the scenes without a key still last
	if got := orderByKeys([]uint{9, 7, 5, 3, 1}, keys, false); !sameIDs(got, 3, 5, 7, 1, 9) {
		t.Fatalf("asc: %v", got)
	}
}

func sameIDs(got []uint, want ...uint) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestUserSortKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInteractionRepository(ctrl)
	s := NewSearchService(nil, nil, repo, nil, nil, nil, zap.NewNop())
	ids := []uint{1, 2, 3}
	repo.EXPECT().GetRatingsBySceneIDs(uint(5), ids).Return(map[uint]float64{2: 4.5}, nil)
	keys, err := s.userSortKeys(data.SceneSearchParams{UserID: 5, Sort: SortMyRatingAsc}, ids)
	if err != nil || keys[2] != 4.5 || len(keys) != 1 {
		t.Fatalf("ratings: %v %v", keys, err)
	}
	liked := time.Unix(100, 0)
	repo.EXPECT().GetLikeTimes(uint(5), ids).Return(map[uint]time.Time{3: liked}, nil)
	if keys, _ = s.userSortKeys(data.SceneSearchParams{UserID: 5, Sort: SortLikedAtDesc}, ids); keys[3] != float64(liked.UnixNano()) {
		t.Fatalf("likes: %v", keys)
	}
	repo.EXPECT().GetWatchStats(uint(5), ids).Return(map[uint]data.SceneWatchStat{1: {SceneID: 1, Count: 7, Last: liked}}, nil).Times(2)
	if keys, _ = s.userSortKeys(data.SceneSearchParams{UserID: 5, Sort: SortMyViewsDesc}, ids); keys[1] != 7 {
		t.Fatalf("views: %v", keys)
	}
	if keys, _ = s.userSortKeys(data.SceneSearchParams{UserID: 5, Sort: SortLastWatchedDesc}, ids); keys[1] != float64(liked.UnixNano()) {
		t.Fatalf("last watched: %v", keys)
	}
	// No user: no key (the sort falls back to the scene ids)
	if keys, _ = s.userSortKeys(data.SceneSearchParams{Sort: SortMyViewsDesc}, ids); len(keys) != 0 {
		t.Fatalf("no user: %v", keys)
	}
}
