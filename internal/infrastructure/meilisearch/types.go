package meilisearch

// SceneDocument represents a scene document in Meilisearch.
type SceneDocument struct {
	ID               uint     `json:"id"`
	Title            string   `json:"title"`
	OriginalFilename string   `json:"original_filename"`
	Path             string   `json:"path"`
	Description      string   `json:"description"`
	Studio           string   `json:"studio"`
	Actors           []string `json:"actors"`
	ActorAliases     []string `json:"actor_aliases"`
	TagIDs           []uint   `json:"tag_ids"`
	TagNames         []string `json:"tag_names"`
	Duration         float64  `json:"duration"`
	Size             int64    `json:"size"`
	Height           int      `json:"height"`
	CreatedAt        int64    `json:"created_at"`
	ProcessingStatus string   `json:"processing_status"`
	ViewCount        int      `json:"view_count"`
	// SourceSite is the normalized site of the scene's source URL ("pornhub.com"), "" when
	// the scene has no source. Always sent (never omitted) so that "no source" filters match.
	SourceSite string `json:"source_site"`
}

// SearchParams contains parameters for searching scenes.
type SearchParams struct {
	Query       string
	TagIDs      []uint
	Actors      []string
	Studio      string
	MinDuration *float64
	MaxDuration *float64
	MinSize     *int64 // bytes
	MaxSize     *int64 // bytes
	// Metadata presence (nil = any)
	HasActors *bool
	HasTags   *bool
	HasStudio *bool
	// SourceSites keeps scenes from one of these normalized sites (OR); HasSource filters on
	// having a source URL at all (nil = any)
	SourceSites []string
	HasSource   *bool
	// ExcludeSceneIDs removes scenes (negative user filters: unwatched, unrated...)
	ExcludeSceneIDs  []uint
	MinHeight        *int
	MaxHeight        *int
	DateAfter        *int64
	DateBefore       *int64
	ProcessingStatus string
	SceneIDs         []uint // Pre-filtered scene IDs (for user-specific filters)
	Sort             string
	SortDir          string
	Offset           int
	Limit            int
	MatchingStrategy string // Meilisearch matching strategy: "last", "all", or "frequency"
	FetchAllIDs      bool   // When true, fetch all matching IDs (ignore Offset/Limit, skip sort)
	// ExcludeFileAttributes searches the query on the non-file attributes only
	// (ContentSearchAttributes): the user may not see paths nor file names
	ExcludeFileAttributes bool
}

// SearchableAttributes are the attributes the query is matched against, by default.
var SearchableAttributes = []string{
	"title",
	"original_filename",
	"path",
	"description",
	"actors",
	"actor_aliases",
	"tag_names",
}

// ContentSearchAttributes are the searchable attributes without the file ones
// (original_filename, path).
var ContentSearchAttributes = []string{
	"title",
	"description",
	"actors",
	"actor_aliases",
	"tag_names",
}

// SearchResult contains the result of a search query.
type SearchResult struct {
	IDs        []uint
	TotalCount int64
}

// FacetCount is one value of a facet with the number of matching documents.
type FacetCount struct {
	Value string
	Count int64
}
