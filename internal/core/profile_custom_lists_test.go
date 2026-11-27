package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func ptrBool(v bool) *bool { return &v }
func ptrInt(v int) *int    { return &v }

// fakeSearcher records the searches of the custom lists.
type fakeSearcher struct {
	calls  []data.SceneSearchParams
	scenes []data.Scene
	total  int64
}

func (f *fakeSearcher) Search(p data.SceneSearchParams) (*SearchResult, error) {
	f.calls = append(f.calls, p)
	return &SearchResult{Scenes: f.scenes, Total: f.total}, nil
}

type fakeTags map[string]uint

func (f fakeTags) GetIDsByNames(names []string) ([]uint, error) {
	var ids []uint
	for _, n := range names {
		if id, ok := f[n]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func TestProfileListDependencies(t *testing.T) {
	cases := []struct {
		name string
		f    data.Filters
		deps string
		size bool
	}{
		{"tags only", data.Filters{SelectedTags: []string{"x"}, Sort: "view_count_desc"}, "", false},
		{"top O", data.Filters{Sort: sortJizzCountDesc}, "jizz", false},
		{"O range", data.Filters{MinJizzCount: ptrInt(1)}, "jizz", false},
		{"top rated", data.Filters{MinRating: ptrFloat(4)}, "ratings", false},
		{"unrated", data.Filters{Rated: ptrBool(false)}, "ratings", false},
		{"my rating sort", data.Filters{Sort: SortMyRatingDesc}, "ratings", false},
		{"recently liked", data.Filters{Liked: ptrBool(true), Sort: SortLikedAtDesc}, "liked", false},
		{"unwatched", data.Filters{Watched: ptrBool(false)}, "activity", false},
		{"most watched", data.Filters{Sort: SortMyViewsDesc}, "activity", false},
		{"markers", data.Filters{MarkerLabels: []string{"kiss"}}, "recent_markers", false},
		{"everything", data.Filters{Liked: ptrBool(true), MinRating: ptrFloat(3), MaxJizzCount: ptrInt(4), Watched: ptrBool(true), HasMarkers: ptrBool(true)}, "activity,recent_markers,liked,ratings,jizz", false},
		{"size", data.Filters{MinSize: ptrInt(100)}, "", true},
		{"size sort", data.Filters{Sort: "size_desc"}, "", true},
	}
	for _, tc := range cases {
		deps, size := ProfileListDependencies(tc.f)
		if got := strings.Join(deps, ","); got != tc.deps || size != tc.size {
			t.Fatalf("%s: deps %q size %v, want %q %v", tc.name, got, size, tc.deps, tc.size)
		}
	}
}

func TestListVisibleTo(t *testing.T) {
	rl := ResolvedProfileList{List: data.ProfileList{Audience: "followers"}, Deps: []string{PrivacyJizz}}
	follower := Relationship{FollowedBy: false, Following: true}
	all := map[string]bool{PrivacyJizz: true}
	if !listVisibleTo(viewerWithScenes, follower, all, rl) {
		t.Fatal("a follower who sees the jizz item is refused")
	}
	if listVisibleTo(viewerWithScenes, follower, map[string]bool{}, rl) {
		t.Fatal("a dependency hidden but the list served")
	}
	if listVisibleTo(viewerWithScenes, Relationship{}, all, rl) {
		t.Fatal("the list audience is ignored")
	}
	if listVisibleTo(ProfileViewer{UserID: 1, Access: NewUserAccess(1, "guest", false)}, follower, all, rl) {
		t.Fatal("served without scenes:view")
	}
	rl.NeedsSize = true
	if listVisibleTo(viewerWithScenes, follower, all, rl) {
		t.Fatal("size criteria served without files:view_size")
	}
	sized := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "mod", false, PermScenesView, PermFilesViewSize)}
	if !listVisibleTo(sized, follower, all, rl) {
		t.Fatal("size criteria refused with files:view_size")
	}
	// The owner always passes
	rl.List.Audience = "private"
	if !listVisibleTo(sized, Relationship{Self: true}, all, rl) {
		t.Fatal("the owner is refused")
	}
}

