package core

import (
	"context"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

func ptrFloat(v float64) *float64 { return &v }

func TestBuildRatingHistogram(t *testing.T) {
	empty := BuildRatingHistogram(&data.ProfileRatingSummary{Buckets: []data.ProfileRatingBucket{}})
	if len(empty.Buckets) != 10 || empty.Total != 0 || empty.Average != nil {
		t.Fatalf("empty histogram: %+v", empty)
	}
	for i, b := range empty.Buckets {
		if want := 0.5 + float64(i)*0.5; b.Rating != want || b.Count != 0 {
			t.Fatalf("bucket %d = %+v, want rating %v and no count", i, b, want)
		}
	}

	h := BuildRatingHistogram(&data.ProfileRatingSummary{
		Buckets: []data.ProfileRatingBucket{{Rating: 0.5, Count: 1}, {Rating: 3.5, Count: 4}, {Rating: 3.6, Count: 1}, {Rating: 5, Count: 2}},
		Average: ptrFloat(3.456789),
	})
	if h.Total != 8 {
		t.Fatalf("total = %d", h.Total)
	}
	counts := map[float64]int64{}
	for _, b := range h.Buckets {
		counts[b.Rating] = b.Count
	}
	// 3.6 (an imported rating) falls into the nearest bar, 3.5; the empty bars stay
	if counts[0.5] != 1 || counts[3.5] != 5 || counts[5] != 2 || counts[1] != 0 || counts[4.5] != 0 {
		t.Fatalf("counts: %v", counts)
	}
	if h.Average == nil || *h.Average != 3.46 {
		t.Fatalf("average: %v", h.Average)
	}
	if nilSummary := BuildRatingHistogram(nil); len(nilSummary.Buckets) != 10 || nilSummary.Total != 0 {
		t.Fatalf("nil summary: %+v", nilSummary)
	}
}

func TestValidProfileRating(t *testing.T) {
	cases := map[float64]bool{0: false, 0.5: true, 1: true, 3.5: true, 3.3: false, 5: true, 5.5: false, -1: false}
	for v, want := range cases {
		if got := ValidProfileRating(v); got != want {
			t.Fatalf("ValidProfileRating(%v) = %v", v, got)
		}
	}
}

var viewerWithScenes = ProfileViewer{UserID: 1, Access: NewUserAccess(1, "user", false, PermScenesView)}

// bob (5): ratings public, jizz left to its default (friends), liked private.
var bobListSettings = []data.UserPrivacySetting{
	{UserID: 5, Item: PrivacyRatings, Audience: "public"},
	{UserID: 5, Item: PrivacyLiked, Audience: "private"},
}

func TestProfileListsPrivacyPerItem(t *testing.T) {
	ctx := context.Background()
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob(bobListSettings)

	// liked is private, jizz is for friends: nothing is read (no repo call expected)
	for _, list := range []ProfileList{ProfileListLiked, ProfileListJizz} {
		if _, err := e.svc.List(ctx, viewerWithScenes, "bob", list, ProfileListInput{Page: 1, Limit: 10}); !apperrors.IsPrivacyRestricted(err) {
			t.Fatalf("%s: err = %v, want privacy restricted", list, err)
		}
	}
	// A superuser is a viewer like any other
	admin := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "admin", true)}
	if _, err := e.svc.List(ctx, admin, "bob", ProfileListLiked, ProfileListInput{Page: 1, Limit: 10}); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("superuser bypassed privacy: %v", err)
	}

	// ratings are public: the histogram and the list are read
	e.repo.EXPECT().RatingSummary(gomock.Any(), uint(5)).Return(&data.ProfileRatingSummary{Buckets: []data.ProfileRatingBucket{{Rating: 4, Count: 2}}, Average: ptrFloat(4)}, nil)
	h, err := e.svc.RatingHistogram(ctx, viewerWithScenes, "bob")
	if err != nil || h.Total != 2 || len(h.Buckets) != 10 {
		t.Fatalf("histogram: %+v, %v", h, err)
	}
}

func TestProfileListsHiddenProfile(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	// ratings public, but the whole profile private: the item alone is not enough
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyRatings, Audience: "public"}, {UserID: 5, Item: PrivacyProfile, Audience: "private"}})
	if _, err := e.svc.RatingHistogram(context.Background(), viewerWithScenes, "bob"); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestProfileListsNeedScenesView(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	guest := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "guest", false)}
	if _, err := e.svc.List(context.Background(), guest, "bob", ProfileListRated, ProfileListInput{Page: 1, Limit: 10}); !apperrors.IsForbidden(err) {
		t.Fatalf("err = %v, want forbidden", err)
	}
	if _, err := e.svc.RatingHistogram(context.Background(), guest, "bob"); !apperrors.IsForbidden(err) {
		t.Fatalf("histogram err = %v, want forbidden", err)
	}
}

