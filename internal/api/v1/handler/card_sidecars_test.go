package handler

import (
	"testing"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
)

func TestParseCardFields_Watched(t *testing.T) {
	f := response.ParseCardFields("rating,liked,jizz_count,watched")
	if !f.Rating || !f.Liked || !f.JizzCount || !f.Watched {
		t.Fatalf("expected the four interaction fields, got %+v", f)
	}
	with := response.CardFields{Watched: true}.With(response.SceneModelFields)
	if !with.Watched || !with.Views || !with.Resolution || !with.Studio || !with.ReleaseDate || with.Tags {
		t.Fatalf("With: got %+v", with)
	}
}

func TestLoadCardSidecars_OnlyRequestedMaps(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInteractionRepository(ctrl)
	ids := []uint{1, 2}

	// only the asked-for maps are loaded
	repo.EXPECT().GetLikesBySceneIDs(uint(7), ids).Return(map[uint]bool{1: true}, nil)
	repo.EXPECT().GetWatchedBySceneIDs(uint(7), ids).Return(map[uint]bool{2: true}, nil)
	s := loadCardSidecars(repo, 7, ids, response.CardFields{Liked: true, Watched: true})
	if !s.Likes[1] || !s.Watched[2] || s.Ratings != nil || s.JizzCounts != nil {
		t.Fatalf("got %+v", s)
	}

	resp := map[string]any{}
	addCardSidecars(resp, s)
	if _, ok := resp["watched"]; !ok {
		t.Fatalf("watched missing from the response: %v", resp)
	}
	if _, ok := resp["ratings"]; ok {
		t.Fatalf("ratings not asked for: %v", resp)
	}
}

func TestLoadCardSidecars_Anonymous(t *testing.T) {
	// no expectation: nothing is loaded without a user
	repo := mocks.NewMockInteractionRepository(gomock.NewController(t))
	s := loadCardSidecars(repo, 0, []uint{1}, response.CardFields{Rating: true, Watched: true})
	if s.Ratings != nil || s.Watched != nil {
		t.Fatalf("got %+v", s)
	}
}
