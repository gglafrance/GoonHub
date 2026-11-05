package core

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
)

func TestResolveGroupPropagatesFailures(t *testing.T) {
	for _, failure := range []string{"metadata", "member flag", "scene lookup", "trash", "best scene", "status"} {
		t.Run(failure, func(t *testing.T) {
			svc, groups, scenes, tags, _, _, _ := newTestDuplicateService(t)
			cause := errors.New("database unavailable")
			groups.EXPECT().GetByIDWithMembers(uint(1)).Return(&data.DuplicateGroup{
				ID: 1, Status: "unresolved", Members: []data.DuplicateGroupMember{
					{GroupID: 1, SceneID: 10}, {GroupID: 1, SceneID: 20},
				},
			}, nil)
			switch failure {
			case "metadata":
				tags.EXPECT().GetSceneTags(uint(10)).Return(nil, cause)
			case "member flag":
				groups.EXPECT().UpdateMemberBest(uint(1), uint(10), true).Return(cause)
			default:
				groups.EXPECT().UpdateMemberBest(uint(1), uint(10), true).Return(nil)
				groups.EXPECT().UpdateMemberBest(uint(1), uint(20), false).Return(nil)
				if failure == "scene lookup" {
					scenes.EXPECT().GetByIDIncludingTrashed(uint(20)).Return(nil, cause)
					break
				}
				scenes.EXPECT().GetByIDIncludingTrashed(uint(20)).Return(&data.Scene{ID: 20}, nil)
				if failure == "trash" {
					scenes.EXPECT().MoveToTrash(uint(20)).Return(nil, cause)
				} else {
					now := time.Now()
					scenes.EXPECT().MoveToTrash(uint(20)).Return(&now, nil)
					if failure == "best scene" {
						groups.EXPECT().SetBestScene(uint(1), uint(10)).Return(cause)
					} else {
						groups.EXPECT().SetBestScene(uint(1), uint(10)).Return(nil)
						groups.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Any()).Return(cause)
					}
				}
			}
			if err := svc.ResolveGroup(1, 10, failure == "metadata"); !errors.Is(err, cause) {
				t.Fatalf("expected underlying failure, got %v", err)
			}
		})
	}
}

type duplicateResolutionIndexer struct {
	SceneIndexer
	deletedIDs []uint
}

func (i *duplicateResolutionIndexer) DeleteSceneIndex(id uint) error {
	i.deletedIDs = append(i.deletedIDs, id)
	return nil
}

func TestResolveGroupUsesTrashLifecycle(t *testing.T) {
	for _, alreadyTrashed := range []bool{false, true} {
		t.Run(fmt.Sprintf("already_trashed=%t", alreadyTrashed), func(t *testing.T) {
			svc, groups, scenes, _, _, _, _ := newTestDuplicateService(t)
			jobs := mocks.NewMockJobHistoryRepository(gomock.NewController(t))
			indexer := &duplicateResolutionIndexer{}
			svc.sceneService.jobHistoryRepo = jobs
			svc.sceneService.indexer = indexer
			subscriber, events := svc.sceneService.EventBus.Subscribe()
			defer svc.sceneService.EventBus.Unsubscribe(subscriber)

			groups.EXPECT().GetByIDWithMembers(uint(1)).Return(&data.DuplicateGroup{
				ID: 1, Status: "unresolved", Members: []data.DuplicateGroupMember{
					{GroupID: 1, SceneID: 10}, {GroupID: 1, SceneID: 20}, {GroupID: 1, SceneID: 30},
				},
			}, nil)
			groups.EXPECT().UpdateMemberBest(uint(1), uint(10), true).Return(nil)
			now := time.Now()
			for _, id := range []uint{20, 30} {
				groups.EXPECT().UpdateMemberBest(uint(1), id, false).Return(nil)
				scene := &data.Scene{ID: id, Title: fmt.Sprintf("Scene %d", id)}
				if alreadyTrashed && id == 20 {
					previouslyTrashed := now.Add(-24 * time.Hour)
					scene.TrashedAt = &previouslyTrashed
				} else {
					scenes.EXPECT().MoveToTrash(id).Return(&now, nil)
				}
				scenes.EXPECT().GetByIDIncludingTrashed(id).Return(scene, nil)
				jobs.EXPECT().CancelPendingJobsForScene(id).Return(int64(2), nil)
			}
			groups.EXPECT().SetBestScene(uint(1), uint(10)).Return(nil)
			groups.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Any()).Return(nil)

			if err := svc.ResolveGroup(1, 10, false); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(indexer.deletedIDs, []uint{20, 30}) {
				t.Fatalf("search removals = %v, want [20 30]", indexer.deletedIDs)
			}
			for _, id := range []uint{20, 30} {
				select {
				case event := <-events:
					if event.Type != "scene:trashed" || event.SceneID != id {
						t.Fatalf("unexpected trash event: %+v", event)
					}
					payload, ok := event.Data.(map[string]any)
					if !ok {
						t.Fatalf("unexpected event data: %#v", event.Data)
					}
					trashedAt, ok := payload["trashed_at"].(*time.Time)
					want := now
					if alreadyTrashed && id == 20 {
						want = now.Add(-24 * time.Hour)
					}
					if !ok || trashedAt == nil || !trashedAt.Equal(want) {
						t.Fatalf("trash timestamp = %v, want %v", trashedAt, want)
					}
				default:
					t.Fatalf("missing trash event for scene %d", id)
				}
			}
			select {
			case event := <-events:
				t.Fatalf("unexpected extra event: %+v", event)
			default:
			}
		})
	}
}

