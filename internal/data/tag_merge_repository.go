package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// TagMergeStats describes what merging source tags into a target tag changes.
type TagMergeStats struct {
	// ScenesRetagged are the non-deleted scenes that carried a source tag (they now carry the
	// target, once).
	ScenesRetagged int64 `json:"scenes_retagged"`
	// MarkersUpdated are the markers that carried a source tag.
	MarkersUpdated int64 `json:"markers_updated"`
	// LabelTagsUpdated are the marker label default tags (marker_label_tags rows) that pointed
	// to a source tag.
	LabelTagsUpdated int64 `json:"label_tags_updated"`
	// PlaylistsUpdated are the playlists that carried a source tag.
	PlaylistsUpdated int64 `json:"playlists_updated"`
	// ReferencesUpdated are the saved searches, saved marker filters and homepage configs
	// (one per user) that referenced a source tag by id or name.
	ReferencesUpdated int64 `json:"references_updated"`
	// SourcesDeleted is the number of source tags deleted.
	SourcesDeleted int64 `json:"sources_deleted"`
}

// errTagMergeDryRun rolls back a dry-run merge after its counts were taken.
var errTagMergeDryRun = errors.New("tag merge dry run")

// MergeTags merges the source tags into the target tag in one transaction: every scene,
// marker, marker label and playlist carrying a source carries the target instead (no
// duplicate rows), saved searches, saved marker filters and homepage sections that reference
// a source by id or name reference the target, and the sources are deleted. With dryRun the
// transaction is rolled back and only the counts are returned. It returns the counts and the
// ids of the non-deleted scenes whose tags changed (to re-index).
func (r *TagRepositoryImpl) MergeTags(target Tag, sources []Tag, dryRun bool) (*TagMergeStats, []uint, error) {
	if len(sources) == 0 {
		return nil, nil, fmt.Errorf("no source tags to merge")
	}
	srcIDs := make([]uint, 0, len(sources))
	srcNames := make([]string, 0, len(sources))
	for _, s := range sources {
		if s.ID == target.ID {
			return nil, nil, fmt.Errorf("cannot merge tag %d into itself", target.ID)
		}
		srcIDs = append(srcIDs, s.ID)
		if s.Name != target.Name {
			srcNames = append(srcNames, s.Name)
		}
	}
	args := map[string]any{"src": srcIDs, "dst": target.ID}

	var stats TagMergeStats
	var sceneIDs []uint
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`SELECT DISTINCT st.scene_id FROM scene_tags st
				JOIN scenes s ON s.id = st.scene_id AND s.deleted_at IS NULL
				WHERE st.tag_id IN @src ORDER BY st.scene_id`, args).Scan(&sceneIDs).Error; err != nil {
			return fmt.Errorf("list source scenes: %w", err)
		}
		stats.ScenesRetagged = int64(len(sceneIDs))

		counts := []struct {
			name string
			sql  string
			dst  *int64
		}{
			{"count markers", `SELECT COUNT(DISTINCT marker_id) FROM marker_tags WHERE tag_id IN @src`, &stats.MarkersUpdated},
			{"count label tags", `SELECT COUNT(*) FROM marker_label_tags WHERE tag_id IN @src`, &stats.LabelTagsUpdated},
			{"count playlists", `SELECT COUNT(DISTINCT playlist_id) FROM playlist_tags WHERE tag_id IN @src`, &stats.PlaylistsUpdated},
		}
		for _, c := range counts {
			if err := tx.Raw(c.sql, args).Scan(c.dst).Error; err != nil {
				return fmt.Errorf("%s: %w", c.name, err)
			}
		}

		steps := []struct {
			name string
			sql  string
		}{
			{"move scenes", `INSERT INTO scene_tags (scene_id, tag_id)
				SELECT DISTINCT scene_id, CAST(@dst AS bigint) FROM scene_tags WHERE tag_id IN @src
				ON CONFLICT (scene_id, tag_id) DO NOTHING`},
			{"drop source scenes", `DELETE FROM scene_tags WHERE tag_id IN @src`},
			// a marker keeps the target as "from label" only if every copy was from its label,
			// so a later label change cannot drop a tag the user had set by hand
			{"move markers", `INSERT INTO marker_tags (marker_id, tag_id, is_from_label)
				SELECT marker_id, CAST(@dst AS bigint), bool_and(is_from_label) FROM marker_tags WHERE tag_id IN @src
				GROUP BY marker_id
				ON CONFLICT (marker_id, tag_id) DO UPDATE
				SET is_from_label = marker_tags.is_from_label AND EXCLUDED.is_from_label`},
			{"drop source markers", `DELETE FROM marker_tags WHERE tag_id IN @src`},
			{"move label tags", `INSERT INTO marker_label_tags (user_id, label, tag_id)
				SELECT DISTINCT user_id, label, CAST(@dst AS bigint) FROM marker_label_tags WHERE tag_id IN @src
				ON CONFLICT (user_id, label, tag_id) DO NOTHING`},
			{"drop source label tags", `DELETE FROM marker_label_tags WHERE tag_id IN @src`},
			{"move playlists", `INSERT INTO playlist_tags (playlist_id, tag_id)
				SELECT DISTINCT playlist_id, CAST(@dst AS bigint) FROM playlist_tags WHERE tag_id IN @src
				ON CONFLICT DO NOTHING`},
			{"drop source playlists", `DELETE FROM playlist_tags WHERE tag_id IN @src`},
		}
		for _, step := range steps {
			if err := tx.Exec(step.sql, args).Error; err != nil {
				return fmt.Errorf("%s: %w", step.name, err)
			}
		}

		if len(srcNames) > 0 {
			for _, ref := range []struct{ table, key string }{
				{"saved_searches", "selected_tags"},
				{"saved_marker_filters", "tags"},
				{"saved_marker_filters", "marker_tags"},
			} {
				n, err := replaceNamesInSavedFilterArray(tx, ref.table, ref.key, srcNames, target.Name)
				if err != nil {
					return err
				}
				stats.ReferencesUpdated += n
			}
		}
		n, err := retargetHomepageTags(tx, srcIDs, srcNames, target)
		if err != nil {
			return err
		}
		stats.ReferencesUpdated += n

		res := tx.Exec(`DELETE FROM tags WHERE id IN @src`, args)
		if res.Error != nil {
			return fmt.Errorf("delete source tags: %w", res.Error)
		}
		stats.SourcesDeleted = res.RowsAffected
		if dryRun {
			return errTagMergeDryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, errTagMergeDryRun) {
		return nil, nil, err
	}
	if dryRun {
		sceneIDs = nil
	}
	return &stats, sceneIDs, nil
}

