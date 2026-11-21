package data

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"goonhub/internal/playability"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Kinds of background library health tasks.
const (
	HealthCheckKindMissingFiles = "missing_files"
	HealthCheckKindFixSizes     = "fix_sizes"
)

// Statuses of a background library health task.
const (
	HealthCheckStatusRunning   = "running"
	HealthCheckStatusCompleted = "completed"
	HealthCheckStatusCancelled = "cancelled"
	HealthCheckStatusFailed    = "failed"
)

// Reasons of a library health check item.
const (
	HealthItemReasonMissing    = "missing"
	HealthItemReasonUnreadable = "unreadable"
)

// Library health categories. The live ones are computed from the scenes table on each
// request; missing_file and unreadable_file come from the last completed missing-file check.
const (
	HealthCategoryMissingFile        = "missing_file"
	HealthCategoryUnreadableFile     = "unreadable_file"
	HealthCategoryCorrupted          = "corrupted"
	HealthCategorySizeZero           = "size_zero"
	HealthCategoryDurationZero       = "duration_zero"
	HealthCategoryUnplayable         = "unplayable"
	HealthCategoryNoThumbnail        = "no_thumbnail"
	HealthCategoryNoSprites          = "no_sprites"
	HealthCategoryNoPreview          = "no_preview"
	HealthCategoryNoFingerprint      = "no_fingerprint"
	HealthCategoryNoStoragePath      = "no_storage_path"
	HealthCategoryOutsideStoragePath = "outside_storage_path"
)

// LiveHealthCategories lists the categories computed live from the scenes table, in display order.
var LiveHealthCategories = []string{
	HealthCategoryCorrupted,
	HealthCategorySizeZero,
	HealthCategoryDurationZero,
	HealthCategoryUnplayable,
	HealthCategoryNoThumbnail,
	HealthCategoryNoSprites,
	HealthCategoryNoPreview,
	HealthCategoryNoFingerprint,
	HealthCategoryNoStoragePath,
	HealthCategoryOutsideStoragePath,
}

// IsHealthCategory reports whether the category is known (live or from the missing-file check).
func IsHealthCategory(category string) bool {
	if category == HealthCategoryMissingFile || category == HealthCategoryUnreadableFile {
		return true
	}
	for _, c := range LiveHealthCategories {
		if c == category {
			return true
		}
	}
	return false
}

// LibraryHealthCheck is one run of a background library health task.
type LibraryHealthCheck struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	Kind         string     `gorm:"not null" json:"kind"`
	Status       string     `gorm:"not null;default:'running'" json:"status"`
	StartedAt    time.Time  `gorm:"not null;default:now()" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	Total        int        `gorm:"not null;default:0" json:"total"`
	Checked      int        `gorm:"not null;default:0" json:"checked"`
	Missing      int        `gorm:"not null;default:0" json:"missing"`
	Unreadable   int        `gorm:"not null;default:0" json:"unreadable"`
	Fixed        int        `gorm:"not null;default:0" json:"fixed"`
	Unchanged    int        `gorm:"not null;default:0" json:"unchanged"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (LibraryHealthCheck) TableName() string {
	return "library_health_checks"
}

// LibraryHealthCheckItem is a file a missing-file check found missing or unreadable.
type LibraryHealthCheckItem struct {
	ID         uint   `gorm:"primarykey"`
	CheckID    uint   `gorm:"not null"`
	SceneID    uint   `gorm:"not null"`
	StoredPath string `gorm:"not null"`
	Reason     string `gorm:"not null"`
	Detail     string `gorm:"not null;default:''"`
}

func (LibraryHealthCheckItem) TableName() string {
	return "library_health_check_items"
}

// HealthSceneRow is a lightweight scene row of a library health drill-down list.
type HealthSceneRow struct {
	ID            uint      `json:"id"`
	Title         string    `json:"title"`
	StoredPath    string    `json:"stored_path"`
	Size          int64     `json:"size"`
	Duration      int       `json:"duration"`
	VideoCodec    string    `json:"video_codec"`
	AudioCodec    string    `json:"audio_codec"`
	StoragePathID *uint     `json:"storage_path_id"`
	IsCorrupted   bool      `json:"is_corrupted"`
	CreatedAt     time.Time `json:"created_at"`
	Detail        string    `json:"detail,omitempty"`
}

// StoragePathSceneStats is the number of scenes and their total size (as stored in the
// database) for one storage path. StoragePathID is nil for scenes in no storage path.
type StoragePathSceneStats struct {
	StoragePathID *uint
	SceneCount    int64
	TotalSize     int64
}

