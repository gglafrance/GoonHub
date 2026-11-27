package data

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Trash list sorts (TrashQuery.Sort).
const (
	TrashSortTrashedAt = "trashed_at"
	TrashSortSize      = "size"
	TrashSortDuration  = "duration"
	TrashSortTitle     = "title"
	TrashSortPurgeAt   = "purge_at"
)

// Trash sources (TrashFilter.Source), derived from what is already stored: a scene not kept
// when its duplicate group was resolved (resolve or "trash all") comes from the duplicates;
// any other trashed scene (watch page, explorer, ...) is "other".
const (
	TrashSourceDuplicates = "duplicates"
	TrashSourceOther      = "other"
)

// IsValidTrashSort reports whether sort is a known trash sort.
func IsValidTrashSort(sort string) bool {
	switch sort {
	case TrashSortTrashedAt, TrashSortSize, TrashSortDuration, TrashSortTitle, TrashSortPurgeAt:
		return true
	}
	return false
}

// TrashFilter narrows the trashed scenes. Zero values mean no filter.
type TrashFilter struct {
	// Query matches the title or the file path (case-insensitive substring)
	Query         string
	StoragePathID *uint
	MinSize       int64 // bytes
	MaxSize       int64 // bytes
	MinDuration   int   // seconds
	MaxDuration   int   // seconds
	TrashedFrom   *time.Time
	TrashedBefore *time.Time // exclusive
	MinHeight     int
	MaxHeight     int
	Codec         string // video codec, case-insensitive
	Source        string // TrashSourceDuplicates or TrashSourceOther
}

// TrashQuery is a page of the trash: filter, sort and the retention used for the purge date.
type TrashQuery struct {
	Filter        TrashFilter
	Sort          string // TrashSort*, default trashed_at
	Desc          bool
	RetentionDays int
	Page          int
	Limit         int
}

// TrashedSceneRow is a trashed scene of the trash list, with its storage path and what is
// known of its duplicate group and file.
type TrashedSceneRow struct {
	ID                  uint
	Title               string
	StoredPath          string
	ThumbnailPath       string
	Size                int64
	Duration            int
	Width               int
	Height              int
	VideoCodec          string
	TrashedAt           time.Time
	StoragePathID       *uint
	StoragePathName     *string
	StoragePathRoot     *string
	StoragePathReadOnly *bool
	// DuplicateGroupID is set when the scene was not kept at the resolution of its group
	DuplicateGroupID *uint
	// DeleteAfter is the scheduled deletion of a resolved duplicate (before the retention)
	DeleteAfter *time.Time
	// PurgeAt is when the scene leaves the trash: the retention or the scheduled deletion
	PurgeAt time.Time
	// FileMissing: the last completed missing-file check found the file missing
	FileMissing bool
}

// TrashTotals counts the trashed scenes and sums their size.
type TrashTotals struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"`
}

// TrashCodecCount is a video codec of the trash with its number of scenes.
type TrashCodecCount struct {
	Codec string `json:"codec"`
	Count int64  `json:"count"`
}

// TrashStoragePathCount is a storage path of the trash with its scenes and their size.
// StoragePathID is nil for the scenes in no storage path.
type TrashStoragePathCount struct {
	StoragePathID *uint   `json:"storage_path_id"`
	Name          *string `json:"name"`
	Count         int64   `json:"count"`
	Size          int64   `json:"size"`
}

// TrashFacets lists what the trash filters can take, over the whole trash.
type TrashFacets struct {
	Codecs       []TrashCodecCount       `json:"codecs"`
	StoragePaths []TrashStoragePathCount `json:"storage_paths"`
	Duplicates   int64                   `json:"duplicates"`
}

// trashDuplicateJoin joins the duplicate membership of a scene not kept at the resolution of
// its group (a scene is in one group at most).
const trashDuplicateJoin = `LEFT JOIN duplicate_group_members dm ON dm.scene_id = s.id AND dm.is_best = false
	AND EXISTS (SELECT 1 FROM duplicate_groups dg WHERE dg.id = dm.group_id AND dg.status = 'resolved')`

