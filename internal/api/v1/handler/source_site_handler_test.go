package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// bindSearchQuery binds a scene search query string like ListScenes does.
func bindSearchQuery(t *testing.T, rawQuery string) request.SearchScenesRequest {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/scenes?"+rawQuery, nil)
	var req request.SearchScenesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		t.Fatalf("bind %q: %v", rawQuery, err)
	}
	return req
}

func TestSearchParamsFromRequest_SourceFilters(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		sites     []string
		hasSource *bool
	}{
		{name: "none", query: ""},
		{name: "one site", query: "source_site=pornhub.com", sites: []string{"pornhub.com"}},
		{
			name:  "normalized and deduplicated",
			query: "source_site=www.PornHub.com,fr.xvideos.com,,pornhub.com",
			sites: []string{"pornhub.com", "xvideos.com"},
		},
		{name: "has a source", query: "has_source=true", hasSource: boolPtr(true)},
		{
			name:      "site and no source",
			query:     "source_site=lpsg.com&has_source=false",
			sites:     []string{"lpsg.com"},
			hasSource: boolPtr(false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := bindSearchQuery(t, tt.query)
			params := searchParamsFromRequest(&req, 3)
			if !reflect.DeepEqual(params.SourceSites, tt.sites) {
				t.Fatalf("SourceSites = %#v, want %#v", params.SourceSites, tt.sites)
			}
			if !reflect.DeepEqual(params.HasSource, tt.hasSource) {
				t.Fatalf("HasSource = %v, want %v", params.HasSource, tt.hasSource)
			}
			if params.UserID != 3 {
				t.Fatalf("UserID = %d, want 3", params.UserID)
			}
		})
	}
}

func TestSearchParamsFromRequest_InvalidHasSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/scenes?has_source=maybe", nil)
	var req request.SearchScenesRequest
	if err := c.ShouldBindQuery(&req); err == nil {
		t.Fatal("expected has_source=maybe to be rejected")
	}
}

func TestListSourceSites_WithoutMeilisearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &SceneHandler{
		SearchService: core.NewSearchService(nil, nil, nil, nil, nil, nil, zap.NewNop()),
	}
	router := gin.New()
	router.GET("/scenes/source-sites", handler.ListSourceSites)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/scenes/source-sites", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 without Meilisearch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestToSourceSiteList(t *testing.T) {
	out, err := json.Marshal(response.ToSourceSiteList([]core.SourceSiteCount{{Site: "pornhub.com", Count: 4}}))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"sites":[{"site":"pornhub.com","count":4}]}` {
		t.Fatalf("unexpected JSON %s", out)
	}
	out, _ = json.Marshal(response.ToSourceSiteList(nil))
	if string(out) != `{"sites":[]}` {
		t.Fatalf("an empty list must not be null, got %s", out)
	}
}

func TestGetScene_IncludesSourceSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, sceneRepo, _ := newTestSceneHandler(t)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID: 1, Title: "T", SourceURL: "https://fr.pornhub.com/view_video.php?viewkey=x",
	}, nil)

	router := gin.New()
	router.GET("/scenes/:id", handler.GetScene)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/scenes/1", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if out["source_site"] != "pornhub.com" || out["title"] != "T" ||
		out["source_url"] != "https://fr.pornhub.com/view_video.php?viewkey=x" {
		t.Fatalf("unexpected scene JSON: %v", out)
	}
}

// A saved search keeps its source filters through the API request, the JSONB column and the
// API answer; sites are normalized on the way in.
func TestSavedSearchFilters_SourceRoundTrip(t *testing.T) {
	var in request.SavedSearchFilters
	body := `{"source_sites":["www.PornHub.com","xvideos.com","pornhub.com",""],"has_source":true}`
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		t.Fatal(err)
	}

	stored := requestFiltersToData(in)
	value, err := stored.Value()
	if err != nil {
		t.Fatal(err)
	}
	var loaded data.Filters
	if err := loaded.Scan(value); err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(response.NewSavedSearchResponse(&data.SavedSearch{Name: "s", Filters: loaded}))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Filters struct {
			SourceSites []string `json:"source_sites"`
			HasSource   *bool    `json:"has_source"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Filters.SourceSites, []string{"pornhub.com", "xvideos.com"}) {
		t.Fatalf("source_sites = %#v", got.Filters.SourceSites)
	}
	if got.Filters.HasSource == nil || !*got.Filters.HasSource {
		t.Fatalf("has_source = %v, want true", got.Filters.HasSource)
	}

	// Without source filters nothing is stored
	empty, _ := requestFiltersToData(request.SavedSearchFilters{}).Value()
	if string(empty.([]byte)) != "{}" {
		t.Fatalf("expected empty filters, got %s", empty)
	}
}

func boolPtr(v bool) *bool { return &v }