// Update saves a tag's editable fields (name, color).
func (r *TagRepositoryImpl) Update(tag *Tag) error {
	res := r.DB.Model(&Tag{}).Where("id = ?", tag.ID).Updates(map[string]any{"name": tag.Name, "color": tag.Color})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// replaceNamesInSavedFilterArray replaces the names from by to in the JSON string array
// filters->key of a saved filter table (saved_searches, saved_marker_filters), without
// duplicates and keeping the order. It returns the number of rows changed.
func replaceNamesInSavedFilterArray(tx *gorm.DB, table, key string, from []string, to string) (int64, error) {
	q := fmt.Sprintf(`UPDATE %s SET updated_at = NOW(), filters = jsonb_set(filters, ARRAY[CAST(@key AS text)], (
			SELECT COALESCE(jsonb_agg(v ORDER BY ord), '[]'::jsonb) FROM (
				SELECT DISTINCT ON (v) v, ord FROM (
					SELECT CASE WHEN e = ANY(CAST(@from AS text[])) THEN CAST(@to AS text) ELSE e END AS v, ord
					FROM jsonb_array_elements_text(filters->(CAST(@key AS text))) WITH ORDINALITY AS x(e, ord)
				) renamed ORDER BY v, ord
			) dedup))
		WHERE jsonb_typeof(filters->(CAST(@key AS text))) = 'array'
			AND jsonb_exists_any(filters->(CAST(@key AS text)), CAST(@from AS text[]))`, table)
	res := tx.Exec(q, map[string]any{"key": key, "from": pq.StringArray(from), "to": to})
	if res.Error != nil {
		return 0, fmt.Errorf("update %s.%s: %w", table, key, res.Error)
	}
	return res.RowsAffected, nil
}

// retargetHomepageTags points the homepage sections of every user (user_settings.homepage_config)
// that reference a source tag to the target, and returns the number of users changed.
func retargetHomepageTags(tx *gorm.DB, srcIDs []uint, srcNames []string, target Tag) (int64, error) {
	type row struct {
		ID     uint
		Config string
	}
	var rows []row
	if err := tx.Raw(`SELECT id, homepage_config::text AS config FROM user_settings`).Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("load homepage configs: %w", err)
	}
	ids := make(map[uint]bool, len(srcIDs))
	for _, id := range srcIDs {
		ids[id] = true
	}
	names := make(map[string]bool, len(srcNames))
	for _, n := range srcNames {
		names[n] = true
	}
	var changed int64
	for _, r := range rows {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(r.Config), &cfg); err != nil || cfg == nil {
			continue // not an object: nothing that could reference a tag
		}
		if !RetargetHomepageTagRefs(cfg, ids, names, target) {
			continue
		}
		out, err := json.Marshal(cfg)
		if err != nil {
			return 0, fmt.Errorf("encode homepage config: %w", err)
		}
		if err := tx.Exec(`UPDATE user_settings SET homepage_config = CAST(? AS jsonb) WHERE id = ?`, string(out), r.ID).Error; err != nil {
			return 0, fmt.Errorf("update homepage config: %w", err)
		}
		changed++
	}
	return changed, nil
}

// RetargetHomepageTagRefs rewrites, in a decoded homepage config (legacy "sections" and every
// tab's "sections"), the references to the source tags: the tag_id of "tag" sections, the
// "tags" name lists of the widget configs and the "items" of browse_tags widgets on scene
// tags. It reports whether anything changed.
func RetargetHomepageTagRefs(cfg map[string]any, srcIDs map[uint]bool, srcNames map[string]bool, target Tag) bool {
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
			typ, _ := section["type"].(string)
			if typ == "tag" && srcIDs[homepageTagID(config["tag_id"])] {
				config["tag_id"] = target.ID
				changed = true
			}
			if renameInList(config, "tags", srcNames, target.Name) {
				changed = true
			}
			if source, _ := config["source"].(string); typ == "browse_tags" && source == "tags" {
				if renameInList(config, "items", srcNames, target.Name) {
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

// homepageTagID reads the tag_id of a "tag" section (a JSON number or a numeric string).
func homepageTagID(v any) uint {
	switch id := v.(type) {
	case float64:
		if id > 0 {
			return uint(id)
		}
	case string:
		if n, err := strconv.ParseUint(id, 10, 32); err == nil {
			return uint(n)
		}
	}
	return 0
}

// renameInList replaces the names of from by to in the string list config[key], without
// duplicates and keeping the order; it reports whether the list changed.
func renameInList(config map[string]any, key string, from map[string]bool, to string) bool {
	list, ok := config[key].([]any)
	if !ok || len(from) == 0 {
		return false
	}
	hit := false
	for _, v := range list {
		if s, ok := v.(string); ok && from[s] {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	out := make([]any, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			out = append(out, v)
			continue
		}
		if from[s] {
			s = to
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	config[key] = out
	return true
}
