package response

import "goonhub/internal/data"

// ShortListItem is a scene card plus the fields the Shorts feed and a scene's
// Shorts tab need: its size for layout and, for created shorts, where in the
// source it was cut from.
type ShortListItem struct {
	SceneListItem
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	ViewCount     int64    `json:"view_count"`
	SourceSceneID *uint    `json:"source_scene_id"`
	SourceStart   *float64 `json:"source_start"`
	SourceEnd     *float64 `json:"source_end"`
}

// ToShortListItems converts scenes to ShortListItems.
func ToShortListItems(scenes []data.Scene) []ShortListItem {
	items := make([]ShortListItem, len(scenes))
	for i, s := range scenes {
		items[i] = ShortListItem{
			SceneListItem: ToSceneListItem(s),
			Width:         s.Width,
			Height:        s.Height,
			ViewCount:     s.ViewCount,
			SourceSceneID: s.SourceSceneID,
			SourceStart:   s.SourceStart,
			SourceEnd:     s.SourceEnd,
		}
	}
	return items
}
