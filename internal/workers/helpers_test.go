package workers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/persistence/migrator"
	"goonhub/internal/jobs"
	"goonhub/pkg/workerproto"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The service tests run on the disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN):
// the leases are the database, faking them would test nothing. Scenes, commits, results and
// the retry policy are fakes. Every test uses its own scene IDs and job IDs.

// jobCounter gives the jobs of a test decreasing priorities: they are claimed in creation order.
var jobCounter atomic.Int64

var (
	testDBOnce sync.Once
	testDB     *gorm.DB
	testDBErr  error
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GOONHUB_TEST_IMPORT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database) to run the remote worker tests")
	}
	testDBOnce.Do(func() {
		if testDBErr = migrator.Run(dsn); testDBErr != nil {
			return
		}
		testDB, testDBErr = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	})
	if testDBErr != nil {
		t.Fatalf("test database: %v", testDBErr)
	}
	return testDB
}

// fakeClock is a settable clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type fakeScenes struct {
	mu      sync.Mutex
	scenes  map[uint]*data.Scene
	sprites map[uint]int // UpdateSprites calls per scene
}

func (f *fakeScenes) GetByID(id uint) (*data.Scene, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.scenes[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeScenes) UpdateSprites(id uint, _, _ string, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sprites[id]++
	return nil
}

func (f *fakeScenes) add(s *data.Scene) {
	f.mu.Lock()
	f.scenes[s.ID] = s
	f.mu.Unlock()
}

func (f *fakeScenes) remove(id uint) {
	f.mu.Lock()
	delete(f.scenes, id)
	f.mu.Unlock()
}

func (f *fakeScenes) spriteCommits(id uint) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sprites[id]
}

type fakeResults struct {
	mu      sync.Mutex
	results []jobs.JobResult
}

func (f *fakeResults) HandleResult(r jobs.JobResult) {
	f.mu.Lock()
	f.results = append(f.results, r)
	f.mu.Unlock()
}

func (f *fakeResults) forJob(jobID string) []jobs.JobResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []jobs.JobResult
	for _, r := range f.results {
		if r.JobID == jobID {
			out = append(out, r)
		}
	}
	return out
}

type fakeRetry struct {
	mu  sync.Mutex
	max int
	dlq []string
}

func (f *fakeRetry) MaxRetries(string) int { return f.max }
func (f *fakeRetry) ToDeadLetter(jobID, _ string, _ uint, _ int, _ string) error {
	f.mu.Lock()
	f.dlq = append(f.dlq, jobID)
	f.mu.Unlock()
	return nil
}

type fakeAnimated struct {
	mu      sync.Mutex
	plans   map[uint]*jobs.AnimatedThumbnailsPlan
	commits int
}

func (f *fakeAnimated) PrepareAnimatedThumbnails(sceneID uint, _ string) (*jobs.AnimatedThumbnailsPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plans[sceneID], nil
}

func (f *fakeAnimated) CommitAnimatedThumbnails(uint, *workerproto.AnimatedThumbnailsResult, string) (int, bool, error) {
	f.mu.Lock()
	f.commits++
	f.mu.Unlock()
	return 1, false, nil
}

type fakeFingerprints struct {
	mu      sync.Mutex
	commits int
	failN   int // the next failN commits fail
}

func (f *fakeFingerprints) Commit(uint, *workerproto.FingerprintResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits++
	if f.failN > 0 {
		f.failN--
		return errors.New("database down")
	}
	return nil
}

type fakeIndex struct {
	mu      sync.Mutex
	removed int
}

func (f *fakeIndex) RemoveSceneFromIndex(uint) error {
	f.mu.Lock()
	f.removed++
	f.mu.Unlock()
	return nil
}

type fakeProcessing struct{ cfg config.ProcessingConfig }

func (f fakeProcessing) GetQualityConfig() processing.QualityConfig {
	return processing.QualityConfig{MaxFrameDimensionSm: 320, FrameQualitySprites: 75}
}
func (f fakeProcessing) GetConfig() config.ProcessingConfig { return f.cfg }

type fakeGate struct {
	mu      sync.Mutex
	blocked map[string]bool
}

func (g *fakeGate) RemoteClaimOptions(phase string) (bool, time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.blocked[phase], time.Time{}
}