func TestProfileJizzListForFriends(t *testing.T) {
	ctx := context.Background()
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.social.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob"}, nil)
	e.social.EXPECT().Relations(gomock.Any(), uint(1), []uint{5}).Return([]data.UserRelation{{UserID: 5, Friend: true}}, nil)
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(5)).Return(nil, nil)
	e.repo.EXPECT().JizzScenes(gomock.Any(), uint(5), data.ProfileListQuery{Sort: data.ProfileSortJizzCount, Page: 2, Limit: 3}).
		Return([]data.ProfileSceneRef{{SceneID: 10, Count: 3}, {SceneID: 11, Count: 1}}, int64(5), nil)
	// scene 11 was trashed since: ScenesByIDs only returns the live ones
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{10, 11}).Return([]data.Scene{{ID: 10}}, nil)

	page, err := e.svc.List(ctx, viewerWithScenes, "bob", ProfileListJizz, ProfileListInput{Sort: data.ProfileSortJizzCount, Page: 2, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 || page.Sort != data.ProfileSortJizzCount || len(page.Items) != 1 {
		t.Fatalf("page: %+v", page)
	}
	if it := page.Items[0]; it.JizzCount != 3 || it.Views != 0 {
		t.Fatalf("item: %+v", it)
	}
}

func TestProfileRatedListQuery(t *testing.T) {
	ctx := context.Background()
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob(bobListSettings)

	bad := []ProfileListInput{
		{Sort: "size_desc", Page: 1, Limit: 10},
		{Rating: ptrFloat(3.3), Page: 1, Limit: 10},
		{Rating: ptrFloat(6), Page: 1, Limit: 10},
	}
	for _, in := range bad {
		if _, err := e.svc.List(ctx, viewerWithScenes, "bob", ProfileListRated, in); !apperrors.IsValidation(err) {
			t.Fatalf("%+v: err = %v, want validation", in, err)
		}
	}
	// A rating filter on another list is refused before anything is read
	if _, err := e.svc.List(ctx, viewerWithScenes, "bob", ProfileListLiked, ProfileListInput{Rating: ptrFloat(4), Page: 1, Limit: 10}); !apperrors.IsValidation(err) {
		t.Fatalf("rating on likes: %v", err)
	}

	e.repo.EXPECT().RatedScenes(gomock.Any(), uint(5), data.ProfileListQuery{Sort: data.ProfileSortRatingDesc, Rating: ptrFloat(4.5), Page: 1, Limit: 10}).
		Return([]data.ProfileSceneRef{{SceneID: 10, Rating: ptrFloat(4.5)}}, int64(1), nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{10}).Return([]data.Scene{{ID: 10}}, nil)
	page, err := e.svc.List(ctx, viewerWithScenes, "bob", ProfileListRated, ProfileListInput{Rating: ptrFloat(4.5), Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Sort != data.ProfileSortRatingDesc || len(page.Items) != 1 || *page.Items[0].Rating != 4.5 {
		t.Fatalf("page: %+v", page)
	}
}

func TestProfileGetListsVisibility(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob(bobListSettings)
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Layout: data.ProfileLayout{{Type: "ratings", Visible: true}, {Type: "jizz", Visible: true, LockedPlaceholder: true}}}, nil)
	e.repo.EXPECT().RatingSummary(gomock.Any(), uint(5)).Return(&data.ProfileRatingSummary{}, nil)
	view, err := e.svc.Get(context.Background(), viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if view.Lists != (ProfileListsVisibility{Ratings: true}) {
		t.Fatalf("lists: %+v", view.Lists)
	}
	// ratings shown (empty), jizz (friends) locked without data
	if got := strings.Join(blockTypes(view), ","); got != "ratings,jizz:locked" {
		t.Fatalf("blocks = %s", got)
	}
	if !view.Blocks[0].Data.Empty || view.Blocks[1].Data != nil {
		t.Fatalf("blocks: %+v", view.Blocks)
	}

	// Without scenes:view no list is open, whatever the privacy
	e2 := newProfileTestEnv(t, config.ProfilesConfig{})
	e2.expectBob(bobListSettings)
	e2.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Layout: data.ProfileLayout{}}, nil)
	view, err = e2.svc.Get(context.Background(), ProfileViewer{UserID: 1, Access: NewUserAccess(1, "guest", false)}, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if view.Lists != (ProfileListsVisibility{}) {
		t.Fatalf("lists without scenes:view: %+v", view.Lists)
	}
}

func TestJizzBlock(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.repo.EXPECT().JizzSummary(gomock.Any(), uint(5)).Return(&data.ProfileJizzSummary{Total: 9, Scenes: 2}, nil)
	e.repo.EXPECT().JizzScenes(gomock.Any(), uint(5), data.ProfileListQuery{Sort: data.ProfileSortJizzRecent, Page: 1, Limit: 6}).
		Return([]data.ProfileSceneRef{{SceneID: 10, Count: 7}, {SceneID: 11, Count: 2}}, int64(2), nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), []uint{10, 11}).Return([]data.Scene{{ID: 10}, {ID: 11}}, nil)
	spec, _ := ProfileBlockSpecFor("jizz")
	out, err := spec.Fetch(context.Background(), &e.svc.deps, ProfileBlockRequest{OwnerID: 5, Config: spec.resolve(nil)})
	if err != nil {
		t.Fatal(err)
	}
	payload := out.Widget.(ProfileJizzPayload)
	if payload.Total != 9 || *out.Total != 2 || out.Empty || out.Scenes[0].JizzCount != 7 {
		t.Fatalf("jizz block: %+v %+v", out, payload)
	}
}
