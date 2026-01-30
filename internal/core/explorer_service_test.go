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
	*mocks.MockVideoRepository,
	*mocks.MockTagRepository,
	*mocks.MockActorRepository,
) {
	ctrl := gomock.NewController(t)
	explorerRepo := mocks.NewMockExplorerRepository(ctrl)
	storagePathRepo := mocks.NewMockStoragePathRepository(ctrl)
	videoRepo := mocks.NewMockVideoRepository(ctrl)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	actorRepo := mocks.NewMockActorRepository(ctrl)

	svc := NewExplorerService(
		explorerRepo,
		storagePathRepo,
		videoRepo,
		tagRepo,
		actorRepo,
		nil, // EventBus
		zap.NewNop(),
		"", // metadataPath
	)
	return svc, explorerRepo, storagePathRepo, videoRepo, tagRepo, actorRepo
}

// =============================================================================
// GetStoragePathsWithCounts Tests
// =============================================================================

func TestGetStoragePathsWithCounts_Success(t *testing.T) {
	svc, explorerRepo, _, _, _, _ := newTestExplorerService(t)

	expected := []data.StoragePathWithCount{
		{StoragePath: data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}, VideoCount: 50},
		{StoragePath: data.StoragePath{ID: 2, Name: "Series", Path: "/data/series"}, VideoCount: 100},
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
	if result[0].VideoCount != 50 {
		t.Fatalf("expected first path video count 50, got %d", result[0].VideoCount)
	}
}

func TestGetStoragePathsWithCounts_Error(t *testing.T) {
	svc, explorerRepo, _, _, _, _ := newTestExplorerService(t)

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
	svc, explorerRepo, storagePathRepo, _, _, _ := newTestExplorerService(t)

	storagePath := &data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}
	storagePathRepo.EXPECT().GetByID(uint(1)).Return(storagePath, nil)

	subfolders := []data.FolderInfo{
		{Name: "Action", Path: "Action", VideoCount: 10},
		{Name: "Comedy", Path: "Comedy", VideoCount: 5},
	}
	explorerRepo.EXPECT().GetSubfolders(uint(1), "").Return(subfolders, nil)

	videos := []data.Video{
		{ID: 1, Title: "Movie 1"},
		{ID: 2, Title: "Movie 2"},
	}
	explorerRepo.EXPECT().GetVideosByFolder(uint(1), "", 1, 24).Return(videos, int64(2), nil)

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
	if len(result.Videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(result.Videos))
	}
	if result.TotalVideos != 2 {
		t.Fatalf("expected total videos 2, got %d", result.TotalVideos)
	}
}

func TestGetFolderContents_StoragePathNotFound(t *testing.T) {
	svc, _, storagePathRepo, _, _, _ := newTestExplorerService(t)

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
	svc, explorerRepo, storagePathRepo, _, _, _ := newTestExplorerService(t)

	storagePath := &data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}
	storagePathRepo.EXPECT().GetByID(uint(1)).Return(storagePath, nil)
	explorerRepo.EXPECT().GetSubfolders(uint(1), "").Return(nil, nil)
	explorerRepo.EXPECT().GetVideosByFolder(uint(1), "", 1, 24).Return(nil, int64(0), nil)

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
// GetFolderVideoIDs Tests
// =============================================================================

func TestGetFolderVideoIDs_Success(t *testing.T) {
	svc, explorerRepo, storagePathRepo, _, _, _ := newTestExplorerService(t)

	storagePath := &data.StoragePath{ID: 1, Name: "Movies", Path: "/data/movies"}
	storagePathRepo.EXPECT().GetByID(uint(1)).Return(storagePath, nil)

	expectedIDs := []uint{1, 2, 3, 4, 5}
	explorerRepo.EXPECT().GetVideoIDsByFolder(uint(1), "Action", false).Return(expectedIDs, nil)

	ids, err := svc.GetFolderVideoIDs(1, "Action", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(ids) != 5 {
		t.Fatalf("expected 5 IDs, got %d", len(ids))
	}
}

func TestGetFolderVideoIDs_StoragePathNotFound(t *testing.T) {
	svc, _, storagePathRepo, _, _, _ := newTestExplorerService(t)

	storagePathRepo.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.GetFolderVideoIDs(999, "", false)
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
	svc, _, _, videoRepo, tagRepo, _ := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}, {ID: 2}, {ID: 3}}
	videoRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return(videos, nil)

	tags := []data.Tag{{ID: 10}, {ID: 11}}
	tagRepo.EXPECT().GetByIDs([]uint{10, 11}).Return(tags, nil)

	tagRepo.EXPECT().BulkAddTagsToVideos([]uint{1, 2, 3}, []uint{10, 11}).Return(nil)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{1, 2, 3},
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
	svc, _, _, videoRepo, tagRepo, _ := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}, {ID: 2}}
	videoRepo.EXPECT().GetByIDs([]uint{1, 2}).Return(videos, nil)

	tagRepo.EXPECT().BulkRemoveTagsFromVideos([]uint{1, 2}, []uint{10}).Return(nil)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{1, 2},
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
	svc, _, _, videoRepo, tagRepo, _ := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}}
	videoRepo.EXPECT().GetByIDs([]uint{1}).Return(videos, nil)

	tags := []data.Tag{{ID: 20}, {ID: 21}}
	tagRepo.EXPECT().GetByIDs([]uint{20, 21}).Return(tags, nil)

	tagRepo.EXPECT().BulkReplaceTagsForVideos([]uint{1}, []uint{20, 21}).Return(nil)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{1},
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

