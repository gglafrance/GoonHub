package response

import (
	"strings"
	"time"

	"goonhub/internal/data"
)

// SceneListItem is a lightweight representation for scene list/grid endpoints.
// Contains only the fields needed for displaying scene cards.
type SceneListItem struct {
	ID               uint      `json:"id"`
	Title            string    `json:"title"`
	Duration         int       `json:"duration"`
	Size             int64     `json:"size"`
	ThumbnailPath    string    `json:"thumbnail_path"`
	PreviewVideoPath string    `json:"preview_video_path"`
	ProcessingStatus string    `json:"processing_status"`
	IsCorrupted      bool      `json:"is_corrupted"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	StoredPath       string    `json:"stored_path"`

	// Optional fields included when requested via card_fields
	ViewCount   *int64   `json:"view_count,omitempty"`
	Width       *int     `json:"width,omitempty"`
	Height      *int     `json:"height,omitempty"`
	FrameRate   *float64 `json:"frame_rate,omitempty"`
	Description *string  `json:"description,omitempty"`
	Studio      *string  `json:"studio,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Actors      []string `json:"actors,omitempty"`
	// ReleaseDate is included with the release_date card field (null when unknown)
	ReleaseDate *time.Time `json:"release_date,omitempty"`
	// Markers of the current user, included together with tags
	Markers []SceneCardMarker `json:"markers,omitempty"`
}

// SceneCardMarker is the marker summary shown in the scene card markers popover.
type SceneCardMarker struct {
	ID           uint   `json:"id"`
	Label        string `json:"label"`
	Color        string `json:"color"`
	Timestamp    int    `json:"timestamp"`
	EndTimestamp *int   `json:"end_timestamp,omitempty"`
}

// ToSceneCardMarkers converts markers to their card summary.
func ToSceneCardMarkers(markers []data.UserSceneMarker) []SceneCardMarker {
	out := make([]SceneCardMarker, len(markers))
	for i, m := range markers {
		out[i] = SceneCardMarker{ID: m.ID, Label: m.Label, Color: m.Color, Timestamp: m.Timestamp, EndTimestamp: m.EndTimestamp}
	}
	return out
}

// CardFields tracks which optional fields should be included in SceneListItem responses.
type CardFields struct {
	Views       bool
	Resolution  bool
	FrameRate   bool
	Description bool
	Studio      bool
	Tags        bool
	Actors      bool
	Rating      bool
	Liked       bool
	JizzCount   bool
	ReleaseDate bool
}

// SceneModelFields are the optional fields read from the scene row itself (no extra query),
// included by the list endpoints without card_fields support (explorer, playlists).
var SceneModelFields = CardFields{Views: true, Resolution: true, Studio: true, ReleaseDate: true}

// HasAny returns true if any field is requested.
func (f CardFields) HasAny() bool {
	return f.Views || f.Resolution || f.FrameRate || f.Description ||
		f.Studio || f.Tags || f.Actors || f.Rating || f.Liked || f.JizzCount || f.ReleaseDate
}

// ParseCardFields parses a comma-separated string of field names into CardFields.
func ParseCardFields(raw string) CardFields {
	if raw == "" {
		return CardFields{}
	}
	var f CardFields
	for _, field := range strings.Split(raw, ",") {
		switch strings.TrimSpace(field) {
		case "views":
			f.Views = true
		case "resolution":
			f.Resolution = true
		case "frame_rate":
			f.FrameRate = true
		case "description":
			f.Description = true
		case "studio":
			f.Studio = true
		case "tags":
			f.Tags = true
		case "actors":
			f.Actors = true
		case "rating":
			f.Rating = true
		case "liked":
			f.Liked = true
		case "jizz_count":
			f.JizzCount = true
		case "release_date":
			f.ReleaseDate = true
		}
	}
	return f
}

// ToSceneListItem converts a full Scene model to a lightweight SceneListItem.
func ToSceneListItem(v data.Scene) SceneListItem {
	return SceneListItem{
		ID:               v.ID,
		Title:            v.Title,
		Duration:         v.Duration,
		Size:             v.Size,
		ThumbnailPath:    v.ThumbnailPath,
		PreviewVideoPath: v.PreviewVideoPath,
		ProcessingStatus: v.ProcessingStatus,
		IsCorrupted:      v.IsCorrupted,
		CreatedAt:        v.CreatedAt,
		UpdatedAt:        v.UpdatedAt,
		StoredPath:       v.StoredPath,
	}
}

// ToSceneListItems converts a slice of Scene models to SceneListItems.
func ToSceneListItems(scenes []data.Scene) []SceneListItem {
	items := make([]SceneListItem, len(scenes))
	for i, v := range scenes {
		items[i] = ToSceneListItem(v)
	}
	return items
}

// ToSceneListItemWithFields converts a Scene model with optional fields based on CardFields.
func ToSceneListItemWithFields(v data.Scene, fields CardFields) SceneListItem {
	item := ToSceneListItem(v)

	if fields.Views {
		vc := v.ViewCount
		item.ViewCount = &vc
	}
	if fields.Resolution {
		w := v.Width
		h := v.Height
		item.Width = &w
		item.Height = &h
	}
	if fields.FrameRate {
		fr := v.FrameRate
		item.FrameRate = &fr
	}
	if fields.Description {
		d := v.Description
		item.Description = &d
	}
	if fields.Studio {
		s := v.Studio
		item.Studio = &s
	}
	if fields.ReleaseDate {
		item.ReleaseDate = v.ReleaseDate
	}
	// Tags and Actors are loaded from join tables by the handler,
	// not from the scene model columns (which may be empty).
	// The handler populates item.Tags and item.Actors after this call.

	return item
}

// ToSceneListItemsWithFields converts a slice of Scene models with optional fields.
func ToSceneListItemsWithFields(scenes []data.Scene, fields CardFields) []SceneListItem {
	items := make([]SceneListItem, len(scenes))
	for i, v := range scenes {
		items[i] = ToSceneListItemWithFields(v, fields)
	}
	return items
}
