package core

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"goonhub/internal/data"
)

type testFingerprintIndex struct {
	removed []uint
	failID  uint
}

func (index *testFingerprintIndex) RemoveSceneFromIndex(id uint) error {
	index.removed = append(index.removed, id)
	if id == index.failID {
		return errors.New("fingerprint storage unavailable")
	}
	return nil
}

func TestHardDeleteSceneCleansFingerprints(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			svc, scenes := newTestSceneService(t)
			index := &testFingerprintIndex{}
			if enabled {
				svc.fingerprintIndex = index
			}
			scenes.EXPECT().HardDelete(uint(10)).DoAndReturn(func(uint) (*data.Scene, error) {
				if enabled && !reflect.DeepEqual(index.removed, []uint{10}) {
					t.Fatal("fingerprints must be removed before deleting the database record")
				}
				return &data.Scene{ID: 10}, nil
			})
			if err := svc.HardDeleteScene(10); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHardDeleteSceneRetainsSceneOnFingerprintFailure(t *testing.T) {
	svc, _ := newTestSceneService(t)
	svc.fingerprintIndex = &testFingerprintIndex{failID: 10}
	if err := svc.HardDeleteScene(10); err == nil {
		t.Fatal("expected cleanup error; database deletion must not be attempted")
	}
}

func TestEmptyTrashReportsFingerprintFailure(t *testing.T) {
	svc, scenes := newTestSceneService(t)
	svc.fingerprintIndex = &testFingerprintIndex{failID: 10}
	scenes.EXPECT().ListTrashed(1, 10000).Return([]data.Scene{{ID: 10}}, int64(1), nil)
	if count, err := svc.EmptyTrash(); count != 0 || err == nil {
		t.Fatalf("expected no deletion and cleanup error, got count=%d err=%v", count, err)
	}
}

func TestBulkDeleteScenesCleansFingerprints(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		t.Run(map[bool]string{false: "trash", true: "permanent"}[permanent], func(t *testing.T) {
			svc, _, _, scenes, _, _, _ := newTestExplorerService(t)
			svc.jobHistoryRepo = nil
			index := &testFingerprintIndex{}
			svc.fingerprintIndex = index
			scenes.EXPECT().GetByIDs([]uint{10}).Return([]data.Scene{{ID: 10}}, nil)
			if permanent {
				scenes.EXPECT().HardDelete(uint(10)).DoAndReturn(func(uint) (*data.Scene, error) {
					if !reflect.DeepEqual(index.removed, []uint{10}) {
						t.Fatal("fingerprints must be removed before deleting the database record")
					}
					return &data.Scene{ID: 10}, nil
				})
			} else {
				now := time.Now()
				scenes.EXPECT().MoveToTrash(uint(10)).Return(&now, nil)
			}
			if count, err := svc.BulkDeleteScenes([]uint{10}, permanent); count != 1 || err != nil {
				t.Fatalf("expected one deletion, got count=%d err=%v", count, err)
			}
			if !permanent && len(index.removed) != 0 {
				t.Fatal("trashing must preserve fingerprints for restoration")
			}
		})
	}
}

func TestBulkDeleteScenesReportsPartialFingerprintFailure(t *testing.T) {
	svc, _, _, scenes, _, _, _ := newTestExplorerService(t)
	svc.jobHistoryRepo = nil
	index := &testFingerprintIndex{failID: 20}
	svc.fingerprintIndex = index
	scenes.EXPECT().GetByIDs([]uint{10, 20}).Return([]data.Scene{{ID: 10}, {ID: 20}}, nil)
	scenes.EXPECT().HardDelete(uint(10)).Return(&data.Scene{ID: 10}, nil)
	if count, err := svc.BulkDeleteScenes([]uint{10, 20}, true); count != 1 || err == nil {
		t.Fatalf("expected one deletion and cleanup error, got count=%d err=%v", count, err)
	}
	if !reflect.DeepEqual(index.removed, []uint{10, 20}) {
		t.Fatalf("unexpected cleanup calls: %v", index.removed)
	}
}
