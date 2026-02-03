package core

import (
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestExplorerService(t *testing.T) (
	*ExplorerService,
	*mocks.MockExplorerRepository,
	*mocks.MockStoragePathRepository,
	*mocks.MockSceneRepository,
	*mocks.MockTagRepository,
	*mocks.MockActorRepository,
	*mocks.MockJobHistoryRepository,
) {
	ctrl := gomock.NewController(t)
	explorerRepo := mocks.NewMockExplorerRepository(ctrl)
	storagePathRepo := mocks.NewMockStoragePathRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	actorRepo := mocks.NewMockActorRepository(ctrl)
	jobHistoryRepo := mocks.NewMockJobHistoryRepository(ctrl)

	svc := NewExplorerService(
		explorerRepo,
		storagePathRepo,
		sceneRepo,
		tagRepo,
		actorRepo,
		jobHistoryRepo,
		nil, // EventBus
		zap.NewNop(),
		"", // metadataPath
	)
	return svc, explorerRepo, storagePathRepo, sceneRepo, tagRepo, actorRepo, jobHistoryRepo
}

// =============================================================================
// GetStoragePathsWithCounts Tests
// =============================================================================

func TestGetStoragePathsWithCounts_Success(t *testing.T) {
	svc, explorerRepo, _, _, _, _, _ := newTestExplorerService(t)

	expected := []data.StoragePathWithCount{
		{StoragePath: data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}, SceneCount: 50},
		{StoragePath: data.StoragePath{ID: 2, Name: "Series", Path: "/data/series"}, SceneCount: 100},
	}
	explorerRepo.EXPECT().GetStoragePathsWithCounts().Return(expected, nil)

	result, err := svc.GetStoragePathsWithCounts()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 storage paths, got %d", len(result))
	}
	if result[0].Name != "Movies" {
		t.Fatalf("expected first path 'Movies', got %q", result[0].Name)
	}
	if result[0].SceneCount != 50 {
		t.Fatalf("expected first path scene count 50, got %d", result[0].SceneCount)
	}
}

func TestGetStoragePathsWithCounts_Error(t *testing.T) {
	svc, explorerRepo, _, _, _, _, _ := newTestExplorerService(t)

	explorerRepo.EXPECT().GetStoragePathsWithCounts().Return(nil, gorm.ErrInvalidDB)

	_, err := svc.GetStoragePathsWithCounts()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsInternal(err) {
		t.Fatalf("expected internal error, got: %v", err)
	}
}

// =============================================================================
// GetFolderContents Tests
// =============================================================================

func TestGetFolderContents_Success(t *testing.T) {
	svc, explorerRepo, storagePathRepo, _, _, _, _ := newTestExplorerService(t)

	storagePath := &data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}
	storagePathRepo.EXPECT().GetByID(uint(1)).Return(storagePath, nil)

	subfolders := []data.FolderInfo{
		{Name: "Action", Path: "Action", SceneCount: 10},
		{Name: "Comedy", Path: "Comedy", SceneCount: 5},
	}
	explorerRepo.EXPECT().GetSubfolders(uint(1), "").Return(subfolders, nil)

	scenes := []data.Scene{
		{ID: 1, Title: "Movie 1"},
		{ID: 2, Title: "Movie 2"},
	}
	explorerRepo.EXPECT().GetScenesByFolder(uint(1), "", 1, 24).Return(scenes, int64(2), nil)

	result, err := svc.GetFolderContents(1, "", 1, 24)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.StoragePath.Name != "Movies" {
		t.Fatalf("expected storage path 'Movies', got %q", result.StoragePath.Name)
	}
	if len(result.Subfolders) != 2 {
		t.Fatalf("expected 2 subfolders, got %d", len(result.Subfolders))
	}
	if len(result.Scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(result.Scenes))
	}
	if result.TotalScenes != 2 {
		t.Fatalf("expected total scenes 2, got %d", result.TotalScenes)
	}
}

