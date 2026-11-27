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

var testTabLimits = ProfileTabLimits{MaxTabs: 8, MaxSections: 3, MaxBlocksPerTab: 4}

func tabIDs(tabs data.ProfileTabs) string {
	ids := make([]string, len(tabs))
	for i, t := range tabs {
		ids[i] = t.ID
	}
	return strings.Join(ids, ",")
}

func TestNormalizeProfileTabsConvertsTheOlderLayout(t *testing.T) {
	// A profile saved before the tabs: its blocks, in order, in one full-width section
	layout := data.ProfileLayout{{Type: "stats", Visible: true}, {Type: "friends", Visible: false, LockedPlaceholder: true}}
	tabs := NormalizeProfileTabs(nil, layout)
	if got := tabIDs(tabs); got != "profile,likes,ratings,jizz" {
		t.Fatalf("tabs = %s", got)
	}
	secs := tabs[0].Sections
	if len(secs) != 1 || secs[0].Template != "1" || len(secs[0].Columns) != 1 {
		t.Fatalf("sections: %+v", secs)
	}
	blocks := secs[0].Columns[0]
	want := NormalizeProfileLayout(layout)
	if len(blocks) != len(want) {
		t.Fatalf("%d blocks, want %d", len(blocks), len(want))
	}
	for i := range want {
		if blocks[i].Type != want[i].Type || blocks[i].Visible != want[i].Visible || blocks[i].ID != want[i].Type {
			t.Fatalf("block %d = %+v, want %+v", i, blocks[i], want[i])
		}
	}
	if !blocks[1].LockedPlaceholder {
		t.Fatal("placeholder lost")
	}
	// No layout at all: the default layout, without the custom list block
	def := DefaultProfileTabs()
	for _, b := range def[0].Sections[0].Columns[0] {
		if b.Type == customListBlockType {
			t.Fatal("custom_list in the default layout")
		}
	}
}

func TestNormalizeStoredProfileTabs(t *testing.T) {
	stored := data.ProfileTabs{
		{ID: "jizz", Kind: "jizz", Visible: false},
		{ID: "x", Kind: "nope"},
		{ID: "disc", Kind: "custom", Title: "Discover", Slug: "discover", Visible: true, Sections: []data.ProfileSection{
			{ID: "s1", Template: "1", Columns: [][]data.ProfileBlockConfig{
				{{ID: "a", Type: "stats", Visible: true, Config: map[string]any{"period": "decade"}}},
				{{ID: "b", Type: "gone", Visible: true}, {ID: "c", Type: "ratings", Visible: true}},
			}},
			{ID: "s2", Template: "weird", Columns: nil},
		}},
		{ID: "profile", Kind: "profile", Visible: false},
		{ID: "jizz2", Kind: "jizz"},
	}
	tabs := NormalizeProfileTabs(stored, nil)
	if got := tabIDs(tabs); got != "jizz,disc,profile,likes,ratings" {
		t.Fatalf("tabs = %s", got)
	}
	if tabs[0].Visible || !tabs[2].Visible {
		t.Fatalf("visibility: jizz %v, profile %v (always shown)", tabs[0].Visible, tabs[2].Visible)
	}
	disc := tabs[1].Sections
	// the second column of a one-column template joins the first; unknown blocks and options
	// are dropped; an unknown template becomes one column
	if len(disc[0].Columns) != 1 || len(disc[0].Columns[0]) != 2 || disc[0].Columns[0][1].ID != "c" {
		t.Fatalf("section 1: %+v", disc[0])
	}
	if len(disc[0].Columns[0][0].Config) != 0 {
		t.Fatalf("invalid option kept: %+v", disc[0].Columns[0][0].Config)
	}
	if disc[1].Template != "1" || len(disc[1].Columns) != 1 {
		t.Fatalf("section 2: %+v", disc[1])
	}
}

