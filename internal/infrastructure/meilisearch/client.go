package meilisearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	meili "github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

// maxValuesPerFacet bounds the values returned per facet (the source sites listed for the
// filter UI). Meilisearch's default of 100 would hide sites on larger libraries.
const maxValuesPerFacet = 1000

// Client wraps the Meilisearch client with application-specific functionality.
type Client struct {
	client       *meili.Client
	indexName    string
	logger       *zap.Logger
	maxTotalHits int64
}

// NewClient creates a new Meilisearch client wrapper.
func NewClient(host, apiKey, indexName string, maxTotalHits int64, logger *zap.Logger) (*Client, error) {
	client := meili.NewClient(meili.ClientConfig{
		Host:   host,
		APIKey: apiKey,
	})

	if maxTotalHits <= 0 {
		maxTotalHits = 100000
	}

	c := &Client{
		client:       client,
		indexName:    indexName,
		logger:       logger,
		maxTotalHits: maxTotalHits,
	}

	// Verify connection and ensure index exists
	if err := c.EnsureIndex(); err != nil {
		return nil, fmt.Errorf("failed to ensure index: %w", err)
	}

	return c, nil
}

// EnsureIndex creates the scenes index if it doesn't exist and configures settings.
func (c *Client) EnsureIndex() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create index if not exists
	_, err := c.client.CreateIndex(&meili.IndexConfig{
		Uid:        c.indexName,
		PrimaryKey: "id",
	})
	if err != nil {
		// Ignore "index already exists" error
		if !strings.Contains(err.Error(), "index_already_exists") {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	index := c.client.Index(c.indexName)

	// Configure searchable attributes
	searchableTask, err := index.UpdateSearchableAttributes(&[]string{
		"title",
		"original_filename",
		"path",
		"description",
		"actors",
		"actor_aliases",
		"tag_names",
	})
	if err != nil {
		return fmt.Errorf("failed to update searchable attributes: %w", err)
	}
	if _, err := c.client.WaitForTask(searchableTask.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for searchable attributes task: %w", err)
	}

	// Configure filterable attributes
	filterableTask, err := index.UpdateFilterableAttributes(&[]string{
		"studio",
		"actors",
		"tag_ids",
		"duration",
		"size",
		"height",
		"created_at",
		"processing_status",
		"id",
		"source_site",
	})
	if err != nil {
		return fmt.Errorf("failed to update filterable attributes: %w", err)
	}
	if _, err := c.client.WaitForTask(filterableTask.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for filterable attributes task: %w", err)
	}

	// Configure sortable attributes
	sortableTask, err := index.UpdateSortableAttributes(&[]string{
		"created_at",
		"title",
		"duration",
		"size",
		"view_count",
	})
	if err != nil {
		return fmt.Errorf("failed to update sortable attributes: %w", err)
	}
	if _, err := c.client.WaitForTask(sortableTask.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for sortable attributes task: %w", err)
	}

	// Configure faceting (filterable attributes are facetable: source_site is listed by facet)
	facetingTask, err := index.UpdateFaceting(&meili.Faceting{
		MaxValuesPerFacet: maxValuesPerFacet,
	})
	if err != nil {
		return fmt.Errorf("failed to update faceting settings: %w", err)
	}
	if _, err := c.client.WaitForTask(facetingTask.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for faceting settings task: %w", err)
	}

	// Configure pagination (maxTotalHits)
	paginationTask, err := index.UpdatePagination(&meili.Pagination{
		MaxTotalHits: c.maxTotalHits,
	})
	if err != nil {
		return fmt.Errorf("failed to update pagination settings: %w", err)
	}
	if _, err := c.client.WaitForTask(paginationTask.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for pagination settings task: %w", err)
	}

	c.logger.Info("meilisearch index configured", zap.String("index", c.indexName), zap.Int64("max_total_hits", c.maxTotalHits))
	return nil
}

// UpdateMaxTotalHits updates the pagination maxTotalHits setting on the Meilisearch index.
func (c *Client) UpdateMaxTotalHits(maxTotalHits int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	index := c.client.Index(c.indexName)
	task, err := index.UpdatePagination(&meili.Pagination{
		MaxTotalHits: maxTotalHits,
	})
	if err != nil {
		return fmt.Errorf("failed to update pagination settings: %w", err)
	}

	if _, err := c.client.WaitForTask(task.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for pagination settings task: %w", err)
	}

	c.maxTotalHits = maxTotalHits
	c.logger.Info("updated meilisearch maxTotalHits", zap.Int64("max_total_hits", maxTotalHits))
	return nil
}

// GetMaxTotalHits returns the current maxTotalHits setting.
func (c *Client) GetMaxTotalHits() int64 {
	return c.maxTotalHits
}

// IndexScene adds or updates a scene document in the index.
// Fire-and-forget: Meilisearch processes the task asynchronously.
func (c *Client) IndexScene(doc SceneDocument) error {
	index := c.client.Index(c.indexName)
	if _, err := index.AddDocuments([]SceneDocument{doc}, "id"); err != nil {
		return fmt.Errorf("failed to index scene: %w", err)
	}

	c.logger.Debug("indexed scene", zap.Uint("id", doc.ID), zap.String("title", doc.Title))
	return nil
}

// UpdateScene updates an existing scene document in the index.
func (c *Client) UpdateScene(doc SceneDocument) error {
	return c.IndexScene(doc) // Meilisearch upserts automatically
}

// DeleteScene removes a scene document from the index.
// Fire-and-forget: Meilisearch processes the task asynchronously.
func (c *Client) DeleteScene(id uint) error {
	index := c.client.Index(c.indexName)
	if _, err := index.DeleteDocument(fmt.Sprintf("%d", id)); err != nil {
		return fmt.Errorf("failed to delete scene: %w", err)
	}

	c.logger.Debug("deleted scene from index", zap.Uint("id", id))
	return nil
}

// BulkDeleteScenes removes multiple scene documents from the index in a single request.
// Fire-and-forget: Meilisearch processes the task asynchronously.
func (c *Client) BulkDeleteScenes(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = fmt.Sprintf("%d", id)
	}

	index := c.client.Index(c.indexName)
	if _, err := index.DeleteDocuments(strIDs); err != nil {
		return fmt.Errorf("failed to bulk delete scenes: %w", err)
	}

	c.logger.Debug("bulk deleted scenes from index", zap.Int("count", len(ids)))
	return nil
}

