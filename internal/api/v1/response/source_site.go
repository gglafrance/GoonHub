package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// SceneDetail is a scene as returned by GET /scenes/:id and the scene details update: the
// scene with the normalized site of its source URL ("" without a source).
type SceneDetail struct {
	*data.Scene
	SourceSite string `json:"source_site"`
}

// NewSceneDetail adds the source site to a scene.
func NewSceneDetail(scene *data.Scene) SceneDetail {
	detail := SceneDetail{Scene: scene}
	if scene != nil {
		detail.SourceSite = core.SourceSiteFromURL(scene.SourceURL)
	}
	return detail
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
