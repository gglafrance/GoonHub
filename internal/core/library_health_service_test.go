package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// healthRepoState backs a MockLibraryHealthRepository with an in-memory state, so the tests
// can drive a whole background task.
type healthRepoState struct {
	mu       sync.Mutex
	paths    []data.HealthScenePath
	items    []data.LibraryHealthCheckItem
	finished []data.LibraryHealthCheck
	updates  map[uint]int64
	progress int
	// block, when set, makes ListScenePaths wait for it to be closed
	block chan struct{}
}

func newHealthTestRepo(t *testing.T, ctrl *gomock.Controller, paths []data.HealthScenePath) (*mocks.MockLibraryHealthRepository, *healthRepoState) {
	t.Helper()
	st := &healthRepoState{paths: paths, updates: map[uint]int64{}}
	repo := mocks.NewMockLibraryHealthRepository(ctrl)
	repo.EXPECT().CountScenePaths(gomock.Any()).DoAndReturn(func(string) (int64, error) {
		return int64(len(st.paths)), nil
	}).AnyTimes()
	repo.EXPECT().CreateCheck(gomock.Any()).DoAndReturn(func(c *data.LibraryHealthCheck) error {
		c.ID = 7
		return nil
	}).AnyTimes()
	repo.EXPECT().ListScenePaths(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, after uint, limit int) ([]data.HealthScenePath, error) {
		st.mu.Lock()
		block := st.block
		st.mu.Unlock()
		if block != nil {
			<-block
		}
		var out []data.HealthScenePath
		for _, p := range st.paths {
			if p.ID > after && len(out) < limit {
				out = append(out, p)
			}
		}
		return out, nil
	}).AnyTimes()
	repo.EXPECT().InsertCheckItems(gomock.Any()).DoAndReturn(func(items []data.LibraryHealthCheckItem) error {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.items = append(st.items, items...)
		return nil
	}).AnyTimes()
	repo.EXPECT().UpdateCheck(gomock.Any()).DoAndReturn(func(*data.LibraryHealthCheck) error {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.progress++
		return nil
	}).AnyTimes()
	repo.EXPECT().FinishCheck(gomock.Any()).DoAndReturn(func(c *data.LibraryHealthCheck) error {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.finished = append(st.finished, *c)
		return nil
	}).AnyTimes()
	repo.EXPECT().UpdateSceneSize(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(id uint, _ string, size int64) (bool, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.updates[id] = size
		return true, nil
	}).AnyTimes()
	return repo, st
}

func (st *healthRepoState) lastFinished(t *testing.T) data.LibraryHealthCheck {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.finished) != 1 {
		t.Fatalf("expected one finished task, got %d", len(st.finished))
	}
	return st.finished[0]
}

func newHealthTestService(repo data.LibraryHealthRepository, sceneRepo data.SceneRepository, concurrency int) *LibraryHealthService {
	return NewLibraryHealthService(repo, sceneRepo, NewStoragePathService(nil, zap.NewNop()), nil, concurrency, zap.NewNop())
}

type healthRecordingIndexer struct {
	mu      sync.Mutex
	indexed []uint
}

