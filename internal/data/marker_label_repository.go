package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MarkerLabel is a server-wide marker label (docs/MARKER_LABELS.md). Its name is unique
// ignoring case.
type MarkerLabel struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Color       string    `gorm:"size:7;not null;default:'#FFFFFF'" json:"color"`
	Description string    `gorm:"not null;default:''" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (MarkerLabel) TableName() string {
	return "marker_labels"
}

// MarkerLabelWithUsage is a label with its usage over every user's markers and its default
// tags (management view).
type MarkerLabelWithUsage struct {
	MarkerLabel
	// MarkerCount is the number of markers with the label (every user).
	MarkerCount int64 `json:"marker_count"`
	// UserCount is the number of users having markers with the label.
	UserCount   int64 `json:"user_count"`
	DefaultTags []Tag `json:"default_tags" gorm:"-"`
}

// MarkerLabelMergeStats describes what merging source labels into a target label changes.
type MarkerLabelMergeStats struct {
	// MarkersMoved are the markers of the sources (they now have the target).
	MarkersMoved int64 `json:"markers_moved"`
	// DefaultTagsAdded are the default tags of the sources the target did not have.
	DefaultTagsAdded int64 `json:"default_tags_added"`
	// ReferencesUpdated are the saved searches, saved marker filters and homepage configs (one
	// per user) that named a source.
	ReferencesUpdated int64 `json:"references_updated"`
	SourcesDeleted    int64 `json:"sources_deleted"`
}

// ErrMarkerLabelNameTaken is returned when another label already has the name (ignoring case).
var ErrMarkerLabelNameTaken = errors.New("marker label name already taken")

// errMarkerLabelMergeDryRun rolls back a dry-run merge after its counts were taken.
var errMarkerLabelMergeDryRun = errors.New("marker label merge dry run")

// MarkerLabelRepository manages the marker label referential and the default tags of labels.
type MarkerLabelRepository interface {
	List() ([]MarkerLabel, error)
	// ListWithUsage returns every label with its usage and default tags, by name.
	ListWithUsage() ([]MarkerLabelWithUsage, error)
	GetByID(id uint) (*MarkerLabel, error)
	GetByIDs(ids []uint) ([]MarkerLabel, error)
	// FindByName returns the label of that name ignoring case and surrounding spaces, or
	// (nil, nil) when there is none.
	FindByName(name string) (*MarkerLabel, error)
	// Create inserts a label; ErrMarkerLabelNameTaken when the name is used.
	Create(label *MarkerLabel) error
	// Ensure returns the label of that name, created (with the default color) when missing.
	Ensure(name string) (*MarkerLabel, bool, error)
	// Update saves the name, color and description of a label. A new name renames the copy
	// on the markers (database trigger) and the references to the old name (saved searches,
	// saved marker filters, homepage widgets), in the same transaction. It returns the number
	// of references changed. ErrMarkerLabelNameTaken when the name is used,
	// gorm.ErrRecordNotFound when the label is gone.
	Update(label *MarkerLabel, oldName string) (int64, error)
	// Delete deletes labels: their markers stay, without label and without the tags they had
	// from it. It returns the number of markers that lost their label.
	Delete(ids []uint) (int64, error)
	// Merge moves the markers and default tags of the sources to the target, renames the
	// references and deletes the sources, in one transaction (rolled back with dryRun).
	Merge(target MarkerLabel, sources []MarkerLabel, dryRun bool) (*MarkerLabelMergeStats, error)
	// CountMarkers returns the number of markers (every user) of each label.
	CountMarkers(ids []uint) (map[uint]int64, error)

	GetDefaultTags(labelID uint) ([]Tag, error)
	// SetDefaultTags replaces the default tags of a label and syncs the tags "from label" of
	// every marker of the label (every user).
	SetDefaultTags(labelID uint, tagIDs []uint) error
}

type MarkerLabelRepositoryImpl struct {
	DB *gorm.DB
}

func NewMarkerLabelRepository(db *gorm.DB) *MarkerLabelRepositoryImpl {
	return &MarkerLabelRepositoryImpl{DB: db}
}

// NormalizeMarkerLabelName trims a label name (the spelling kept).
func NormalizeMarkerLabelName(name string) string {
	return strings.TrimSpace(name)
}

// markerLabelKeys returns the lowercased trimmed names, empty ones dropped: the arguments
// of the case-insensitive label matching (lower(marker_labels.name) IN ?).
func markerLabelKeys(names []string) []string {
	keys := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		k := strings.ToLower(strings.TrimSpace(n))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	return keys
}

// markerLabelInSQL is the condition keeping the markers (of the table aliased as alias) whose
// label is one of the names bound as its single argument (markerLabelKeys), ignoring case.
func markerLabelInSQL(alias string) string {
	return alias + ".label_id IN (SELECT id FROM marker_labels WHERE lower(name) IN ?)"
}

func (r *MarkerLabelRepositoryImpl) List() ([]MarkerLabel, error) {
	var labels []MarkerLabel
	if err := r.DB.Order("lower(name) ASC, id ASC").Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}

func (r *MarkerLabelRepositoryImpl) ListWithUsage() ([]MarkerLabelWithUsage, error) {
	var labels []MarkerLabelWithUsage
	err := r.DB.Raw(`SELECT ml.*, COUNT(m.id) AS marker_count, COUNT(DISTINCT m.user_id) AS user_count
		FROM marker_labels ml
		LEFT JOIN user_scene_markers m ON m.label_id = ml.id
		GROUP BY ml.id
		ORDER BY lower(ml.name) ASC, ml.id ASC`).Scan(&labels).Error
	if err != nil {
		return nil, err
	}
	type row struct {
		LabelID uint
		Tag
	}
	var rows []row
	if err := r.DB.Raw(`SELECT lt.label_id, t.* FROM marker_label_tags lt
		JOIN tags t ON t.id = lt.tag_id ORDER BY t.name ASC`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	byLabel := make(map[uint][]Tag)
	for _, r := range rows {
		byLabel[r.LabelID] = append(byLabel[r.LabelID], r.Tag)
	}
	for i := range labels {
		labels[i].DefaultTags = byLabel[labels[i].ID]
		if labels[i].DefaultTags == nil {
			labels[i].DefaultTags = []Tag{}
		}
	}
	return labels, nil
}

func (r *MarkerLabelRepositoryImpl) GetByID(id uint) (*MarkerLabel, error) {
	var label MarkerLabel
	if err := r.DB.First(&label, id).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

func (r *MarkerLabelRepositoryImpl) GetByIDs(ids []uint) ([]MarkerLabel, error) {
	if len(ids) == 0 {
		return []MarkerLabel{}, nil
	}
	var labels []MarkerLabel
	if err := r.DB.Where("id IN ?", ids).Order("id ASC").Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}

func (r *MarkerLabelRepositoryImpl) FindByName(name string) (*MarkerLabel, error) {
	key := strings.ToLower(NormalizeMarkerLabelName(name))
	if key == "" {
		return nil, nil
	}
	var label MarkerLabel
	err := r.DB.Where("lower(name) = ?", key).First(&label).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &label, nil
}

func (r *MarkerLabelRepositoryImpl) Create(label *MarkerLabel) error {
	if err := r.DB.Create(label).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrMarkerLabelNameTaken
		}
		return err
	}
	return nil
}

func (r *MarkerLabelRepositoryImpl) Ensure(name string) (*MarkerLabel, bool, error) {
	name = NormalizeMarkerLabelName(name)
	if name == "" {
		return nil, false, fmt.Errorf("empty marker label name")
	}
	var created []MarkerLabel
	if err := r.DB.Raw(`INSERT INTO marker_labels (name) VALUES (?)
		ON CONFLICT ((lower(name))) DO NOTHING RETURNING *`, name).Scan(&created).Error; err != nil {
		return nil, false, err
	}
	if len(created) == 1 {
		return &created[0], true, nil
	}
	existing, err := r.FindByName(name)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, fmt.Errorf("marker label %q vanished", name)
	}
	return existing, false, nil
}

func (r *MarkerLabelRepositoryImpl) Update(label *MarkerLabel, oldName string) (int64, error) {
	var refs int64
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&MarkerLabel{}).Where("id = ?", label.ID).Updates(map[string]any{
			"name":        label.Name,
			"color":       label.Color,
			"description": label.Description,
			"updated_at":  time.Now(),
		})
		if res.Error != nil {
			if isUniqueViolation(res.Error) {
				return ErrMarkerLabelNameTaken
			}
			return fmt.Errorf("update marker label: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if oldName == label.Name {
			return nil
		}
		n, err := renameMarkerLabelRefs(tx, []string{oldName}, label.Name)
		refs = n
		return err
	})
	if err != nil {
		return 0, err
	}
	return refs, nil
}

func (r *MarkerLabelRepositoryImpl) Delete(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var cleared int64
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM marker_tags WHERE is_from_label
				AND marker_id IN (SELECT id FROM user_scene_markers WHERE label_id IN ?)`, ids).Error; err != nil {
			return fmt.Errorf("drop label tags of markers: %w", err)
		}
		if err := tx.Raw(`SELECT COUNT(*) FROM user_scene_markers WHERE label_id IN ?`, ids).Scan(&cleared).Error; err != nil {
			return fmt.Errorf("count markers: %w", err)
		}
		// markers.label_id is ON DELETE SET NULL (the trigger empties their label)
		if err := tx.Exec(`DELETE FROM marker_labels WHERE id IN ?`, ids).Error; err != nil {
			return fmt.Errorf("delete marker labels: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return cleared, nil
}

func (r *MarkerLabelRepositoryImpl) Merge(target MarkerLabel, sources []MarkerLabel, dryRun bool) (*MarkerLabelMergeStats, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("no source labels to merge")
	}
	srcIDs := make([]uint, 0, len(sources))
	srcNames := make([]string, 0, len(sources))
	for _, s := range sources {
		if s.ID == target.ID {
			return nil, fmt.Errorf("cannot merge marker label %d into itself", target.ID)
		}
		srcIDs = append(srcIDs, s.ID)
		srcNames = append(srcNames, s.Name)
	}
	args := map[string]any{"src": srcIDs, "dst": target.ID}

	var stats MarkerLabelMergeStats
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`SELECT COUNT(*) FROM user_scene_markers WHERE label_id IN @src`, args).
			Scan(&stats.MarkersMoved).Error; err != nil {
			return fmt.Errorf("count markers: %w", err)
		}
		res := tx.Exec(`INSERT INTO marker_label_tags (label_id, tag_id)
			SELECT DISTINCT CAST(@dst AS bigint), tag_id FROM marker_label_tags WHERE label_id IN @src
			ON CONFLICT (label_id, tag_id) DO NOTHING`, args)
		if res.Error != nil {
			return fmt.Errorf("merge default tags: %w", res.Error)
		}
		stats.DefaultTagsAdded = res.RowsAffected
		if err := tx.Exec(`UPDATE user_scene_markers SET label_id = @dst WHERE label_id IN @src`, args).Error; err != nil {
			return fmt.Errorf("move markers: %w", err)
		}
		if err := syncLabelTagsOfMarkers(tx, target.ID); err != nil {
			return err
		}
		n, err := renameMarkerLabelRefs(tx, srcNames, target.Name)
		if err != nil {
			return err
		}
		stats.ReferencesUpdated = n
		res = tx.Exec(`DELETE FROM marker_labels WHERE id IN @src`, args)
		if res.Error != nil {
			return fmt.Errorf("delete source labels: %w", res.Error)
		}
		stats.SourcesDeleted = res.RowsAffected
		if dryRun {
			return errMarkerLabelMergeDryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, errMarkerLabelMergeDryRun) {
		return nil, err
	}
	return &stats, nil
}

