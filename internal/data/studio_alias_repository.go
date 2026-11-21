package data

import (
	"fmt"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// StudioMergeStats describes what merging a source studio into a target studio moves.
type StudioMergeStats struct {
	// Scenes of the source studio (non-deleted scenes).
	Scenes int64 `json:"scenes"`
	// ChildStudios have the source studio as parent or network.
	ChildStudios int64 `json:"child_studios"`
	// Ratings and Likes are the user ratings/likes moved to the target: those of users
	// who had not rated/liked the target already (the target's own are kept).
	Ratings int64 `json:"ratings"`
	Likes   int64 `json:"likes"`
}

// FindByNameOrAlias returns the active studios (other than excludeID, 0 = none) whose name or
// one of whose aliases equals one of names, case-insensitively, oldest first.
func (r *StudioRepositoryImpl) FindByNameOrAlias(names []string, excludeID uint) ([]Studio, error) {
	if len(names) == 0 {
		return []Studio{}, nil
	}
	arr := pq.StringArray(names)
	var studios []Studio
	q := r.DB.Where(nameOrAliasCondition, arr, arr)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Order("id ASC").Find(&studios).Error; err != nil {
		return nil, err
	}
	return studios, nil
}

const studioMergeStatsQuery = `SELECT
	(SELECT COUNT(*) FROM scenes WHERE studio_id = @src AND deleted_at IS NULL) AS scenes,
	(SELECT COUNT(*) FROM studios WHERE deleted_at IS NULL AND id <> @dst
		AND (parent_id = @src OR network_id = @src)) AS child_studios,
	(SELECT COUNT(*) FROM user_studio_ratings r WHERE r.studio_id = @src
		AND NOT EXISTS (SELECT 1 FROM user_studio_ratings t WHERE t.user_id = r.user_id AND t.studio_id = @dst)) AS ratings,
	(SELECT COUNT(*) FROM user_studio_likes l WHERE l.studio_id = @src
		AND NOT EXISTS (SELECT 1 FROM user_studio_likes t WHERE t.user_id = l.user_id AND t.studio_id = @dst)) AS likes`

func studioMergeStats(db *gorm.DB, targetID, sourceID uint) (*StudioMergeStats, error) {
	var stats StudioMergeStats
	if err := db.Raw(studioMergeStatsQuery, map[string]any{"src": sourceID, "dst": targetID}).Scan(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetMergePreview counts what merging sourceID into targetID would move.
func (r *StudioRepositoryImpl) GetMergePreview(targetID, sourceID uint) (*StudioMergeStats, error) {
	return studioMergeStats(r.DB, targetID, sourceID)
}

// Merge merges source into target in one transaction: the source's scenes (studio_id and the
// denormalized scenes.studio), child studios (parent/network), ratings and likes go to the
// target (the target's own ratings/likes win), homepage sections and saved filters that point
// to the source point to the target, target (with the fields the caller already merged, such
// as aliases and logo) is saved and source is soft-deleted. It returns what was moved and the
// IDs of the moved scenes (to re-index).
func (r *StudioRepositoryImpl) Merge(target *Studio, source *Studio) (*StudioMergeStats, []uint, error) {
	if target.ID == source.ID {
		return nil, nil, fmt.Errorf("cannot merge studio %d into itself", target.ID)
	}
	// the target must not end up its own parent/network
	if target.ParentID != nil && *target.ParentID == source.ID {
		target.ParentID = source.ParentID
		if target.ParentID != nil && *target.ParentID == target.ID {
			target.ParentID = nil
		}
	}
	if target.NetworkID != nil && *target.NetworkID == source.ID {
		target.NetworkID = source.NetworkID
		if target.NetworkID != nil && *target.NetworkID == target.ID {
			target.NetworkID = nil
		}
	}

	var stats *StudioMergeStats
	var moved []uint
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if stats, err = studioMergeStats(tx, target.ID, source.ID); err != nil {
			return fmt.Errorf("count merge: %w", err)
		}
		ids := map[string]any{"src": source.ID, "dst": target.ID, "name": target.Name}

		if err := tx.Raw(`SELECT id FROM scenes WHERE studio_id = @src AND deleted_at IS NULL ORDER BY id`, ids).
			Scan(&moved).Error; err != nil {
			return fmt.Errorf("list source scenes: %w", err)
		}
		steps := []struct {
			name string
			sql  string
		}{
			{"move scenes", `UPDATE scenes SET studio_id = @dst, studio = @name WHERE studio_id = @src`},
			{"move child studios", `UPDATE studios SET parent_id = @dst WHERE parent_id = @src AND id <> @dst`},
			{"move network studios", `UPDATE studios SET network_id = @dst WHERE network_id = @src AND id <> @dst`},
			{"move ratings", `UPDATE user_studio_ratings r SET studio_id = @dst, updated_at = NOW()
				WHERE r.studio_id = @src
				AND NOT EXISTS (SELECT 1 FROM user_studio_ratings t WHERE t.user_id = r.user_id AND t.studio_id = @dst)`},
			{"drop source ratings", `DELETE FROM user_studio_ratings WHERE studio_id = @src`},
			{"move likes", `UPDATE user_studio_likes l SET studio_id = @dst
				WHERE l.studio_id = @src
				AND NOT EXISTS (SELECT 1 FROM user_studio_likes t WHERE t.user_id = l.user_id AND t.studio_id = @dst)`},
			{"drop source likes", `DELETE FROM user_studio_likes WHERE studio_id = @src`},
		}
		for _, step := range steps {
			if err := tx.Exec(step.sql, ids).Error; err != nil {
				return fmt.Errorf("%s: %w", step.name, err)
			}
		}
		if err := replaceHomepageUUID(tx, source.UUID.String(), target.UUID.String()); err != nil {
			return err
		}
		if source.Name != target.Name {
			for _, table := range []string{"saved_searches", "saved_marker_filters"} {
				if err := renameInSavedFilterString(tx, table, "studio", source.Name, target.Name); err != nil {
					return err
				}
			}
		}
		if err := tx.Save(target).Error; err != nil {
			return fmt.Errorf("save target studio: %w", err)
		}
		if err := tx.Delete(&Studio{}, source.ID).Error; err != nil {
			return fmt.Errorf("delete source studio: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return stats, moved, nil
}
