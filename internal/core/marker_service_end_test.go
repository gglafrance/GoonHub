package core

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestMarkerService(t *testing.T) (*MarkerService, *mocks.MockMarkerRepository, *mocks.MockSceneRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	markerRepo := mocks.NewMockMarkerRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	return &MarkerService{markerRepo: markerRepo, sceneRepo: sceneRepo, logger: zap.NewNop()}, markerRepo, sceneRepo
}

func TestValidateMarkerEnd(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		end      *int
		duration int
		wantErr  bool
	}{
		{"no end", 10, nil, 100, false},
		{"valid range", 10, ptr(20), 100, false},
		{"end at duration", 10, ptr(100), 100, false},
		{"unknown duration", 10, ptr(5000), 0, false},
		{"end equal start", 10, ptr(10), 100, true},
		{"end before start", 10, ptr(5), 100, true},
		{"end beyond duration", 10, ptr(101), 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMarkerEnd(tt.start, tt.end, tt.duration)
			if tt.wantErr {
				if !apperrors.IsValidation(err) {
					t.Fatalf("expected validation error, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateMarker_SetEnd(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
	sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil)
	markerRepo.EXPECT().Update(marker).Return(nil)

	got, err := svc.UpdateMarker(1, 3, nil, nil, nil, ptr(25), false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EndTimestamp == nil || *got.EndTimestamp != 25 {
		t.Fatalf("expected end 25, got %v", got.EndTimestamp)
	}
}

func TestUpdateMarker_ClearEnd(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10, EndTimestamp: ptr(25)}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
	markerRepo.EXPECT().Update(marker).Return(nil)

	got, err := svc.UpdateMarker(1, 3, nil, nil, nil, ptr(40), true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EndTimestamp != nil {
		t.Fatalf("expected end cleared, got %v", *got.EndTimestamp)
	}
}

// Updating another field (here the color) without end_timestamp nor clear_end_timestamp
// keeps the end of the range.
func TestUpdateMarker_OmittedEndUnchanged(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10, EndTimestamp: ptr(25)}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
	markerRepo.EXPECT().Update(marker).Return(nil)

	color := "#00FF00"
	got, err := svc.UpdateMarker(1, 3, nil, &color, nil, nil, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EndTimestamp == nil || *got.EndTimestamp != 25 {
		t.Fatalf("expected end 25 kept, got %v", got.EndTimestamp)
	}
}

// Clearing the end while moving the start past the old end is valid: the old end is gone.
func TestUpdateMarker_ClearEndWithStartPastOldEnd(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10, EndTimestamp: ptr(25)}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
	sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil)
	markerRepo.EXPECT().Update(marker).Return(nil)
	// The start moved: the new thumbnail is queued (not made inline)
	svc.thumbnailQueue = make(chan markerThumbnailTask, 1)

	got, err := svc.UpdateMarker(1, 3, nil, nil, ptr(30), nil, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Timestamp != 30 || got.EndTimestamp != nil {
		t.Fatalf("expected start 30 without end, got %d %v", got.Timestamp, got.EndTimestamp)
	}
	if len(svc.thumbnailQueue) != 1 {
		t.Fatalf("expected the thumbnail to be queued, got %d tasks", len(svc.thumbnailQueue))
	}
}

func TestUpdateMarker_ClearEndOfOtherUsersMarker(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 2, SceneID: 2, Timestamp: 10, EndTimestamp: ptr(25)}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)

	_, err := svc.UpdateMarker(1, 3, nil, nil, nil, nil, true, false)
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if marker.EndTimestamp == nil {
		t.Fatal("end of the other user's marker was cleared")
	}
}

func TestUpdateMarker_StartMovedPastExistingEnd(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10, EndTimestamp: ptr(25)}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
	sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil)

	_, err := svc.UpdateMarker(1, 3, nil, nil, ptr(30), nil, false, false)
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdateMarker_EndBeyondDuration(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10}
	markerRepo.EXPECT().GetByID(uint(3)).Return(marker, nil)
	sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil)

	_, err := svc.UpdateMarker(1, 3, nil, nil, nil, ptr(150), false, false)
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