// BulkIndex adds multiple scene documents to the index.
// Fire-and-forget: Meilisearch processes the task asynchronously.
func (c *Client) BulkIndex(docs []SceneDocument) error {
	if len(docs) == 0 {
		return nil
	}

	index := c.client.Index(c.indexName)
	if _, err := index.AddDocuments(docs, "id"); err != nil {
		return fmt.Errorf("failed to bulk index: %w", err)
	}

	c.logger.Info("bulk indexed scenes", zap.Int("count", len(docs)))
	return nil
}

// Search performs a search query and returns matching scene IDs with total count.
func (c *Client) Search(params SearchParams) (*SearchResult, error) {
	index := c.client.Index(c.indexName)

	// Build filter string
	filters := c.buildFilters(params)

	// Build sort array
	sort := c.buildSort(params)

	searchReq := &meili.SearchRequest{
		AttributesToRetrieve: []string{"id"},
		ShowMatchesPosition:  false,
	}

	if params.FetchAllIDs {
		// Fetch all matching IDs: use maxTotalHits as limit, no offset, no sort
		searchReq.Limit = c.maxTotalHits
		searchReq.Offset = 0
		sort = nil
	} else {
		searchReq.Limit = int64(params.Limit)
		searchReq.Offset = int64(params.Offset)
	}

	if len(filters) > 0 {
		searchReq.Filter = filters
	}

	if len(sort) > 0 {
		searchReq.Sort = sort
	}

	if params.MatchingStrategy != "" {
		searchReq.MatchingStrategy = params.MatchingStrategy
	}

	result, err := index.Search(params.Query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Extract IDs from hits
	ids := make([]uint, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if m, ok := hit.(map[string]interface{}); ok {
			if id, ok := m["id"].(float64); ok {
				ids = append(ids, uint(id))
			}
		}
	}

	return &SearchResult{
		IDs:        ids,
		TotalCount: result.EstimatedTotalHits,
	}, nil
}

// SourceSiteFacets returns the source sites of the indexed scenes with their scene counts,
// from the source_site facet. Scenes without a source are left out.
func (c *Client) SourceSiteFacets() ([]FacetCount, error) {
	index := c.client.Index(c.indexName)
	result, err := index.Search("", &meili.SearchRequest{
		AttributesToRetrieve: []string{"id"},
		Limit:                1, // only the facet distribution is used (0 means the default 20)
		Facets:               []string{"source_site"},
	})
	if err != nil {
		return nil, fmt.Errorf("source site facet search failed: %w", err)
	}
	return parseFacetDistribution(result.FacetDistribution, "source_site"), nil
}

// parseFacetDistribution reads one facet of a search's facetDistribution
// ({"facet": {"value": count}}), skipping empty values.
func parseFacetDistribution(distribution interface{}, facet string) []FacetCount {
	facets, ok := distribution.(map[string]interface{})
	if !ok {
		return nil
	}
	values, ok := facets[facet].(map[string]interface{})
	if !ok {
		return nil
	}
	counts := make([]FacetCount, 0, len(values))
	for value, raw := range values {
		if value == "" {
			continue
		}
		var count int64
		switch n := raw.(type) {
		case float64:
			count = int64(n)
		case int64:
			count = n
		case int:
			count = int64(n)
		case json.Number:
			parsed, err := n.Int64()
			if err != nil {
				continue
			}
			count = parsed
		default:
			continue
		}
		if count > 0 {
			counts = append(counts, FacetCount{Value: value, Count: count})
		}
	}
	return counts
}

// buildFilters constructs the filter string for Meilisearch.
func (c *Client) buildFilters(params SearchParams) []string {
	var filters []string

	// Tag filter (AND logic - must have all specified tags)
	for _, tagID := range params.TagIDs {
		filters = append(filters, fmt.Sprintf("tag_ids = %d", tagID))
	}

	// Actor filter (OR logic - must have at least one specified actor)
	if len(params.Actors) > 0 {
		actorFilters := make([]string, len(params.Actors))
		for i, actor := range params.Actors {
			actorFilters[i] = fmt.Sprintf("actors = \"%s\"", escapeFilterValue(actor))
		}
		filters = append(filters, "("+strings.Join(actorFilters, " OR ")+")")
	}

	// Studio filter
	if params.Studio != "" {
		filters = append(filters, fmt.Sprintf("studio = \"%s\"", escapeFilterValue(params.Studio)))
	}

	// Duration range
	if params.MinDuration != nil {
		filters = append(filters, fmt.Sprintf("duration >= %f", *params.MinDuration))
	}
	if params.MaxDuration != nil {
		filters = append(filters, fmt.Sprintf("duration <= %f", *params.MaxDuration))
	}

	// Metadata presence
	for _, presence := range []struct {
		field string
		value *bool
	}{
		{"actors", params.HasActors},
		{"tag_ids", params.HasTags},
		{"studio", params.HasStudio},
	} {
		if presence.value == nil {
			continue
		}
		if *presence.value {
			filters = append(filters, presence.field+" IS NOT EMPTY")
		} else {
			filters = append(filters, presence.field+" IS EMPTY")
		}
	}

	// Source site (OR logic - from one of the specified sites)
	if len(params.SourceSites) > 0 {
		sites := make([]string, len(params.SourceSites))
		for i, site := range params.SourceSites {
			sites[i] = fmt.Sprintf("\"%s\"", escapeFilterValue(site))
		}
		filters = append(filters, "source_site IN ["+strings.Join(sites, ", ")+"]")
	}

	// Source presence. Documents indexed before source_site existed have no such attribute:
	// they count as "no source" (IS NOT EMPTY alone would match a missing attribute).
	if params.HasSource != nil {
		if *params.HasSource {
			filters = append(filters, "(source_site EXISTS AND source_site IS NOT EMPTY)")
		} else {
			filters = append(filters, "(source_site NOT EXISTS OR source_site IS EMPTY)")
		}
	}

	// Excluded scenes
	if len(params.ExcludeSceneIDs) > 0 {
		ids := make([]string, len(params.ExcludeSceneIDs))
		for i, id := range params.ExcludeSceneIDs {
			ids[i] = strconv.FormatUint(uint64(id), 10)
		}
		filters = append(filters, "id NOT IN ["+strings.Join(ids, ", ")+"]")
	}

	// File size range (bytes)
	if params.MinSize != nil {
		filters = append(filters, fmt.Sprintf("size >= %d", *params.MinSize))
	}
	if params.MaxSize != nil {
		filters = append(filters, fmt.Sprintf("size <= %d", *params.MaxSize))
	}

	// Height range
	if params.MinHeight != nil {
		filters = append(filters, fmt.Sprintf("height >= %d", *params.MinHeight))
	}
	if params.MaxHeight != nil {
		filters = append(filters, fmt.Sprintf("height <= %d", *params.MaxHeight))
	}

	// Date range
	if params.DateAfter != nil {
		filters = append(filters, fmt.Sprintf("created_at >= %d", *params.DateAfter))
	}
	if params.DateBefore != nil {
		filters = append(filters, fmt.Sprintf("created_at <= %d", *params.DateBefore))
	}

	// Processing status
	if params.ProcessingStatus != "" {
		filters = append(filters, fmt.Sprintf("processing_status = \"%s\"", params.ProcessingStatus))
	}

	// Pre-filtered scene IDs (for user-specific filters)
	if len(params.SceneIDs) > 0 {
		idStrs := make([]string, len(params.SceneIDs))
		for i, id := range params.SceneIDs {
			idStrs[i] = fmt.Sprintf("id = %d", id)
		}
		filters = append(filters, "("+strings.Join(idStrs, " OR ")+")")
	}

	return filters
}

// buildSort constructs the sort array for Meilisearch.
func (c *Client) buildSort(params SearchParams) []string {
	if params.Sort == "" {
		return nil
	}

	// Map frontend sort fields to Meilisearch fields
	sortField := params.Sort
	switch sortField {
	case "date", "created_at":
		sortField = "created_at"
	case "title", "name":
		sortField = "title"
	case "duration", "length":
		sortField = "duration"
	case "size":
		sortField = "size"
	case "view_count", "views":
		sortField = "view_count"
	default:
		// For relevance or unknown, don't specify sort (use default ranking)
		return nil
	}

	direction := "desc"
	if params.SortDir == "asc" {
		direction = "asc"
	}

	return []string{fmt.Sprintf("%s:%s", sortField, direction)}
}

// escapeFilterValue escapes special characters in filter values.
func escapeFilterValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// ClearIndex removes all documents from the index.
func (c *Client) ClearIndex() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	index := c.client.Index(c.indexName)
	task, err := index.DeleteAllDocuments()
	if err != nil {
		return fmt.Errorf("failed to clear index: %w", err)
	}

	if _, err := c.client.WaitForTask(task.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for clear task: %w", err)
	}

	c.logger.Info("cleared meilisearch index", zap.String("index", c.indexName))
	return nil
}

// Health checks if Meilisearch is healthy.
func (c *Client) Health() error {
	health, err := c.client.Health()
	if err != nil {
		return fmt.Errorf("meilisearch health check failed: %w", err)
	}
	if health.Status != "available" {
		return fmt.Errorf("meilisearch unhealthy: %s", health.Status)
	}
	return nil
}