func TestValidateListFilters(t *testing.T) {
	owner := viewerWithScenes
	bad := map[string]data.Filters{
		"sort":       {Sort: "DROP TABLE"},
		"match":      {MatchType: "fuzzy"},
		"resolution": {Resolution: "8k"},
		"date":       {MinDate: "01/02/2026"},
		"rating":     {MinRating: ptrFloat(6)},
		"negative":   {MinDuration: ptrInt(-1)},
		"query":      {Query: strings.Repeat("x", 201)},
		"tags":       {SelectedTags: make([]string, 51)},
		"empty tag":  {SelectedTags: []string{""}},
	}
	for name, f := range bad {
		if _, err := validateListFilters(f, owner); !apperrors.IsValidation(err) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if _, err := validateListFilters(data.Filters{MaxSize: ptrInt(10)}, owner); !apperrors.IsForbidden(err) {
		t.Fatalf("size without files:view_size: %v", err)
	}
	f, err := validateListFilters(data.Filters{Query: "  hello ", Sort: SortMyRatingDesc, MinDate: "2026-01-02", Resolution: "1080p"}, owner)
	if err != nil || f.Query != "hello" {
		t.Fatalf("valid: %+v %v", f, err)
	}
}

func TestCustomListSearchParams(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	searcher := &fakeSearcher{}
	e.svc.WithCustomLists(searcher, fakeTags{"blonde": 3}, nil)
	f := data.Filters{SelectedTags: []string{"blonde"}, MinSize: ptrInt(500), Sort: "size_desc", Query: "x", MinJizzCount: ptrInt(2), Resolution: "4k", MatchType: "strict"}

	// A viewer without file permissions: no size bound nor sort, the text not matched on paths
	p, ok, err := e.svc.searchParams(f, 5, viewerWithScenes, 1, 6)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if p.UserID != 5 || p.MinSizeMB != 0 || p.Sort != "created_at_desc" || !p.HideFilePaths || p.MinJizzCount != 2 ||
		p.MinHeight != 2160 || p.MatchingStrategy != "all" || len(p.TagIDs) != 1 || p.TagIDs[0] != 3 {
		t.Fatalf("params: %+v", p)
	}
	all := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "mod", false, PermScenesView, PermFilesViewSize, PermFilesViewPaths)}
	if p, _, _ = e.svc.searchParams(f, 5, all, 2, 6); p.MinSizeMB != 500 || p.Sort != "size_desc" || p.HideFilePaths || p.Page != 2 {
		t.Fatalf("params with file permissions: %+v", p)
	}
	// Tags that do not exist: nothing matches (never a wider list)
	scenes, total, err := e.svc.runList(context.Background(), 5, viewerWithScenes, data.Filters{SelectedTags: []string{"nope"}}, 1, 6)
	if err != nil || len(scenes) != 0 || total != 0 || len(searcher.calls) != 0 {
		t.Fatalf("unknown tag: %v %d %v %d", scenes, total, err, len(searcher.calls))
	}
}

// bob (5) has a "Top O" list (jizz: friends by default) placed on his profile tab.
func (e *profileTestEnv) bobWithList(audience string) *fakeSearcher {
	return e.bobWithListBlock(audience, "", map[string]any{"list_id": 7.0, "count": 6.0})
}

// bobWithListFigures: the same list with a figures setting.
func (e *profileTestEnv) bobWithListFigures(audience, figures string) *fakeSearcher {
	return e.bobWithListBlock(audience, figures, map[string]any{"list_id": 7.0, "count": 6.0})
}

// bobWithListBlock: the same list with a figures setting and a block config.
func (e *profileTestEnv) bobWithListBlock(audience, figures string, config map[string]any) *fakeSearcher {
	searcher := &fakeSearcher{scenes: []data.Scene{{ID: 10}, {ID: 11, TrashedAt: ptrTime()}}, total: 40}
	e.svc.WithCustomLists(searcher, nil, nil)
	tabs := data.ProfileTabs{{ID: "profile", Kind: "profile", Visible: true, Sections: []data.ProfileSection{{ID: "s", Template: "1", Columns: [][]data.ProfileBlockConfig{
		{{ID: "top", Type: customListBlockType, Visible: true, LockedPlaceholder: true, Config: config}},
	}}}}}
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Tabs: tabs}, nil).AnyTimes()
	list := data.ProfileList{ID: 7, UserID: 5, Title: "Top O", Audience: audience, Figures: figures, Filters: data.Filters{Sort: sortJizzCountDesc}}
	e.repo.EXPECT().ListsByIDs(gomock.Any(), uint(5), []uint{7}).Return([]data.ProfileList{list}, nil).AnyTimes()
	e.repo.EXPECT().GetList(gomock.Any(), uint(5), uint(7)).Return(&list, nil).AnyTimes()
	return searcher
}

