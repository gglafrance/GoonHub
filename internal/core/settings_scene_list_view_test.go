package core

import (
	"fmt"
	"slices"
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

func TestWithSceneListView(t *testing.T) {
	tests := []struct {
		name        string
		view        data.SceneListView
		wantPages   map[string]string
		wantColumns []string
		wantErr     bool
	}{
		{
			name:        "valid",
			view:        data.SceneListView{Pages: map[string]string{"search": "table", "actor": "grid"}, Columns: []string{"title", "release_date"}},
			wantPages:   map[string]string{"search": "table", "actor": "grid"},
			wantColumns: []string{"title", "release_date"},
		},
		{
			name:        "empty columns use the defaults",
			view:        data.SceneListView{Pages: map[string]string{"explorer": "table"}},
			wantPages:   map[string]string{"explorer": "table"},
			wantColumns: data.DefaultSceneListViewColumns(),
		},
		{
			name:        "nil pages",
			view:        data.SceneListView{Columns: []string{"thumbnail", "title"}},
			wantPages:   map[string]string{},
			wantColumns: []string{"thumbnail", "title"},
		},
		{name: "unknown page", view: data.SceneListView{Pages: map[string]string{"markers": "table"}}, wantErr: true},
		{name: "unknown mode", view: data.SceneListView{Pages: map[string]string{"search": "list"}}, wantErr: true},
		{name: "unknown column", view: data.SceneListView{Columns: []string{"title", "codec"}}, wantErr: true},
		{name: "duplicate column", view: data.SceneListView{Columns: []string{"title", "title"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &data.UserSettings{SceneListView: data.DefaultSceneListView()}
			view := tt.view
			err := WithSceneListView(&view)(settings)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(settings.SceneListView.Pages) != len(tt.wantPages) {
				t.Fatalf("pages = %v, want %v", settings.SceneListView.Pages, tt.wantPages)
			}
			for page, mode := range tt.wantPages {
				if settings.SceneListView.Pages[page] != mode {
					t.Fatalf("pages = %v, want %v", settings.SceneListView.Pages, tt.wantPages)
				}
			}
			if !slices.Equal(settings.SceneListView.Columns, tt.wantColumns) {
				t.Fatalf("columns = %v, want %v", settings.SceneListView.Columns, tt.wantColumns)
			}
		})
	}
}

func TestWithSceneListView_NilKeepsCurrent(t *testing.T) {
	current := data.SceneListView{Pages: map[string]string{"studio": "table"}, Columns: []string{"title"}}
	settings := &data.UserSettings{SceneListView: current}
	if err := WithSceneListView(nil)(settings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.SceneListView.Pages["studio"] != "table" || !slices.Equal(settings.SceneListView.Columns, []string{"title"}) {
		t.Fatalf("view changed: %+v", settings.SceneListView)
	}
}

func TestGetSettings_DefaultSceneListView(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, fmt.Errorf("record not found"))

	settings, err := svc.GetSettings(1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(settings.SceneListView.Pages) != 0 || !slices.Equal(settings.SceneListView.Columns, data.DefaultSceneListViewColumns()) {
		t.Fatalf("unexpected default scene list view: %+v", settings.SceneListView)
	}
}

func TestUpdateAllSettings_SceneListView(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{
		UserID:          1,
		HomepageConfig:  data.DefaultHomepageConfig(),
		ParsingRules:    data.DefaultParsingRulesSettings(),
		SortPreferences: data.DefaultSortPreferences(),
		SceneListView:   data.DefaultSceneListView(),
	}, nil)
	settingsRepo.EXPECT().Upsert(gomock.Any()).Return(nil)

	autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
	view := &data.SceneListView{Pages: map[string]string{"playlist": "table"}, Columns: []string{"title", "duration"}}
	settings, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "grouped", scc, WithSceneListView(view))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if settings.SceneListView.Pages["playlist"] != "table" || !slices.Equal(settings.SceneListView.Columns, []string{"title", "duration"}) {
		t.Fatalf("scene list view not saved: %+v", settings.SceneListView)
	}
}

func TestUpdateAllSettings_InvalidSceneListView(t *testing.T) {
	svc, settingsRepo, _ := newTestSettingsService(t)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{UserID: 1}, nil)

	autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
	view := &data.SceneListView{Pages: map[string]string{"search": "mosaic"}}
	if _, err := svc.UpdateAllSettings(1, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "grouped", scc, WithSceneListView(view)); err == nil {
		t.Fatal("expected an error for an invalid view mode")
	}
}