func TestGetFolderContents_StoragePathNotFound(t *testing.T) {
	svc, _, storagePathRepo, _, _, _, _ := newTestExplorerService(t)

	storagePathRepo.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.GetFolderContents(999, "", 1, 24)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestGetFolderContents_DefaultPagination(t *testing.T) {
	svc, explorerRepo, storagePathRepo, _, _, _, _ := newTestExplorerService(t)

	storagePath := &data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}
	storagePathRepo.EXPECT().GetByID(uint(1)).Return(storagePath, nil)
	explorerRepo.EXPECT().GetSubfolders(uint(1), "").Return(nil, nil)
	explorerRepo.EXPECT().GetScenesByFolder(uint(1), "", 1, 24).Return(nil, int64(0), nil)

	// Pass invalid page and limit values
	result, err := svc.GetFolderContents(1, "", 0, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Should default to page 1, limit 24
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
	if result.Limit != 24 {
		t.Fatalf("expected limit 24, got %d", result.Limit)
	}
}

// =============================================================================
// GetFolderSceneIDs Tests
// =============================================================================

func TestGetFolderSceneIDs_Success(t *testing.T) {
	svc, explorerRepo, storagePathRepo, _, _, _, _ := newTestExplorerService(t)

	storagePath := &data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}
	storagePathRepo.EXPECT().GetByID(uint(1)).Return(storagePath, nil)

	expectedIDs := []uint{1, 2, 3, 4, 5}
	explorerRepo.EXPECT().GetSceneIDsByFolder(uint(1), "Action", false).Return(expectedIDs, nil)

	ids, err := svc.GetFolderSceneIDs(1, "Action", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(ids) != 5 {
		t.Fatalf("expected 5 IDs, got %d", len(ids))
	}
}

func TestGetFolderSceneIDs_StoragePathNotFound(t *testing.T) {
	svc, _, storagePathRepo, _, _, _, _ := newTestExplorerService(t)

	storagePathRepo.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.GetFolderSceneIDs(999, "", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

// =============================================================================
// BulkUpdateTags Tests
// =============================================================================

func TestBulkUpdateTags_AddMode_Success(t *testing.T) {
	svc, _, _, sceneRepo, tagRepo, _, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}, {ID: 2}, {ID: 3}}
	sceneRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return(scenes, nil)

	tags := []data.Tag{{ID: 10}, {ID: 11}}
	tagRepo.EXPECT().GetByIDs([]uint{10, 11}).Return(tags, nil)

	tagRepo.EXPECT().BulkAddTagsToScenes([]uint{1, 2, 3}, []uint{10, 11}).Return(nil)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{1, 2, 3},
		TagIDs:   []uint{10, 11},
		Mode:     "add",
	}
	updated, err := svc.BulkUpdateTags(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 3 {
		t.Fatalf("expected 3 updated, got %d", updated)
	}
}

func TestBulkUpdateTags_RemoveMode_Success(t *testing.T) {
	svc, _, _, sceneRepo, tagRepo, _, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}, {ID: 2}}
	sceneRepo.EXPECT().GetByIDs([]uint{1, 2}).Return(scenes, nil)

	tagRepo.EXPECT().BulkRemoveTagsFromScenes([]uint{1, 2}, []uint{10}).Return(nil)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{1, 2},
		TagIDs:   []uint{10},
		Mode:     "remove",
	}
	updated, err := svc.BulkUpdateTags(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated, got %d", updated)
	}
}

func TestBulkUpdateTags_ReplaceMode_Success(t *testing.T) {
	svc, _, _, sceneRepo, tagRepo, _, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}}
	sceneRepo.EXPECT().GetByIDs([]uint{1}).Return(scenes, nil)

	tags := []data.Tag{{ID: 20}, {ID: 21}}
	tagRepo.EXPECT().GetByIDs([]uint{20, 21}).Return(tags, nil)

	tagRepo.EXPECT().BulkReplaceTagsForScenes([]uint{1}, []uint{20, 21}).Return(nil)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{1},
		TagIDs:   []uint{20, 21},
		Mode:     "replace",
	}
	updated, err := svc.BulkUpdateTags(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 updated, got %d", updated)
	}
}

func TestBulkUpdateTags_EmptySceneIDs(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{},
		TagIDs:   []uint{1},
		Mode:     "add",
	}
	_, err := svc.BulkUpdateTags(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "at least one scene ID") {
		t.Fatalf("expected 'at least one scene ID' error, got: %v", err)
	}
}

func TestBulkUpdateTags_InvalidMode(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{1},
		TagIDs:   []uint{1},
		Mode:     "invalid",
	}
	_, err := svc.BulkUpdateTags(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mode must be") {
		t.Fatalf("expected mode validation error, got: %v", err)
	}
}

func TestBulkUpdateTags_SceneNotFound(t *testing.T) {
	svc, _, _, sceneRepo, _, _, _ := newTestExplorerService(t)

	// Return only 2 scenes when 3 were requested
	scenes := []data.Scene{{ID: 1}, {ID: 2}}
	sceneRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return(scenes, nil)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{1, 2, 3},
		TagIDs:   []uint{10},
		Mode:     "add",
	}
	_, err := svc.BulkUpdateTags(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "one or more scenes not found") {
		t.Fatalf("expected 'scenes not found' error, got: %v", err)
	}
}

