package core

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
)

// fakeFigureReader serves fixed interactions of the owner and records which kinds were read.
type fakeFigureReader struct {
	reads []string
}

var figureAt = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

func (f *fakeFigureReader) GetJizzCountsBySceneIDs(_ uint, _ []uint) (map[uint]int, error) {
	f.reads = append(f.reads, "jizz_counts")
	return map[uint]int{10: 12}, nil
}

func (f *fakeFigureReader) GetLastJizzTimes(_ uint, _ []uint) (map[uint]time.Time, error) {
	f.reads = append(f.reads, "last_jizz")
	return map[uint]time.Time{10: figureAt}, nil
}

func (f *fakeFigureReader) GetRatingsBySceneIDs(_ uint, _ []uint) (map[uint]float64, error) {
	f.reads = append(f.reads, "ratings")
	return map[uint]float64{10: 4.5}, nil
}

func (f *fakeFigureReader) GetLikeTimes(_ uint, _ []uint) (map[uint]time.Time, error) {
	f.reads = append(f.reads, "likes")
	return map[uint]time.Time{10: figureAt}, nil
}

func (f *fakeFigureReader) GetWatchStats(_ uint, _ []uint) (map[uint]data.SceneWatchStat, error) {
	f.reads = append(f.reads, "views")
	return map[uint]data.SceneWatchStat{10: {SceneID: 10, Count: 34, Last: figureAt}}, nil
}

