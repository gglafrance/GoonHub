package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"goonhub/internal/api/middleware"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
)

type fakeShortsSearcher struct {
	got    data.SceneSearchParams
	calls  int
	result *core.SearchResult
}

func (f *fakeShortsSearcher) Search(params data.SceneSearchParams) (*core.SearchResult, error) {
	f.calls++
	f.got = params
	if f.result != nil {
		return f.result, nil
	}
	return &core.SearchResult{Scenes: []data.Scene{}}, nil
}

type fakeShortsSettings struct {
	maxDuration int
	canCreate   bool
	updateErr   error
	gotMax      int
	gotDir      *string
	gotSearch   string
}

func (f *fakeShortsSettings) GetMaxDuration() (int, error)      { return f.maxDuration, nil }
func (f *fakeShortsSettings) GetSearchDefault() (string, error) { return "hide", nil }
func (f *fakeShortsSettings) CanCreate() bool                   { return f.canCreate }
func (f *fakeShortsSettings) GetSettings() (*core.ShortsSettings, error) {
	return &core.ShortsSettings{MaxDuration: f.maxDuration}, nil
}
func (f *fakeShortsSettings) UpdateSettings(maxDuration int, saveDir *string, searchDefault string) (*core.ShortsSettings, error) {
	f.gotMax, f.gotDir, f.gotSearch = maxDuration, saveDir, searchDefault
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if !core.IsValidShortsMaxDuration(maxDuration) {
		return nil, apperrors.NewValidationErrorWithField("max_duration", "max_duration must be a whole number of seconds from 1")
	}
	return &core.ShortsSettings{MaxDuration: maxDuration}, nil
}

type fakeShortCreator struct {
	err     error
	gotID   uint
	gotFrom float64
	gotTo   float64
}

func (f *fakeShortCreator) CreateShort(sourceID uint, start, end float64, title string) (*core.ShortClipTask, error) {
	f.gotID, f.gotFrom, f.gotTo = sourceID, start, end
	if f.err != nil {
		return nil, f.err
	}
	return &core.ShortClipTask{TaskID: "task-1", SourceSceneID: sourceID}, nil
}

func (f *fakeShortCreator) ListTasks(sourceID uint) []core.ShortClipTask { return nil }

func newTestShortsHandler(maxDuration int) (*ShortsHandler, *fakeShortsSearcher, *fakeShortsSettings, *fakeShortCreator) {
	search := &fakeShortsSearcher{}
	settings := &fakeShortsSettings{maxDuration: maxDuration, canCreate: true}
	creator := &fakeShortCreator{}
	return &ShortsHandler{
		Search:          search,
		Settings:        settings,
		Creator:         creator,
		MaxItemsPerPage: 100,
	}, search, settings, creator
}

func doRequest(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestListShorts_ParamsFromSettingsAndQuery(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		maxDur     int
		wantSort   string
		wantIsClip *bool
		wantSeed   int64
	}{
		{"defaults", "", 60, "created_at_desc", nil, 0},
		{"newest", "?sort=newest", 60, "created_at_desc", nil, 0},
		{"random keeps seed", "?sort=random&seed=12345", 120, "random", nil, 12345},
		{"only clips", "?clips=only", 60, "created_at_desc", shortsBoolPtr(true), 0},
		{"hide clips", "?clips=hide", 60, "created_at_desc", shortsBoolPtr(false), 0},
		{"all clips", "?clips=all", 60, "created_at_desc", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, search, _, _ := newTestShortsHandler(tt.maxDur)
			router := gin.New()
			router.GET("/shorts", h.ListShorts)

			w := doRequest(router, "GET", "/shorts"+tt.query, nil)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
			}

			p := search.got
			if p.MinDuration != 1 || p.MaxDuration != tt.maxDur {
				t.Fatalf("duration range = %d-%d, want 1-%d", p.MinDuration, p.MaxDuration, tt.maxDur)
			}
			if p.Sort != tt.wantSort || p.Seed != tt.wantSeed {
				t.Fatalf("sort/seed = %q/%d", p.Sort, p.Seed)
			}
			if (p.IsClip == nil) != (tt.wantIsClip == nil) || (p.IsClip != nil && *p.IsClip != *tt.wantIsClip) {
				t.Fatalf("IsClip = %v, want %v", p.IsClip, tt.wantIsClip)
			}
			if p.Page != 1 || p.Limit != 10 {
				t.Fatalf("page/limit = %d/%d", p.Page, p.Limit)
			}

			var body map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if body["max_duration"] != float64(tt.maxDur) {
				t.Fatalf("max_duration in response = %v", body["max_duration"])
			}
		})
	}
}