// testEnv is a Service on the test database with fakes around it.
type testEnv struct {
	t        *testing.T
	db       *gorm.DB
	svc      *Service
	clock    *fakeClock
	scenes   *fakeScenes
	results  *fakeResults
	retry    *fakeRetry
	animated *fakeAnimated
	fp       *fakeFingerprints
	index    *fakeIndex
	gate     *fakeGate
	router   *Router
	dirs     config.ProcessingConfig
	phase    workerproto.Phase
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := openTestDB(t)
	root := t.TempDir()
	e := &testEnv{
		t: t, db: db,
		clock:    &fakeClock{t: time.Now().UTC().Truncate(time.Millisecond)},
		scenes:   &fakeScenes{scenes: map[uint]*data.Scene{}, sprites: map[uint]int{}},
		results:  &fakeResults{},
		retry:    &fakeRetry{max: 3},
		animated: &fakeAnimated{plans: map[uint]*jobs.AnimatedThumbnailsPlan{}},
		fp:       &fakeFingerprints{},
		index:    &fakeIndex{},
		gate:     &fakeGate{blocked: map[string]bool{}},
		dirs: config.ProcessingConfig{
			SpriteDir: filepath.Join(root, "sprites"), VttDir: filepath.Join(root, "vtt"),
			FrameInterval: 5, GridCols: 12, GridRows: 8,
		},
	}
	settings, err := NewSettings(config.WorkersConfig{Enabled: true, LocalPhases: []string{"sprites", "animated_thumbnails", "fingerprint"},
		AllowedCIDRs: []string{"127.0.0.0/8", "192.0.2.0/24"}}, filepath.Join(root, "staging"))
	if err != nil {
		t.Fatal(err)
	}
	settings.ClaimPollInterval = 20 * time.Millisecond
	e.router = NewRouter(true, settings.LocalPhases, nil)
	e.svc = e.newService(settings)
	return e
}

// newService builds a Service on the same database and fakes (a server restart builds a new one).
func (e *testEnv) newService(settings Settings) *Service {
	return NewService(Deps{
		Settings:           settings,
		Workers:            data.NewWorkerRepository(e.db),
		Leases:             data.NewJobLeaseRepository(e.db),
		Scenes:             e.scenes,
		Router:             e.router,
		Staging:            NewStaging(settings.StagingRoot, settings.MaxArtifactBytes),
		Processing:         fakeProcessing{cfg: e.dirs},
		Animated:           e.animated,
		Fingerprints:       e.fp,
		FingerprintIndex:   e.index,
		DuplicationEnabled: true,
		Results:            e.results,
		Retry:              e.retry,
		Gate:               e.gate,
		MediaSecret:        []byte("test-media-secret"),
		Now:                e.clock.Now,
	})
}

// worker creates a worker (with its token) and registers it.
func (e *testEnv) worker(caps workerproto.Capabilities) (*data.Worker, string) {
	e.t.Helper()
	name := "w-" + uuid.NewString()[:6]
	created, err := e.svc.CreateWorker(WorkerInput{Name: &name})
	if err != nil {
		e.t.Fatalf("create worker: %v", err)
	}
	id := created.Worker.ID
	e.t.Cleanup(func() {
		e.db.Model(&data.JobHistory{}).Where("worker_id = ?", id).Update("worker_id", nil)
		e.db.Delete(&data.Worker{}, id)
	})
	w := e.auth(created.Token)
	if caps.Phases == nil {
		caps = defaultCaps()
	}
	resp, err := e.svc.Register(w, workerproto.RegisterRequest{APIVersion: 1, Name: name, Capabilities: caps}, "http://127.0.0.1:8080")
	if err != nil {
		e.t.Fatalf("register: %v", err)
	}
	if resp.SessionID == "" {
		e.t.Fatal("no session")
	}
	return e.auth(created.Token), created.Token
}

func defaultCaps() workerproto.Capabilities {
	return workerproto.Capabilities{
		Phases:           workerproto.RemotePhases,
		Slots:            map[workerproto.Phase]int{"sprites": 4, "animated_thumbnails": 4, "fingerprint": 4},
		InputModes:       []workerproto.InputMode{workerproto.InputDirect, workerproto.InputIndirect},
		AudioFingerprint: true,
	}
}

// auth reloads a worker as the middleware does for each request.
func (e *testEnv) auth(token string) *data.Worker {
	e.t.Helper()
	w, err := e.svc.Authenticate(token)
	if err != nil {
		e.t.Fatalf("authenticate: %v", err)
	}
	return w
}

