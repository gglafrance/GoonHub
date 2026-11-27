package core

import (
	"testing"
	"time"

	"goonhub/internal/data"

	"gorm.io/gorm"
)

// A scene without a stored path makes thumbnail generation fail before ffmpeg runs, so
// these tests only see the repository calls of the queue itself.

func TestMarkerThumbnailQueue_WorkerReloadsMarker(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()
	svc.thumbnailWorkers = 1

	markerRepo.EXPECT().GetByID(uint(5)).Return(&data.UserSceneMarker{ID: 5, SceneID: 2, Timestamp: 40}, nil)
	sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil)

	svc.Start()
	svc.queueThumbnail(&data.UserSceneMarker{ID: 5, SceneID: 2, Timestamp: 10}, &data.Scene{ID: 2})
	svc.Stop(5 * time.Second)
}

func TestMarkerThumbnailQueue_DeletedMarkerSkipped(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()

	markerRepo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)

	// No scene lookup and no thumbnail write are expected
	svc.runThumbnailTask(markerThumbnailTask{markerID: 9})
}

func TestMarkerThumbnailQueue_InlineWhenNotStarted(t *testing.T) {
	svc, _, _ := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()

	// No reload: the thumbnail is made at once from the given marker
	svc.queueThumbnail(&data.UserSceneMarker{ID: 5, SceneID: 2}, &data.Scene{ID: 2})
}

func TestMarkerThumbnailQueue_InlineWhenFull(t *testing.T) {
	svc, _, _ := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()
	// Unbuffered and no worker: the send can't go through
	svc.thumbnailQueue = make(chan markerThumbnailTask)

	done := make(chan struct{})
	go func() {
		svc.queueThumbnail(&data.UserSceneMarker{ID: 5, SceneID: 2}, &data.Scene{ID: 2})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("queueThumbnail blocked on a full queue")
	}
}

func TestMarkerThumbnailQueue_StopWithoutStart(t *testing.T) {
	svc, _, _ := newTestMarkerService(t)
	svc.Stop(time.Second)
	svc.Stop(time.Second)
}

func TestMarkerThumbnailQueue_StopDrainsQueue(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()
	svc.thumbnailWorkers = 2

	for id := uint(1); id <= 10; id++ {
		markerRepo.EXPECT().GetByID(id).Return(&data.UserSceneMarker{ID: id, SceneID: 2}, nil)
	}
	sceneRepo.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2}, nil).Times(10)

	svc.Start()
	for id := uint(1); id <= 10; id++ {
		svc.queueThumbnail(&data.UserSceneMarker{ID: id, SceneID: 2}, &data.Scene{ID: 2})
	}
	svc.Stop(5 * time.Second)
}
