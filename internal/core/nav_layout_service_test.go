package core

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// todayHeader is the header before the layouts existed: the default layout must equal it.
func todayHeader() data.NavLayout {
	return data.NavLayout{
		Version: NavLayoutVersion,
		Bars: data.NavLayoutBars{
			Top: data.NavBarZones{
				Left:   []string{"logo"},
				Center: []string{},
				Right: []string{"command_palette", "profile", "shortcuts", "safe_mode", "preview_sound",
					"job_status", "presence", "messages", "friends", "settings", "logout"},
			},
			Nav: data.NavBarZones{
				Left:   []string{"search", "markers", "playlists", "together", "history", "actors", "studios", "folder:manage"},
				Center: []string{},
				Right:  []string{},
			},
		},
		Folders: []data.NavFolder{{ID: "manage", Name: "Manage", Icon: "heroicons:wrench-screwdriver", Items: []string{"triage", "explorer", "duplicates"}}},
		Hidden:  []string{},
	}
}

func superuserAccess() *UserAccess { return NewUserAccess(1, "admin", true) }

func viewerAccess(perms ...string) *UserAccess {
	return NewUserAccess(1, "user", false, append([]string{PermScenesView}, perms...)...)
}

func newNavLayoutTestService(t *testing.T) (*NavLayoutService, *mocks.MockNavLayoutRepository, *EventBus) {
	t.Helper()
	repo := mocks.NewMockNavLayoutRepository(gomock.NewController(t))
	bus := NewEventBus(zap.NewNop())
	return NewNavLayoutService(repo, bus, zap.NewNop()), repo, bus
}

func TestDefaultNavLayout_IsTodaysHeader(t *testing.T) {
	if got := DefaultNavLayout(); !reflect.DeepEqual(got, todayHeader()) {
		t.Fatalf("default layout = %+v\nwant %+v", got, todayHeader())
	}
	// it is a valid layout
	if _, err := normalizeNavLayout(DefaultNavLayout(), superuserAccess().Has, true); err != nil {
		t.Fatalf("default layout refused: %v", err)
	}
}

func TestNavItemCatalog_Consistent(t *testing.T) {
	ids := map[string]bool{}
	for _, spec := range NavItemCatalog() {
		if ids[spec.ID] {
			t.Fatalf("duplicate item %q", spec.ID)
		}
		ids[spec.ID] = true
		if !spec.allowsBar(spec.DefaultBar) {
			t.Fatalf("item %q cannot go in its default bar", spec.ID)
		}
		if spec.DefaultFolder != "" && !spec.Folderable {
			t.Fatalf("item %q has a default folder but cannot go in a folder", spec.ID)
		}
	}
	for _, id := range []string{"logo", "settings", "logout"} {
		if !navItemsByID[id].Required {
			t.Fatalf("%q must be required", id)
		}
	}
	gated := map[string]string{"job_status": PermJobsViewStatus, "together": PermTogetherUse, "explorer": PermExplorerAccess, "duplicates": PermDuplicatesManage}
	for id, perm := range gated {
		if navItemsByID[id].Permission != perm {
			t.Fatalf("%q permission = %q, want %q", id, navItemsByID[id].Permission, perm)
		}
	}
}

func TestNavLayoutGet_DefaultWithoutAdminItems(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	repo.EXPECT().Get(gomock.Any(), uint(1)).Return(nil, nil)

	view, err := svc.Get(context.Background(), viewerAccess())
	if err != nil {
		t.Fatal(err)
	}
	if view.Customized {
		t.Fatal("a user without a stored layout is not customized")
	}
	if slices.Contains(view.Layout.Bars.Top.Right, "job_status") {
		t.Fatal("job status shown without jobs:view_status")
	}
	if slices.Contains(view.Layout.Bars.Nav.Left, "together") {
		t.Fatal("together shown without goon2gether:use")
	}
	if got := view.Layout.Folders[0].Items; !reflect.DeepEqual(got, []string{"triage"}) {
		t.Fatalf("manage folder items = %v, want [triage]", got)
	}
}

func TestNavLayoutGet_SuperuserGetsEverything(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	repo.EXPECT().Get(gomock.Any(), uint(1)).Return(nil, nil)
	view, err := svc.Get(context.Background(), superuserAccess())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(view.Layout, todayHeader()) {
		t.Fatalf("superuser layout = %+v", view.Layout)
	}
}

