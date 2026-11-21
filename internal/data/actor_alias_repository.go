package data

import (
	"fmt"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// ActorMergeStats describes what merging a source actor into a target actor moves.
type ActorMergeStats struct {
	// Scenes credits the source actor (non-deleted scenes).
	Scenes int64 `json:"scenes"`
	// SharedScenes are the scenes among Scenes that already credit the target actor.
	SharedScenes int64 `json:"shared_scenes"`
	// Ratings and Likes are the user ratings/likes moved to the target: those of users
	// who had not rated/liked the target already (the target's own are kept).
	Ratings int64 `json:"ratings"`
	Likes   int64 `json:"likes"`
}

// nameOrAliasCondition matches rows whose name or one of whose aliases equals, case-insensitively,
// one of the names of the text[] parameter (used twice).
const nameOrAliasCondition = `(lower(name) IN (SELECT lower(n) FROM unnest(?::text[]) AS n)
	OR EXISTS (SELECT 1 FROM unnest(aliases) AS al(v) WHERE lower(al.v) IN (SELECT lower(n) FROM unnest(?::text[]) AS n)))`

// FindByNameOrAlias returns the active actors (other than excludeID, 0 = none) whose name or
// one of whose aliases equals one of names, case-insensitively, oldest first.
func (r *ActorRepositoryImpl) FindByNameOrAlias(names []string, excludeID uint) ([]Actor, error) {
	if len(names) == 0 {
		return []Actor{}, nil
	}
	arr := pq.StringArray(names)
	var actors []Actor
	q := r.DB.Where(nameOrAliasCondition, arr, arr)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Order("id ASC").Find(&actors).Error; err != nil {
		return nil, err
	}
	return actors, nil
}

const actorMergeStatsQuery = `SELECT
	(SELECT COUNT(*) FROM scene_actors sa JOIN scenes s ON s.id = sa.scene_id AND s.deleted_at IS NULL
		WHERE sa.actor_id = @src) AS scenes,
	(SELECT COUNT(*) FROM scene_actors sa JOIN scenes s ON s.id = sa.scene_id AND s.deleted_at IS NULL
		WHERE sa.actor_id = @src
		AND EXISTS (SELECT 1 FROM scene_actors t WHERE t.scene_id = sa.scene_id AND t.actor_id = @dst)) AS shared_scenes,
	(SELECT COUNT(*) FROM user_actor_ratings r WHERE r.actor_id = @src
		AND NOT EXISTS (SELECT 1 FROM user_actor_ratings t WHERE t.user_id = r.user_id AND t.actor_id = @dst)) AS ratings,
	(SELECT COUNT(*) FROM user_actor_likes l WHERE l.actor_id = @src
		AND NOT EXISTS (SELECT 1 FROM user_actor_likes t WHERE t.user_id = l.user_id AND t.actor_id = @dst)) AS likes`

func actorMergeStats(db *gorm.DB, targetID, sourceID uint) (*ActorMergeStats, error) {
	var stats ActorMergeStats
	if err := db.Raw(actorMergeStatsQuery, map[string]any{"src": sourceID, "dst": targetID}).Scan(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetMergePreview counts what merging sourceID into targetID would move.
func (r *ActorRepositoryImpl) GetMergePreview(targetID, sourceID uint) (*ActorMergeStats, error) {
	return actorMergeStats(r.DB, targetID, sourceID)
}

// Merge merges source into target in one transaction: the source's scenes, ratings and likes
// go to the target (the target's own ratings/likes win), homepage sections and saved filters
// that point to the source point to the target, target (with the fields the caller already
// merged, such as aliases and photo) is saved and source is soft-deleted. It returns what was
// moved and the IDs of all the target's scenes after the merge (to re-index).
func (r *ActorRepositoryImpl) Merge(target *Actor, source *Actor) (*ActorMergeStats, []uint, error) {
	if target.ID == source.ID {
		return nil, nil, fmt.Errorf("cannot merge actor %d into itself", target.ID)
	}
	var stats *ActorMergeStats
	var sceneIDs []uint
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if stats, err = actorMergeStats(tx, target.ID, source.ID); err != nil {
			return fmt.Errorf("count merge: %w", err)
		}
		ids := map[string]any{"src": source.ID, "dst": target.ID}

		var moved []uint
		if err := tx.Raw(`SELECT scene_id FROM scene_actors WHERE actor_id = @src`, ids).Scan(&moved).Error; err != nil {
			return fmt.Errorf("list source scenes: %w", err)
		}
		steps := []struct {
			name string
			sql  string
		}{
			{"move scenes", `INSERT INTO scene_actors (scene_id, actor_id)
				SELECT scene_id, @dst FROM scene_actors WHERE actor_id = @src
				ON CONFLICT DO NOTHING`},
			{"drop source scenes", `DELETE FROM scene_actors WHERE actor_id = @src`},
			{"move ratings", `UPDATE user_actor_ratings r SET actor_id = @dst, updated_at = NOW()
				WHERE r.actor_id = @src
				AND NOT EXISTS (SELECT 1 FROM user_actor_ratings t WHERE t.user_id = r.user_id AND t.actor_id = @dst)`},
			{"drop source ratings", `DELETE FROM user_actor_ratings WHERE actor_id = @src`},
			{"move likes", `UPDATE user_actor_likes l SET actor_id = @dst
				WHERE l.actor_id = @src
				AND NOT EXISTS (SELECT 1 FROM user_actor_likes t WHERE t.user_id = l.user_id AND t.actor_id = @dst)`},
			{"drop source likes", `DELETE FROM user_actor_likes WHERE actor_id = @src`},
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
			if err := renameInSavedFilterArray(tx, "saved_searches", "selected_actors", source.Name, target.Name); err != nil {
				return err
			}
			if err := renameInSavedFilterArray(tx, "saved_marker_filters", "actors", source.Name, target.Name); err != nil {
				return err
			}
		}
		if err := tx.Save(target).Error; err != nil {
			return fmt.Errorf("save target actor: %w", err)
		}
		if err := tx.Delete(&Actor{}, source.ID).Error; err != nil {
			return fmt.Errorf("delete source actor: %w", err)
		}
		if len(moved) > 0 {
			// denormalized scenes.actors of the scenes that changed
			if err := tx.Exec(`UPDATE scenes s SET actors = COALESCE((
					SELECT array_agg(a.name ORDER BY a.name) FROM scene_actors sa
					JOIN actors a ON a.id = sa.actor_id AND a.deleted_at IS NULL
					WHERE sa.scene_id = s.id), '{}')
				WHERE s.id IN ?`, moved).Error; err != nil {
				return fmt.Errorf("update denormalized actors: %w", err)
			}
		}
		if err := tx.Raw(`SELECT sa.scene_id FROM scene_actors sa
				JOIN scenes s ON s.id = sa.scene_id AND s.deleted_at IS NULL
				WHERE sa.actor_id = @dst ORDER BY sa.scene_id`, ids).Scan(&sceneIDs).Error; err != nil {
			return fmt.Errorf("list target scenes: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return stats, sceneIDs, nil
}

// replaceHomepageUUID points the homepage sections (user_settings.homepage_config) that
// reference an actor or studio by UUID to another one.
func replaceHomepageUUID(tx *gorm.DB, from, to string) error {
	if err := tx.Exec(`UPDATE user_settings SET homepage_config = replace(homepage_config::text, CAST(@from AS text), CAST(@to AS text))::jsonb
		WHERE strpos(homepage_config::text, CAST(@from AS text)) > 0`, map[string]any{"from": from, "to": to}).Error; err != nil {
		return fmt.Errorf("update homepage sections: %w", err)
	}
	return nil
}

// renameInSavedFilterArray replaces the name from by to in the JSON string array filters->key
// of a saved filter table (saved_searches, saved_marker_filters), without duplicates and
// keeping the order.
func renameInSavedFilterArray(tx *gorm.DB, table, key, from, to string) error {
	q := fmt.Sprintf(`UPDATE %s SET updated_at = NOW(), filters = jsonb_set(filters, ARRAY[CAST(@key AS text)], (
			SELECT COALESCE(jsonb_agg(v ORDER BY ord), '[]'::jsonb) FROM (
				SELECT DISTINCT ON (v) v, ord FROM (
					SELECT CASE WHEN e = CAST(@from AS text) THEN CAST(@to AS text) ELSE e END AS v, ord
					FROM jsonb_array_elements_text(filters->(CAST(@key AS text))) WITH ORDINALITY AS x(e, ord)
				) renamed ORDER BY v, ord
			) dedup))
		WHERE jsonb_typeof(filters->(CAST(@key AS text))) = 'array' AND jsonb_exists(filters->(CAST(@key AS text)), CAST(@from AS text))`, table)
	if err := tx.Exec(q, map[string]any{"key": key, "from": from, "to": to}).Error; err != nil {
		return fmt.Errorf("update %s.%s: %w", table, key, err)
	}
	return nil
}

// renameInSavedFilterString replaces the name from by to in the JSON string filters->key of a
// saved filter table.
func renameInSavedFilterString(tx *gorm.DB, table, key, from, to string) error {
	q := fmt.Sprintf(`UPDATE %s SET updated_at = NOW(), filters = jsonb_set(filters, ARRAY[CAST(@key AS text)], to_jsonb(CAST(@to AS text)))
		WHERE filters->>(CAST(@key AS text)) = CAST(@from AS text)`, table)
	if err := tx.Exec(q, map[string]any{"key": key, "from": from, "to": to}).Error; err != nil {
		return fmt.Errorf("update %s.%s: %w", table, key, err)
	}
	return nil
}
