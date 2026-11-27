package processing

import (
	"errors"
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// bulkTestQueue is a JobQueueRecorder that records the pending jobs created.
type bulkTestQueue struct {
	created []uint
}

func (q *bulkTestQueue) RecordJobStart(string, uint, string, string)                    {}
func (q *bulkTestQueue) RecordJobStartWithRetry(string, uint, string, string, int, int) {}
func (q *bulkTestQueue) RecordJobComplete(string, uint, string)                         {}
func (q *bulkTestQueue) RecordJobCancelled(string)                                      {}
func (q *bulkTestQueue) RecordJobFailedWithRetry(string, uint, string, error)           {}
func (q *bulkTestQueue) UpdateProgress(string, int)                                     {}
func (q *bulkTestQueue) CreatePendingJob(_ string, sceneID uint, _, _, _ string) error {
	q.created = append(q.created, sceneID)
	return nil
}
func (q *bulkTestQueue) CreatePendingJobWithPriority(_ string, sceneID uint, _, _ string, _ int, _ string) error {
	q.created = append(q.created, sceneID)
	return nil
}
func (q *bulkTestQueue) CreatePendingJobWithRetry(_ string, sceneID uint, _, _ string, _, _ int, _ string) error {
	q.created = append(q.created, sceneID)
	return nil
}
func (q *bulkTestQueue) ExistsPendingOrRunning(uint, string) (bool, error) { return false, nil }

type bulkTestChecker struct {
	ids   map[uint]bool
	err   error
	calls int
}

func (c *bulkTestChecker) FailingSceneIDs(string) (map[uint]bool, error) {
	c.calls++
	return c.ids, c.err
}

func newBulkTestSubmitter(t *testing.T, checker FailingChecker) (*JobSubmitter, *mocks.MockSceneRepository, *bulkTestQueue) {
	t.Helper()
	repo := mocks.NewMockSceneRepository(gomock.NewController(t))
	repo.EXPECT().GetByID(gomock.Any()).Return(&data.Scene{Title: "t"}, nil).AnyTimes()
	queue := &bulkTestQueue{}
	js := NewJobSubmitter(repo, &PoolManager{}, NewPhaseTracker(nil, false), queue, zap.NewNop())
	js.SetFailingChecker(checker)
	return js, repo, queue
}

func bulkScenes(ids ...uint) []data.Scene {
	scenes := make([]data.Scene, len(ids))
	for i, id := range ids {
		scenes[i] = data.Scene{ID: id, Duration: 60}
	}
	return scenes
}

func TestSubmitBulkPhase_MissingSkipsFailingScenes(t *testing.T) {
	checker := &bulkTestChecker{ids: map[uint]bool{2: true}}
	js, repo, queue := newBulkTestSubmitter(t, checker)
	repo.EXPECT().GetScenesNeedingPhase("sprites").Return(bulkScenes(1, 2, 3), nil)

	result, err := js.SubmitBulkPhase("sprites", "missing", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Submitted != 2 || result.SkippedFailing != 1 || result.Skipped != 0 {
		t.Fatalf("result %+v, want 2 submitted and 1 failing skipped", result)
	}
	if len(queue.created) != 2 || queue.created[0] != 1 || queue.created[1] != 3 {
		t.Fatalf("created %v, want scenes 1 and 3", queue.created)
	}
}

func TestSubmitBulkPhase_IncludeFailingAndAllModeRunEverything(t *testing.T) {
	checker := &bulkTestChecker{ids: map[uint]bool{2: true}}
	js, repo, queue := newBulkTestSubmitter(t, checker)
	repo.EXPECT().GetScenesNeedingPhase("sprites").Return(bulkScenes(1, 2, 3), nil)
	result, err := js.SubmitBulkPhase("sprites", "missing", "", nil, true)
	if err != nil || result.Submitted != 3 || result.SkippedFailing != 0 {
		t.Fatalf("include_failing: %+v, %v", result, err)
	}

	repo.EXPECT().GetAll().Return(bulkScenes(1, 2, 3), nil)
	result, err = js.SubmitBulkPhase("sprites", "all", "", nil, false)
	if err != nil || result.Submitted != 3 || result.SkippedFailing != 0 {
		t.Fatalf("all mode: %+v, %v", result, err)
	}
	if checker.calls != 0 {
		t.Fatalf("the failing scenes must not be looked up, got %d calls", checker.calls)
	}
	if len(queue.created) != 6 {
		t.Fatalf("created %d jobs, want 6", len(queue.created))
	}
}

func TestSubmitBulkPhase_CheckerErrorSkipsNothing(t *testing.T) {
	js, repo, _ := newBulkTestSubmitter(t, &bulkTestChecker{err: errors.New("db down")})
	repo.EXPECT().GetScenesNeedingPhase("metadata").Return(bulkScenes(1, 2), nil)
	result, err := js.SubmitBulkPhase("metadata", "missing", "", nil, false)
	if err != nil || result.Submitted != 2 || result.SkippedFailing != 0 {
		t.Fatalf("result %+v, %v", result, err)
	}
}
