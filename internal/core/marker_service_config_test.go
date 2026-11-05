package core

import "testing"

func TestMarkerServiceApplyQualityConfig(t *testing.T) {
	s := &MarkerService{
		markerThumbnailType:         "static",
		markerAnimatedDuration:      10,
		scenePreviewEnabled:         false,
		scenePreviewSegments:        12,
		scenePreviewSegmentDuration: 1.0,
		markerPreviewCRF:            32,
		scenePreviewCRF:             27,
	}

	s.ApplyQualityConfig(ProcessingQualityConfig{
		MarkerThumbnailType:         "animated",
		MarkerAnimatedDuration:      15,
		ScenePreviewEnabled:         true,
		ScenePreviewSegments:        2,
		ScenePreviewSegmentDuration: 10,
		MarkerPreviewCRF:            29,
		ScenePreviewCRF:             23,
	})

	if s.markerThumbnailType != "animated" {
		t.Fatalf("markerThumbnailType = %q, want animated", s.markerThumbnailType)
	}
	if s.markerAnimatedDuration != 15 {
		t.Fatalf("markerAnimatedDuration = %d, want 15", s.markerAnimatedDuration)
	}
	if !s.scenePreviewEnabled {
		t.Fatal("scenePreviewEnabled = false, want true")
	}
	if s.scenePreviewSegments != 2 {
		t.Fatalf("scenePreviewSegments = %d, want 2", s.scenePreviewSegments)
	}
	if s.scenePreviewSegmentDuration != 10 {
		t.Fatalf("scenePreviewSegmentDuration = %v, want 10", s.scenePreviewSegmentDuration)
	}
	if s.markerPreviewCRF != 29 {
		t.Fatalf("markerPreviewCRF = %d, want 29", s.markerPreviewCRF)
	}
	if s.scenePreviewCRF != 23 {
		t.Fatalf("scenePreviewCRF = %d, want 23", s.scenePreviewCRF)
	}
}

func TestMarkerServiceApplyQualityConfigKeepsCurrentOnZero(t *testing.T) {
	s := &MarkerService{
		markerThumbnailType:         "animated",
		markerAnimatedDuration:      15,
		scenePreviewEnabled:         true,
		scenePreviewSegments:        2,
		scenePreviewSegmentDuration: 10,
		markerPreviewCRF:            29,
		scenePreviewCRF:             23,
	}

	s.ApplyQualityConfig(ProcessingQualityConfig{ScenePreviewEnabled: true})

	if s.markerThumbnailType != "animated" || s.markerAnimatedDuration != 15 ||
		s.scenePreviewSegments != 2 || s.scenePreviewSegmentDuration != 10 ||
		s.markerPreviewCRF != 29 || s.scenePreviewCRF != 23 {
		t.Fatalf("zero values overwrote settings: %+v", s)
	}
}