func (r *MarkerLabelRepositoryImpl) CountMarkers(ids []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}
	type row struct {
		LabelID uint
		Count   int64
	}
	var rows []row
	if err := r.DB.Raw(`SELECT label_id, COUNT(*) AS count FROM user_scene_markers
		WHERE label_id IN ? GROUP BY label_id`, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.LabelID] = r.Count
	}
	return counts, nil
}

func (r *MarkerLabelRepositoryImpl) GetDefaultTags(labelID uint) ([]Tag, error) {
	var tags []Tag
	err := r.DB.Table("tags").
		Joins("JOIN marker_label_tags lt ON lt.tag_id = tags.id").
		Where("lt.label_id = ?", labelID).
		Order("tags.name ASC").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *MarkerLabelRepositoryImpl) SetDefaultTags(labelID uint, tagIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("label_id = ?", labelID).Delete(&MarkerLabelTag{}).Error; err != nil {
			return fmt.Errorf("clear default tags: %w", err)
		}
		if len(tagIDs) > 0 {
			rows := make([]MarkerLabelTag, len(tagIDs))
			for i, id := range tagIDs {
				rows[i] = MarkerLabelTag{LabelID: labelID, TagID: id}
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
				return fmt.Errorf("insert default tags: %w", err)
			}
		}
		return syncLabelTagsOfMarkers(tx, labelID)
	})
}

