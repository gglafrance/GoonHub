package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestDuplicateService(t *testing.T) (
	*DuplicateService,
	*mocks.MockDuplicateGroupRepository,
	*mocks.MockSceneRepository,
	*mocks.MockTagRepository,
	*mocks.MockActorRepository,
	*mocks.MockMarkerRepository,
	*mocks.MockInteractionRepository,
) {
	ctrl := gomock.NewController(t)
	groupRepo := mocks.NewMockDuplicateGroupRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	actorRepo := mocks.NewMockActorRepository(ctrl)
	markerRepo := mocks.NewMockMarkerRepository(ctrl)
	interactionRepo := mocks.NewMockInteractionRepository(ctrl)

	eventBus := NewEventBus(zap.NewNop())
	svc := NewDuplicateService(groupRepo, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo, eventBus, zap.NewNop())
	return svc, groupRepo, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo
}

func TestGetStats_Success(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)

	groupRepo.EXPECT().CountByStatus().Return(map[string]int64{
		"unresolved": 10,
		"resolved":   5,
		"dismissed":  3,
	}, nil)

	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Unresolved != 10 {
		t.Fatalf("expected Unresolved=10, got %d", stats.Unresolved)
	}
	if stats.Resolved != 5 {
		t.Fatalf("expected Resolved=5, got %d", stats.Resolved)
	}
	if stats.Dismissed != 3 {
		t.Fatalf("expected Dismissed=3, got %d", stats.Dismissed)
	}
	if stats.Total != 18 {
		t.Fatalf("expected Total=18, got %d", stats.Total)
	}
}

func TestGetStats_Error(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)

	groupRepo.EXPECT().CountByStatus().Return(nil, fmt.Errorf("db connection lost"))

	stats, err := svc.GetStats()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stats != nil {
		t.Fatalf("expected nil stats, got %+v", stats)
	}
}

func TestScoreBestVariant_HigherResWins(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 2,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 100, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 200, MatchType: "audio"},
		},
	}

	// Scene 100: 1080p h264
	scene100 := &data.Scene{
		ID:         100,
		Duration:   600,
		Width:      1920,
		Height:     1080,
		VideoCodec: "h264",
		BitRate:    5_000_000,
	}
	// Scene 200: 4K h264
	scene200 := &data.Scene{
		ID:         200,
		Duration:   600,
		Width:      3840,
		Height:     2160,
		VideoCodec: "h264",
		BitRate:    5_000_000,
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)
	sceneRepo.EXPECT().GetByID(uint(100)).Return(scene100, nil)
	sceneRepo.EXPECT().GetByID(uint(200)).Return(scene200, nil)
	groupRepo.EXPECT().SetBestScene(uint(1), uint(200)).Return(nil)

	bestID, err := svc.ScoreBestVariant(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bestID != 200 {
		t.Fatalf("expected best scene ID=200 (4K), got %d", bestID)
	}
}

func TestScoreBestVariant_BetterCodecWins(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 2,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 100, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 200, MatchType: "audio"},
		},
	}

	// Same resolution, same duration, same bitrate - only codec differs
	scene100 := &data.Scene{
		ID:         100,
		Duration:   600,
		Width:      1920,
		Height:     1080,
		VideoCodec: "h264",
		BitRate:    5_000_000,
	}
	scene200 := &data.Scene{
		ID:         200,
		Duration:   600,
		Width:      1920,
		Height:     1080,
		VideoCodec: "hevc",
		BitRate:    5_000_000,
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)
	sceneRepo.EXPECT().GetByID(uint(100)).Return(scene100, nil)
	sceneRepo.EXPECT().GetByID(uint(200)).Return(scene200, nil)
	groupRepo.EXPECT().SetBestScene(uint(1), uint(200)).Return(nil)

	bestID, err := svc.ScoreBestVariant(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bestID != 200 {
		t.Fatalf("expected best scene ID=200 (hevc), got %d", bestID)
	}
}