func TestBulkUpdateTags_TagNotFound(t *testing.T) {
	svc, _, _, sceneRepo, tagRepo, _, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}}
	sceneRepo.EXPECT().GetByIDs([]uint{1}).Return(scenes, nil)

	// Return only 1 tag when 2 were requested
	tags := []data.Tag{{ID: 10}}
	tagRepo.EXPECT().GetByIDs([]uint{10, 11}).Return(tags, nil)

	req := BulkUpdateTagsRequest{
		SceneIDs: []uint{1},
		TagIDs:   []uint{10, 11},
		Mode:     "add",
	}
	_, err := svc.BulkUpdateTags(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "one or more tags not found") {
		t.Fatalf("expected 'tags not found' error, got: %v", err)
	}
}

// =============================================================================
// BulkUpdateActors Tests
// =============================================================================

func TestBulkUpdateActors_AddMode_Success(t *testing.T) {
	svc, _, _, sceneRepo, _, actorRepo, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}, {ID: 2}}
	sceneRepo.EXPECT().GetByIDs([]uint{1, 2}).Return(scenes, nil)

	actors := []data.Actor{{ID: 100}, {ID: 101}}
	actorRepo.EXPECT().GetByIDs([]uint{100, 101}).Return(actors, nil)

	actorRepo.EXPECT().BulkAddActorsToScenes([]uint{1, 2}, []uint{100, 101}).Return(nil)

	req := BulkUpdateActorsRequest{
		SceneIDs: []uint{1, 2},
		ActorIDs: []uint{100, 101},
		Mode:     "add",
	}
	updated, err := svc.BulkUpdateActors(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated, got %d", updated)
	}
}

func TestBulkUpdateActors_RemoveMode_Success(t *testing.T) {
	svc, _, _, sceneRepo, _, actorRepo, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}}
	sceneRepo.EXPECT().GetByIDs([]uint{1}).Return(scenes, nil)

	actorRepo.EXPECT().BulkRemoveActorsFromScenes([]uint{1}, []uint{100}).Return(nil)

	req := BulkUpdateActorsRequest{
		SceneIDs: []uint{1},
		ActorIDs: []uint{100},
		Mode:     "remove",
	}
	updated, err := svc.BulkUpdateActors(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 updated, got %d", updated)
	}
}

func TestBulkUpdateActors_EmptySceneIDs(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateActorsRequest{
		SceneIDs: []uint{},
		ActorIDs: []uint{1},
		Mode:     "add",
	}
	_, err := svc.BulkUpdateActors(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestBulkUpdateActors_ActorNotFound(t *testing.T) {
	svc, _, _, sceneRepo, _, actorRepo, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}}
	sceneRepo.EXPECT().GetByIDs([]uint{1}).Return(scenes, nil)

	// Return only 1 actor when 2 were requested
	actors := []data.Actor{{ID: 100}}
	actorRepo.EXPECT().GetByIDs([]uint{100, 101}).Return(actors, nil)

	req := BulkUpdateActorsRequest{
		SceneIDs: []uint{1},
		ActorIDs: []uint{100, 101},
		Mode:     "add",
	}
	_, err := svc.BulkUpdateActors(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "one or more actors not found") {
		t.Fatalf("expected 'actors not found' error, got: %v", err)
	}
}

// =============================================================================
// BulkUpdateStudio Tests
// =============================================================================

func TestBulkUpdateStudio_Success(t *testing.T) {
	svc, _, _, sceneRepo, _, _, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}, {ID: 2}, {ID: 3}}
	sceneRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return(scenes, nil)

	sceneRepo.EXPECT().BulkUpdateStudio([]uint{1, 2, 3}, "New Studio").Return(nil)

	req := BulkUpdateStudioRequest{
		SceneIDs: []uint{1, 2, 3},
		Studio:   "New Studio",
	}
	updated, err := svc.BulkUpdateStudio(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 3 {
		t.Fatalf("expected 3 updated, got %d", updated)
	}
}

func TestBulkUpdateStudio_EmptySceneIDs(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateStudioRequest{
		SceneIDs: []uint{},
		Studio:   "Some Studio",
	}
	_, err := svc.BulkUpdateStudio(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestBulkUpdateStudio_ClearStudio(t *testing.T) {
	svc, _, _, sceneRepo, _, _, _ := newTestExplorerService(t)

	scenes := []data.Scene{{ID: 1}}
	sceneRepo.EXPECT().GetByIDs([]uint{1}).Return(scenes, nil)

	// Empty string should clear the studio
	sceneRepo.EXPECT().BulkUpdateStudio([]uint{1}, "").Return(nil)

	req := BulkUpdateStudioRequest{
		SceneIDs: []uint{1},
		Studio:   "",
	}
	updated, err := svc.BulkUpdateStudio(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 updated, got %d", updated)
	}
}
