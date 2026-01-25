package meilisearch

import (
	"context"
	"fmt"
	"strings"
	"time"

	meili "github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

// Client wraps the Meilisearch client with application-specific functionality.
type Client struct {
	client    *meili.Client
	indexName string
	logger    *zap.Logger
}

// NewClient creates a new Meilisearch client wrapper.
func NewClient(host, apiKey, indexName string, logger *zap.Logger) (*Client, error) {
	client := meili.NewClient(meili.ClientConfig{
		Host:   host,
		APIKey: apiKey,
	})

	c := &Client{
		client:    client,
		indexName: indexName,
		logger:    logger,
	}

	// Verify connection and ensure index exists
	if err := c.EnsureIndex(); err != nil {
		return nil, fmt.Errorf("failed to ensure index: %w", err)
	}

	return c, nil
}

// EnsureIndex creates the videos index if it doesn't exist and configures settings.
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
		"description",
		"actors",
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
		"height",
		"created_at",
		"processing_status",
		"id",
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
	})
	if err != nil {
		return fmt.Errorf("failed to update sortable attributes: %w", err)
	}
	if _, err := c.client.WaitForTask(sortableTask.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for sortable attributes task: %w", err)
	}

	c.logger.Info("meilisearch index configured", zap.String("index", c.indexName))
	return nil
}

// IndexVideo adds or updates a video document in the index.
func (c *Client) IndexVideo(doc VideoDocument) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	index := c.client.Index(c.indexName)
	task, err := index.AddDocuments([]VideoDocument{doc}, "id")
	if err != nil {
		return fmt.Errorf("failed to index video: %w", err)
	}

	if _, err := c.client.WaitForTask(task.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for index task: %w", err)
	}

	c.logger.Debug("indexed video", zap.Uint("id", doc.ID), zap.String("title", doc.Title))
	return nil
}

// UpdateVideo updates an existing video document in the index.
func (c *Client) UpdateVideo(doc VideoDocument) error {
	return c.IndexVideo(doc) // Meilisearch upserts automatically
}

// DeleteVideo removes a video document from the index.
func (c *Client) DeleteVideo(id uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	index := c.client.Index(c.indexName)
	task, err := index.DeleteDocument(fmt.Sprintf("%d", id))
	if err != nil {
		return fmt.Errorf("failed to delete video: %w", err)
	}

	if _, err := c.client.WaitForTask(task.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for delete task: %w", err)
	}

	c.logger.Debug("deleted video from index", zap.Uint("id", id))
	return nil
}

// BulkIndex adds multiple video documents to the index.
func (c *Client) BulkIndex(docs []VideoDocument) error {
	if len(docs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	index := c.client.Index(c.indexName)
	task, err := index.AddDocuments(docs, "id")
	if err != nil {
		return fmt.Errorf("failed to bulk index: %w", err)
	}

	if _, err := c.client.WaitForTask(task.TaskUID, meili.WaitParams{Context: ctx, Interval: 100 * time.Millisecond}); err != nil {
		return fmt.Errorf("failed to wait for bulk index task: %w", err)
	}

	c.logger.Info("bulk indexed videos", zap.Int("count", len(docs)))
	return nil
}

// Search performs a search query and returns matching video IDs with total count.
func (c *Client) Search(params SearchParams) (*SearchResult, error) {
	index := c.client.Index(c.indexName)

	// Build filter string
	filters := c.buildFilters(params)

	// Build sort array
	sort := c.buildSort(params)

	searchReq := &meili.SearchRequest{
		Limit:                 int64(params.Limit),
		Offset:                int64(params.Offset),
		AttributesToRetrieve:  []string{"id"},
		ShowMatchesPosition:   false,
	}

	if len(filters) > 0 {
		searchReq.Filter = filters
	}

	if len(sort) > 0 {
		searchReq.Sort = sort
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

	// Pre-filtered video IDs (for user-specific filters)
	if len(params.VideoIDs) > 0 {
		idStrs := make([]string, len(params.VideoIDs))
		for i, id := range params.VideoIDs {
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