func TestNavLayoutGet_LostPermissionHidesTheItem(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	stored := todayHeader()
	stored.Bars.Top.Center = []string{"duplicates", "explorer"}
	stored.Folders[0].Items = []string{"triage"}
	stored.Hidden = []string{"job_status"}
	repo.EXPECT().Get(gomock.Any(), uint(1)).Return(&stored, nil)

	view, err := svc.Get(context.Background(), viewerAccess(PermExplorerAccess))
	if err != nil {
		t.Fatalf("a lost permission must not fail: %v", err)
	}
	if !view.Customized {
		t.Fatal("stored layout not marked customized")
	}
	if !reflect.DeepEqual(view.Layout.Bars.Top.Center, []string{"explorer"}) {
		t.Fatalf("top center = %v, want [explorer]", view.Layout.Bars.Top.Center)
	}
	if len(view.Layout.Hidden) != 0 {
		t.Fatalf("hidden = %v, the job status is not offered without its permission", view.Layout.Hidden)
	}
}

func TestNavLayoutGet_RepairsAStoredLayout(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	stored := todayHeader()
	stored.Bars.Nav.Left = append(stored.Bars.Nav.Left, "removed_item", "search", "folder:gone")
	stored.Bars.Nav.Right = []string{"logo"}
	stored.Hidden = []string{"logout"}
	repo.EXPECT().Get(gomock.Any(), uint(1)).Return(&stored, nil)

	view, err := svc.Get(context.Background(), superuserAccess())
	if err != nil {
		t.Fatal(err)
	}
	want := todayHeader()
	if !reflect.DeepEqual(view.Layout, want) {
		t.Fatalf("repaired layout = %+v\nwant %+v", view.Layout, want)
	}
}

func TestNavLayoutSave_Validation(t *testing.T) {
	tooManyFolders := func(l *data.NavLayout) {
		for i := 0; i <= MaxNavFolders; i++ {
			id := "f" + strings.Repeat("x", i+1)
			l.Folders = append(l.Folders, data.NavFolder{ID: id, Name: "F", Items: []string{}})
			l.Bars.Nav.Right = append(l.Bars.Nav.Right, NavFolderRefPrefix+id)
		}
	}
	cases := []struct {
		name   string
		change func(*data.NavLayout)
	}{
		{"unknown item", func(l *data.NavLayout) { l.Bars.Top.Center = []string{"nope"} }},
		{"item twice", func(l *data.NavLayout) { l.Bars.Top.Center = []string{"search"} }},
		{"item in a folder and a zone", func(l *data.NavLayout) { l.Folders[0].Items = append(l.Folders[0].Items, "history") }},
		{"logo in the nav row", func(l *data.NavLayout) {
			l.Bars.Top.Left = []string{}
			l.Bars.Nav.Right = []string{"logo"}
		}},
		{"job status in a folder", func(l *data.NavLayout) {
			l.Bars.Top.Right = slices.DeleteFunc(slices.Clone(l.Bars.Top.Right), func(s string) bool { return s == "job_status" })
			l.Folders[0].Items = append(l.Folders[0].Items, "job_status")
		}},
		{"required item hidden", func(l *data.NavLayout) {
			l.Bars.Top.Right = l.Bars.Top.Right[:len(l.Bars.Top.Right)-1]
			l.Hidden = []string{"logout"}
		}},
		{"placed and hidden", func(l *data.NavLayout) { l.Hidden = []string{"history"} }},
		{"unknown hidden item", func(l *data.NavLayout) { l.Hidden = []string{"nope"} }},
		{"too many folders", tooManyFolders},
		{"empty folder name", func(l *data.NavLayout) { l.Folders[0].Name = "   " }},
		{"long folder name", func(l *data.NavLayout) { l.Folders[0].Name = strings.Repeat("a", MaxNavFolderNameLength+1) }},
		{"control character in a name", func(l *data.NavLayout) { l.Folders[0].Name = "a\nb" }},
		{"bad folder id", func(l *data.NavLayout) {
			l.Folders[0].ID = "Bad ID"
			l.Bars.Nav.Left[len(l.Bars.Nav.Left)-1] = "folder:Bad ID"
		}},
		{"bad folder icon", func(l *data.NavLayout) { l.Folders[0].Icon = "<script>" }},
		{"folder defined twice", func(l *data.NavLayout) { l.Folders = append(l.Folders, l.Folders[0]) }},
		{"unknown folder", func(l *data.NavLayout) { l.Bars.Top.Center = []string{"folder:nope"} }},
		{"folder placed twice", func(l *data.NavLayout) { l.Bars.Top.Center = []string{"folder:manage"} }},
		{"folder not placed", func(l *data.NavLayout) { l.Bars.Nav.Left = l.Bars.Nav.Left[:len(l.Bars.Nav.Left)-1] }},
		{"wrong version", func(l *data.NavLayout) { l.Version = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newNavLayoutTestService(t) // no repository call expected
			l := todayHeader()
			tc.change(&l)
			_, err := svc.Save(context.Background(), superuserAccess(), l)
			if !apperrors.IsValidation(err) {
				t.Fatalf("err = %v, want a validation error", err)
			}
		})
	}
}