func TestResolveGroup_Success(t *testing.T) {
	svc, groupRepo, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 3,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 10, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 20, MatchType: "audio"},
			{ID: 3, GroupID: 1, SceneID: 30, MatchType: "audio"},
		},
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)

	// Merge metadata: tags
	tagRepo.EXPECT().GetSceneTags(uint(10)).Return([]data.Tag{
		{ID: 1, Name: "tag-a"},
	}, nil)
	tagRepo.EXPECT().GetSceneTags(uint(20)).Return([]data.Tag{
		{ID: 2, Name: "tag-b"},
	}, nil)
	tagRepo.EXPECT().GetSceneTags(uint(30)).Return([]data.Tag{
		{ID: 1, Name: "tag-a"}, // duplicate, should not be re-added
	}, nil)
	tagRepo.EXPECT().BulkAddTagsToScenes([]uint{10}, []uint{2}).Return(nil)

	// Merge metadata: actors
	actorRepo.EXPECT().GetSceneActors(uint(10)).Return([]data.Actor{
		{ID: 100},
	}, nil)
	actorRepo.EXPECT().GetSceneActors(uint(20)).Return([]data.Actor{
		{ID: 200},
	}, nil)
	actorRepo.EXPECT().GetSceneActors(uint(30)).Return([]data.Actor{
		{ID: 100}, // duplicate, should not be re-added
	}, nil)
	actorRepo.EXPECT().BulkAddActorsToScenes([]uint{10}, []uint{200}).Return(nil)

	// Merge view counts and studio: batch fetch scenes
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{
		{ID: 10, Title: "Scene 10", ViewCount: 100},
		{ID: 20, Title: "Scene 20", ViewCount: 50},
		{ID: 30, Title: "Scene 30", ViewCount: 25},
	}, nil)

	// Merge markers
	markerRepo.EXPECT().ReassignMarkersToScene(uint(20), uint(10)).Return(nil)
	markerRepo.EXPECT().ReassignMarkersToScene(uint(30), uint(10)).Return(nil)

	// Merge interactions
	interactionRepo.EXPECT().ReassignInteractionsToScene(uint(20), uint(10)).Return(nil)
	interactionRepo.EXPECT().ReassignInteractionsToScene(uint(30), uint(10)).Return(nil)

	// Update is_best flags for all members
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(10), true).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(20), false).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(30), false).Return(nil)

	// Trash non-best scenes
	now := time.Now()
	sceneRepo.EXPECT().MoveToTrash(uint(20)).Return(&now, nil)
	sceneRepo.EXPECT().MoveToTrash(uint(30)).Return(&now, nil)

	// Set best scene and update status
	groupRepo.EXPECT().SetBestScene(uint(1), uint(10)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Any()).Return(nil)

	err := svc.ResolveGroup(1, 10, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveGroup_InvalidMember(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 2,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 10, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 20, MatchType: "audio"},
		},
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)

	err := svc.ResolveGroup(1, 999, false)
	if err == nil {
		t.Fatal("expected error for invalid member, got nil")
	}
}

func TestResolveGroup_NoMerge(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 2,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 10, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 20, MatchType: "audio"},
		},
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)

	// No tag/actor merge calls expected (mergeMetadata=false)

	// Update is_best flags
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(10), true).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(20), false).Return(nil)

	// Trash non-best
	now := time.Now()
	sceneRepo.EXPECT().MoveToTrash(uint(20)).Return(&now, nil)

	// Set best and update status
	groupRepo.EXPECT().SetBestScene(uint(1), uint(10)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Any()).Return(nil)

	err := svc.ResolveGroup(1, 10, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDismissGroup_Success(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)

	groupRepo.EXPECT().UpdateStatus(uint(1), "dismissed", (*time.Time)(nil)).Return(nil)

	err := svc.DismissGroup(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetBest_Success(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 3,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 10, IsBest: true, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 20, IsBest: false, MatchType: "audio"},
			{ID: 3, GroupID: 1, SceneID: 30, IsBest: false, MatchType: "audio"},
		},
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)

	// Update is_best: scene 20 becomes best, others become false
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(10), false).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(20), true).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(30), false).Return(nil)

	groupRepo.EXPECT().SetBestScene(uint(1), uint(20)).Return(nil)

	err := svc.SetBest(1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetBest_InvalidMember(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)

	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "unresolved",
		SceneCount: 2,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 10, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 20, MatchType: "audio"},
		},
	}

	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)

	err := svc.SetBest(1, 999)
	if err == nil {
		t.Fatal("expected error for invalid member, got nil")
	}
}

func TestScoreScene_CodecRanking(t *testing.T) {
	// scoreScene is a package-level function, testable directly via white-box access
	tests := []struct {
		name      string
		codec     string
		wantBonus int64
	}{
		{"av1 gets 3M bonus", "av1", 3_000_000},
		{"hevc gets 2M bonus", "hevc", 2_000_000},
		{"h265 gets 2M bonus", "h265", 2_000_000},
		{"h264 gets 1M bonus", "h264", 1_000_000},
		{"unknown gets no bonus", "vp9", 0},
		{"empty gets no bonus", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &data.Scene{
				Duration:   0,
				Width:      0,
				Height:     0,
				VideoCodec: tt.codec,
				BitRate:    0,
			}
			score := scoreScene(scene)
			if score != tt.wantBonus {
				t.Fatalf("scoreScene with codec %q: got %d, want %d", tt.codec, score, tt.wantBonus)
			}
		})
	}

	// Also verify the full scoring formula with concrete values:
	// score = duration*1000 + width*height + codecRank + bitRate/1000
	scene := &data.Scene{
		Duration:   120,       // 120 * 1000 = 120_000
		Width:      1920,      // 1920 * 1080 = 2_073_600
		Height:     1080,
		VideoCodec: "h264",    // 1_000_000
		BitRate:    8_000_000, // 8_000_000 / 1000 = 8_000
	}
	got := scoreScene(scene)
	want := int64(120_000 + 2_073_600 + 1_000_000 + 8_000) // = 3_201_600
	if got != want {
		t.Fatalf("scoreScene full formula: got %d, want %d", got, want)
	}
}

