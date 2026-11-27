package core

import (
	"errors"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// fakeFiles stands in for os.Stat: path -> (size, modification time). A missing path fails.
type fakeFiles map[string]struct {
	size int64
	mod  time.Time
}

func (f fakeFiles) stat(path string) (int64, time.Time, error) {
	info, ok := f[path]
	if !ok {
		return 0, time.Time{}, errors.New("no such file")
	}
	return info.size, info.mod, nil
}

var failTestMod = time.Date(2026, 11, 20, 10, 0, 0, 123456789, time.UTC)

func newTestFailureService(t *testing.T, threshold int) (*SceneFailureService, *mocks.MockScenePhaseFailureRepository, *mocks.MockSceneRepository, fakeFiles) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockScenePhaseFailureRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	svc := NewSceneFailureService(repo, sceneRepo, config.ProcessingConfig{FailingThreshold: threshold}, zap.NewNop())
	files := fakeFiles{}
	svc.stat = files.stat
	return svc, repo, sceneRepo, files
}

// failRow builds a counter whose recorded file state matches the given size and the test date.
func failRow(id, sceneID uint, phase string, failures int, path string, size int64) data.ScenePhaseFailureRow {
	mod := failTestMod.Truncate(time.Microsecond)
	return data.ScenePhaseFailureRow{
		ScenePhaseFailure: data.ScenePhaseFailure{
			ID: id, SceneID: sceneID, Phase: phase, Failures: failures,
			LastError: "video file is corrupted", LastFailedAt: failTestMod.Add(time.Duration(id) * time.Minute),
			FileSize: &size, FileModTime: &mod,
		},
		SceneTitle: "scene", StoredPath: path,
	}
}

func TestSceneFailureService_RecordFailureReadsFileAndReportsFailing(t *testing.T) {
	svc, repo, sceneRepo, files := newTestFailureService(t, 3)
	files["/v/a.mp4"] = struct {
		size int64
		mod  time.Time
	}{100, failTestMod}
	sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, StoredPath: "/v/a.mp4"}, nil).Times(2)

	repo.EXPECT().RecordFailure(gomock.Any()).DoAndReturn(func(rec data.ScenePhaseFailureRecord) (*data.ScenePhaseFailure, error) {
		if rec.SceneID != 7 || rec.Phase != "sprites" || rec.Error != "ffmpeg failed" {
			t.Fatalf("unexpected record %+v", rec)
		}
		if rec.FileSize == nil || *rec.FileSize != 100 || rec.FileModTime == nil ||
			!rec.FileModTime.Equal(failTestMod.Truncate(time.Microsecond)) {
			t.Fatalf("file state not recorded (microseconds): %+v", rec)
		}
		return &data.ScenePhaseFailure{Failures: 2}, nil
	})
	if svc.RecordFailure(7, "sprites", "ffmpeg failed") {
		t.Fatal("2 failures with a threshold of 3 must not be failing")
	}

	repo.EXPECT().RecordFailure(gomock.Any()).Return(&data.ScenePhaseFailure{Failures: 3}, nil)
	if !svc.RecordFailure(7, "sprites", "ffmpeg failed") {
		t.Fatal("3 failures with a threshold of 3 must be failing")
	}
}

func TestSceneFailureService_RecordFailureMissingFileAndErrors(t *testing.T) {
	svc, repo, sceneRepo, _ := newTestFailureService(t, 3)
	sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, StoredPath: "/v/gone.mp4"}, nil)
	repo.EXPECT().RecordFailure(gomock.Any()).DoAndReturn(func(rec data.ScenePhaseFailureRecord) (*data.ScenePhaseFailure, error) {
		if rec.FileSize != nil || rec.FileModTime != nil {
			t.Fatalf("an unreadable file is recorded without state, got %+v", rec)
		}
		return nil, errors.New("db down")
	})
	if svc.RecordFailure(7, "metadata", "boom") {
		t.Fatal("a tracking error never makes a scene failing")
	}
}