// syncLabelTagsOfMarkers makes the tags "from label" of every marker of a label its current
// default tags: the others are dropped, the missing ones added (a tag the marker has by hand
// stays as it is).
func syncLabelTagsOfMarkers(tx *gorm.DB, labelID uint) error {
	if err := tx.Exec(`DELETE FROM marker_tags mt
			USING user_scene_markers m
			WHERE mt.marker_id = m.id AND m.label_id = ? AND mt.is_from_label
				AND mt.tag_id NOT IN (SELECT tag_id FROM marker_label_tags WHERE label_id = ?)`,
		labelID, labelID).Error; err != nil {
		return fmt.Errorf("drop old label tags of markers: %w", err)
	}
	if err := tx.Exec(`INSERT INTO marker_tags (marker_id, tag_id, is_from_label)
			SELECT m.id, lt.tag_id, TRUE FROM user_scene_markers m
			JOIN marker_label_tags lt ON lt.label_id = m.label_id
			WHERE m.label_id = ?
			ON CONFLICT (marker_id, tag_id) DO NOTHING`, labelID).Error; err != nil {
		return fmt.Errorf("add label tags to markers: %w", err)
	}
	return nil
}

// renameMarkerLabelRefs replaces the label names from (ignoring case and surrounding spaces)
// by to in the stored references: saved searches (selected_marker_labels), saved marker
// filters (labels) and the homepage widgets of every user. It returns the number of rows
// changed.
func renameMarkerLabelRefs(tx *gorm.DB, from []string, to string) (int64, error) {
	keys := markerLabelKeys(from)
	if len(keys) == 0 {
		return 0, nil
	}
	var refs int64
	for _, ref := range []struct{ table, key string }{
		{"saved_searches", "selected_marker_labels"},
		{"saved_marker_filters", "labels"},
	} {
		q := fmt.Sprintf(`UPDATE %s SET updated_at = NOW(), filters = jsonb_set(filters, ARRAY[CAST(@key AS text)], (
				SELECT COALESCE(jsonb_agg(v ORDER BY ord), '[]'::jsonb) FROM (
					SELECT DISTINCT ON (lower(v)) v, ord FROM (
						SELECT CASE WHEN lower(btrim(e)) = ANY(CAST(@from AS text[])) THEN CAST(@to AS text) ELSE e END AS v, ord
						FROM jsonb_array_elements_text(filters->(CAST(@key AS text))) WITH ORDINALITY AS x(e, ord)
					) renamed ORDER BY lower(v), ord
				) dedup))
			WHERE jsonb_typeof(filters->(CAST(@key AS text))) = 'array'
				AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(filters->(CAST(@key AS text))) e
					WHERE lower(btrim(e)) = ANY(CAST(@from AS text[])))`, ref.table)
		res := tx.Exec(q, map[string]any{"key": ref.key, "from": pq.StringArray(keys), "to": to})
		if res.Error != nil {
			return 0, fmt.Errorf("update %s.%s: %w", ref.table, ref.key, res.Error)
		}
		refs += res.RowsAffected
	}
	n, err := renameHomepageMarkerLabels(tx, keys, to)
	if err != nil {
		return 0, err
	}
	return refs + n, nil
}