func validTabs() data.ProfileTabs {
	return data.ProfileTabs{
		{ID: "profile", Kind: "profile", Visible: true, Sections: []data.ProfileSection{
			{ID: "top", Template: "2-1", Columns: [][]data.ProfileBlockConfig{
				{{ID: "fav", Type: "favorite_scenes", Visible: true}},
				{{ID: "st", Type: "stats", Visible: true, Config: map[string]any{"period": "year"}}},
			}},
		}},
		{ID: "t1", Kind: "custom", Title: "Top O!", Icon: "heroicons:fire", Visible: true, Sections: []data.ProfileSection{
			{ID: "s", Template: "1-1-1", Columns: [][]data.ProfileBlockConfig{{}, {{ID: "rt", Type: "ratings", Visible: true}}, {}}},
		}},
		{ID: "likes", Kind: "likes", Visible: false},
	}
}

func TestValidateProfileTabs(t *testing.T) {
	out, err := ValidateProfileTabs(validTabs(), testTabLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := tabIDs(out); got != "profile,t1,likes,ratings,jizz" {
		t.Fatalf("tabs = %s", got)
	}
	if out[1].Slug != "top-o" || out[0].Sections[0].Columns[1][0].Config["period"] != "year" {
		t.Fatalf("normalized: %+v", out[1])
	}

	cases := map[string]func(tabs data.ProfileTabs) data.ProfileTabs{
		"too many tabs": func(tabs data.ProfileTabs) data.ProfileTabs {
			for i := 0; i < 8; i++ {
				tabs = append(tabs, data.ProfileTab{ID: "c" + string(rune('a'+i)), Kind: "custom", Title: "x" + string(rune('a'+i))})
			}
			return tabs
		},
		"duplicate tab id":   func(tabs data.ProfileTabs) data.ProfileTabs { tabs[1].ID = "profile"; return tabs },
		"invalid tab id":     func(tabs data.ProfileTabs) data.ProfileTabs { tabs[1].ID = "Bad Id"; return tabs },
		"reserved custom id": func(tabs data.ProfileTabs) data.ProfileTabs { tabs[2].ID = "l"; tabs[1].ID = "jizz"; return tabs },
		"no title":           func(tabs data.ProfileTabs) data.ProfileTabs { tabs[1].Title = "  "; return tabs },
		"bad icon":           func(tabs data.ProfileTabs) data.ProfileTabs { tabs[1].Icon = "<script>"; return tabs },
		"builtin twice": func(tabs data.ProfileTabs) data.ProfileTabs {
			tabs[2] = data.ProfileTab{ID: "profile", Kind: "profile"}
			return tabs
		},
		"builtin wrong id":   func(tabs data.ProfileTabs) data.ProfileTabs { tabs[2].ID = "my-likes"; return tabs },
		"unknown kind":       func(tabs data.ProfileTabs) data.ProfileTabs { tabs[2].Kind = "feed"; return tabs },
		"sections on a list": func(tabs data.ProfileTabs) data.ProfileTabs { tabs[2].Sections = tabs[1].Sections; return tabs },
		"unknown template":   func(tabs data.ProfileTabs) data.ProfileTabs { tabs[0].Sections[0].Template = "4-4-4-4"; return tabs },
		"columns mismatch":   func(tabs data.ProfileTabs) data.ProfileTabs { tabs[0].Sections[0].Template = "1"; return tabs },
		"duplicate block id": func(tabs data.ProfileTabs) data.ProfileTabs {
			tabs[1].Sections[0].Columns[1][0].ID = "fav"
			return tabs
		},
		"unknown block": func(tabs data.ProfileTabs) data.ProfileTabs {
			tabs[0].Sections[0].Columns[0][0].Type = "ads"
			return tabs
		},
		"unknown option": func(tabs data.ProfileTabs) data.ProfileTabs {
			tabs[0].Sections[0].Columns[0][0].Config = map[string]any{"x": 1.0}
			return tabs
		},
		"duplicate section id": func(tabs data.ProfileTabs) data.ProfileTabs {
			tabs[0].Sections = append(tabs[0].Sections, tabs[0].Sections[0])
			return tabs
		},
		"too many sections": func(tabs data.ProfileTabs) data.ProfileTabs {
			for _, id := range []string{"a", "b", "c"} {
				tabs[1].Sections = append(tabs[1].Sections, data.ProfileSection{ID: id, Template: "1", Columns: [][]data.ProfileBlockConfig{{}}})
			}
			return tabs
		},
		"too many blocks": func(tabs data.ProfileTabs) data.ProfileTabs {
			col := []data.ProfileBlockConfig{}
			for _, id := range []string{"b1", "b2", "b3", "b4", "b5"} {
				col = append(col, data.ProfileBlockConfig{ID: id, Type: "stats"})
			}
			tabs[1].Sections[0].Columns[0] = col
			return tabs
		},
		"duplicate slug": func(tabs data.ProfileTabs) data.ProfileTabs {
			return append(tabs, data.ProfileTab{ID: "t2", Kind: "custom", Title: "Top O"})
		},
	}
	for name, mutate := range cases {
		if _, err := ValidateProfileTabs(mutate(validTabs()), testTabLimits); !apperrors.IsValidation(err) {
			t.Fatalf("%s: err = %v, want a validation error", name, err)
		}
	}
}

func TestProfileSlug(t *testing.T) {
	for in, want := range map[string]string{"Top O!": "top-o", "  Discover  ": "discover", "Été 2026": "t-2026", "***": ""} {
		if got := ProfileSlug(in); got != want {
			t.Fatalf("ProfileSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// bob's tabs: the profile tab (stats), a custom tab with only a jizz block (jizz: friends by
// default), and a hidden custom tab.
func bobTabs() data.ProfileTabs {
	return data.ProfileTabs{
		{ID: "profile", Kind: "profile", Visible: true, Sections: []data.ProfileSection{
			{ID: "s", Template: "1-1", Columns: [][]data.ProfileBlockConfig{{{ID: "st", Type: "stats", Visible: true}}, {}}},
		}},
		{ID: "o", Kind: "custom", Title: "O", Slug: "o", Visible: true, Sections: []data.ProfileSection{
			{ID: "s", Template: "1", Columns: [][]data.ProfileBlockConfig{{{ID: "jz", Type: "jizz", Visible: true}}}},
		}},
		{ID: "secret", Kind: "custom", Title: "Secret", Slug: "secret", Visible: false, Sections: []data.ProfileSection{
			{ID: "s", Template: "1", Columns: [][]data.ProfileBlockConfig{{{ID: "rt", Type: "ratings", Visible: true}}}},
		}},
	}
}

func TestProfileGetTabsPerViewer(t *testing.T) {
	ctx := context.Background()
	// stats public; a stranger does not see the O tab (its only block is for friends)
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.expectBob([]data.UserPrivacySetting{{UserID: 5, Item: PrivacyStats, Audience: "public"}})
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Tabs: bobTabs()}, nil).AnyTimes()
	e.repo.EXPECT().Stats(gomock.Any(), uint(5), gomock.Any()).Return(&data.ProfileStats{Views: 1}, nil)
	e.repo.EXPECT().TopTags(gomock.Any(), uint(5), gomock.Any(), gomock.Any()).Return(nil, nil)
	e.repo.EXPECT().TopActors(gomock.Any(), uint(5), gomock.Any(), gomock.Any()).Return(nil, nil)
	e.repo.EXPECT().TopStudios(gomock.Any(), uint(5), gomock.Any(), gomock.Any()).Return(nil, nil)
	view, err := e.svc.Get(ctx, viewerWithScenes, "bob")
	if err != nil {
		t.Fatal(err)
	}
	var tabs []string
	for _, tab := range view.Tabs {
		tabs = append(tabs, tab.ID)
	}
	if got := strings.Join(tabs, ","); got != "profile" {
		t.Fatalf("tabs for a stranger = %s", got)
	}
	if view.Tab != "profile" || len(view.Sections) != 1 || len(view.Sections[0].Columns) != 2 || view.Sections[0].Columns[0][0].ID != "st" {
		t.Fatalf("sections: %+v", view.Sections)
	}
	// Asking for the hidden tab, by slug: missing, nothing fetched
	view, err = e.svc.Get(ctx, viewerWithScenes, "bob", "o")
	if err != nil || !view.TabMissing || len(view.Sections) != 0 {
		t.Fatalf("hidden tab: %+v %v", view, err)
	}
}

func TestProfileGetCustomTabForFriend(t *testing.T) {
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	e.social.EXPECT().GetUserByUsername(gomock.Any(), "bob").Return(&data.SocialUser{ID: 5, Username: "bob"}, nil)
	e.social.EXPECT().Relations(gomock.Any(), uint(1), []uint{5}).Return([]data.UserRelation{{UserID: 5, Friend: true}}, nil).AnyTimes()
	e.social.EXPECT().Counts(gomock.Any(), uint(5)).Return(data.SocialCounts{}, nil)
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(5)).Return(nil, nil).AnyTimes()
	e.repo.EXPECT().GetProfile(gomock.Any(), uint(5)).Return(&data.UserProfile{UserID: 5, Tabs: bobTabs()}, nil)
	e.repo.EXPECT().JizzSummary(gomock.Any(), uint(5)).Return(&data.ProfileJizzSummary{Total: 2, Scenes: 1}, nil)
	e.repo.EXPECT().JizzScenes(gomock.Any(), uint(5), gomock.Any()).Return(nil, int64(0), nil)
	e.repo.EXPECT().ScenesByIDs(gomock.Any(), gomock.Any()).Return([]data.Scene{}, nil)
	view, err := e.svc.Get(context.Background(), viewerWithScenes, "bob", "o")
	if err != nil {
		t.Fatal(err)
	}
	var tabs []string
	for _, tab := range view.Tabs {
		tabs = append(tabs, tab.ID)
	}
	// the hidden custom tab is never listed; a friend sees the list tabs (friends by default)
	if got := strings.Join(tabs, ","); got != "profile,o,likes,ratings,jizz" {
		t.Fatalf("tabs for a friend = %s", got)
	}
	if view.Tab != "o" || len(view.Blocks) != 1 || view.Blocks[0].Spec.Type != "jizz" {
		t.Fatalf("served: %s %+v", view.Tab, view.Blocks)
	}
}

func TestSetTabsChecksTheLists(t *testing.T) {
	ctx := context.Background()
	e := newProfileTestEnv(t, config.ProfilesConfig{})
	tabs := validTabs()
	tabs[1].Sections[0].Columns[0] = []data.ProfileBlockConfig{{ID: "cl", Type: customListBlockType, Visible: true, Config: map[string]any{"list_id": 9.0}}}
	e.repo.EXPECT().ListsByIDs(gomock.Any(), uint(5), []uint{9}).Return(nil, nil)
	if _, err := e.svc.SetTabs(ctx, 5, tabs); !apperrors.IsValidation(err) {
		t.Fatalf("someone else's list: %v", err)
	}
	tabs = validTabs()
	tabs[1].Sections[0].Columns[0] = []data.ProfileBlockConfig{{ID: "cl", Type: customListBlockType, Visible: true}}
	if _, err := e.svc.SetTabs(ctx, 5, tabs); !apperrors.IsValidation(err) {
		t.Fatalf("no list: %v", err)
	}
	tabs = validTabs()
	tabs[1].Sections[0].Columns[0] = []data.ProfileBlockConfig{{ID: "cl", Type: customListBlockType, Visible: true, Config: map[string]any{"list_id": 9.0}}}
	e.repo.EXPECT().ListsByIDs(gomock.Any(), uint(5), []uint{9}).Return([]data.ProfileList{{ID: 9, UserID: 5}}, nil)
	e.repo.EXPECT().SaveTabs(gomock.Any(), uint(5), gomock.Any()).Return(nil)
	if _, err := e.svc.SetTabs(ctx, 5, tabs); err != nil {
		t.Fatal(err)
	}
	e.repo.EXPECT().SaveTabs(gomock.Any(), uint(5), data.ProfileTabs(nil)).Return(nil)
	if out, err := e.svc.SetTabs(ctx, 5, nil); err != nil || tabIDs(out) != "profile,likes,ratings,jizz" {
		t.Fatalf("reset: %v %v", out, err)
	}
}
