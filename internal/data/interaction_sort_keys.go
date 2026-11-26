package data

import "time"

// Keys of the user-specific scene sorts of the search (core/search_user_sorts.go): when the user
// liked each scene, and how many times and when last they watched it.

// SceneWatchStat is a user's views of a scene and the date of the last one.
type SceneWatchStat struct {
	SceneID uint      `gorm:"column:scene_id"`
	Count   int64     `gorm:"column:views"`
	Last    time.Time `gorm:"column:last_watched"`
}

func (r *InteractionRepositoryImpl) GetLikeTimes(userID uint, sceneIDs []uint) (map[uint]time.Time, error) {
	out := make(map[uint]time.Time, len(sceneIDs))
	if len(sceneIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		SceneID uint      `gorm:"column:scene_id"`
		At      time.Time `gorm:"column:at"`
	}
	if err := r.DB.Raw(`SELECT scene_id, created_at AS at FROM user_scene_likes WHERE user_id = ? AND scene_id IN ?`, userID, sceneIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SceneID] = row.At
	}
	return out, nil
}

func (r *InteractionRepositoryImpl) GetWatchStats(userID uint, sceneIDs []uint) (map[uint]SceneWatchStat, error) {
	out := make(map[uint]SceneWatchStat, len(sceneIDs))
	if len(sceneIDs) == 0 {
		return out, nil
	}
	var rows []SceneWatchStat
	if err := r.DB.Raw(`SELECT scene_id, COUNT(*) AS views, MAX(watched_at) AS last_watched FROM user_scene_watches
		WHERE user_id = ? AND scene_id IN ? GROUP BY scene_id`, userID, sceneIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SceneID] = row
	}
	return out, nil
}