func TestEnrichGroup_IncludesTrashedMembers(t *testing.T) {
	svc, _, sceneRepo, _, _, _, _ := newTestDuplicateService(t)

	trashedAt := time.Now().Add(-24 * time.Hour)
	group := &data.DuplicateGroup{
		ID:         1,
		Status:     "resolved",
		SceneCount: 3,
		Members: []data.DuplicateGroupMember{
			{ID: 1, GroupID: 1, SceneID: 10, IsBest: true, MatchType: "audio"},
			{ID: 2, GroupID: 1, SceneID: 20, IsBest: false, MatchType: "audio"},
			{ID: 3, GroupID: 1, SceneID: 30, IsBest: false, MatchType: "audio"},
		},
	}

	// Scene 10 is not trashed, scenes 20 and 30 are trashed
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20, 30}).Return([]data.Scene{
		{ID: 10, Title: "Best Scene", Width: 1920, Height: 1080, TrashedAt: nil},
		{ID: 20, Title: "Trashed Scene 1", Width: 1280, Height: 720, TrashedAt: &trashedAt},
		{ID: 30, Title: "Trashed Scene 2", Width: 1280, Height: 720, TrashedAt: &trashedAt},
	}, nil)

	enriched, err := svc.enrichGroup(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(enriched.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(enriched.Members))
	}

	// Verify non-trashed member
	if enriched.Members[0].IsTrashed {
		t.Fatal("expected member 0 (scene 10) to not be trashed")
	}
	if enriched.Members[0].TrashedAt != nil {
		t.Fatal("expected member 0 TrashedAt to be nil")
	}

	// Verify trashed members
	if !enriched.Members[1].IsTrashed {
		t.Fatal("expected member 1 (scene 20) to be trashed")
	}
	if enriched.Members[1].TrashedAt == nil {
		t.Fatal("expected member 1 TrashedAt to be non-nil")
	}

	if !enriched.Members[2].IsTrashed {
		t.Fatal("expected member 2 (scene 30) to be trashed")
	}
}

func TestMergeMetadata_Markers(t *testing.T) {
	svc, _, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo := newTestDuplicateService(t)

	// Tags: no tags to merge
	tagRepo.EXPECT().GetSceneTags(uint(10)).Return([]data.Tag{}, nil)
	tagRepo.EXPECT().GetSceneTags(uint(20)).Return([]data.Tag{}, nil)

	// Actors: no actors to merge
	actorRepo.EXPECT().GetSceneActors(uint(10)).Return([]data.Actor{}, nil)
	actorRepo.EXPECT().GetSceneActors(uint(20)).Return([]data.Actor{}, nil)

	// View counts / studio fetch
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{
		{ID: 10, Title: "Best", ViewCount: 100},
		{ID: 20, Title: "Other", ViewCount: 50},
	}, nil)

	// Markers: should be reassigned
	markerRepo.EXPECT().ReassignMarkersToScene(uint(20), uint(10)).Return(nil)

	// Interactions: should be reassigned
	interactionRepo.EXPECT().ReassignInteractionsToScene(uint(20), uint(10)).Return(nil)

	err := svc.mergeMetadata(10, []uint{20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeMetadata_Studio(t *testing.T) {
	svc, _, sceneRepo, tagRepo, actorRepo, markerRepo, interactionRepo := newTestDuplicateService(t)

	// Tags: no tags
	tagRepo.EXPECT().GetSceneTags(uint(10)).Return([]data.Tag{}, nil)
	tagRepo.EXPECT().GetSceneTags(uint(20)).Return([]data.Tag{}, nil)

	// Actors: no actors
	actorRepo.EXPECT().GetSceneActors(uint(10)).Return([]data.Actor{}, nil)
	actorRepo.EXPECT().GetSceneActors(uint(20)).Return([]data.Actor{}, nil)

	// Best scene has no studio, other scene has one
	studioID := uint(5)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{
		{ID: 10, Title: "Best Scene", StudioID: nil, ViewCount: 100},
		{ID: 20, Title: "Other Scene", StudioID: &studioID, Studio: "Great Studio", ViewCount: 50},
	}, nil)

	// Studio should be copied to best scene
	sceneRepo.EXPECT().UpdateSceneMetadata(uint(10), "Best Scene", "", "Great Studio", (*time.Time)(nil), "").Return(nil)

	// Markers
	markerRepo.EXPECT().ReassignMarkersToScene(uint(20), uint(10)).Return(nil)

	// Interactions
	interactionRepo.EXPECT().ReassignInteractionsToScene(uint(20), uint(10)).Return(nil)

	err := svc.mergeMetadata(10, []uint{20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