// HealthScenePath is the minimum a background task needs to look at a scene file.
type HealthScenePath struct {
	ID         uint
	StoredPath string
}

// Scene selections of the background tasks.
const (
	HealthScenesAll      = "all"       // every active scene (missing-file check)
	HealthScenesSizeZero = "size_zero" // active scenes stored with size 0 (fix sizes)
)

// LibraryHealthRepository runs the library health queries and stores the background task results.
type LibraryHealthRepository interface {
	// CountLiveCategories counts the active scenes in each live category, plus the total.
	CountLiveCategories() (counts map[string]int64, total int64, err error)
	// ListCategoryScenes lists the scenes of a category, ordered by id. For missing_file and
	// unreadable_file, it lists the items of checkID (0 = none yet).
	ListCategoryScenes(category string, checkID uint, page, limit int) ([]HealthSceneRow, int64, error)
	// CountCheckItems counts the items of a check per reason, only for scenes that are still
	// active and still at the checked path.
	CountCheckItems(checkID uint) (map[string]int64, error)
	// CountScenesNeedingPhase counts the scenes a "missing" bulk run of the phase would queue.
	CountScenesNeedingPhase(phase string) (int64, error)
	// StoragePathSceneStats groups the active scenes by storage path.
	StoragePathSceneStats() ([]StoragePathSceneStats, error)

	// CountScenePaths counts the scenes of a selection (HealthScenesAll or HealthScenesSizeZero).
	CountScenePaths(selection string) (int64, error)
	// ListScenePaths returns up to limit scenes of a selection with an id above afterID, by id.
	ListScenePaths(selection string, afterID uint, limit int) ([]HealthScenePath, error)
	// UpdateSceneSize sets the size of a scene still stored with size 0 at the given path.
	// It reports whether a row was updated.
	UpdateSceneSize(id uint, storedPath string, size int64) (bool, error)

	CreateCheck(check *LibraryHealthCheck) error
	UpdateCheck(check *LibraryHealthCheck) error
	InsertCheckItems(items []LibraryHealthCheckItem) error
	// FinishCheck saves the final state of a check. When it completed, the items of the
	// earlier checks of the same kind are dropped; otherwise its own (partial) items are.
	FinishCheck(check *LibraryHealthCheck) error
	// GetLatestCheck returns the most recent check of a kind, or of a kind and status when
	// status is not empty. It returns nil when there is none.
	GetLatestCheck(kind, status string) (*LibraryHealthCheck, error)
	// MarkInterruptedChecksFailed marks the checks left running by a restart as failed.
	MarkInterruptedChecksFailed() error
}

type LibraryHealthRepositoryImpl struct {
	DB *gorm.DB
}

func NewLibraryHealthRepository(db *gorm.DB) *LibraryHealthRepositoryImpl {
	return &LibraryHealthRepositoryImpl{DB: db}
}

const activeSceneCondition = "scenes.deleted_at IS NULL AND scenes.trashed_at IS NULL"

// storagePathPrefixes returns the directory prefixes of the storage paths, both as stored and
// cleaned (a relative "./data/videos" path gives "./data/videos/" and "data/videos/").
func (r *LibraryHealthRepositoryImpl) storagePathPrefixes() ([]string, error) {
	var paths []string
	if err := r.DB.Model(&StoragePath{}).Pluck("path", &paths).Error; err != nil {
		return nil, fmt.Errorf("failed to list storage paths: %w", err)
	}
	seen := make(map[string]struct{}, len(paths)*2)
	prefixes := make([]string, 0, len(paths)*2)
	add := func(p string) {
		if p == "" {
			return
		}
		if !strings.HasSuffix(p, "/") {
			p += "/"
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			prefixes = append(prefixes, p)
		}
	}
	for _, p := range paths {
		add(p)
		add(filepath.Clean(p))
	}
	return prefixes, nil
}