func TestCustomListBlockPrivacy(t *testing.T) {
	ctx := context.Background()
	// A stranger: the jizz item (friends) hides the list, only the placeholder is sent
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob(nil)
	searcher := e.bobWithList("public")
	view, err := e.svc.Get(ctx, viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Blocks) != 1 || !view.Blocks[0].Locked || view.Blocks[0].Data != nil || len(searcher.calls) != 0 {
		t.Fatalf("stranger: %+v, %d searches", view.Blocks, len(searcher.calls))
	}
	if _, err := e.svc.BrowseList(ctx, viewerWithScenes, "bob", 7, 1, 24); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("browse by a stranger: %v", err)
	}

	// jizz made public: the list runs with bob's interactions, trashed scenes skipped
	e = newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyJizz, Audience: "public"}})
	searcher = e.bobWithList("public")
	view, err = e.svc.Get(ctx, viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	b := view.Blocks[0]
	if b.Locked || b.Data == nil || len(b.Data.Scenes) != 1 || *b.Data.Total != 40 || b.Data.Title != "Top O" {
		t.Fatalf("block: %+v", b)
	}
	if p := searcher.calls[0]; p.UserID != 5 || p.Sort != sortJizzCountDesc || p.Limit != 6 {
		t.Fatalf("search: %+v", p)
	}
	page, err := e.svc.BrowseList(ctx, viewerWithScenes, "bob", 7, 2, 24)
	if err != nil || page.Total != 40 || len(page.Items) != 1 || searcher.calls[1].Page != 2 {
		t.Fatalf("browse: %+v %v", page, err)
	}

	// The list's own audience: friends only
	e = newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyJizz, Audience: "public"}})
	e.bobWithList("friends")
	if _, err := e.svc.BrowseList(ctx, viewerWithScenes, "bob", 7, 1, 24); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("browse past the list audience: %v", err)
	}
	// A superuser is a viewer like any other
	admin := ProfileViewer{UserID: 1, Access: NewUserAccess(1, "admin", true)}
	if _, err := e.svc.BrowseList(ctx, admin, "bob", 7, 1, 24); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("superuser bypass: %v", err)
	}
}

func TestCustomListCRUD(t *testing.T) {
	ctx := context.Background()
	e := newProfileTestEnv(t, config.ProfilesConfig{MaxLists: 2})
	saved := &fakeSavedSearches{byUUID: map[string]*data.SavedSearch{}}
	e.svc.WithCustomLists(&fakeSearcher{}, nil, saved)
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(5)).Return(nil, nil).AnyTimes()
	e.social.EXPECT().GetFriendRequestPolicy(gomock.Any(), uint(5)).Return("everyone", nil).AnyTimes()
	owner := ProfileViewer{UserID: 5, Access: NewUserAccess(5, "user", false, PermScenesView)}

	e.repo.EXPECT().CountLists(gomock.Any(), uint(5)).Return(int64(2), nil)
	if _, err := e.svc.CreateList(ctx, owner, CustomListInput{Title: "x"}); !apperrors.IsValidation(err) {
		t.Fatalf("over the cap: %v", err)
	}

	e.repo.EXPECT().CountLists(gomock.Any(), uint(5)).Return(int64(0), nil).AnyTimes()
	if _, err := e.svc.CreateList(ctx, owner, CustomListInput{Title: " ", Filters: data.Filters{}}); !apperrors.IsValidation(err) {
		t.Fatalf("no title: %v", err)
	}
	if _, err := e.svc.CreateList(ctx, owner, CustomListInput{Title: "x", Audience: "world"}); !apperrors.IsValidation(err) {
		t.Fatalf("bad audience: %v", err)
	}
	// Someone else's saved search is refused
	other := uuid.New()
	saved.byUUID[other.String()] = &data.SavedSearch{UUID: other, UserID: 6}
	if _, err := e.svc.CreateList(ctx, owner, CustomListInput{Title: "x", SavedSearchUUID: other.String()}); !apperrors.IsValidation(err) {
		t.Fatalf("someone else's saved search: %v", err)
	}

	mine := uuid.New()
	saved.byUUID[mine.String()] = &data.SavedSearch{UUID: mine, UserID: 5, Filters: data.Filters{Liked: ptrBool(true), Sort: SortLikedAtDesc}}
	e.repo.EXPECT().CreateList(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *data.ProfileList) error {
		l.ID = 3
		return nil
	})
	info, err := e.svc.CreateList(ctx, owner, CustomListInput{Title: " Recently liked ", SavedSearchUUID: mine.String(), Audience: "friends"})
	if err != nil {
		t.Fatal(err)
	}
	if info.List.Title != "Recently liked" || info.List.Audience != "friends" || !info.FollowsSavedSearch ||
		len(info.Dependencies) != 1 || info.Dependencies[0].Key != PrivacyLiked || info.Dependencies[0].Audience != AudienceFriends {
		t.Fatalf("created: %+v", info)
	}

	e.repo.EXPECT().DeleteList(gomock.Any(), uint(5), uint(9)).Return(false, nil)
	if err := e.svc.DeleteList(ctx, 5, 9); !apperrors.IsNotFound(err) {
		t.Fatalf("delete unknown: %v", err)
	}
}

type fakeSavedSearches struct {
	data.SavedSearchRepository
	byUUID map[string]*data.SavedSearch
}

func (f *fakeSavedSearches) GetByUUID(id string) (*data.SavedSearch, error) {
	if s, ok := f.byUUID[id]; ok {
		return s, nil
	}
	return nil, apperrors.NewNotFoundError("saved search", id)
}

func ptrTime() *time.Time {
	now := time.Now()
	return &now
}
