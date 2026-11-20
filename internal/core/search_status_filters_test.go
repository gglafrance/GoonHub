package core

import (
	"reflect"
	"sort"
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newStatusFilterSearchService(t *testing.T) (*SearchService, *mocks.MockInteractionRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	interactions := mocks.NewMockInteractionRepository(ctrl)
	return NewSearchService(nil, nil, interactions, nil, nil, nil, zap.NewNop()), interactions
}

func TestUserStatusFilters_Includes(t *testing.T) {
	svc, interactions := newStatusFilterSearchService(t)
	yes := true
	interactions.EXPECT().GetLikedSceneIDs(uint(1)).Return([]uint{1, 2, 3, 4}, nil)
	interactions.EXPECT().GetWatchedSceneIDs(uint(1)).Return([]uint{2, 3, 9}, nil)
	interactions.EXPECT().GetMarkedSceneIDs(uint(1)).Return([]uint{3, 2}, nil)

	params := data.SceneSearchParams{UserID: 1, Liked: &yes, Watched: &yes, HasMarkers: &yes}
	if !svc.hasUserFilters(params) {
		t.Fatal("status filters must count as user filters")
	}
	ids, err := svc.getUserFilteredIDs(params)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if !reflect.DeepEqual(ids, []uint{2, 3}) {
		t.Fatalf("liked AND watched AND with markers = %v, want [2 3]", ids)
	}
}

func TestUserStatusFilters_Excludes(t *testing.T) {
	svc, interactions := newStatusFilterSearchService(t)
	no := false
	interactions.EXPECT().GetWatchedSceneIDs(uint(1)).Return([]uint{1, 2}, nil)
	interactions.EXPECT().GetRatedSceneIDs(uint(1), 0.0, 0.0).Return([]uint{2, 3}, nil)

	params := data.SceneSearchParams{UserID: 1, Watched: &no, Rated: &no}
	if svc.hasUserFilters(params) {
		t.Fatal("negative status filters must not restrict to an inclusion list")
	}
	ids, err := svc.getUserExcludedIDs(params)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if !reflect.DeepEqual(ids, []uint{1, 2, 3}) {
		t.Fatalf("excluded = %v, want [1 2 3]", ids)
	}
}

func TestUserStatusFilters_AnonymousExcludesNothing(t *testing.T) {
	svc, _ := newStatusFilterSearchService(t)
	no := false
	ids, err := svc.getUserExcludedIDs(data.SceneSearchParams{Watched: &no})
	if err != nil || len(ids) != 0 {
		t.Fatalf("expected no exclusion without user, got %v, %v", ids, err)
	}
}