func TestBulkUpdateTags_EmptyVideoIDs(t *testing.T) {
	svc, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{},
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
	if !strings.Contains(err.Error(), "at least one video ID") {
		t.Fatalf("expected 'at least one video ID' error, got: %v", err)
	}
}

func TestBulkUpdateTags_InvalidMode(t *testing.T) {
	svc, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{1},
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

func TestBulkUpdateTags_VideoNotFound(t *testing.T) {
	svc, _, _, videoRepo, _, _ := newTestExplorerService(t)

	// Return only 2 videos when 3 were requested
	videos := []data.Video{{ID: 1}, {ID: 2}}
	videoRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return(videos, nil)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{1, 2, 3},
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
	if !strings.Contains(err.Error(), "one or more videos not found") {
		t.Fatalf("expected 'videos not found' error, got: %v", err)
	}
}

func TestBulkUpdateTags_TagNotFound(t *testing.T) {
	svc, _, _, videoRepo, tagRepo, _ := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}}
	videoRepo.EXPECT().GetByIDs([]uint{1}).Return(videos, nil)

	// Return only 1 tag when 2 were requested
	tags := []data.Tag{{ID: 10}}
	tagRepo.EXPECT().GetByIDs([]uint{10, 11}).Return(tags, nil)

	req := BulkUpdateTagsRequest{
		VideoIDs: []uint{1},
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
	svc, _, _, videoRepo, _, actorRepo := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}, {ID: 2}}
	videoRepo.EXPECT().GetByIDs([]uint{1, 2}).Return(videos, nil)

	actors := []data.Actor{{ID: 100}, {ID: 101}}
	actorRepo.EXPECT().GetByIDs([]uint{100, 101}).Return(actors, nil)

	actorRepo.EXPECT().BulkAddActorsToVideos([]uint{1, 2}, []uint{100, 101}).Return(nil)

	req := BulkUpdateActorsRequest{
		VideoIDs: []uint{1, 2},
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
	svc, _, _, videoRepo, _, actorRepo := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}}
	videoRepo.EXPECT().GetByIDs([]uint{1}).Return(videos, nil)

	actorRepo.EXPECT().BulkRemoveActorsFromVideos([]uint{1}, []uint{100}).Return(nil)

	req := BulkUpdateActorsRequest{
		VideoIDs: []uint{1},
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

func TestBulkUpdateActors_EmptyVideoIDs(t *testing.T) {
	svc, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateActorsRequest{
		VideoIDs: []uint{},
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
	svc, _, _, videoRepo, _, actorRepo := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}}
	videoRepo.EXPECT().GetByIDs([]uint{1}).Return(videos, nil)

	// Return only 1 actor when 2 were requested
	actors := []data.Actor{{ID: 100}}
	actorRepo.EXPECT().GetByIDs([]uint{100, 101}).Return(actors, nil)

	req := BulkUpdateActorsRequest{
		VideoIDs: []uint{1},
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
	svc, _, _, videoRepo, _, _ := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}, {ID: 2}, {ID: 3}}
	videoRepo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return(videos, nil)

	videoRepo.EXPECT().BulkUpdateStudio([]uint{1, 2, 3}, "New Studio").Return(nil)

	req := BulkUpdateStudioRequest{
		VideoIDs: []uint{1, 2, 3},
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

func TestBulkUpdateStudio_EmptyVideoIDs(t *testing.T) {
	svc, _, _, _, _, _ := newTestExplorerService(t)

	req := BulkUpdateStudioRequest{
		VideoIDs: []uint{},
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
	svc, _, _, videoRepo, _, _ := newTestExplorerService(t)

	videos := []data.Video{{ID: 1}}
	videoRepo.EXPECT().GetByIDs([]uint{1}).Return(videos, nil)

	// Empty string should clear the studio
	videoRepo.EXPECT().BulkUpdateStudio([]uint{1}, "").Return(nil)

	req := BulkUpdateStudioRequest{
		VideoIDs: []uint{1},
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