func TestNavLayoutSave_StripsForbiddenItems(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	l := todayHeader()
	// a regular user tries to put the admin pages in the top bar and hide the job status
	l.Folders[0].Items = []string{"triage"}
	l.Bars.Top.Right = slices.DeleteFunc(slices.Clone(l.Bars.Top.Right), func(s string) bool { return s == "job_status" })
	l.Bars.Top.Center = []string{"duplicates", "explorer"}
	l.Hidden = []string{"job_status"}

	var saved data.NavLayout
	repo.EXPECT().Save(gomock.Any(), uint(1), gomock.Any()).DoAndReturn(func(_ context.Context, _ uint, layout data.NavLayout) error {
		saved = layout
		return nil
	})
	view, err := svc.Save(context.Background(), viewerAccess(PermTogetherUse), l)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Bars.Top.Center) != 0 || len(saved.Hidden) != 0 {
		t.Fatalf("forbidden items stored: center=%v hidden=%v", saved.Bars.Top.Center, saved.Hidden)
	}
	if !slices.Contains(saved.Bars.Nav.Left, "together") {
		t.Fatal("together dropped although allowed")
	}
	if !reflect.DeepEqual(view.Layout, saved) || !view.Customized {
		t.Fatalf("answer %+v differs from the stored layout %+v", view.Layout, saved)
	}
}

func TestNavLayoutSave_FoldersAndZones(t *testing.T) {
	svc, repo, bus := newNavLayoutTestService(t)
	subID, events := bus.SubscribeUser(1)
	defer bus.Unsubscribe(subID)

	l := data.NavLayout{
		Version: NavLayoutVersion,
		Bars: data.NavLayoutBars{
			Top: data.NavBarZones{Left: []string{"logo"}, Center: []string{"search", "folder:library"}, Right: []string{"folder:me"}},
			Nav: data.NavBarZones{Right: []string{"markers"}},
		},
		Folders: []data.NavFolder{
			{ID: "library", Name: "  Library  ", Icon: "heroicons:film", Items: []string{"actors", "studios"}},
			{ID: "me", Name: "Me", Items: []string{"profile", "settings", "logout"}},
		},
		Hidden: []string{"playlists", "history"},
	}
	var saved data.NavLayout
	repo.EXPECT().Save(gomock.Any(), uint(1), gomock.Any()).DoAndReturn(func(_ context.Context, _ uint, layout data.NavLayout) error {
		saved = layout
		return nil
	})
	if _, err := svc.Save(context.Background(), viewerAccess(), l); err != nil {
		t.Fatal(err)
	}
	if saved.Folders[0].Name != "Library" {
		t.Fatalf("folder name not trimmed: %q", saved.Folders[0].Name)
	}
	if saved.Bars.Nav.Left == nil || saved.Bars.Top.Center[1] != "folder:library" {
		t.Fatalf("zones not kept: %+v", saved.Bars)
	}
	select {
	case ev := <-events:
		if ev.Type != NavLayoutChangedEvent || ev.UserID != 1 {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no layout event for the other tabs")
	}
}

func TestNavLayoutReset(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	repo.EXPECT().Delete(gomock.Any(), uint(1)).Return(nil)
	view, err := svc.Reset(context.Background(), superuserAccess())
	if err != nil {
		t.Fatal(err)
	}
	if view.Customized || !reflect.DeepEqual(view.Layout, todayHeader()) {
		t.Fatalf("reset view = %+v", view)
	}
}

func TestNavLayoutService_Errors(t *testing.T) {
	svc, repo, _ := newNavLayoutTestService(t)
	if _, err := svc.Get(context.Background(), nil); err == nil {
		t.Fatal("no access must fail")
	}
	repo.EXPECT().Get(gomock.Any(), uint(1)).Return(nil, errors.New("db down"))
	if _, err := svc.Get(context.Background(), superuserAccess()); err == nil {
		t.Fatal("a repository error must fail")
	}
}