func (r *healthRecordingIndexer) IndexScene(*data.Scene) error       { return nil }
func (r *healthRecordingIndexer) UpdateSceneIndex(*data.Scene) error { return nil }
func (r *healthRecordingIndexer) DeleteSceneIndex(uint) error        { return nil }
func (r *healthRecordingIndexer) BulkDeleteSceneIndex([]uint) error  { return nil }
func (r *healthRecordingIndexer) BulkUpdateSceneIndex(scenes []data.Scene) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range scenes {
		r.indexed = append(r.indexed, s.ID)
	}
	return nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryHealthService_MissingFileCheck(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.mp4")
	writeFile(t, present, "video")
	subdir := filepath.Join(dir, "a-directory.mp4")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(present)
	if err != nil {
		t.Fatal(err)
	}

	paths := []data.HealthScenePath{
		{ID: 1, StoredPath: present},
		{ID: 2, StoredPath: filepath.Join(dir, "gone.mp4")},
		{ID: 3, StoredPath: subdir},
		{ID: 4, StoredPath: filepath.Join(present, "under-a-file.mp4")}, // ENOTDIR
		{ID: 5, StoredPath: ""},
	}
	// More paths than a DB batch, so the keyset pagination is exercised
	for i := 0; i < healthPathBatchSize+5; i++ {
		paths = append(paths, data.HealthScenePath{ID: uint(10 + i), StoredPath: present})
	}
	ctrl := gomock.NewController(t)
	repo, st := newHealthTestRepo(t, ctrl, paths)
	svc := newHealthTestService(repo, nil, 3)

	task, err := svc.StartTask(data.HealthCheckKindMissingFiles)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if task.Total != len(paths) || task.Status != data.HealthCheckStatusRunning {
		t.Fatalf("started task: %+v", task)
	}
	svc.wait()

	got := st.lastFinished(t)
	if got.Status != data.HealthCheckStatusCompleted || got.CompletedAt == nil {
		t.Fatalf("finished task: %+v", got)
	}
	if got.Checked != len(paths) || got.Missing != 3 || got.Unreadable != 1 {
		t.Fatalf("counts: checked=%d missing=%d unreadable=%d", got.Checked, got.Missing, got.Unreadable)
	}
	sort.Slice(st.items, func(i, j int) bool { return st.items[i].SceneID < st.items[j].SceneID })
	if len(st.items) != 4 {
		t.Fatalf("items: %+v", st.items)
	}
	wantReasons := map[uint]string{2: data.HealthItemReasonMissing, 3: data.HealthItemReasonUnreadable, 4: data.HealthItemReasonMissing, 5: data.HealthItemReasonMissing}
	for _, item := range st.items {
		if item.CheckID != 7 || item.Reason != wantReasons[item.SceneID] {
			t.Fatalf("item %+v, want reason %q", item, wantReasons[item.SceneID])
		}
	}
	if len(st.updates) != 0 {
		t.Fatalf("a missing-file check must not update sizes: %v", st.updates)
	}
	if status := svc.GetTaskStatus(); status.Running {
		t.Fatal("task still reported running")
	}

	// The media files are only read
	after, err := os.Stat(present)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("media file changed: %v", err)
	}
	if _, err := os.Stat(subdir); err != nil {
		t.Fatalf("directory changed: %v", err)
	}
}

func TestLibraryHealthService_FixSizes(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.mp4")
	writeFile(t, full, "12345")
	empty := filepath.Join(dir, "empty.mp4")
	writeFile(t, empty, "")

	paths := []data.HealthScenePath{
		{ID: 1, StoredPath: full},
		{ID: 2, StoredPath: empty},
		{ID: 3, StoredPath: filepath.Join(dir, "gone.mp4")},
	}
	ctrl := gomock.NewController(t)
	repo, st := newHealthTestRepo(t, ctrl, paths)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	sceneRepo.EXPECT().GetByIDs([]uint{1}).Return([]data.Scene{{ID: 1, Size: 5}}, nil)
	indexer := &healthRecordingIndexer{}
	svc := newHealthTestService(repo, sceneRepo, 2)
	svc.SetIndexer(indexer)

	if _, err := svc.StartTask(data.HealthCheckKindFixSizes); err != nil {
		t.Fatalf("start: %v", err)
	}
	svc.wait()

	got := st.lastFinished(t)
	if got.Status != data.HealthCheckStatusCompleted || got.Kind != data.HealthCheckKindFixSizes {
		t.Fatalf("finished task: %+v", got)
	}
	if got.Checked != 3 || got.Fixed != 1 || got.Unchanged != 1 || got.Missing != 1 {
		t.Fatalf("counts: %+v", got)
	}
	if len(st.updates) != 1 || st.updates[1] != 5 {
		t.Fatalf("size updates: %v, want scene 1 -> 5", st.updates)
	}
	if len(st.items) != 0 {
		t.Fatalf("a size fix stores no items: %+v", st.items)
	}
	if len(indexer.indexed) != 1 || indexer.indexed[0] != 1 {
		t.Fatalf("re-indexed scenes: %v", indexer.indexed)
	}
	if info, err := os.Stat(full); err != nil || info.Size() != 5 {
		t.Fatalf("media file changed: %v", err)
	}
}