// trashBase selects the trashed scenes (not soft-deleted) matching the filter.
func (r *SceneRepositoryImpl) trashBase(f TrashFilter) *gorm.DB {
	q := r.DB.Table("scenes AS s").Where("s.trashed_at IS NOT NULL AND s.deleted_at IS NULL")
	if term := strings.TrimSpace(f.Query); term != "" {
		like := "%" + escapeLike(term) + "%"
		q = q.Where("(s.title ILIKE ? OR s.stored_path ILIKE ?)", like, like)
	}
	if f.StoragePathID != nil {
		if *f.StoragePathID == 0 {
			q = q.Where("s.storage_path_id IS NULL")
		} else {
			q = q.Where("s.storage_path_id = ?", *f.StoragePathID)
		}
	}
	if f.MinSize > 0 {
		q = q.Where("s.size >= ?", f.MinSize)
	}
	if f.MaxSize > 0 {
		q = q.Where("s.size <= ?", f.MaxSize)
	}
	if f.MinDuration > 0 {
		q = q.Where("s.duration >= ?", f.MinDuration)
	}
	if f.MaxDuration > 0 {
		q = q.Where("s.duration <= ?", f.MaxDuration)
	}
	if f.TrashedFrom != nil {
		q = q.Where("s.trashed_at >= ?", *f.TrashedFrom)
	}
	if f.TrashedBefore != nil {
		q = q.Where("s.trashed_at < ?", *f.TrashedBefore)
	}
	if f.MinHeight > 0 {
		q = q.Where("s.height >= ?", f.MinHeight)
	}
	if f.MaxHeight > 0 {
		q = q.Where("s.height <= ?", f.MaxHeight)
	}
	if codec := strings.TrimSpace(f.Codec); codec != "" {
		q = q.Where("LOWER(s.video_codec) = LOWER(?)", codec)
	}
	duplicateSource := `EXISTS (SELECT 1 FROM duplicate_group_members m JOIN duplicate_groups g ON g.id = m.group_id
		WHERE m.scene_id = s.id AND m.is_best = false AND g.status = 'resolved')`
	switch f.Source {
	case TrashSourceDuplicates:
		q = q.Where(duplicateSource)
	case TrashSourceOther:
		q = q.Where("NOT " + duplicateSource)
	}
	return q
}

// escapeLike escapes the LIKE wildcards of a user search term (backslash is the default
// escape character of PostgreSQL).
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// trashPurgeExpr is when a trashed scene leaves the trash: the retention, or the scheduled
// deletion of a resolved duplicate when sooner (LEAST ignores NULL).
const trashPurgeExpr = "LEAST(s.trashed_at + make_interval(days => ?), dm.delete_after)"

// trashOrder returns the ORDER BY of a trash query (ties broken by id) and its arguments.
func trashOrder(q TrashQuery) (string, []any) {
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	switch q.Sort {
	case TrashSortSize:
		return fmt.Sprintf("s.size %s, s.id %s", dir, dir), nil
	case TrashSortDuration:
		return fmt.Sprintf("s.duration %s, s.id %s", dir, dir), nil
	case TrashSortTitle:
		return fmt.Sprintf("LOWER(s.title) %s, s.id %s", dir, dir), nil
	case TrashSortPurgeAt:
		return fmt.Sprintf("%s %s, s.id %s", trashPurgeExpr, dir, dir), []any{q.RetentionDays}
	default:
		return fmt.Sprintf("s.trashed_at %s, s.id %s", dir, dir), nil
	}
}

// trashOrderClause wraps an ORDER BY with arguments (Order ignores a gorm.Expr).
func trashOrderClause(order string, args []any) clause.OrderBy {
	return clause.OrderBy{Expression: clause.Expr{SQL: order, Vars: args, WithoutParentheses: true}}
}