// scene adds a scene to the fake store.
func (e *testEnv) scene(audio string) *data.Scene {
	id := uint(rand.IntN(1_000_000_000) + 1_000_000_000)
	path := filepath.Join(e.t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("source video bytes"), 0644); err != nil {
		e.t.Fatal(err)
	}
	s := &data.Scene{ID: id, Title: "scene", StoredPath: path, Duration: 120, Width: 1920, Height: 1080, AudioCodec: audio, Size: 18}
	e.scenes.add(s)
	return s
}

// pending creates a pending job of a phase for a scene. Each test uses its own phase rows
// through unique scenes; only the jobs of the test's scenes are ever claimable thanks to
// the high priority it gives them.
func (e *testEnv) pending(s *data.Scene, phase workerproto.Phase) *data.JobHistory {
	e.t.Helper()
	now := e.clock.Now()
	row := &data.JobHistory{JobID: uuid.NewString(), SceneID: s.ID, Phase: string(phase), Status: data.JobStatusPending,
		CreatedAt: now, StartedAt: now, IsRetryable: true, Priority: int(1_000_000_000 - jobCounter.Add(1))}
	if err := e.db.Create(row).Error; err != nil {
		e.t.Fatalf("create job: %v", err)
	}
	e.t.Cleanup(func() { e.db.Delete(&data.JobHistory{}, row.ID) })
	return row
}

func (e *testEnv) job(jobID string) data.JobHistory {
	e.t.Helper()
	var j data.JobHistory
	if err := e.db.Where("job_id = ?", jobID).First(&j).Error; err != nil {
		e.t.Fatalf("load job: %v", err)
	}
	return j
}

// claimJob claims until the given job is leased to w (other tests' pending rows of the same
// phase are released again).
func (e *testEnv) claimJob(w *data.Worker, j *data.JobHistory) workerproto.Assignment {
	e.t.Helper()
	for i := 0; i < 20; i++ {
		as, err := e.svc.ClaimFor(w, map[workerproto.Phase]int{workerproto.Phase(j.Phase): 3})
		if err != nil {
			e.t.Fatalf("claim: %v", err)
		}
		var found *workerproto.Assignment
		for k := range as {
			if as[k].JobID == j.JobID {
				found = &as[k]
			} else {
				e.svc.Leases.ReleaseLease(as[k].JobID, as[k].Lease.Generation, w.ID, false)
			}
		}
		if found != nil {
			return *found
		}
	}
	e.t.Fatalf("job %s never claimed", j.JobID)
	return workerproto.Assignment{}
}

func (e *testEnv) session(w *data.Worker) string { return w.SessionID }

func protoCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	return AsError(err).Code
}

// spritesResult writes the artifacts of a sprites result in the staging and returns the
// complete request.
func (e *testEnv) uploadSprites(a workerproto.Assignment) workerproto.CompleteRequest {
	e.t.Helper()
	files := map[string][]byte{
		workerproto.SpriteSheetName(a.SceneID, 1): []byte("sheet one"),
		workerproto.VTTName(a.SceneID):            []byte("WEBVTT\n"),
	}
	req := workerproto.CompleteRequest{Generation: a.Lease.Generation, Phase: workerproto.PhaseSprites,
		Result: workerproto.PhaseResult{Sprites: &workerproto.SpritesResult{
			SpriteSheets: []string{workerproto.SpriteSheetName(a.SceneID, 1)}, VTT: workerproto.VTTName(a.SceneID),
			SheetCount: 1, FrameInterval: 5}}}
	for _, name := range []string{workerproto.SpriteSheetName(a.SceneID, 1), workerproto.VTTName(a.SceneID)} {
		body := files[name]
		sum, size, _ := workerproto.SHA256Reader(bytesReader(body))
		if _, err := e.svc.Staging.Put(a.JobID, a.Lease.Generation, name, 0, size, sum, bytesReader(body)); err != nil {
			e.t.Fatalf("upload %s: %v", name, err)
		}
		req.Artifacts = append(req.Artifacts, workerproto.ArtifactRef{Name: name, Size: size, SHA256: sum})
	}
	return req
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func uintPtr(v uint) *uint { return &v }

var _ = pq.StringArray{}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