func TestSceneFailureService_ThresholdZeroNeverFailing(t *testing.T) {
	svc, repo, sceneRepo, _ := newTestFailureService(t, 0)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
	repo.EXPECT().RecordFailure(gomock.Any()).Return(&data.ScenePhaseFailure{Failures: 50}, nil)
	if svc.RecordFailure(1, "metadata", "boom") {
		t.Fatal("threshold 0 disables failing")
	}
	if failing, err := svc.IsFailing(1, "metadata"); err != nil || failing {
		t.Fatalf("IsFailing with threshold 0 = %v, %v", failing, err)
	}
	ids, err := svc.FailingSceneIDs("metadata")
	if err != nil || len(ids) != 0 {
		t.Fatalf("FailingSceneIDs with threshold 0 = %v, %v", ids, err)
	}
}

func TestSceneFailureService_IsFailing(t *testing.T) {
	svc, repo, _, files := newTestFailureService(t, 3)
	files["/v/a.mp4"] = struct {
		size int64
		mod  time.Time
	}{100, failTestMod}

	tests := []struct {
		name string
		row  *data.ScenePhaseFailureRow
		err  error
		want bool
	}{
		{"no counter", nil, gorm.ErrRecordNotFound, false},
		{"below threshold", ptrRow(failRow(1, 7, "sprites", 2, "/v/a.mp4", 100)), nil, false},
		{"at threshold, same file", ptrRow(failRow(1, 7, "sprites", 3, "/v/a.mp4", 100)), nil, true},
		{"file size changed", ptrRow(failRow(1, 7, "sprites", 5, "/v/a.mp4", 99)), nil, false},
		{"file gone since", ptrRow(failRow(1, 7, "sprites", 5, "/v/missing.mp4", 100)), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.EXPECT().GetByScenePhase(uint(7), "sprites").Return(tt.row, tt.err)
			got, err := svc.IsFailing(7, "sprites")
			if err != nil || got != tt.want {
				t.Fatalf("IsFailing = %v, %v; want %v", got, err, tt.want)
			}
		})
	}

	repo.EXPECT().GetByScenePhase(uint(7), "sprites").Return(nil, errors.New("db down"))
	if _, err := svc.IsFailing(7, "sprites"); err == nil {
		t.Fatal("a database error must be returned")
	}
}

func ptrRow(r data.ScenePhaseFailureRow) *data.ScenePhaseFailureRow { return &r }

func TestSceneFailureService_ModTimeChangeGivesAFreshChance(t *testing.T) {
	svc, repo, _, files := newTestFailureService(t, 3)
	files["/v/a.mp4"] = struct {
		size int64
		mod  time.Time
	}{100, failTestMod.Add(time.Hour)} // re-downloaded: same size, newer file
	repo.EXPECT().GetByScenePhase(uint(7), "metadata").Return(ptrRow(failRow(1, 7, "metadata", 17, "/v/a.mp4", 100)), nil)
	if failing, _ := svc.IsFailing(7, "metadata"); failing {
		t.Fatal("a replaced file must get a fresh chance")
	}
}

func TestSceneFailureService_FailingSceneIDs(t *testing.T) {
	svc, repo, _, files := newTestFailureService(t, 3)
	for _, p := range []string{"/v/1.mp4", "/v/2.mp4"} {
		files[p] = struct {
			size int64
			mod  time.Time
		}{100, failTestMod}
	}
	repo.EXPECT().List(data.ScenePhaseFailureFilter{MinFailures: 3, Phase: "sprites"}).Return([]data.ScenePhaseFailureRow{
		failRow(1, 11, "sprites", 3, "/v/1.mp4", 100),
		failRow(2, 12, "sprites", 4, "/v/2.mp4", 200), // file changed
		failRow(3, 13, "sprites", 9, "/v/2.mp4", 100),
	}, nil)
	ids, err := svc.FailingSceneIDs("sprites")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || !ids[11] || !ids[13] {
		t.Fatalf("failing scenes = %v, want 11 and 13", ids)
	}
}

