package response

import (
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestParseCardFields_ReleaseDate(t *testing.T) {
	f := ParseCardFields("views, release_date")
	if !f.ReleaseDate || !f.Views || !f.HasAny() {
		t.Fatalf("unexpected fields: %+v", f)
	}
	if ParseCardFields("views").ReleaseDate {
		t.Fatal("release_date must only be included on request")
	}
}

func TestToSceneListItemWithFields_SceneModelFields(t *testing.T) {
	release := time.Date(2024, 3, 9, 0, 0, 0, 0, time.UTC)
	scene := data.Scene{ID: 7, Title: "t", ViewCount: 3, Width: 1920, Height: 1080, Studio: "S", ReleaseDate: &release}

	item := ToSceneListItemWithFields(scene, SceneModelFields, FullFileVisibility)
	if item.ViewCount == nil || *item.ViewCount != 3 {
		t.Fatalf("view count = %v", item.ViewCount)
	}
	if item.Height == nil || *item.Height != 1080 || item.Width == nil || *item.Width != 1920 {
		t.Fatalf("resolution = %v x %v", item.Width, item.Height)
	}
	if item.Studio == nil || *item.Studio != "S" {
		t.Fatalf("studio = %v", item.Studio)
	}
	if item.ReleaseDate == nil || !item.ReleaseDate.Equal(release) {
		t.Fatalf("release date = %v", item.ReleaseDate)
	}

	plain := ToSceneListItemWithFields(scene, CardFields{Views: true}, FullFileVisibility)
	if plain.ReleaseDate != nil || plain.Studio != nil {
		t.Fatalf("unrequested fields included: %+v", plain)
	}
}