// renameHomepageMarkerLabels applies RenameHomepageMarkerLabelRefs to every user's homepage
// config and returns the number of users changed.
func renameHomepageMarkerLabels(tx *gorm.DB, keys []string, to string) (int64, error) {
	type row struct {
		ID     uint
		Config string
	}
	var rows []row
	if err := tx.Raw(`SELECT id, homepage_config::text AS config FROM user_settings`).Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("load homepage configs: %w", err)
	}
	from := make(map[string]bool, len(keys))
	for _, k := range keys {
		from[k] = true
	}
	var changed int64
	for _, r := range rows {
		cfg, ok := decodeHomepageConfig(r.Config)
		if !ok || !RenameHomepageMarkerLabelRefs(cfg, from, to) {
			continue
		}
		out, err := encodeHomepageConfig(cfg)
		if err != nil {
			return 0, err
		}
		if err := tx.Exec(`UPDATE user_settings SET homepage_config = CAST(? AS jsonb) WHERE id = ?`, out, r.ID).Error; err != nil {
			return 0, fmt.Errorf("update homepage config: %w", err)
		}
		changed++
	}
	return changed, nil
}

// RenameHomepageMarkerLabelRefs rewrites, in a decoded homepage config (legacy "sections" and
// every tab's "sections"), the marker label names of the widget configs: the "labels" list
// (or comma-separated string) of the marker widgets and the "items" of browse_tags widgets on
// labels. from holds the lowercased trimmed names to replace. It reports whether anything
// changed.
func RenameHomepageMarkerLabelRefs(cfg map[string]any, from map[string]bool, to string) bool {
	changed := false
	visit := func(sections any) {
		list, _ := sections.([]any)
		for _, s := range list {
			section, _ := s.(map[string]any)
			if section == nil {
				continue
			}
			config, _ := section["config"].(map[string]any)
			if config == nil {
				continue
			}
			if renameLabelsIn(config, "labels", from, to) {
				changed = true
			}
			typ, _ := section["type"].(string)
			source, _ := config["source"].(string)
			if typ == "browse_tags" && (source == "" || source == "labels") {
				if renameLabelsIn(config, "items", from, to) {
					changed = true
				}
			}
		}
	}
	visit(cfg["sections"])
	tabs, _ := cfg["tabs"].([]any)
	for _, t := range tabs {
		if tab, ok := t.(map[string]any); ok {
			visit(tab["sections"])
		}
	}
	return changed
}

