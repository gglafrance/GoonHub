package core

import (
	"reflect"
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/infrastructure/meilisearch"

	"go.uber.org/zap"
)

func intPtr(n int) *int { return &n }

func TestSavedSearchUsesSize(t *testing.T) {
	tests := []struct {
		name string
		f    data.Filters
		want bool
	}{
		{"nothing", data.Filters{Query: "x", Sort: "title_asc"}, false},
		{"min size", data.Filters{MinSize: intPtr(10)}, true},
		{"max size", data.Filters{MaxSize: intPtr(10)}, true},
		{"zero bounds", data.Filters{MinSize: intPtr(0), MaxSize: intPtr(0)}, false},
		{"size sort", data.Filters{Sort: "size_desc"}, true},
	}
	for _, tc := range tests {
		if got := SavedSearchUsesSize(tc.f); got != tc.want {
			t.Fatalf("%s: got %v", tc.name, got)
		}
	}
	stripped := WithoutSizeFilters(data.Filters{Query: "q", MinSize: intPtr(1), MaxSize: intPtr(2), Sort: "size_asc"})
	if stripped.MinSize != nil || stripped.MaxSize != nil || stripped.Sort != "" || stripped.Query != "q" {
		t.Fatalf("unexpected filters %+v", stripped)
	}
	if kept := WithoutSizeFilters(data.Filters{Sort: "title_asc"}); kept.Sort != "title_asc" {
		t.Fatalf("other sorts are kept: %+v", kept)
	}
}

func TestSettingsSizeSorts(t *testing.T) {
	size, title := "size_desc", "title_asc"
	hp := &data.HomepageConfig{Tabs: []data.HomepageTab{{ID: "home", Sections: []data.HomepageSection{{ID: "a", Sort: "size_asc"}, {ID: "b", Sort: "title_asc"}}}}}
	tests := []struct {
		name  string
		sort  *string
		hp    *data.HomepageConfig
		prefs *data.SortPreferences
		want  bool
	}{
		{"nothing", nil, nil, nil, false},
		{"default sort", &size, nil, nil, true},
		{"other default sort", &title, nil, nil, false},
		{"homepage section", nil, hp, nil, true},
		{"actor scenes", nil, nil, &data.SortPreferences{ActorScenes: "size_asc"}, true},
		{"studio scenes", nil, nil, &data.SortPreferences{StudioScenes: "size_desc"}, true},
		{"other prefs", nil, nil, &data.SortPreferences{Actors: "name_asc", ActorScenes: "title_asc"}, false},
	}
	for _, tc := range tests {
		if got := SettingsUseSizeSort(tc.sort, tc.hp, tc.prefs); got != tc.want {
			t.Fatalf("%s: got %v", tc.name, got)
		}
	}

	settings := &data.UserSettings{
		DefaultSortOrder: "size_desc",
		HomepageConfig:   *hp,
		SortPreferences:  data.SortPreferences{ActorScenes: "size_asc", StudioScenes: "title_asc"},
	}
	clean := WithoutSizeSorts(settings)
	if clean.DefaultSortOrder != DefaultSceneSort || clean.SortPreferences.ActorScenes != "" || clean.SortPreferences.StudioScenes != "title_asc" {
		t.Fatalf("unexpected settings %+v", clean)
	}
	if clean.HomepageConfig.Tabs[0].Sections[0].Sort != "" || clean.HomepageConfig.Tabs[0].Sections[1].Sort != "title_asc" {
		t.Fatalf("unexpected sections %+v", clean.HomepageConfig.Tabs[0].Sections)
	}
	if settings.DefaultSortOrder != "size_desc" || settings.HomepageConfig.Tabs[0].Sections[0].Sort != "size_asc" {
		t.Fatal("the stored settings must not be modified")
	}
}

func TestBuildMeiliParams_HideFilePaths(t *testing.T) {
	svc := NewSearchService(nil, nil, nil, nil, nil, nil, zap.NewNop())
	if got := svc.buildMeiliParams(data.SceneSearchParams{Page: 1, Limit: 5, Query: "x"}, nil); got.ExcludeFileAttributes {
		t.Fatal("file attributes are searched by default")
	}
	got := svc.buildMeiliParams(data.SceneSearchParams{Page: 1, Limit: 5, Query: "x", HideFilePaths: true}, nil)
	if !got.ExcludeFileAttributes {
		t.Fatal("expected the file attributes to be excluded")
	}
	for _, a := range meilisearch.ContentSearchAttributes {
		if a == "path" || a == "original_filename" {
			t.Fatalf("content attributes must not contain %s", a)
		}
	}
	if !reflect.DeepEqual(meilisearch.ContentSearchAttributes, []string{"title", "description", "actors", "actor_aliases", "tag_names"}) {
		t.Fatalf("content attributes = %v", meilisearch.ContentSearchAttributes)
	}
}

var sortProbeSeen string

func init() {
	registerHomepageWidget("test_sort_probe", HomepageWidgetSpec{Fetch: func(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
		sortProbeSeen = section.Sort
		return emptyWidgetData(section, nil), nil
	}})
}

// A section sorted by size (stored before migration 094) falls back to its default sort for a
// user without files:view_size.
func TestHomepageSection_SizeSortFallsBack(t *testing.T) {
	svc := NewHomepageService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	section := data.HomepageSection{ID: "p", Type: "test_sort_probe", Sort: "size_desc"}

	svc.SetWidgetDeps(HomepageWidgetDeps{Permissions: fakePermissionChecker{}})
	if _, err := svc.fetchSectionData(1, section); err != nil {
		t.Fatal(err)
	}
	if sortProbeSeen != "" {
		t.Fatalf("size sort must be dropped, got %q", sortProbeSeen)
	}

	svc.SetWidgetDeps(HomepageWidgetDeps{Permissions: fakePermissionChecker{PermFilesViewSize: true}})
	if _, err := svc.fetchSectionData(1, section); err != nil {
		t.Fatal(err)
	}
	if sortProbeSeen != "size_desc" {
		t.Fatalf("size sort kept with files:view_size, got %q", sortProbeSeen)
	}
}
