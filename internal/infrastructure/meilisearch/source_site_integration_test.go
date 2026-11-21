package meilisearch

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	meili "github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

// TestSourceSiteFilters_Integration checks the source_site filters and facet against a real
// Meilisearch. Set GOONHUB_TEST_MEILISEARCH_URL (and GOONHUB_TEST_MEILISEARCH_KEY when the
// instance has a master key) to run it; it uses a throwaway index.
func TestSourceSiteFilters_Integration(t *testing.T) {
	host := os.Getenv("GOONHUB_TEST_MEILISEARCH_URL")
	if host == "" {
		t.Skip("GOONHUB_TEST_MEILISEARCH_URL not set")
	}
	indexName := fmt.Sprintf("test_source_site_%d", time.Now().UnixNano())
	client, err := NewClient(host, os.Getenv("GOONHUB_TEST_MEILISEARCH_KEY"), indexName, 1000, zap.NewNop())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() {
		if _, err := client.client.DeleteIndex(indexName); err != nil {
			t.Logf("failed to delete test index: %v", err)
		}
	})

	settings, err := client.client.Index(indexName).GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	filterable := map[string]bool{}
	for _, attr := range settings.FilterableAttributes {
		filterable[attr] = true
	}
	if !filterable["source_site"] {
		t.Fatalf("source_site is not filterable: %v", settings.FilterableAttributes)
	}
	if settings.Faceting == nil || settings.Faceting.MaxValuesPerFacet != maxValuesPerFacet {
		t.Fatalf("faceting = %+v, want maxValuesPerFacet %d", settings.Faceting, maxValuesPerFacet)
	}

	docs := []SceneDocument{
		{ID: 1, Title: "a", SourceSite: "pornhub.com"},
		{ID: 2, Title: "b", SourceSite: "pornhub.com"},
		{ID: 3, Title: "c", SourceSite: "xvideos.com"},
		{ID: 4, Title: "d", SourceSite: "lpsg.com"},
		{ID: 5, Title: "e", SourceSite: ""},
	}
	index := client.client.Index(indexName)
	waitTask := func(task *meili.TaskInfo, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("add documents: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		done, err := client.client.WaitForTask(task.TaskUID, meili.WaitParams{Context: ctx, Interval: 50 * time.Millisecond})
		if err != nil || done.Status != meili.TaskStatusSucceeded {
			t.Fatalf("task %d: %v %+v", task.TaskUID, err, done)
		}
	}
	waitTask(index.AddDocuments(docs, "id"))
	// A document indexed before source_site existed: no such attribute at all
	waitTask(index.AddDocuments([]map[string]interface{}{{"id": 6, "title": "f"}}, "id"))

	search := func(params SearchParams) []uint {
		t.Helper()
		params.Limit = 100
		result, err := client.Search(params)
		if err != nil {
			t.Fatalf("Search(%+v): %v", params, err)
		}
		sort.Slice(result.IDs, func(i, j int) bool { return result.IDs[i] < result.IDs[j] })
		return result.IDs
	}
	yes, no := true, false
	cases := []struct {
		name   string
		params SearchParams
		want   []uint
	}{
		{"one site", SearchParams{SourceSites: []string{"pornhub.com"}}, []uint{1, 2}},
		{"sites are OR-ed", SearchParams{SourceSites: []string{"pornhub.com", "lpsg.com"}}, []uint{1, 2, 4}},
		{"unknown site", SearchParams{SourceSites: []string{"nowhere.com"}}, []uint{}},
		{"has a source", SearchParams{HasSource: &yes}, []uint{1, 2, 3, 4}},
		{"no source", SearchParams{HasSource: &no}, []uint{5, 6}},
		{"site and has a source", SearchParams{SourceSites: []string{"xvideos.com"}, HasSource: &yes}, []uint{3}},
		{"site and no source", SearchParams{SourceSites: []string{"xvideos.com"}, HasSource: &no}, []uint{}},
	}
	for _, tc := range cases {
		if got := search(tc.params); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}

	facets, err := client.SourceSiteFacets()
	if err != nil {
		t.Fatalf("SourceSiteFacets: %v", err)
	}
	sort.Slice(facets, func(i, j int) bool { return facets[i].Value < facets[j].Value })
	want := []FacetCount{{Value: "lpsg.com", Count: 1}, {Value: "pornhub.com", Count: 2}, {Value: "xvideos.com", Count: 1}}
	if !reflect.DeepEqual(facets, want) {
		t.Fatalf("facets = %+v, want %+v", facets, want)
	}
}
