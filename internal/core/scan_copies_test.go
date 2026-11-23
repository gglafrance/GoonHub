package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/pkg/oshash"

	"go.uber.org/zap"
)

// fakeScanSceneRepo keeps scenes in memory and implements the scene repository methods the
// scan uses (the others panic through the nil embedded interface).
type fakeScanSceneRepo struct {
	data.SceneRepository
	scenes  []*data.Scene
	deleted map[uint]bool
	nextID  uint
}

func newFakeScanSceneRepo() *fakeScanSceneRepo {
	return &fakeScanSceneRepo{deleted: map[uint]bool{}, nextID: 1}
}

// add records a scene; storagePathID 0 means no storage path (uploaded scene).
func (r *fakeScanSceneRepo) add(path string, size int64, storagePathID uint) *data.Scene {
	sc := &data.Scene{ID: r.nextID, StoredPath: path, OriginalFilename: filepath.Base(path), Size: size}
	if storagePathID != 0 {
		sc.StoragePathID = &storagePathID
	}
	r.nextID++
	r.scenes = append(r.scenes, sc)
	return sc
}

func (r *fakeScanSceneRepo) live(sc *data.Scene) bool {
	return !r.deleted[sc.ID] && sc.TrashedAt == nil
}

func (r *fakeScanSceneRepo) GetAllStoredPathSet() (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for _, sc := range r.scenes {
		if r.live(sc) && sc.StoragePathID != nil {
			out[sc.StoredPath] = struct{}{}
		}
	}
	return out, nil
}

func (r *fakeScanSceneRepo) GetScanLookupEntries() ([]data.ScanLookupEntry, error) {
	out := make([]data.ScanLookupEntry, 0, len(r.scenes))
	for _, sc := range r.scenes {
		out = append(out, data.ScanLookupEntry{ID: sc.ID, StoredPath: sc.StoredPath, Size: sc.Size, OriginalFilename: sc.OriginalFilename, IsDeleted: r.deleted[sc.ID], IsTrashed: sc.TrashedAt != nil})
	}
	return out, nil
}

func (r *fakeScanSceneRepo) GetScenePathsForMissingDetection() ([]data.ScenePathInfo, error) {
	var out []data.ScenePathInfo
	for _, sc := range r.scenes {
		if r.live(sc) && sc.StoragePathID != nil {
			out = append(out, data.ScenePathInfo{ID: sc.ID, StoredPath: sc.StoredPath, StoragePathID: *sc.StoragePathID, Title: sc.Title})
		}
	}
	return out, nil
}

func (r *fakeScanSceneRepo) CreateInBatches(scenes []*data.Scene, _ int) error {
	for _, sc := range scenes {
		sc.ID = r.nextID
		r.nextID++
		r.scenes = append(r.scenes, sc)
	}
	return nil
}

func (r *fakeScanSceneRepo) MarkAsMissing(id uint) error {
	r.deleted[id] = true
	return nil
}

func (r *fakeScanSceneRepo) Restore(id uint) error {
	delete(r.deleted, id)
	return nil
}

// UpdateStoredPath only updates a scene that is not soft-deleted, like the GORM default scope.
func (r *fakeScanSceneRepo) UpdateStoredPath(id uint, newPath string, storagePathID *uint) error {
	for _, sc := range r.scenes {
		if sc.ID == id && !r.deleted[id] {
			sc.StoredPath = newPath
			sc.StoragePathID = storagePathID
		}
	}
	return nil
}

func (r *fakeScanSceneRepo) byPath(path string) *data.Scene {
	for _, sc := range r.scenes {
		if sc.StoredPath == path {
			return sc
		}
	}
	return nil
}

type fakeSceneCopyRepo struct {
	copies  map[string]data.SceneCopy
	touched [][]string
}

func (r *fakeSceneCopyRepo) GetAllStoredPathSet() (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for p := range r.copies {
		out[p] = struct{}{}
	}
	return out, nil
}

func (r *fakeSceneCopyRepo) CreateBatch(copies []*data.SceneCopy) error {
	for _, c := range copies {
		if _, ok := r.copies[c.StoredPath]; !ok {
			r.copies[c.StoredPath] = *c
		}
	}
	return nil
}

func (r *fakeSceneCopyRepo) TouchLastSeen(paths []string, _ time.Time) error {
	r.touched = append(r.touched, append([]string(nil), paths...))
	return nil
}

func (r *fakeSceneCopyRepo) ListBySceneID(sceneID uint) ([]data.SceneCopy, error) {
	var out []data.SceneCopy
	for _, c := range r.copies {
		if c.SceneID == sceneID {
			out = append(out, c)
		}
	}
	return out, nil
}

type fakeScanHistoryRepo struct {
	data.ScanHistoryRepository
}