func TestLibraryHealthService_OneTaskAtATimeAndCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo, st := newHealthTestRepo(t, ctrl, []data.HealthScenePath{{ID: 1, StoredPath: "/nowhere"}})
	release := make(chan struct{})
	st.block = release
	defer close(release)
	svc := newHealthTestService(repo, nil, 2)

	if _, err := svc.StartTask(data.HealthCheckKindMissingFiles); err != nil {
		t.Fatalf("start: %v", err)
	}
	status := svc.GetTaskStatus()
	if !status.Running || status.Task == nil || status.Task.Kind != data.HealthCheckKindMissingFiles {
		t.Fatalf("status while running: %+v", status)
	}
	if _, err := svc.StartTask(data.HealthCheckKindFixSizes); !apperrors.IsConflict(err) {
		t.Fatalf("second task: want a conflict, got %v", err)
	}

	if err := svc.CancelTask(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	svc.wait()
	if got := st.lastFinished(t); got.Status != data.HealthCheckStatusCancelled {
		t.Fatalf("cancelled task: %+v", got)
	}
	if err := svc.CancelTask(); !apperrors.IsValidation(err) {
		t.Fatalf("cancel with no task: want a validation error, got %v", err)
	}
}

func TestLibraryHealthService_StatConcurrencyIsBounded(t *testing.T) {
	var paths []data.HealthScenePath
	for i := 1; i <= 40; i++ {
		paths = append(paths, data.HealthScenePath{ID: uint(i), StoredPath: "/x"})
	}
	ctrl := gomock.NewController(t)
	repo, st := newHealthTestRepo(t, ctrl, paths)
	svc := newHealthTestService(repo, nil, 3)

	var active, peak int32
	svc.statFile = func(string) (fs.FileInfo, error) {
		n := atomic.AddInt32(&active, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return nil, fs.ErrNotExist
	}

	if _, err := svc.StartTask(data.HealthCheckKindMissingFiles); err != nil {
		t.Fatal(err)
	}
	svc.wait()
	if got := st.lastFinished(t); got.Checked != 40 || got.Missing != 40 {
		t.Fatalf("counts: %+v", got)
	}
	if p := atomic.LoadInt32(&peak); p > 3 || p < 1 {
		t.Fatalf("peak concurrent stats %d, want 1..3", p)
	}
}

func TestLibraryHealthService_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockLibraryHealthRepository(ctrl)
	svc := newHealthTestService(repo, nil, 0)
	if svc.concurrency != healthDefaultConcurrency {
		t.Fatalf("default concurrency: %d", svc.concurrency)
	}
	if _, err := svc.StartTask("delete_everything"); !apperrors.IsValidation(err) {
		t.Fatalf("unknown task: %v", err)
	}
	if _, _, err := svc.ListScenes("nope", 1, 10); !apperrors.IsValidation(err) {
		t.Fatalf("unknown category: %v", err)
	}
	if _, err := svc.PreviewAction("scan"); !apperrors.IsValidation(err) {
		t.Fatalf("unknown action: %v", err)
	}
}

type fakeHealthProcessing struct{ duplication, preview bool }

func (f fakeHealthProcessing) IsDuplicationEnabled() bool { return f.duplication }
func (f fakeHealthProcessing) GetProcessingQualityConfig() ProcessingQualityConfig {
	return ProcessingQualityConfig{ScenePreviewEnabled: f.preview}
}

func TestLibraryHealthService_PreviewAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockLibraryHealthRepository(ctrl)
	repo.EXPECT().CountScenesNeedingPhase("fingerprint").Return(int64(30900), nil).Times(2)
	repo.EXPECT().CountScenePaths(data.HealthScenesSizeZero).Return(int64(12), nil)
	svc := NewLibraryHealthService(repo, nil, NewStoragePathService(nil, zap.NewNop()), fakeHealthProcessing{}, 2, zap.NewNop())

	p, err := svc.PreviewAction("fingerprint")
	if err != nil || p.Count != 30900 || p.Enabled || p.Reason == "" {
		t.Fatalf("fingerprint preview with duplication off: %+v (err %v)", p, err)
	}
	svc.processing = fakeHealthProcessing{duplication: true}
	if p, err := svc.PreviewAction("fingerprint"); err != nil || !p.Enabled || p.Count != 30900 {
		t.Fatalf("fingerprint preview: %+v (err %v)", p, err)
	}
	if p, err := svc.PreviewAction(HealthActionFixSizes); err != nil || p.Count != 12 || !p.Enabled {
		t.Fatalf("fix sizes preview: %+v (err %v)", p, err)
	}
}

