package core

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// readOnlyFixture is a temp layout with a read-only storage path (Stash), a writable one
// (Library) and a data directory holding generated files.
type readOnlyFixture struct {
	roDir, rwDir, metaDir string
	guard                 *MediaFileGuard
}

func newReadOnlyFixture(t *testing.T) readOnlyFixture {
	t.Helper()
	base := t.TempDir()
	f := readOnlyFixture{
		roDir:   filepath.Join(base, "stash"),
		rwDir:   filepath.Join(base, "library"),
		metaDir: filepath.Join(base, "data"),
	}
	f.guard = newTestMediaFileGuard(t,
		data.StoragePath{ID: 1, Name: "Library", Path: f.rwDir},
		data.StoragePath{ID: 2, Name: "Stash", Path: f.roDir, ReadOnly: true},
	)
	return f
}

// scene creates a scene file plus its generated thumbnails and sprites, returns the scene and
// the generated file paths.
func (f readOnlyFixture) scene(t *testing.T, id uint, dir string, storagePathID uint) (*data.Scene, []string) {
	t.Helper()
	video := filepath.Join(dir, fmt.Sprintf("scene-%d.mp4", id))
	writeTestFile(t, video)
	generated := []string{
		filepath.Join(f.metaDir, "thumbnails", fmt.Sprintf("%d_thumb_sm.webp", id)),
		filepath.Join(f.metaDir, "thumbnails", fmt.Sprintf("%d_thumb_lg.webp", id)),
		filepath.Join(f.metaDir, "sprites", fmt.Sprintf("%d_sheet_001.jpg", id)),
		filepath.Join(f.metaDir, "vtt", fmt.Sprintf("%d.vtt", id)),
	}
	for _, g := range generated {
		writeTestFile(t, g)
	}
	return &data.Scene{
		ID:              id,
		StoredPath:      video,
		StoragePathID:   &storagePathID,
		ThumbnailPath:   generated[0],
		SpriteSheetPath: generated[2],
		VttPath:         generated[3],
	}, generated
}

func assertAllRemoved(t *testing.T, paths []string) {
	t.Helper()
	for _, p := range paths {
		assertFileExists(t, p, false)
	}
}

func newGuardedSceneService(t *testing.T, f readOnlyFixture) (*SceneService, *mocks.MockSceneRepository) {
	t.Helper()
	svc, repo := newTestSceneService(t)
	svc.MetadataPath = f.metaDir
	svc.mediaGuard = f.guard
	return svc, repo
}