// liveCategoryCondition returns the SQL condition (on the scenes table) of a live category.
// It is the single definition used by both the counts and the drill-down lists.
func liveCategoryCondition(category string, prefixes []string) (string, []any, bool) {
	switch category {
	case HealthCategoryCorrupted:
		return "scenes.is_corrupted", nil, true
	case HealthCategorySizeZero:
		return "scenes.size = 0", nil, true
	case HealthCategoryDurationZero:
		return "scenes.duration = 0", nil, true
	case HealthCategoryUnplayable:
		cond, args := playability.SQLUnplayable("scenes.stored_path", "scenes.video_codec", "scenes.audio_codec")
		return cond, args, true
	case HealthCategoryNoThumbnail:
		return "COALESCE(scenes.thumbnail_path, '') = ''", nil, true
	case HealthCategoryNoSprites:
		return "COALESCE(scenes.sprite_sheet_path, '') = ''", nil, true
	case HealthCategoryNoPreview:
		return "COALESCE(scenes.preview_video_path, '') = ''", nil, true
	case HealthCategoryNoFingerprint:
		return "scenes.fingerprint_type IS NULL", nil, true
	case HealthCategoryNoStoragePath:
		return "(scenes.storage_path_id IS NULL OR NOT EXISTS (SELECT 1 FROM storage_paths sp WHERE sp.id = scenes.storage_path_id))", nil, true
	case HealthCategoryOutsideStoragePath:
		return "NOT EXISTS (SELECT 1 FROM unnest(?::text[]) AS p(prefix) WHERE starts_with(scenes.stored_path, p.prefix))",
			[]any{pq.StringArray(prefixes)}, true
	default:
		return "", nil, false
	}
}

func (r *LibraryHealthRepositoryImpl) CountLiveCategories() (map[string]int64, int64, error) {
	prefixes, err := r.storagePathPrefixes()
	if err != nil {
		return nil, 0, err
	}
	selects := []string{"COUNT(*) AS total"}
	var args []any
	for _, category := range LiveHealthCategories {
		cond, condArgs, _ := liveCategoryCondition(category, prefixes)
		selects = append(selects, fmt.Sprintf("COUNT(*) FILTER (WHERE %s) AS %s", cond, category))
		args = append(args, condArgs...)
	}
	query := "SELECT " + strings.Join(selects, ", ") + " FROM scenes WHERE " + activeSceneCondition

	row := map[string]any{}
	if err := r.DB.Raw(query, args...).Scan(&row).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count library health categories: %w", err)
	}
	counts := make(map[string]int64, len(LiveHealthCategories))
	for _, category := range LiveHealthCategories {
		counts[category] = toInt64(row[category])
	}
	return counts, toInt64(row["total"]), nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

const healthSceneColumns = "scenes.id, scenes.title, scenes.stored_path, scenes.size, scenes.duration, " +
	"scenes.video_codec, scenes.audio_codec, scenes.storage_path_id, scenes.is_corrupted, scenes.created_at"

func (r *LibraryHealthRepositoryImpl) ListCategoryScenes(category string, checkID uint, page, limit int) ([]HealthSceneRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	// base builds a fresh query each time: the count and the page must not share a statement
	var base func() *gorm.DB
	columns := healthSceneColumns
	switch category {
	case HealthCategoryMissingFile, HealthCategoryUnreadableFile:
		if checkID == 0 {
			return []HealthSceneRow{}, 0, nil
		}
		reason := HealthItemReasonMissing
		if category == HealthCategoryUnreadableFile {
			reason = HealthItemReasonUnreadable
		}
		base = func() *gorm.DB { return r.checkItemsQuery(checkID).Where("i.reason = ?", reason) }
		columns += ", i.detail"
	default:
		prefixes, err := r.storagePathPrefixes()
		if err != nil {
			return nil, 0, err
		}
		cond, args, ok := liveCategoryCondition(category, prefixes)
		if !ok {
			return nil, 0, fmt.Errorf("unknown library health category: %s", category)
		}
		base = func() *gorm.DB { return r.DB.Table("scenes").Where(activeSceneCondition).Where(cond, args...) }
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count %s scenes: %w", category, err)
	}
	rows := []HealthSceneRow{}
	if err := base().Select(columns).Order("scenes.id").Limit(limit).Offset(offset).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list %s scenes: %w", category, err)
	}
	return rows, total, nil
}

// checkItemsQuery joins the items of a check with the scenes still active at the checked path.
func (r *LibraryHealthRepositoryImpl) checkItemsQuery(checkID uint) *gorm.DB {
	return r.DB.Table("library_health_check_items AS i").
		Joins("JOIN scenes ON scenes.id = i.scene_id AND scenes.stored_path = i.stored_path").
		Where("i.check_id = ?", checkID).
		Where(activeSceneCondition)
}

func (r *LibraryHealthRepositoryImpl) CountCheckItems(checkID uint) (map[string]int64, error) {
	counts := map[string]int64{HealthItemReasonMissing: 0, HealthItemReasonUnreadable: 0}
	if checkID == 0 {
		return counts, nil
	}
	var rows []struct {
		Reason string
		Count  int64
	}
	if err := r.checkItemsQuery(checkID).Select("i.reason, COUNT(*) AS count").Group("i.reason").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to count check items: %w", err)
	}
	for _, row := range rows {
		counts[row.Reason] = row.Count
	}
	return counts, nil
}