func TestLibraryHealthService_GetSummary(t *testing.T) {
	dir := t.TempDir()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockLibraryHealthRepository(ctrl)
	storageRepo := mocks.NewMockStoragePathRepository(ctrl)

	storageRepo.EXPECT().List().Return([]data.StoragePath{{ID: 1, Name: "Main", Path: dir}, {ID: 2, Name: "Empty", Path: dir}}, nil)
	one, gone := uint(1), uint(99)
	repo.EXPECT().CountLiveCategories().Return(map[string]int64{data.HealthCategorySizeZero: 3}, int64(10), nil)
	repo.EXPECT().GetLatestCheck(data.HealthCheckKindMissingFiles, data.HealthCheckStatusCompleted).
		Return(&data.LibraryHealthCheck{ID: 5, Kind: data.HealthCheckKindMissingFiles, Status: data.HealthCheckStatusCompleted}, nil)
	repo.EXPECT().CountCheckItems(uint(5)).Return(map[string]int64{data.HealthItemReasonMissing: 2, data.HealthItemReasonUnreadable: 1}, nil)
	repo.EXPECT().GetLatestCheck(data.HealthCheckKindFixSizes, "").Return(nil, nil)
	repo.EXPECT().StoragePathSceneStats().Return([]data.StoragePathSceneStats{
		{StoragePathID: &one, SceneCount: 7, TotalSize: 700},
		{StoragePathID: nil, SceneCount: 2, TotalSize: 20},
		{StoragePathID: &gone, SceneCount: 1, TotalSize: 5},
	}, nil)

	svc := NewLibraryHealthService(repo, nil, NewStoragePathService(storageRepo, zap.NewNop()), fakeHealthProcessing{duplication: true, preview: true}, 2, zap.NewNop())
	summary, err := svc.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalScenes != 10 || summary.Counts[data.HealthCategorySizeZero] != 3 ||
		summary.Counts[data.HealthCategoryMissingFile] != 2 || summary.Counts[data.HealthCategoryUnreadableFile] != 1 {
		t.Fatalf("counts: %+v", summary)
	}
	if !summary.FingerprintEnabled || !summary.ScenePreviewEnabled || summary.LastMissingCheck == nil || summary.LastFixSizes != nil || summary.Task.Running {
		t.Fatalf("flags: %+v", summary)
	}
	if len(summary.StoragePaths) != 3 {
		t.Fatalf("storage paths: %+v", summary.StoragePaths)
	}
	main, empty, orphan := summary.StoragePaths[0], summary.StoragePaths[1], summary.StoragePaths[2]
	if main.SceneCount != 7 || main.TotalSize != 700 || main.DiskUsage == nil || main.DiskUsage.TotalBytes == 0 {
		t.Fatalf("main path: %+v", main)
	}
	if empty.SceneCount != 0 || empty.ID == nil || *empty.ID != 2 {
		t.Fatalf("empty path: %+v", empty)
	}
	if orphan.ID != nil || orphan.SceneCount != 3 || orphan.TotalSize != 25 {
		t.Fatalf("scenes in no storage path: %+v", orphan)
	}
}

func TestLibraryHealthService_ListScenesUsesLastCompletedCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockLibraryHealthRepository(ctrl)
	repo.EXPECT().GetLatestCheck(data.HealthCheckKindMissingFiles, data.HealthCheckStatusCompleted).Return(&data.LibraryHealthCheck{ID: 9}, nil)
	repo.EXPECT().ListCategoryScenes(data.HealthCategoryMissingFile, uint(9), 2, healthMaxPageLimit).Return([]data.HealthSceneRow{{ID: 1}}, int64(101), nil)
	repo.EXPECT().ListCategoryScenes(data.HealthCategorySizeZero, uint(0), 1, 20).Return(nil, int64(0), nil)
	svc := newHealthTestService(repo, nil, 1)

	rows, total, err := svc.ListScenes(data.HealthCategoryMissingFile, 2, 500)
	if err != nil || len(rows) != 1 || total != 101 {
		t.Fatalf("missing list: %v %d %v", rows, total, err)
	}
	if _, _, err := svc.ListScenes(data.HealthCategorySizeZero, 0, 0); err != nil {
		t.Fatal(err)
	}
}
