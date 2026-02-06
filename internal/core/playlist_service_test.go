package core

import (
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestPlaylistService(t *testing.T) (*PlaylistService, *mocks.MockPlaylistRepository, *mocks.MockSceneRepository, *mocks.MockTagRepository) {
	ctrl := gomock.NewController(t)
	playlistRepo := mocks.NewMockPlaylistRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	tagRepo := mocks.NewMockTagRepository(ctrl)

	svc := NewPlaylistService(playlistRepo, sceneRepo, tagRepo, zap.NewNop())
	return svc, playlistRepo, sceneRepo, tagRepo
}

func TestCreatePlaylist_Success(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	playlistRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(p *data.Playlist) error {
		if p.Name != "My Playlist" {
			t.Fatalf("expected name 'My Playlist', got %q", p.Name)
		}
		if p.Visibility != "public" {
			t.Fatalf("expected visibility 'public', got %q", p.Visibility)
		}
		p.ID = 1
		p.UUID = uuid.New()
		return nil
	})

	playlistRepo.EXPECT().GetByID(uint(1)).Return(&data.Playlist{
		ID:         1,
		UUID:       uuid.New(),
		UserID:     1,
		Name:       "My Playlist",
		Visibility: "public",
		User:       data.User{ID: 1, Username: "admin"},
	}, nil)

	desc := "A test playlist"
	result, err := svc.Create(1, CreatePlaylistInput{
		Name:        "My Playlist",
		Description: &desc,
		Visibility:  "public",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Name != "My Playlist" {
		t.Fatalf("expected name 'My Playlist', got %q", result.Name)
	}
}

func TestCreatePlaylist_EmptyName(t *testing.T) {
	svc, _, _, _ := newTestPlaylistService(t)

	_, err := svc.Create(1, CreatePlaylistInput{Name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestCreatePlaylist_NameTooLong(t *testing.T) {
	svc, _, _, _ := newTestPlaylistService(t)

	longName := strings.Repeat("a", 256)
	_, err := svc.Create(1, CreatePlaylistInput{Name: longName})
	if err == nil {
		t.Fatal("expected error for long name")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestCreatePlaylist_InvalidVisibility(t *testing.T) {
	svc, _, _, _ := newTestPlaylistService(t)

	_, err := svc.Create(1, CreatePlaylistInput{Name: "Test", Visibility: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid visibility")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestGetByUUID_OwnerAccess(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:         1,
		UUID:       testUUID,
		UserID:     1,
		Name:       "My Playlist",
		Visibility: "private",
		User:       data.User{ID: 1, Username: "admin"},
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().GetSceneCount(uint(1)).Return(int64(5), nil)
	playlistRepo.EXPECT().GetTotalDuration(uint(1)).Return(int64(3600), nil)
	playlistRepo.EXPECT().GetPlaylistTags(uint(1)).Return([]data.Tag{}, nil)
	playlistRepo.EXPECT().GetThumbnailScenes(uint(1), 4).Return([]data.Scene{}, nil)
	playlistRepo.EXPECT().GetLikeStatus(uint(1), uint(1)).Return(false, nil)
	playlistRepo.EXPECT().GetLikeCount(uint(1)).Return(int64(0), nil)
	playlistRepo.EXPECT().GetPlaylistScenes(uint(1)).Return([]data.PlaylistScene{}, nil)
	playlistRepo.EXPECT().GetProgress(uint(1), uint(1)).Return(nil, nil)

	detail, err := svc.GetByUUID(1, testUUID.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if detail.Name != "My Playlist" {
		t.Fatalf("expected name 'My Playlist', got %q", detail.Name)
	}
}

func TestGetByUUID_PublicAccessByNonOwner(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:         1,
		UUID:       testUUID,
		UserID:     1,
		Name:       "Public Playlist",
		Visibility: "public",
		User:       data.User{ID: 1, Username: "admin"},
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().GetSceneCount(uint(1)).Return(int64(3), nil)
	playlistRepo.EXPECT().GetTotalDuration(uint(1)).Return(int64(1800), nil)
	playlistRepo.EXPECT().GetPlaylistTags(uint(1)).Return([]data.Tag{}, nil)
	playlistRepo.EXPECT().GetThumbnailScenes(uint(1), 4).Return([]data.Scene{}, nil)
	playlistRepo.EXPECT().GetLikeStatus(uint(2), uint(1)).Return(false, nil)
	playlistRepo.EXPECT().GetLikeCount(uint(1)).Return(int64(1), nil)
	playlistRepo.EXPECT().GetPlaylistScenes(uint(1)).Return([]data.PlaylistScene{}, nil)
	playlistRepo.EXPECT().GetProgress(uint(2), uint(1)).Return(nil, nil)

	// User 2 accessing user 1's public playlist
	detail, err := svc.GetByUUID(2, testUUID.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if detail.Name != "Public Playlist" {
		t.Fatalf("expected name 'Public Playlist', got %q", detail.Name)
	}
}

func TestGetByUUID_PrivateDenied(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:         1,
		UUID:       testUUID,
		UserID:     1,
		Name:       "Private Playlist",
		Visibility: "private",
		User:       data.User{ID: 1, Username: "admin"},
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)

	// User 2 trying to access user 1's private playlist
	_, err := svc.GetByUUID(2, testUUID.String())
	if err == nil {
		t.Fatal("expected error for private playlist access by non-owner")
	}
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got: %v", err)
	}
}

func TestGetByUUID_NotFound(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.GetByUUID(1, testUUID.String())
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestUpdatePlaylist_OwnerOnly(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:         1,
		UUID:       testUUID,
		UserID:     1,
		Name:       "Original",
		Visibility: "private",
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)

	newName := "Updated"
	playlistRepo.EXPECT().Update(gomock.Any()).Return(nil)

	result, err := svc.Update(1, testUUID.String(), UpdatePlaylistInput{Name: &newName})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Name != "Updated" {
		t.Fatalf("expected name 'Updated', got %q", result.Name)
	}
}

func TestUpdatePlaylist_NonOwnerForbidden(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:         1,
		UUID:       testUUID,
		UserID:     1,
		Name:       "Original",
		Visibility: "private",
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)

	newName := "Hacked"
	_, err := svc.Update(2, testUUID.String(), UpdatePlaylistInput{Name: &newName})
	if err == nil {
		t.Fatal("expected error for non-owner update")
	}
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got: %v", err)
	}
}

func TestDeletePlaylist_OwnerOnly(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().Delete(uint(1)).Return(nil)

	err := svc.Delete(1, testUUID.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeletePlaylist_NonOwnerForbidden(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)

	err := svc.Delete(2, testUUID.String())
	if err == nil {
		t.Fatal("expected error for non-owner delete")
	}
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got: %v", err)
	}
}

func TestAddScenes_DuplicateConflict(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().AddScenes(uint(1), []uint{42}).Return(data.ErrDuplicateSceneSentinel())

	err := svc.AddScenes(1, testUUID.String(), []uint{42})
	if err == nil {
		t.Fatal("expected error for duplicate scene")
	}
	if !apperrors.IsConflict(err) {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestRemoveScene_Success(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().RemoveScene(uint(1), uint(42)).Return(nil)

	err := svc.RemoveScene(1, testUUID.String(), 42)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRemoveScenes_Success(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().RemoveScenes(uint(1), []uint{10, 20, 30}).Return(nil)

	err := svc.RemoveScenes(1, testUUID.String(), []uint{10, 20, 30})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRemoveScenes_NonOwnerForbidden(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)

	err := svc.RemoveScenes(2, testUUID.String(), []uint{10})
	if err == nil {
		t.Fatal("expected error for non-owner bulk remove")
	}
	if !apperrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got: %v", err)
	}
}

func TestRemoveScenes_EmptyIDs(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)

	err := svc.RemoveScenes(1, testUUID.String(), []uint{})
	if err == nil {
		t.Fatal("expected error for empty scene IDs")
	}
	if !apperrors.IsValidation(err) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestReorderScenes_Success(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().ReorderScenes(uint(1), []uint{3, 1, 2}).Return(nil)

	err := svc.ReorderScenes(1, testUUID.String(), []uint{3, 1, 2})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestToggleLike_Success(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:         1,
		UUID:       testUUID,
		UserID:     1,
		Visibility: "public",
	}

	// First toggle: like
	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().ToggleLike(uint(2), uint(1)).Return(true, nil)

	liked, err := svc.ToggleLike(2, testUUID.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !liked {
		t.Fatal("expected liked to be true")
	}
}

func TestProgress_UpsertAndGet(t *testing.T) {
	svc, playlistRepo, _, _ := newTestPlaylistService(t)

	testUUID := uuid.New()
	playlist := &data.Playlist{
		ID:     1,
		UUID:   testUUID,
		UserID: 1,
	}

	// Update progress
	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().UpsertProgress(uint(1), uint(1), uint(42), 125.5).Return(nil)

	err := svc.UpdateProgress(1, testUUID.String(), 42, 125.5)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Get progress
	sceneID := uint(42)
	playlistRepo.EXPECT().GetByUUID(testUUID.String()).Return(playlist, nil)
	playlistRepo.EXPECT().GetProgress(uint(1), uint(1)).Return(&data.PlaylistProgress{
		UserID:        1,
		PlaylistID:    1,
		LastSceneID:   &sceneID,
		LastPositionS: 125.5,
	}, nil)

	resume, err := svc.GetProgress(1, testUUID.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resume.PositionS != 125.5 {
		t.Fatalf("expected position 125.5, got %f", resume.PositionS)
	}
}