func TestHardDeleteScene_ReadOnlyKeepsVideo(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	scene, generated := f.scene(t, 10, f.roDir, 2)

	repo.EXPECT().HardDelete(uint(10)).Return(scene, nil)

	if err := svc.HardDeleteScene(10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileExists(t, scene.StoredPath, true)
	assertAllRemoved(t, generated)
}

func TestHardDeleteScene_WritableRemovesVideo(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	scene, generated := f.scene(t, 11, f.rwDir, 1)

	repo.EXPECT().HardDelete(uint(11)).Return(scene, nil)

	if err := svc.HardDeleteScene(11); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileExists(t, scene.StoredPath, false)
	assertAllRemoved(t, generated)
}

func TestHardDeleteScene_ReadOnlyByPrefixWithoutStoragePathID(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	scene, _ := f.scene(t, 12, filepath.Join(f.roDir, "sub"), 1)
	scene.StoragePathID = nil

	repo.EXPECT().HardDelete(uint(12)).Return(scene, nil)

	if err := svc.HardDeleteScene(12); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileExists(t, scene.StoredPath, true)
}

func TestEmptyTrash_ReadOnlyKeepsVideos(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	ro, _ := f.scene(t, 20, f.roDir, 2)
	rw, _ := f.scene(t, 21, f.rwDir, 1)

	repo.EXPECT().ListTrashed(1, 10000).Return([]data.Scene{*ro, *rw}, int64(2), nil)
	repo.EXPECT().HardDelete(uint(20)).Return(ro, nil)
	repo.EXPECT().HardDelete(uint(21)).Return(rw, nil)

	deleted, err := svc.EmptyTrash()
	if err != nil || deleted != 2 {
		t.Fatalf("expected 2 scene rows deleted, got %d, %v", deleted, err)
	}
	assertFileExists(t, ro.StoredPath, true)
	assertFileExists(t, rw.StoredPath, false)
}

func TestDeleteScene_ReadOnlyKeepsVideo(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	scene, _ := f.scene(t, 30, f.roDir, 2)

	repo.EXPECT().GetByID(uint(30)).Return(scene, nil)
	repo.EXPECT().Delete(uint(30)).Return(nil)

	if err := svc.DeleteScene(30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileExists(t, scene.StoredPath, true)
}

func TestTrashCleanupWorker_ReadOnlyKeepsVideo(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	ro, _ := f.scene(t, 40, f.roDir, 2)
	rw, _ := f.scene(t, 41, f.rwDir, 1)

	repo.EXPECT().GetExpiredTrashScenes(7).Return([]data.Scene{*ro, *rw}, nil)
	repo.EXPECT().HardDelete(uint(40)).Return(ro, nil)
	repo.EXPECT().HardDelete(uint(41)).Return(rw, nil)

	worker := NewTrashCleanupWorker(svc, repo, nil, nil, zap.NewNop())
	worker.cleanup()

	assertFileExists(t, ro.StoredPath, true)
	assertFileExists(t, rw.StoredPath, false)
}

func TestRunDeletionCleanup_ReadOnlyKeepsVideo(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	svc.sceneService.MetadataPath = f.metaDir
	svc.sceneService.mediaGuard = f.guard
	trashedAt := time.Now()
	ro, _ := f.scene(t, 50, f.roDir, 2)
	ro.TrashedAt = &trashedAt
	rw, _ := f.scene(t, 51, f.rwDir, 1)
	rw.TrashedAt = &trashedAt

	groupRepo.EXPECT().ListMembersDueForDeletion(gomock.Any(), deletionCleanupBatch).Return([]data.DuplicateGroupMember{
		{GroupID: 1, SceneID: 50}, {GroupID: 1, SceneID: 51},
	}, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(50)).Return(ro, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(51)).Return(rw, nil)
	sceneRepo.EXPECT().HardDelete(uint(50)).Return(ro, nil)
	sceneRepo.EXPECT().HardDelete(uint(51)).Return(rw, nil)

	deleted, err := svc.RunDeletionCleanup()
	if err != nil || deleted != 2 {
		t.Fatalf("expected 2 scenes deleted, got %d, %v", deleted, err)
	}
	assertFileExists(t, ro.StoredPath, true)
	assertFileExists(t, rw.StoredPath, false)
}

func TestBulkDeleteScenes_PermanentReadOnlyKeepsVideo(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, _, _, scenes, _, _, _ := newTestExplorerService(t)
	svc.jobHistoryRepo = nil
	svc.metadataPath = f.metaDir
	svc.mediaGuard = f.guard
	ro, roGenerated := f.scene(t, 60, f.roDir, 2)
	rw, _ := f.scene(t, 61, f.rwDir, 1)

	scenes.EXPECT().GetByIDs([]uint{60, 61}).Return([]data.Scene{*ro, *rw}, nil)
	scenes.EXPECT().HardDelete(uint(60)).Return(ro, nil)
	scenes.EXPECT().HardDelete(uint(61)).Return(rw, nil)

	deleted, err := svc.BulkDeleteScenes([]uint{60, 61}, true)
	if err != nil || deleted != 2 {
		t.Fatalf("expected 2 scenes deleted, got %d, %v", deleted, err)
	}
	assertFileExists(t, ro.StoredPath, true)
	assertFileExists(t, rw.StoredPath, false)
	// Generated files of the read-only scene still go
	assertFileExists(t, roGenerated[0], false)
	assertFileExists(t, roGenerated[2], false)
	assertFileExists(t, roGenerated[3], false)
}

// newTestUploadFile builds a multipart file header for UploadScene.
func newTestUploadFile(t *testing.T, name string) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("scene", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("video content")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["scene"][0]
}

func TestUploadScene_IntoReadOnlyPathRefused(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, _ := newGuardedSceneService(t, f)
	uploadDir := filepath.Join(f.roDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatal(err)
	}
	svc.ScenePath = uploadDir

	_, err := svc.UploadScene(newTestUploadFile(t, "clip.mp4"), "")
	if !apperrors.IsValidation(err) || apperrors.GetCode(err) != apperrors.CodeStoragePathReadOnly {
		t.Fatalf("expected a read-only validation error, got %v", err)
	}
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("nothing must be written into a read-only path, found %d files", len(entries))
	}
}

func TestUploadScene_IntoWritablePathAllowed(t *testing.T) {
	f := newReadOnlyFixture(t)
	svc, repo := newGuardedSceneService(t, f)
	if err := os.MkdirAll(f.rwDir, 0755); err != nil {
		t.Fatal(err)
	}
	svc.ScenePath = f.rwDir

	repo.EXPECT().Create(gomock.Any()).Return(nil)

	scene, err := svc.UploadScene(newTestUploadFile(t, "clip.mp4"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFileExists(t, scene.StoredPath, true)
}