func TestSceneFailureService_ListTotalsFilterAndPagination(t *testing.T) {
	svc, repo, _, files := newTestFailureService(t, 3)
	files["/v/a.mp4"] = struct {
		size int64
		mod  time.Time
	}{100, failTestMod}
	rows := []data.ScenePhaseFailureRow{
		failRow(5, 1, "metadata", 17, "/v/a.mp4", 100),
		failRow(4, 2, "sprites", 14, "/v/a.mp4", 100),
		failRow(3, 3, "metadata", 3, "/v/a.mp4", 100),
		failRow(2, 4, "metadata", 3, "/v/a.mp4", 1), // file changed: not failing
		failRow(1, 5, "metadata", 4, "/v/a.mp4", 100),
	}
	repo.EXPECT().List(data.ScenePhaseFailureFilter{MinFailures: 3, ErrorQuery: "corrupt"}).Return(rows, nil)

	list, err := svc.List(FailingFilter{Phase: "metadata", ErrorQuery: "corrupt"}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 3 || list.Page != 2 || list.Limit != 2 || list.Threshold != 3 {
		t.Fatalf("unexpected page meta %+v", list)
	}
	if list.Totals["metadata"] != 3 || list.Totals["sprites"] != 1 {
		t.Fatalf("totals per phase = %v, want metadata 3, sprites 1 (phase filter ignored)", list.Totals)
	}
	if len(list.Items) != 1 || list.Items[0].ID != 1 || list.Items[0].Phase != "metadata" {
		t.Fatalf("page 2 = %+v, want the last metadata row", list.Items)
	}

	repo.EXPECT().List(gomock.Any()).Return(rows, nil)
	ids, err := svc.FailingIDs(FailingFilter{Phase: "sprites"})
	if err != nil || len(ids) != 1 || ids[0] != 4 {
		t.Fatalf("FailingIDs = %v, %v; want [4]", ids, err)
	}
}

// fakeSubmitter records the jobs the Retry action queues.
type fakeSubmitter struct {
	queued []string
	fail   map[uint]bool
}

func (f *fakeSubmitter) SubmitPhaseWithPriority(sceneID uint, phase string, priority int) error {
	if f.fail[sceneID] {
		return errors.New("metadata must be extracted first")
	}
	if priority != 1 {
		return errors.New("retry must use the manual priority")
	}
	f.queued = append(f.queued, phase)
	return nil
}

func TestSceneFailureService_RetryClearsThenQueues(t *testing.T) {
	svc, repo, _, _ := newTestFailureService(t, 3)
	if _, err := svc.Retry([]uint{1}); err == nil {
		t.Fatal("retry without a submitter must fail")
	}
	sub := &fakeSubmitter{fail: map[uint]bool{2: true}}
	svc.SetSubmitter(sub)

	repo.EXPECT().GetByIDs([]uint{1, 2, 3}).Return([]data.ScenePhaseFailureRow{
		failRow(1, 1, "sprites", 3, "", 0),
		failRow(2, 2, "thumbnail", 3, "", 0),
	}, nil)
	gomock.InOrder(
		repo.EXPECT().Clear(uint(1), "sprites").Return(nil),
		repo.EXPECT().Clear(uint(2), "thumbnail").Return(nil),
	)
	result, err := svc.Retry([]uint{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if result.Retried != 1 || result.Cleared != 2 || result.Errors != 1 {
		t.Fatalf("result %+v, want 1 retried, 2 cleared, 1 error", result)
	}
	if len(sub.queued) != 1 || sub.queued[0] != "sprites" {
		t.Fatalf("queued %v", sub.queued)
	}
}

func TestSceneFailureService_Clear(t *testing.T) {
	svc, repo, _, _ := newTestFailureService(t, 3)
	repo.EXPECT().DeleteByIDs([]uint{4, 5}).Return(int64(2), nil)
	result, err := svc.Clear([]uint{4, 5})
	if err != nil || result.Cleared != 2 || result.Retried != 0 {
		t.Fatalf("Clear = %+v, %v", result, err)
	}
	repo.EXPECT().DeleteByIDs(gomock.Any()).Return(int64(0), errors.New("db down"))
	if _, err := svc.Clear([]uint{4}); err == nil {
		t.Fatal("a database error must be returned")
	}
}
