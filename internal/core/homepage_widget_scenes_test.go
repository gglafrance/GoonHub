package core

import (
	"strings"
	"testing"
	"time"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

func TestSceneWidgetsRegistered(t *testing.T) {
	for _, widgetType := range []string{"discover", "rediscover", "roulette", "on_this_day", "folder_latest",
		"because_you_watched", "showcase_4k", "long_form"} {
		if !IsHomepageWidgetType(widgetType) {
			t.Fatalf("widget %q is not registered", widgetType)
		}
	}
}

func TestSceneWidgetValidation(t *testing.T) {
	tests := []struct {
		name       string
		widgetType string
		config     map[string]interface{}
		wantErr    string
	}{
		{"discover ok", "discover", map[string]interface{}{"min_duration": 10.0, "max_duration": 60.0, "tags": []interface{}{"Pup"}}, ""},
		{"discover min > max", "discover", map[string]interface{}{"min_duration": 100.0, "max_duration": 60.0}, "min_duration"},
		{"discover negative", "discover", map[string]interface{}{"min_duration": -5.0}, "durations"},
		{"rediscover ok", "rediscover", map[string]interface{}{"days": 90.0, "min_rating": 4.5}, ""},
		{"rediscover days", "rediscover", map[string]interface{}{"days": 0.0}, "days"},
		{"rediscover rating", "rediscover", map[string]interface{}{"min_rating": 6.0}, "min_rating"},
		{"rediscover no criterion", "rediscover", map[string]interface{}{"include_liked": false, "include_jizzed": false, "min_rating": 0.0}, "criterion"},
		{"on this day ok", "on_this_day", map[string]interface{}{"mode": "added"}, ""},
		{"on this day mode", "on_this_day", map[string]interface{}{"mode": "later"}, "mode"},
		{"folder needs a source", "folder_latest", map[string]interface{}{}, "storage path or a folder"},
		{"folder ok", "folder_latest", map[string]interface{}{"folder": "coomer-dl"}, ""},
		{"4k order", "showcase_4k", map[string]interface{}{"order": "oldest"}, "order"},
		{"4k height", "showcase_4k", map[string]interface{}{"min_height": 0.0}, "min_height"},
		{"long form nothing", "long_form", map[string]interface{}{"include_in_progress": false, "include_unwatched": false}, "include"},
		{"roulette ok", "roulette", map[string]interface{}{"unwatched_only": false}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := data.HomepageSection{Type: tt.widgetType, Config: tt.config}
			err := validateHomepageWidgetConfig(&section)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSceneWidgetFilterParsing(t *testing.T) {
	cfg := map[string]interface{}{
		"min_duration": 30.0, "max_duration": "120", "storage_path_id": 3.0, "folder": " coomer-dl ",
		"tags": "Pup, Solo", "min_height": -1.0,
	}
	svc := NewHomepageService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	svc.SetWidgetDeps(HomepageWidgetDeps{Permissions: fakePermissionChecker{PermFilesViewPaths: true}})
	f := sceneWidgetFilter(svc, 1, cfg)
	if f.MinDuration != 30 || f.MaxDuration != 120 || f.StoragePathID != 3 || f.Folder != "coomer-dl" ||
		len(f.Tags) != 2 || f.MinHeight != 0 {
		t.Fatalf("unexpected filter: %+v", f)
	}

	// without files:view_paths the path filters (path oracles) are dropped
	svc.SetWidgetDeps(HomepageWidgetDeps{Permissions: fakePermissionChecker{}})
	f = sceneWidgetFilter(svc, 1, cfg)
	if f.StoragePathID != 0 || f.Folder != "" || f.MinDuration != 30 {
		t.Fatalf("path filters must be dropped: %+v", f)
	}
}

func TestFolderLatestNeedsPaths(t *testing.T) {
	svc := NewHomepageService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	svc.SetWidgetDeps(HomepageWidgetDeps{Permissions: fakePermissionChecker{}})
	got, err := fetchFolderLatestWidget(svc, 1, data.HomepageSection{Type: "folder_latest", Config: map[string]interface{}{"folder": "x"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload, ok := got.Widget.(folderLatestPayload)
	if !ok || !payload.Forbidden || payload.Folder != "" || len(got.Scenes) != 0 {
		t.Fatalf("expected a forbidden empty widget, got %+v", got.Widget)
	}
}

func TestRediscoverOptionsDefaults(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	opts, days := rediscoverOptions(map[string]interface{}{}, now)
	if days != 180 || opts.MinRating != 4 || !opts.IncludeLiked || !opts.IncludeJizzed ||
		!opts.NotWatchedSince.Equal(now.AddDate(0, 0, -180)) {
		t.Fatalf("unexpected defaults: %+v, %d", opts, days)
	}
	opts, _ = rediscoverOptions(map[string]interface{}{"min_rating": 3.5}, now)
	if opts.MinRating != 3.5 {
		t.Fatalf("fractional rating lost: %v", opts.MinRating)
	}
}

func TestGroupOnThisDay(t *testing.T) {
	groups, ids := groupOnThisDay([]data.OnThisDayRow{
		{SceneID: 1, Year: 2025, Kind: "watched"},
		{SceneID: 2, Year: 2025, Kind: "added"},
		{SceneID: 1, Year: 2023, Kind: "watched"},
	})
	if len(groups) != 2 || groups[0].Year != 2025 || len(groups[0].Watched) != 1 || len(groups[0].Added) != 1 ||
		groups[1].Year != 2023 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("scene IDs must be distinct and ordered: %v", ids)
	}
	empty, _ := groupOnThisDay(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatal("empty result must be an empty slice")
	}
}

func TestPickRelatedScenesWithoutWatchedCheck(t *testing.T) {
	trashedAt := time.Now()
	candidates := []data.Scene{{ID: 1}, {ID: 2}, {ID: 3, TrashedAt: &trashedAt}, {ID: 4}, {ID: 5}}
	picked, err := pickRelatedScenes(nil, 7, 1, candidates, false, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(picked) != 2 || picked[0].ID != 2 || picked[1].ID != 4 {
		t.Fatalf("source and trashed scenes must be skipped, limit applied: %+v", picked)
	}
}

func TestSceneWidgetsWithoutDatabase(t *testing.T) {
	svc := &HomepageService{}
	section := data.HomepageSection{ID: "s", Type: "discover", Limit: 5, Config: map[string]interface{}{}}
	if _, err := fetchDiscoverWidget(svc, 1, section); err == nil {
		t.Fatal("expected an error without a database")
	}
	// because_you_watched degrades to an empty section without the related scenes service
	got, err := fetchBecauseYouWatchedWidget(svc, 1, section)
	if err != nil || len(got.Scenes) != 0 {
		t.Fatalf("expected an empty section, got %+v, %v", got, err)
	}
}