func (r *LibraryHealthRepositoryImpl) CountScenesNeedingPhase(phase string) (int64, error) {
	query := scenesNeedingPhaseQuery(r.DB, phase)
	if query == nil {
		return 0, fmt.Errorf("unknown phase: %s", phase)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count scenes needing %s: %w", phase, err)
	}
	return count, nil
}

func (r *LibraryHealthRepositoryImpl) StoragePathSceneStats() ([]StoragePathSceneStats, error) {
	var stats []StoragePathSceneStats
	err := r.DB.Table("scenes").
		Select("storage_path_id, COUNT(*) AS scene_count, COALESCE(SUM(size), 0) AS total_size").
		Where(activeSceneCondition).
		Group("storage_path_id").
		Scan(&stats).Error
	if err != nil {
		return nil, fmt.Errorf("failed to group scenes by storage path: %w", err)
	}
	return stats, nil
}

func (r *LibraryHealthRepositoryImpl) scenePathsQuery(selection string) (*gorm.DB, error) {
	query := r.DB.Table("scenes").Where(activeSceneCondition)
	switch selection {
	case HealthScenesAll:
		return query, nil
	case HealthScenesSizeZero:
		return query.Where("scenes.size = 0"), nil
	default:
		return nil, fmt.Errorf("unknown scene selection: %s", selection)
	}
}

func (r *LibraryHealthRepositoryImpl) CountScenePaths(selection string) (int64, error) {
	query, err := r.scenePathsQuery(selection)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count scenes: %w", err)
	}
	return count, nil
}

func (r *LibraryHealthRepositoryImpl) ListScenePaths(selection string, afterID uint, limit int) ([]HealthScenePath, error) {
	query, err := r.scenePathsQuery(selection)
	if err != nil {
		return nil, err
	}
	var paths []HealthScenePath
	if err := query.Select("scenes.id, scenes.stored_path").
		Where("scenes.id > ?", afterID).
		Order("scenes.id").
		Limit(limit).
		Scan(&paths).Error; err != nil {
		return nil, fmt.Errorf("failed to list scene paths: %w", err)
	}
	return paths, nil
}

func (r *LibraryHealthRepositoryImpl) UpdateSceneSize(id uint, storedPath string, size int64) (bool, error) {
	// UpdateColumn keeps updated_at: only the recorded size changes, not the scene
	res := r.DB.Model(&Scene{}).
		Where("id = ? AND stored_path = ? AND size = 0", id, storedPath).
		UpdateColumn("size", size)
	if res.Error != nil {
		return false, fmt.Errorf("failed to update scene size: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *LibraryHealthRepositoryImpl) CreateCheck(check *LibraryHealthCheck) error {
	return r.DB.Create(check).Error
}

func (r *LibraryHealthRepositoryImpl) UpdateCheck(check *LibraryHealthCheck) error {
	return r.DB.Save(check).Error
}

func (r *LibraryHealthRepositoryImpl) InsertCheckItems(items []LibraryHealthCheckItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.DB.CreateInBatches(items, 500).Error
}

func (r *LibraryHealthRepositoryImpl) FinishCheck(check *LibraryHealthCheck) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(check).Error; err != nil {
			return fmt.Errorf("failed to save check: %w", err)
		}
		var res *gorm.DB
		if check.Status == HealthCheckStatusCompleted {
			res = tx.Where("check_id IN (?)",
				tx.Model(&LibraryHealthCheck{}).Select("id").Where("kind = ? AND id <> ?", check.Kind, check.ID),
			).Delete(&LibraryHealthCheckItem{})
		} else {
			res = tx.Where("check_id = ?", check.ID).Delete(&LibraryHealthCheckItem{})
		}
		if res.Error != nil {
			return fmt.Errorf("failed to drop stale check items: %w", res.Error)
		}
		return nil
	})
}

func (r *LibraryHealthRepositoryImpl) GetLatestCheck(kind, status string) (*LibraryHealthCheck, error) {
	query := r.DB.Where("kind = ?", kind)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var checks []LibraryHealthCheck
	if err := query.Order("started_at DESC, id DESC").Limit(1).Find(&checks).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest check: %w", err)
	}
	if len(checks) == 0 {
		return nil, nil
	}
	return &checks[0], nil
}

func (r *LibraryHealthRepositoryImpl) MarkInterruptedChecksFailed() error {
	now := time.Now()
	return r.DB.Model(&LibraryHealthCheck{}).
		Where("status = ?", HealthCheckStatusRunning).
		Updates(map[string]any{
			"status":        HealthCheckStatusFailed,
			"completed_at":  now,
			"error_message": "Interrupted by server restart",
		}).Error
}