func (fakeScanHistoryRepo) Update(*data.ScanHistory) error { return nil }

type fakeStoragePathRepo struct {
	data.StoragePathRepository
	paths []data.StoragePath
}

func (r fakeStoragePathRepo) List() ([]data.StoragePath, error) { return r.paths, nil }

// content returns n bytes of a pattern shifted by seed (no period aligned on 8 bytes).
func content(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i%251) + seed
	}
	return b
}

func writeScanFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A library scan records a byte-identical copy of an existing scene as a copy instead of a
// new scene, still creates a scene for a different file of the same size, still detects a
// move, never treats a file as a copy of a scene whose file is missing, and on a second scan
// skips the recorded copies (copies_found) without hashing anything again.
func TestScanRecognisesCopiesOfExistingScenes(t *testing.T) {
	root := t.TempDir()
	original := content(200*1024, 0)
	differentTail := bytes.Clone(original)
	differentTail[len(differentTail)-100] ^= 0xff // same size, same head, different tail
	moved := content(150*1024, 3)
	missingSized := content(170*1024, 5)
	twin := content(90*1024, 7)

	files := map[string][]byte{
		"a/original.mp4":    original,      // file of the existing scene
		"b/original.mp4":    original,      // copy, same name (old file still there: not a move)
		"c/renamed.mkv":     original,      // copy, other name
		"d/other.mp4":       differentTail, // same size, different content: new scene
		"e/twin-1.mp4":      twin,          // new scene
		"f/twin-2.mp4":      twin,          // copy of the scene created for e/twin-1.mp4 in the same scan
		"g/fresh.mp4":       missingSized,  // same size as a live scene whose file is missing: new scene
		"new/moved.mp4":     moved,         // moved file
		"h/not-a-video.txt": original,      // ignored
	}
	for rel, b := range files {
		writeScanFile(t, filepath.Join(root, rel), b)
	}

	const spID = 1
	sceneRepo := newFakeScanSceneRepo()
	origScene := sceneRepo.add(filepath.Join(root, "a/original.mp4"), int64(len(original)), spID)
	movedScene := sceneRepo.add(filepath.Join(root, "old/moved.mp4"), int64(len(moved)), spID)
	// An uploaded scene (no storage path, so never marked missing) whose file is gone
	sceneRepo.add(filepath.Join(root, "gone/lost.mp4"), int64(len(missingSized)), 0)
	// A trashed scene with the same content as e/twin-1.mp4, outside the storage path: a
	// trashed scene is never a copy target
	trashedPath := filepath.Join(t.TempDir(), "twin-0.mp4")
	writeScanFile(t, trashedPath, twin)
	trashedAt := time.Now()
	sceneRepo.add(trashedPath, int64(len(twin)), spID).TrashedAt = &trashedAt

	copyRepo := &fakeSceneCopyRepo{copies: map[string]data.SceneCopy{}}
	storage := NewStoragePathService(fakeStoragePathRepo{paths: []data.StoragePath{{ID: spID, Path: root}}}, zap.NewNop())
	svc := NewScanService(storage, sceneRepo, copyRepo, fakeScanHistoryRepo{}, nil, nil, zap.NewNop())
	hashCalls := map[string]int{}
	svc.hashFile = func(p string) (string, error) {
		hashCalls[p]++
		return oshash.FromFile(p)
	}

	// First scan
	scan := &data.ScanHistory{Status: "running"}
	svc.runScan(context.Background(), scan)

	if scan.Status != "completed" || scan.Errors != 0 {
		t.Fatalf("scan status %s, errors %d", scan.Status, scan.Errors)
	}
	if scan.FilesFound != 8 || scan.VideosAdded != 3 || scan.VideosMoved != 1 || scan.VideosSkipped != 1 || scan.CopiesFound != 3 {
		t.Fatalf("first scan counters: found %d added %d moved %d skipped %d copies %d, want 8 3 1 1 3",
			scan.FilesFound, scan.VideosAdded, scan.VideosMoved, scan.VideosSkipped, scan.CopiesFound)
	}
	for _, rel := range []string{"b/original.mp4", "c/renamed.mkv", "f/twin-2.mp4"} {
		if sc := sceneRepo.byPath(filepath.Join(root, rel)); sc != nil {
			t.Fatalf("a scene was created for the copy %s", rel)
		}
	}
	for _, rel := range []string{"d/other.mp4", "g/fresh.mp4"} {
		if sc := sceneRepo.byPath(filepath.Join(root, rel)); sc == nil || sc.ID == 0 {
			t.Fatalf("no scene created for %s", rel)
		}
	}
	twinScene := sceneRepo.byPath(filepath.Join(root, "e/twin-1.mp4"))
	if twinScene == nil {
		t.Fatal("no scene created for e/twin-1.mp4")
	}
	if movedScene.StoredPath != filepath.Join(root, "new/moved.mp4") || sceneRepo.deleted[movedScene.ID] {
		t.Fatalf("move not detected: scene path %s, deleted %v", movedScene.StoredPath, sceneRepo.deleted[movedScene.ID])
	}

	wantCopies := map[string]uint{
		filepath.Join(root, "b/original.mp4"): origScene.ID,
		filepath.Join(root, "c/renamed.mkv"):  origScene.ID,
		filepath.Join(root, "f/twin-2.mp4"):   twinScene.ID,
	}
	if len(copyRepo.copies) != len(wantCopies) {
		t.Fatalf("recorded copies: %+v", copyRepo.copies)
	}
	origHash, err := oshash.FromFile(filepath.Join(root, "a/original.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	for p, sceneID := range wantCopies {
		c, ok := copyRepo.copies[p]
		if !ok || c.SceneID != sceneID || c.StoragePathID == nil || *c.StoragePathID != spID || c.Size == 0 || len(c.Oshash) != 16 {
			t.Fatalf("copy %s: %+v, want scene %d", p, c, sceneID)
		}
	}
	if got := copyRepo.copies[filepath.Join(root, "c/renamed.mkv")].Oshash; got != origHash {
		t.Fatalf("copy oshash %s, want %s", got, origHash)
	}
	for p, n := range hashCalls {
		if n != 1 {
			t.Fatalf("%s hashed %d times in one scan", p, n)
		}
	}
	if _, ok := hashCalls[filepath.Join(root, "new/moved.mp4")]; ok {
		t.Fatal("a file without a same-size live scene must not be hashed")
	}

	// The files are never touched
	for rel, b := range files {
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || !bytes.Equal(got, b) {
			t.Fatalf("file %s changed or removed (%v)", rel, err)
		}
	}

	// Second scan: nothing added, the copies are skipped and counted, last_seen_at updated once
	scenesBefore := len(sceneRepo.scenes)
	clear(hashCalls)
	scan2 := &data.ScanHistory{Status: "running"}
	svc.runScan(context.Background(), scan2)
	if scan2.Status != "completed" || scan2.Errors != 0 {
		t.Fatalf("second scan status %s, errors %d", scan2.Status, scan2.Errors)
	}
	if scan2.VideosAdded != 0 || scan2.VideosMoved != 0 || scan2.CopiesFound != 3 || scan2.VideosSkipped != 5 || len(sceneRepo.scenes) != scenesBefore {
		t.Fatalf("second scan counters: added %d moved %d copies %d skipped %d (scenes %d -> %d)",
			scan2.VideosAdded, scan2.VideosMoved, scan2.CopiesFound, scan2.VideosSkipped, scenesBefore, len(sceneRepo.scenes))
	}
	if len(hashCalls) != 0 {
		t.Fatalf("second scan hashed files: %v", hashCalls)
	}
	if len(copyRepo.touched) != 1 {
		t.Fatalf("last_seen_at updates: %d, want 1 batched update", len(copyRepo.touched))
	}
	touched := copyRepo.touched[0]
	sort.Strings(touched)
	var want []string
	for p := range wantCopies {
		want = append(want, p)
	}
	sort.Strings(want)
	if len(touched) != len(want) {
		t.Fatalf("touched %v, want %v", touched, want)
	}
	for i := range want {
		if touched[i] != want[i] {
			t.Fatalf("touched %v, want %v", touched, want)
		}
	}
}

// When the file of the same-size scene cannot be read, the new file becomes a scene.
func TestMatchCopyIgnoresUnreadableCandidates(t *testing.T) {
	root := t.TempDir()
	b := content(100*1024, 1)
	newFile := filepath.Join(root, "new.mp4")
	writeScanFile(t, newFile, b)
	idx := &scanLookupIndex{bySize: map[int64][]copyCandidate{
		int64(len(b)): {{sceneID: 7, path: filepath.Join(root, "missing.mp4")}},
	}}
	hashes := newOshashCache(oshash.FromFile)
	if id, _, ok := matchCopy(newFile, int64(len(b)), idx, hashes, func() {}); ok {
		t.Fatalf("matched scene %d whose file is missing", id)
	}
	// A pending candidate that could not be created (ID still 0 after the flush) is skipped
	other := filepath.Join(root, "other.mp4")
	writeScanFile(t, other, b)
	flushed := 0
	idx.bySize[int64(len(b))] = []copyCandidate{{path: other, pending: &data.Scene{}}}
	if _, _, ok := matchCopy(newFile, int64(len(b)), idx, hashes, func() { flushed++ }); ok {
		t.Fatal("matched a pending scene without an ID")
	}
	if flushed != 1 {
		t.Fatalf("flush called %d times, want 1", flushed)
	}
}