func TestAutoListFigures(t *testing.T) {
	cases := []struct {
		name string
		f    data.Filters
		want string
	}{
		{"top O", data.Filters{Sort: sortJizzCountDesc}, "jizz"},
		{"least O", data.Filters{Sort: sortJizzCountAsc}, "jizz"},
		{"last O", data.Filters{Sort: sortLastJizzDesc}, "jizz"},
		{"top rated", data.Filters{Sort: SortMyRatingDesc}, "rating"},
		{"recently liked", data.Filters{Liked: ptrBool(true), Sort: SortLikedAtDesc}, "liked"},
		{"most watched", data.Filters{Sort: SortMyViewsDesc}, "views"},
		{"last watched", data.Filters{Sort: SortLastWatchedDesc}, "views"},
		{"newest", data.Filters{Sort: "created_at_desc"}, ""},
		{"tags only", data.Filters{SelectedTags: []string{"x"}}, ""},
		{"rating filter, title sort", data.Filters{MinRating: ptrFloat(4), Sort: "title_asc"}, "rating"},
		{"rated only", data.Filters{Rated: ptrBool(true)}, "rating"},
		{"unrated, unwatched: nothing to show", data.Filters{Rated: ptrBool(false), Watched: ptrBool(false)}, ""},
		{"watched filter", data.Filters{Watched: ptrBool(true)}, "views"},
		{"top O with an O range: once", data.Filters{Sort: sortJizzCountDesc, MinJizzCount: ptrInt(2)}, "jizz"},
		{"top O, liked: the sort first", data.Filters{Sort: sortJizzCountDesc, Liked: ptrBool(true)}, "jizz,liked"},
		{"capped at two", data.Filters{Sort: SortMyViewsDesc, Liked: ptrBool(true), MinRating: ptrFloat(3), MinJizzCount: ptrInt(1)}, "views,jizz"},
		{"filters only, catalog order", data.Filters{Liked: ptrBool(true), MinRating: ptrFloat(3)}, "rating,liked"},
	}
	for _, tc := range cases {
		if got := strings.Join(AutoListFigures(tc.f), ","); got != tc.want {
			t.Fatalf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestListFiguresSetting(t *testing.T) {
	topO := data.Filters{Sort: sortJizzCountDesc}
	cases := map[string]string{"": "jizz", "auto": "jizz", "none": "", "rating": "rating", "views": "views", "bogus": ""}
	for setting, want := range cases {
		if got := strings.Join(ListFigures(setting, topO), ","); got != want {
			t.Fatalf("%q: %q, want %q", setting, got, want)
		}
	}
	for _, v := range []string{"auto", "none", "jizz", "rating", "liked", "views"} {
		if !ValidListFiguresSetting(v) {
			t.Fatalf("%q refused", v)
		}
	}
	for _, v := range []string{"", "list", "at", "JIZZ"} {
		if ValidListFiguresSetting(v) {
			t.Fatalf("%q accepted as a list setting", v)
		}
	}
}

func TestEffectiveListFigures(t *testing.T) {
	rl := ResolvedProfileList{List: data.ProfileList{Figures: "auto"}, Filters: data.Filters{Sort: sortJizzCountDesc, Liked: ptrBool(true)}}
	all := map[string]bool{PrivacyJizz: true, PrivacyRatings: true, PrivacyLiked: true, PrivacyActivity: true}
	check := func(name string, got []string, want string) {
		t.Helper()
		if g := strings.Join(got, ","); g != want {
			t.Fatalf("%s: %q, want %q", name, g, want)
		}
	}
	check("the list setting", effectiveListFigures(rl, "", all), "jizz,liked")
	check("block: as the list", effectiveListFigures(rl, ListFiguresList, all), "jizz,liked")
	check("block override", effectiveListFigures(rl, ListFigureRating, all), "rating")
	check("block none", effectiveListFigures(rl, ListFiguresNone, all), "")
	// A figure whose item the viewer may not see is omitted, the others stay
	check("liked hidden", effectiveListFigures(rl, "", map[string]bool{PrivacyJizz: true}), "jizz")
	// Ratings shown on a Top O list: the ratings item is checked on its own
	rl.List.Figures = ListFigureRating
	check("rating hidden", effectiveListFigures(rl, "", map[string]bool{PrivacyJizz: true, PrivacyLiked: true}), "")
	rl.List.Figures = ListFiguresNone
	check("list none", effectiveListFigures(rl, "", all), "")
	check("block auto over a list none", effectiveListFigures(rl, ListFiguresAuto, all), "jizz,liked")
}

func TestFillListFigures(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	reader := &fakeFigureReader{}
	e.svc.WithListFigures(reader)
	items := []ProfileSceneItem{{Scene: data.Scene{ID: 10}}, {Scene: data.Scene{ID: 11}}}
	if err := e.svc.fillListFigures(context.Background(), 5, items, []string{ListFigureJizz, ListFigureViews}); err != nil {
		t.Fatal(err)
	}
	f := items[0].Figures
	if f == nil || f.JizzCount != 12 || f.LastJizzAt == nil || !f.LastJizzAt.Equal(figureAt) || f.Views != 34 || f.LastWatchedAt == nil {
		t.Fatalf("scene 10: %+v", f)
	}
	// Only the kinds asked for are read
	if f.Rating != nil || f.LikedAt != nil {
		t.Fatalf("unasked figures: %+v", f)
	}
	sort.Strings(reader.reads)
	if got := strings.Join(reader.reads, ","); got != "jizz_counts,last_jizz,views" {
		t.Fatalf("reads: %s", got)
	}
	// A scene without the interaction has empty figures
	if g := items[1].Figures; g == nil || g.JizzCount != 0 || g.LastJizzAt != nil || g.Views != 0 {
		t.Fatalf("scene 11: %+v", g)
	}
	// No kind: nothing read
	reader.reads = nil
	if err := e.svc.fillListFigures(context.Background(), 5, items, nil); err != nil || len(reader.reads) != 0 {
		t.Fatalf("no kind: %v %v", err, reader.reads)
	}
}

// customListBlockOf returns the custom_list block of a profile view.
func customListBlockOf(t *testing.T, v *ProfileView) ProfileBlockView {
	t.Helper()
	for _, b := range v.Blocks {
		if b.Spec.Type == customListBlockType {
			return b
		}
	}
	t.Fatal("no custom list block")
	return ProfileBlockView{}
}

func TestCustomListFiguresPrivacy(t *testing.T) {
	ctx := context.Background()
	// bob's Top O list, jizz public: a stranger gets the O figures of the block and the page
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyJizz, Audience: "public"}})
	e.bobWithList("public")
	reader := &fakeFigureReader{}
	e.svc.WithListFigures(reader)
	view, err := e.svc.Get(ctx, viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	b := customListBlockOf(t, view)
	payload, _ := b.Data.Widget.(ProfileCustomListPayload)
	if strings.Join(payload.Figures, ",") != "jizz" || b.Data.Scenes[0].Figures == nil || b.Data.Scenes[0].Figures.JizzCount != 12 {
		t.Fatalf("block figures: %+v %+v", payload, b.Data.Scenes[0].Figures)
	}
	page, err := e.svc.BrowseList(ctx, viewerWithScenes, "bob", 7, 1, 24)
	if err != nil || strings.Join(page.Figures, ",") != "jizz" || page.Items[0].Figures.JizzCount != 12 {
		t.Fatalf("browse figures: %+v %v", page, err)
	}

	// The list set to show the rating: ratings (friends by default) hidden from the stranger,
	// the figure is omitted (never read) while the list stays visible
	e = newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyJizz, Audience: "public"}})
	e.bobWithListFigures("public", ListFigureRating)
	reader = &fakeFigureReader{}
	e.svc.WithListFigures(reader)
	page, err = e.svc.BrowseList(ctx, viewerWithScenes, "bob", 7, 1, 24)
	if err != nil || len(page.Figures) != 0 || page.Items[0].Figures != nil || len(reader.reads) != 0 {
		t.Fatalf("hidden rating: %+v %v, reads %v", page, err, reader.reads)
	}
	view, err = e.svc.Get(ctx, viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if b := customListBlockOf(t, view); b.Locked || b.Data == nil || b.Data.Scenes[0].Figures != nil || len(reader.reads) != 0 {
		t.Fatalf("hidden rating in the block: %+v, reads %v", b, reader.reads)
	}
	// A superuser is a viewer like any other
	admin := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "admin", true)}
	if page, err = e.svc.BrowseList(ctx, admin, "bob", 7, 1, 24); err != nil || len(page.Figures) != 0 {
		t.Fatalf("superuser bypass: %+v %v", page, err)
	}
	// The owner sees the figure
	bob := ProfileViewer{UserID: 5, Access: NewUserAccess(5, "user", false, PermScenesView)}
	if page, err = e.svc.BrowseList(ctx, bob, "bob", 7, 1, 24); err != nil || strings.Join(page.Figures, ",") != "rating" ||
		page.Items[0].Figures == nil || *page.Items[0].Figures.Rating != 4.5 {
		t.Fatalf("owner: %+v %v", page, err)
	}

	// Ratings made public: the stranger gets it too
	e = newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyJizz, Audience: "public"}, {UserID: 5, Item: PrivacyRatings, Audience: "public"}})
	e.bobWithListFigures("public", ListFigureRating)
	e.svc.WithListFigures(&fakeFigureReader{})
	if page, err = e.svc.BrowseList(ctx, viewerWithScenes, "bob", 7, 1, 24); err != nil || strings.Join(page.Figures, ",") != "rating" {
		t.Fatalf("public rating: %+v %v", page, err)
	}
}

