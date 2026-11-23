package handler

import (
	"net/http/httptest"
	"testing"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func historyCardsContext(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/history/by-date"+query, nil)
	return c
}

func historyEntries() []core.WatchHistoryEntry {
	scene := &data.Scene{ID: 7, Title: "Seven", Duration: 600}
	return []core.WatchHistoryEntry{
		{Watch: data.UserSceneWatch{ID: 1, SceneID: 7}, Scene: scene},
		{Watch: data.UserSceneWatch{ID: 2, SceneID: 7}, Scene: scene}, // watched twice
		{Watch: data.UserSceneWatch{ID: 3, SceneID: 9}},               // scene gone
	}
}

// Without card_fields the response stays the previous one (no sidecar maps).
func TestHistoryEntriesResponse_WithoutCardFields(t *testing.T) {
	h := NewWatchHistoryHandler(nil, nil)
	resp := h.historyEntriesResponse(historyCardsContext(""), historyEntries(), 1)
	entries, ok := resp["entries"].([]response.WatchHistoryEntryResponse)
	if !ok || len(entries) != 3 {
		t.Fatalf("entries = %#v", resp["entries"])
	}
	if _, has := resp["ratings"]; has {
		t.Fatal("ratings sent without card_fields")
	}
}

// With the user's card fields the history scenes carry the card data (tags here) and the
// user's ratings come next to the entries, like on the scene lists.
func TestHistoryEntriesResponse_FollowsTheCardTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	tags := mocks.NewMockTagRepository(ctrl)
	interactions := mocks.NewMockInteractionRepository(ctrl)
	tags.EXPECT().GetSceneTagsMultiple([]uint{7}).Return(map[uint][]data.Tag{7: {{Name: "solo"}, {Name: "outdoor"}}}, nil)
	interactions.EXPECT().GetRatingsBySceneIDs(uint(1), []uint{7}).Return(map[uint]float64{7: 4.5}, nil)

	h := NewWatchHistoryHandler(nil, nil).WithCardData(nil, interactions, tags, nil)
	resp := h.historyEntriesResponse(historyCardsContext("?card_fields=tags,rating"), historyEntries(), 1)

	entries := resp["entries"].([]response.WatchHistoryEntryResponse)
	if len(entries) != 3 {
		t.Fatalf("%d entries, want 3", len(entries))
	}
	for _, i := range []int{0, 1} {
		if entries[i].Scene == nil || len(entries[i].Scene.Tags) != 2 || entries[i].Scene.Tags[0] != "solo" {
			t.Fatalf("entry %d scene = %#v, want the card tags", i, entries[i].Scene)
		}
	}
	if entries[2].Scene != nil {
		t.Fatalf("entry without scene got %#v", entries[2].Scene)
	}
	ratings, ok := resp["ratings"].(map[uint]float64)
	if !ok || ratings[7] != 4.5 {
		t.Fatalf("ratings = %#v", resp["ratings"])
	}
}