func TestListShorts_ReturnsSeedAndItems(t *testing.T) {
	h, search, _, _ := newTestShortsHandler(60)
	src := uint(3)
	start, end := 1.0, 9.0
	search.result = &core.SearchResult{
		Scenes: []data.Scene{{ID: 8, Title: "clip", Duration: 8, Width: 1080, Height: 1920, SourceSceneID: &src, SourceStart: &start, SourceEnd: &end}},
		Total:  1,
		Seed:   77,
	}
	router := gin.New()
	router.GET("/shorts", h.ListShorts)

	w := doRequest(router, "GET", "/shorts?sort=random", nil)
	var body struct {
		Data []struct {
			ID            uint     `json:"id"`
			Width         int      `json:"width"`
			Height        int      `json:"height"`
			SourceSceneID *uint    `json:"source_scene_id"`
			SourceStart   *float64 `json:"source_start"`
		} `json:"data"`
		Seed  int64 `json:"seed"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Seed != 77 || body.Total != 1 || len(body.Data) != 1 {
		t.Fatalf("body = %+v", body)
	}
	item := body.Data[0]
	if item.ID != 8 || item.Width != 1080 || item.Height != 1920 || item.SourceSceneID == nil || *item.SourceSceneID != 3 || *item.SourceStart != 1 {
		t.Fatalf("item = %+v", item)
	}
}

func TestListShorts_BadParams(t *testing.T) {
	for _, q := range []string{"?sort=oldest", "?clips=maybe"} {
		h, search, _, _ := newTestShortsHandler(60)
		router := gin.New()
		router.GET("/shorts", h.ListShorts)

		w := doRequest(router, "GET", "/shorts"+q, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", q, w.Code)
		}
		if search.calls != 0 {
			t.Fatalf("%s: search ran for bad params", q)
		}
	}
}

type fakePermissions map[string]bool

func (f fakePermissions) HasPermission(role, permission string) bool {
	return f[role+"/"+permission]
}

func TestGetShortsConfig(t *testing.T) {
	for _, tt := range []struct {
		role       string
		wantUpload bool
	}{{"uploader", true}, {"viewer", false}} {
		h, _, settings, _ := newTestShortsHandler(120)
		settings.canCreate = false
		h.RBAC = fakePermissions{"uploader/scenes:upload": true}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user", &core.UserPayload{UserID: 1, Role: tt.role})
			c.Next()
		})
		router.GET("/shorts/config", h.GetConfig)

		w := doRequest(router, "GET", "/shorts/config", nil)
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != 200 || body["max_duration"] != float64(120) || body["can_create"] != false || body["can_upload"] != tt.wantUpload || body["search_default"] != "hide" {
			t.Fatalf("%s: status %d body %v", tt.role, w.Code, body)
		}
	}
}

func TestCreateShort_Statuses(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       any
		creatorErr error
		want       int
	}{
		{"accepted", "/scenes/42/shorts", map[string]any{"start": 10.5, "end": 20}, nil, http.StatusAccepted},
		{"start at zero is accepted", "/scenes/42/shorts", map[string]any{"start": 0, "end": 20}, nil, http.StatusAccepted},
		{"missing end", "/scenes/42/shorts", map[string]any{"start": 10}, nil, http.StatusBadRequest},
		{"bad id", "/scenes/abc/shorts", map[string]any{"start": 1, "end": 2}, nil, http.StatusBadRequest},
		{"invalid range", "/scenes/42/shorts", map[string]any{"start": 10, "end": 10.2}, apperrors.NewValidationError("the short must be at least 1 second long"), http.StatusBadRequest},
		{"source missing", "/scenes/42/shorts", map[string]any{"start": 1, "end": 5}, apperrors.ErrSceneNotFound(42), http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _, creator := newTestShortsHandler(60)
			creator.err = tt.creatorErr
			router := gin.New()
			router.POST("/scenes/:id/shorts", h.CreateShort)

			w := doRequest(router, "POST", tt.path, tt.body)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d, body %s", w.Code, tt.want, w.Body.String())
			}
			if tt.want == http.StatusAccepted {
				var body map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &body)
				if body["task_id"] != "task-1" || creator.gotID != 42 {
					t.Fatalf("body = %v, id = %d", body, creator.gotID)
				}
			}
		})
	}
}

func TestListSceneShorts(t *testing.T) {
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	h, _, _, _ := newTestShortsHandler(60)
	h.SceneRepo = sceneRepo

	start := 12.0
	sceneRepo.EXPECT().ListBySourceScene(uint(42), 1, 50).Return([]data.Scene{{ID: 5, SourceStart: &start}}, int64(1), nil)

	router := gin.New()
	router.GET("/scenes/:id/shorts", h.ListSceneShorts)
	w := doRequest(router, "GET", "/scenes/42/shorts", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Data    []map[string]any `json:"data"`
		Total   int64            `json:"total"`
		Pending []any            `json:"pending"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Total != 1 || len(body.Data) != 1 || body.Pending == nil {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestShortsSettingsEndpoints(t *testing.T) {
	newRouter := func(role string) (*gin.Engine, *fakeShortsSettings) {
		h, _, settings, _ := newTestShortsHandler(60)
		router := gin.New()
		admin := router.Group("/admin")
		admin.Use(func(c *gin.Context) {
			c.Set("user", &core.UserPayload{UserID: 1, Role: role})
			c.Next()
		})
		admin.Use(middleware.RequireRole("admin"))
		admin.GET("/shorts-settings", h.GetShortsSettings)
		admin.PUT("/shorts-settings", h.UpdateShortsSettings)
		return router, settings
	}

	for _, d := range []int{60, 120, 45, 90, 600} {
		router, settings := newRouter("admin")
		w := doRequest(router, "PUT", "/admin/shorts-settings", map[string]any{"max_duration": d, "save_dir": nil})
		if w.Code != 200 || settings.gotMax != d || settings.gotDir != nil {
			t.Fatalf("max_duration %d: status %d", d, w.Code)
		}
	}

	for _, d := range []int{-1, 0} {
		router, _ := newRouter("admin")
		w := doRequest(router, "PUT", "/admin/shorts-settings", map[string]any{"max_duration": d})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("max_duration %d: status %d, want 400", d, w.Code)
		}
	}

	t.Run("search_default passed through", func(t *testing.T) {
		router, settings := newRouter("admin")
		w := doRequest(router, "PUT", "/admin/shorts-settings", map[string]any{"max_duration": 60, "search_default": "only_clips"})
		if w.Code != 200 || settings.gotSearch != "only_clips" {
			t.Fatalf("status %d search %q", w.Code, settings.gotSearch)
		}
	})

	t.Run("save_dir passed through", func(t *testing.T) {
		router, settings := newRouter("admin")
		w := doRequest(router, "PUT", "/admin/shorts-settings", map[string]any{"max_duration": 60, "save_dir": "/srv/clips"})
		if w.Code != 200 || settings.gotDir == nil || *settings.gotDir != "/srv/clips" {
			t.Fatalf("status %d dir %v", w.Code, settings.gotDir)
		}
	})

	t.Run("unwritable folder is a 400", func(t *testing.T) {
		router, settings := newRouter("admin")
		settings.updateErr = apperrors.NewValidationErrorWithField("save_dir", "not writable")
		w := doRequest(router, "PUT", "/admin/shorts-settings", map[string]any{"max_duration": 60, "save_dir": "/ro"})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", w.Code)
		}
	})

	t.Run("non-admin is forbidden", func(t *testing.T) {
		router, settings := newRouter("user")
		if w := doRequest(router, "GET", "/admin/shorts-settings", nil); w.Code != http.StatusForbidden {
			t.Fatalf("GET status %d, want 403", w.Code)
		}
		if w := doRequest(router, "PUT", "/admin/shorts-settings", map[string]any{"max_duration": 60}); w.Code != http.StatusForbidden {
			t.Fatalf("PUT status %d, want 403", w.Code)
		}
		if settings.gotMax != 0 {
			t.Fatal("settings changed by a non-admin")
		}
	})
}

func TestParseClipsFilter(t *testing.T) {
	for _, v := range []string{"", "all"} {
		if b, err := parseClipsFilter(v); err != nil || b != nil {
			t.Fatalf("%q: %v %v", v, b, err)
		}
	}
	if b, err := parseClipsFilter("only"); err != nil || b == nil || !*b {
		t.Fatal("only")
	}
	if b, err := parseClipsFilter("hide"); err != nil || b == nil || *b {
		t.Fatal("hide")
	}
	if _, err := parseClipsFilter("x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseShortsSearchFilter(t *testing.T) {
	h := &SceneHandler{ShortsSettings: &fakeShortsSettings{maxDuration: 90}}
	for _, v := range []string{"", "all"} {
		if b, s, err := h.parseShortsSearchFilter(v); err != nil || b != nil || s != nil {
			t.Fatalf("%q: %v %v %v", v, b, s, err)
		}
	}
	if b, s, err := h.parseShortsSearchFilter("only_clips"); err != nil || b == nil || !*b || s != nil {
		t.Fatal("only_clips")
	}
	if b, s, err := h.parseShortsSearchFilter("hide_clips"); err != nil || b == nil || *b || s != nil {
		t.Fatal("hide_clips")
	}
	if b, s, err := h.parseShortsSearchFilter("only"); err != nil || b != nil || s == nil || !s.Only || s.MaxDuration != 90 {
		t.Fatalf("only: %v %+v %v", b, s, err)
	}
	if b, s, err := h.parseShortsSearchFilter("hide"); err != nil || b != nil || s == nil || s.Only || s.MaxDuration != 90 {
		t.Fatalf("hide: %v %+v %v", b, s, err)
	}
	if _, _, err := h.parseShortsSearchFilter("x"); !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func shortsBoolPtr(b bool) *bool { return &b }