// renameLabelsIn renames the labels of config[key], a list of names or a comma-separated
// string, matching from ignoring case and spaces, without duplicates (ignoring case).
func renameLabelsIn(config map[string]any, key string, from map[string]bool, to string) bool {
	var names []string
	asString := false
	switch v := config[key].(type) {
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok {
				names = append(names, s)
			}
		}
	case string:
		asString = true
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				names = append(names, s)
			}
		}
	default:
		return false
	}
	hit := false
	for _, n := range names {
		if from[strings.ToLower(strings.TrimSpace(n))] {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	out := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if from[strings.ToLower(strings.TrimSpace(n))] {
			n = to
		}
		k := strings.ToLower(n)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, n)
	}
	if asString {
		config[key] = strings.Join(out, ",")
		return true
	}
	list := make([]any, len(out))
	for i, n := range out {
		list[i] = n
	}
	config[key] = list
	return true
}

// decodeHomepageConfig decodes a homepage config; false when it is not a JSON object.
func decodeHomepageConfig(raw string) (map[string]any, bool) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil || cfg == nil {
		return nil, false
	}
	return cfg, true
}

func encodeHomepageConfig(cfg map[string]any) (string, error) {
	out, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("encode homepage config: %w", err)
	}
	return string(out), nil
}

var _ MarkerLabelRepository = (*MarkerLabelRepositoryImpl)(nil)
