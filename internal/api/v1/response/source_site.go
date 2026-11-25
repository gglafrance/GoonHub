package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// SceneDetail is a scene as returned by GET /scenes/:id, the upload and the scene details
// update: the scene with the normalized site of its source URL ("" without a source), the
// playback info when set, without the file information the user may not see.
type SceneDetail struct {
	Scene      *data.Scene
	SourceSite string
	// Playback (how the player gets the scene) is added when set
	Playback   any
	Visibility FileVisibility
}

// NewSceneDetail adds the source site to a scene.
func NewSceneDetail(scene *data.Scene, vis FileVisibility) SceneDetail {
	detail := SceneDetail{Scene: scene, Visibility: vis}
	if scene != nil {
		detail.SourceSite = core.SourceSiteFromURL(scene.SourceURL)
	}
	return detail
}

// MarshalJSON writes the scene fields (minus the hidden ones), source_site and playback.
func (d SceneDetail) MarshalJSON() ([]byte, error) {
	if d.Scene == nil {
		return []byte("null"), nil
	}
	extra := map[string]any{"source_site": d.SourceSite}
	if d.Playback != nil {
		extra["playback"] = d.Playback
	}
	return marshalSceneWithout(d.Scene, d.Visibility, extra)
}

// SourceSiteItem is a source site with its scene count.
type SourceSiteItem struct {
	Site  string `json:"site"`
	Count int64  `json:"count"`
}

// SourceSiteList is the answer of GET /scenes/source-sites.
type SourceSiteList struct {
	Sites []SourceSiteItem `json:"sites"`
}

// ToSourceSiteList converts source site counts (never a null list).
func ToSourceSiteList(sites []core.SourceSiteCount) SourceSiteList {
	items := make([]SourceSiteItem, len(sites))
	for i, s := range sites {
		items[i] = SourceSiteItem{Site: s.Site, Count: s.Count}
	}
	return SourceSiteList{Sites: items}
}