// ListTrash returns a page of the trashed scenes matching the filter and the totals (count and
// size) of the whole filter.
func (r *SceneRepositoryImpl) ListTrash(q TrashQuery) ([]TrashedSceneRow, TrashTotals, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 20
	}
	totals, err := r.trashTotals(q.Filter)
	if err != nil {
		return nil, TrashTotals{}, err
	}
	rows := []TrashedSceneRow{}
	if totals.Count == 0 {
		return rows, totals, nil
	}
	order, orderArgs := trashOrder(q)
	err = r.trashBase(q.Filter).
		Select(`s.id, s.title, s.stored_path, s.thumbnail_path, s.size, s.duration, s.width, s.height,
			s.video_codec, s.trashed_at, s.storage_path_id,
			sp.name AS storage_path_name, sp.path AS storage_path_root, sp.read_only AS storage_path_read_only,
			dm.group_id AS duplicate_group_id, dm.delete_after, `+trashPurgeExpr+` AS purge_at,
			EXISTS (SELECT 1 FROM library_health_check_items i
				JOIN library_health_checks c ON c.id = i.check_id AND c.kind = ? AND c.status = ?
				WHERE i.scene_id = s.id AND i.stored_path = s.stored_path AND i.reason = ?) AS file_missing`,
			q.RetentionDays, HealthCheckKindMissingFiles, HealthCheckStatusCompleted, HealthItemReasonMissing).
		Joins("LEFT JOIN storage_paths sp ON sp.id = s.storage_path_id").
		Joins(trashDuplicateJoin).
		Order(trashOrderClause(order, orderArgs)).
		Limit(q.Limit).Offset((q.Page - 1) * q.Limit).
		Scan(&rows).Error
	if err != nil {
		return nil, TrashTotals{}, fmt.Errorf("failed to list trashed scenes: %w", err)
	}
	return rows, totals, nil
}

// TrashTotals returns the number and total size of the trashed scenes matching the filter
// (the whole trash with an empty filter).
func (r *SceneRepositoryImpl) TrashTotals(f TrashFilter) (TrashTotals, error) {
	return r.trashTotals(f)
}

func (r *SceneRepositoryImpl) trashTotals(f TrashFilter) (TrashTotals, error) {
	var totals TrashTotals
	if err := r.trashBase(f).Select("COUNT(*) AS count, COALESCE(SUM(s.size), 0)::bigint AS size").
		Scan(&totals).Error; err != nil {
		return TrashTotals{}, fmt.Errorf("failed to count trashed scenes: %w", err)
	}
	return totals, nil
}

// TrashIDSize is the ID and size of a trashed scene ("select all" of the trash page).
type TrashIDSize struct {
	ID   uint  `json:"id"`
	Size int64 `json:"size"`
}

// ListTrashIDs returns the IDs and sizes of the trashed scenes matching the filter, in the
// query's sort, at most limit, with the number of matching scenes.
func (r *SceneRepositoryImpl) ListTrashIDs(q TrashQuery, limit int) ([]TrashIDSize, int64, error) {
	var total int64
	if err := r.trashBase(q.Filter).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count trashed scenes: %w", err)
	}
	items := []TrashIDSize{}
	if total == 0 || limit < 1 {
		return items, total, nil
	}
	order, orderArgs := trashOrder(q)
	if err := r.trashBase(q.Filter).
		Select("s.id, s.size").
		Joins(trashDuplicateJoin).
		Order(trashOrderClause(order, orderArgs)).
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list trashed scene ids: %w", err)
	}
	return items, total, nil
}

// TrashFacets returns the video codecs and storage paths of the whole trash and the number of
// scenes from the duplicates.
func (r *SceneRepositoryImpl) TrashFacets() (TrashFacets, error) {
	facets := TrashFacets{Codecs: []TrashCodecCount{}, StoragePaths: []TrashStoragePathCount{}}
	if err := r.trashBase(TrashFilter{}).
		Select("COALESCE(s.video_codec, '') AS codec, COUNT(*) AS count").
		Group("COALESCE(s.video_codec, '')").
		Order("count DESC, codec").
		Scan(&facets.Codecs).Error; err != nil {
		return facets, fmt.Errorf("failed to list trash codecs: %w", err)
	}
	if err := r.trashBase(TrashFilter{}).
		Select("s.storage_path_id, sp.name, COUNT(*) AS count, COALESCE(SUM(s.size), 0)::bigint AS size").
		Joins("LEFT JOIN storage_paths sp ON sp.id = s.storage_path_id").
		Group("s.storage_path_id, sp.name").
		Order("sp.name NULLS LAST, s.storage_path_id").
		Scan(&facets.StoragePaths).Error; err != nil {
		return facets, fmt.Errorf("failed to list trash storage paths: %w", err)
	}
	if err := r.trashBase(TrashFilter{Source: TrashSourceDuplicates}).
		Count(&facets.Duplicates).Error; err != nil {
		return facets, fmt.Errorf("failed to count trashed duplicates: %w", err)
	}
	return facets, nil
}