func TestCustomListBlockFiguresOverride(t *testing.T) {
	ctx := context.Background()
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyJizz, Audience: "public"}, {UserID: 5, Item: PrivacyActivity, Audience: "public"}})
	e.bobWithListBlock("public", "", map[string]any{"list_id": 7.0, "figures": "views"})
	reader := &fakeFigureReader{}
	e.svc.WithListFigures(reader)
	view, err := e.svc.Get(ctx, viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	b := customListBlockOf(t, view)
	payload, _ := b.Data.Widget.(ProfileCustomListPayload)
	if strings.Join(payload.Figures, ",") != "views" || b.Data.Scenes[0].Figures.Views != 34 || b.Data.Scenes[0].Figures.JizzCount != 0 {
		t.Fatalf("override: %+v %+v", payload, b.Data.Scenes[0].Figures)
	}
	if b.Config.String("figures") != "views" {
		t.Fatalf("config: %+v", b.Config)
	}
}

func TestCustomListFiguresValidation(t *testing.T) {
	owner := viewerWithScenes
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	list := data.ProfileList{}
	if err := e.svc.checkListInput(owner, &CustomListInput{Title: "x", Figures: "list"}, &list); !apperrors.IsValidation(err) {
		t.Fatalf("figures list: %v", err)
	}
	if err := e.svc.checkListInput(owner, &CustomListInput{Title: "x"}, &list); err != nil || list.Figures != ListFiguresAuto {
		t.Fatalf("default: %q %v", list.Figures, err)
	}
	if err := e.svc.checkListInput(owner, &CustomListInput{Title: "x", Figures: "none"}, &list); err != nil || list.Figures != ListFiguresNone {
		t.Fatalf("none: %q %v", list.Figures, err)
	}
}