func TestMergeMetadataPropagatesFailures(t *testing.T) {
	for _, failure := range []string{"source tags", "write tags", "source actors", "write actors", "scenes", "studio", "markers", "interactions"} {
		t.Run(failure, func(t *testing.T) {
			svc, _, scenes, tags, actors, markers, interactions := newTestDuplicateService(t)
			cause := errors.New("metadata write failed")
			svc.tagRepo, svc.actorRepo, svc.markerRepo, svc.interactionRepo = nil, nil, nil, nil
			switch failure {
			case "source tags", "write tags":
				svc.tagRepo = tags
				tags.EXPECT().GetSceneTags(uint(10)).Return(nil, nil)
				if failure == "source tags" {
					tags.EXPECT().GetSceneTags(uint(20)).Return(nil, cause)
				} else {
					tags.EXPECT().GetSceneTags(uint(20)).Return([]data.Tag{{ID: 1}}, nil)
					tags.EXPECT().BulkAddTagsToScenes([]uint{10}, []uint{1}).Return(cause)
				}
			case "source actors", "write actors":
				svc.actorRepo = actors
				actors.EXPECT().GetSceneActors(uint(10)).Return(nil, nil)
				if failure == "source actors" {
					actors.EXPECT().GetSceneActors(uint(20)).Return(nil, cause)
				} else {
					actors.EXPECT().GetSceneActors(uint(20)).Return([]data.Actor{{ID: 1}}, nil)
					actors.EXPECT().BulkAddActorsToScenes([]uint{10}, []uint{1}).Return(cause)
				}
			case "scenes":
				scenes.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20}).Return(nil, cause)
			default:
				other := data.Scene{ID: 20}
				if failure == "studio" {
					studioID := uint(5)
					other.StudioID, other.Studio = &studioID, "Studio"
					scenes.EXPECT().UpdateSceneMetadata(uint(10), "", "", "Studio", (*time.Time)(nil), "").Return(cause)
				}
				scenes.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20}).Return([]data.Scene{{ID: 10}, other}, nil)
				if failure == "markers" {
					svc.markerRepo = markers
					markers.EXPECT().ReassignMarkersToScene(uint(20), uint(10)).Return(cause)
				}
				if failure == "interactions" {
					svc.interactionRepo = interactions
					interactions.EXPECT().ReassignInteractionsToScene(uint(20), uint(10)).Return(cause)
				}
			}
			if err := svc.mergeMetadata(10, []uint{20}); !errors.Is(err, cause) {
				t.Fatalf("expected underlying metadata failure, got %v", err)
			}
		})
	}
}
