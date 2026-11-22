package data

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrTagNameTaken is returned by RenameTag when another tag already has the new name
// (unique constraint on tags.name).
var ErrTagNameTaken = errors.New("tag name already taken")

// RenameTag saves tag (its new name and its color) and, in the same transaction, renames the
// references to its previous name oldName: saved searches, saved marker filters, homepage
// widget configs and the legacy scenes.tags arrays. It returns the number of saved filters
// and homepage configs (one per user) changed, and the ids of the non-deleted scenes carrying
// the tag, to re-index (tags are indexed by name).
func (r *TagRepositoryImpl) RenameTag(tag Tag, oldName string) (int64, []uint, error) {
	var refs int64
	var sceneIDs []uint
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&Tag{}).Where("id = ?", tag.ID).Updates(map[string]any{"name": tag.Name, "color": tag.Color})
		if res.Error != nil {
			if isUniqueViolation(res.Error) {
				return ErrTagNameTaken
			}
			return fmt.Errorf("update tag: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if oldName == tag.Name {
			return nil
		}

		from := []string{oldName}
		for _, ref := range []struct{ table, key string }{
			{"saved_searches", "selected_tags"},
			{"saved_marker_filters", "tags"},
			{"saved_marker_filters", "marker_tags"},
		} {
			n, err := replaceNamesInSavedFilterArray(tx, ref.table, ref.key, from, tag.Name)
			if err != nil {
				return err
			}
			refs += n
		}
		n, err := retargetHomepageTags(tx, nil, from, tag)
		if err != nil {
			return err
		}
		refs += n

		// legacy name array of the scenes (filled by imports): renamed without duplicates
		if err := tx.Exec(`UPDATE scenes SET tags = ARRAY(
				SELECT v FROM (
					SELECT DISTINCT ON (v) v, ord
					FROM unnest(array_replace(tags, CAST(@old AS text), CAST(@new AS text))) WITH ORDINALITY AS x(v, ord)
					ORDER BY v, ord
				) dedup ORDER BY ord)
			WHERE CAST(@old AS text) = ANY(tags)`,
			map[string]any{"old": oldName, "new": tag.Name}).Error; err != nil {
			return fmt.Errorf("update scenes.tags: %w", err)
		}

		if err := tx.Raw(`SELECT DISTINCT st.scene_id FROM scene_tags st
				JOIN scenes s ON s.id = st.scene_id AND s.deleted_at IS NULL
				WHERE st.tag_id = ? ORDER BY st.scene_id`, tag.ID).Scan(&sceneIDs).Error; err != nil {
			return fmt.Errorf("list tag scenes: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return refs, sceneIDs, nil
}

// isUniqueViolation tells whether err is a PostgreSQL unique violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var state interface{ SQLState() string }
	return errors.As(err, &state) && state.SQLState() == "23505"
}
