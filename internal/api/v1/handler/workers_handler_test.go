package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/persistence/migrator"
	"goonhub/internal/jobs"
	"goonhub/internal/workers"
	"goonhub/pkg/workerproto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Handler tests of the worker API and of its admin API (auth, gate, status codes, uploads,
// signed media with Range). They run on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).

type whScenes struct {
	mu     sync.Mutex
	scenes map[uint]*data.Scene
}

func (s *whScenes) GetByID(id uint) (*data.Scene, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc, ok := s.scenes[id]; ok {
		cp := *sc
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (s *whScenes) UpdateSprites(uint, string, string, int) error { return nil }

type whNoop struct{}

func (whNoop) HandleResult(jobs.JobResult) {}
func (whNoop) PrepareAnimatedThumbnails(uint, string) (*jobs.AnimatedThumbnailsPlan, error) {
	return nil, nil
}
func (whNoop) CommitAnimatedThumbnails(uint, *workerproto.AnimatedThumbnailsResult, string) (int, bool, error) {
	return 0, false, nil
}
func (whNoop) Commit(uint, *workerproto.FingerprintResult) error { return nil }

type whProcessing struct{ root string }

func (whProcessing) GetQualityConfig() processing.QualityConfig {
	return processing.QualityConfig{MaxFrameDimensionSm: 320, FrameQualitySprites: 75}
}
func (p whProcessing) GetConfig() config.ProcessingConfig {
	return config.ProcessingConfig{FrameInterval: 5, GridCols: 12, GridRows: 8,
		SpriteDir: filepath.Join(p.root, "sprites"), VttDir: filepath.Join(p.root, "vtt")}
}

type workerAPITest struct {
	t      *testing.T
	db     *gorm.DB
	svc    *workers.Service
	router *gin.Engine
	scenes *whScenes
}

func newWorkerAPITest(t *testing.T, enabled bool) *workerAPITest {
	t.Helper()
	dsn := os.Getenv("GOONHUB_TEST_IMPORT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database) to run the worker API tests")
	}
	if err := migrator.Run(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	root := t.TempDir()
	settings, err := workers.NewSettings(config.WorkersConfig{Enabled: enabled, AllowedCIDRs: []string{"192.0.2.0/24"},
		MaxArtifactBytes: 1 << 20, LocalPhases: []string{"sprites", "animated_thumbnails", "fingerprint"}}, filepath.Join(root, "staging"))
	if err != nil {
		t.Fatal(err)
	}
	scenes := &whScenes{scenes: map[uint]*data.Scene{}}
	svc := workers.NewService(workers.Deps{
		Settings: settings, Workers: data.NewWorkerRepository(db), Leases: data.NewJobLeaseRepository(db),
		Scenes: scenes, Router: workers.NewRouter(enabled, settings.LocalPhases, nil),
		Staging:    workers.NewStaging(settings.StagingRoot, settings.MaxArtifactBytes),
		Processing: whProcessing{root: root}, Animated: whNoop{}, Fingerprints: whNoop{}, Results: whNoop{},
		DuplicationEnabled: true, MediaSecret: []byte("secret"),
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewWorkerHandler(svc, nil)
	v1 := r.Group("/api/v1")
	v1.GET("/workers/media/:job_id", h.Gate(), h.Media)
	v1.HEAD("/workers/media/:job_id", h.Gate(), h.Media)
	wg := v1.Group("/workers", h.Auth())
	wg.POST("/register", h.Register)
	wg.POST("/claim", h.Claim)
	wg.POST("/heartbeat", h.Heartbeat)
	wg.POST("/release-all", h.ReleaseAll)
	wg.POST("/jobs/:job_id/complete", h.Complete)
	wg.POST("/jobs/:job_id/fail", h.Fail)
	wg.POST("/jobs/:job_id/release", h.Release)
	wg.POST("/jobs/:job_id/media-url", h.MediaURL)
	wg.PUT("/jobs/:job_id/artifacts/:name", h.UploadArtifact)
	wg.GET("/jobs/:job_id/artifacts/:name", h.ArtifactStatus)
	admin := v1.Group("/admin")
	admin.GET("/workers", h.AdminList)
	admin.POST("/workers", h.AdminCreate)
	admin.PATCH("/workers/:id", h.AdminUpdate)
	admin.DELETE("/workers/:id", h.AdminDelete)
	admin.POST("/workers/:id/token", h.AdminRotateToken)
	admin.GET("/worker-routing", h.AdminGetRouting)
	admin.PUT("/worker-routing", h.AdminUpdateRouting)
	return &workerAPITest{t: t, db: db, svc: svc, router: r, scenes: scenes}
}

type apiResp struct {
	code int
	body []byte
	hdr  http.Header
}

func (a apiResp) errCode() string {
	var e workerproto.ErrorResponse
	json.Unmarshal(a.body, &e)
	return e.Code
}

func (x *workerAPITest) do(method, path, token string, body any, headers map[string]string) apiResp {
	x.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		reader = bytes.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = "192.0.2.10:5555"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	x.router.ServeHTTP(rec, req)
	return apiResp{code: rec.Code, body: rec.Body.Bytes(), hdr: rec.Header()}
}

// createWorker creates a worker through the admin API and returns its id and token.
func (x *workerAPITest) createWorker() (uint, string) {
	x.t.Helper()
	r := x.do("POST", "/api/v1/admin/workers", "", map[string]any{"name": "api-" + uuid.NewString()[:6], "max_concurrency": 2}, nil)
	if r.code != http.StatusCreated {
		x.t.Fatalf("create worker: %d %s", r.code, r.body)
	}
	var created workers.CreatedWorker
	if err := json.Unmarshal(r.body, &created); err != nil || created.Token == "" {
		x.t.Fatalf("created %s", r.body)
	}
	id := created.Worker.ID
	x.t.Cleanup(func() {
		x.db.Model(&data.JobHistory{}).Where("worker_id = ?", id).Update("worker_id", nil)
		x.db.Delete(&data.Worker{}, id)
	})
	return id, created.Token
}

func (x *workerAPITest) register(token string) string {
	x.t.Helper()
	r := x.do("POST", "/api/v1/workers/register", token, workerproto.RegisterRequest{APIVersion: 1, Name: "x",
		Capabilities: workerproto.Capabilities{Phases: workerproto.RemotePhases,
			Slots: map[workerproto.Phase]int{"sprites": 2}}}, map[string]string{"X-Goonhub-Worker-Api": "1"})
	if r.code != http.StatusOK {
		x.t.Fatalf("register: %d %s", r.code, r.body)
	}
	var resp workerproto.RegisterResponse
	json.Unmarshal(r.body, &resp)
	return resp.SessionID
}

// leasedJob creates a scene and a pending sprites job and claims it through the API.
func (x *workerAPITest) leasedJob(token, session string) workerproto.Assignment {
	x.t.Helper()
	id := uint(rand.IntN(1_000_000_000) + 1_000_000_000)
	path := filepath.Join(x.t.TempDir(), "video.mp4")
	os.WriteFile(path, []byte("0123456789abcdefghij"), 0644)
	x.scenes.mu.Lock()
	x.scenes.scenes[id] = &data.Scene{ID: id, StoredPath: path, Duration: 100, Width: 640, Height: 360}
	x.scenes.mu.Unlock()
	now := time.Now()
	row := &data.JobHistory{JobID: uuid.NewString(), SceneID: id, Phase: "sprites", Status: data.JobStatusPending,
		CreatedAt: now, StartedAt: now, IsRetryable: true, Priority: 2_000_000_000}
	if err := x.db.Create(row).Error; err != nil {
		x.t.Fatal(err)
	}
	x.t.Cleanup(func() { x.db.Delete(&data.JobHistory{}, row.ID) })
	r := x.do("POST", "/api/v1/workers/claim", token, workerproto.ClaimRequest{SessionID: session,
		Slots: map[workerproto.Phase]int{"sprites": 1}}, nil)
	if r.code != http.StatusOK {
		x.t.Fatalf("claim: %d %s", r.code, r.body)
	}
	var resp workerproto.ClaimResponse
	json.Unmarshal(r.body, &resp)
	for _, a := range resp.Assignments {
		if a.JobID == row.JobID {
			return a
		}
	}
	x.t.Fatalf("job not claimed: %s", r.body)
	return workerproto.Assignment{}
}

func TestWorkerAPIGateAndAuth(t *testing.T) {
	x := newWorkerAPITest(t, true)
	id, token := x.createWorker()

	cases := []struct {
		name  string
		token string
		hdr   map[string]string
		addr  string
		code  int
		ecode string
	}{
		{"no token", "", nil, "", http.StatusUnauthorized, workerproto.CodeUnauthorized},
		{"garbage token", "nope", nil, "", http.StatusUnauthorized, workerproto.CodeUnauthorized},
		{"wrong secret", fmt.Sprintf("ghw_%d_wrong", id), nil, "", http.StatusUnauthorized, workerproto.CodeUnauthorized},
		{"unsupported version header", token, map[string]string{"X-Goonhub-Worker-Api": "9"}, "", http.StatusUpgradeRequired, workerproto.CodeUnsupportedAPIVersion},
		{"address outside the allowed CIDRs", token, nil, "203.0.113.5:1", http.StatusForbidden, "address_not_allowed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/workers/heartbeat", strings.NewReader(`{}`))
			req.RemoteAddr = "192.0.2.10:1"
			if c.addr != "" {
				req.RemoteAddr = c.addr
			}
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			for k, v := range c.hdr {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			x.router.ServeHTTP(rec, req)
			r := apiResp{code: rec.Code, body: rec.Body.Bytes()}
			if r.code != c.code || r.errCode() != c.ecode {
				t.Fatalf("got %d %s, want %d %s", r.code, r.body, c.code, c.ecode)
			}
		})
	}

	// Valid token, no session yet: unknown_session.
	if r := x.do("POST", "/api/v1/workers/heartbeat", token, workerproto.HeartbeatRequest{SessionID: "x"}, nil); r.code != http.StatusConflict || r.errCode() != workerproto.CodeUnknownSession {
		t.Fatalf("heartbeat before register: %d %s", r.code, r.body)
	}
	if r := x.do("POST", "/api/v1/workers/register", token, workerproto.RegisterRequest{APIVersion: 2}, nil); r.code != http.StatusUpgradeRequired {
		t.Fatalf("register v2: %d", r.code)
	}
	if r := x.do("POST", "/api/v1/workers/register", token, []byte("{not json"), nil); r.code != http.StatusBadRequest || r.errCode() != workerproto.CodeInvalidRequest {
		t.Fatalf("bad json: %d %s", r.code, r.body)
	}
	session := x.register(token)
	if r := x.do("POST", "/api/v1/workers/heartbeat", token, workerproto.HeartbeatRequest{SessionID: session}, nil); r.code != http.StatusOK {
		t.Fatalf("heartbeat: %d %s", r.code, r.body)
	}

	// Disabled worker: 403 worker_disabled. Revoked: 401.
	if r := x.do("PATCH", fmt.Sprintf("/api/v1/admin/workers/%d", id), "", map[string]any{"enabled": false}, nil); r.code != http.StatusOK {
		t.Fatalf("disable: %d %s", r.code, r.body)
	}
	if r := x.do("POST", "/api/v1/workers/heartbeat", token, workerproto.HeartbeatRequest{SessionID: session}, nil); r.code != http.StatusForbidden || r.errCode() != workerproto.CodeWorkerDisabled {
		t.Fatalf("disabled worker: %d %s", r.code, r.body)
	}
	x.do("PATCH", fmt.Sprintf("/api/v1/admin/workers/%d", id), "", map[string]any{"enabled": true, "revoke_token": true}, nil)
	if r := x.do("POST", "/api/v1/workers/heartbeat", token, workerproto.HeartbeatRequest{SessionID: session}, nil); r.code != http.StatusUnauthorized {
		t.Fatalf("revoked worker: %d %s", r.code, r.body)
	}
}

func TestWorkerAPIDisabled(t *testing.T) {
	x := newWorkerAPITest(t, false)
	_, token := x.createWorker()
	if r := x.do("POST", "/api/v1/workers/register", token, workerproto.RegisterRequest{APIVersion: 1}, nil); r.code != http.StatusNotFound {
		t.Fatalf("disabled worker API answered %d", r.code)
	}
	if r := x.do("GET", "/api/v1/workers/media/x?g=1&exp=1&sig=0", "", nil, nil); r.code != http.StatusNotFound {
		t.Fatalf("disabled media answered %d", r.code)
	}
}

func TestWorkerAPIUploadCompleteAndMedia(t *testing.T) {
	x := newWorkerAPITest(t, true)
	_, token := x.createWorker()
	session := x.register(token)
	a := x.leasedJob(token, session)
	gen := strconv.FormatInt(a.Lease.Generation, 10)
	vtt := workerproto.VTTName(a.SceneID)
	sheet := workerproto.SpriteSheetName(a.SceneID, 1)
	vttBody := []byte("WEBVTT\n\n")
	vttSum, _, _ := workerproto.SHA256Reader(bytes.NewReader(vttBody))
	put := func(name string, offset int, size int, sum string, body []byte, g string) apiResp {
		return x.do("PUT", workerproto.ArtifactPath(a.JobID, name), token, body, map[string]string{
			"X-Lease-Generation": g, "Upload-Offset": strconv.Itoa(offset), "X-Artifact-Size": strconv.Itoa(size),
			"X-Artifact-Sha256": sum, "Content-Type": "application/octet-stream"})
	}

	if r := put(vtt, 0, len(vttBody), vttSum, vttBody, "99"); r.code != http.StatusConflict || r.errCode() != workerproto.CodeLeaseLost {
		t.Fatalf("upload with a stale generation: %d %s", r.code, r.body)
	}
	if r := put("a.b%20c", 0, 1, vttSum, []byte("x"), gen); r.code != http.StatusBadRequest || r.errCode() != workerproto.CodeInvalidArtifactName {
		t.Fatalf("bad name: %d %s", r.code, r.body)
	}
	if r := put(vtt, 0, 2<<20, vttSum, []byte("x"), gen); r.code != http.StatusRequestEntityTooLarge || r.errCode() != workerproto.CodeArtifactTooLarge {
		t.Fatalf("too large: %d %s", r.code, r.body)
	}
	if r := x.do("PUT", workerproto.ArtifactPath(a.JobID, vtt), token, vttBody, nil); r.code != http.StatusBadRequest {
		t.Fatalf("missing headers: %d", r.code)
	}
	// Resumable upload: first half, status, offset mismatch, rest.
	if r := put(vtt, 0, len(vttBody), vttSum, vttBody[:3], gen); r.code != http.StatusOK {
		t.Fatalf("first chunk: %d %s", r.code, r.body)
	}
	r := x.do("GET", workerproto.ArtifactPath(a.JobID, vtt)+"?generation="+gen, token, nil, nil)
	var st workerproto.UploadStatus
	json.Unmarshal(r.body, &st)
	if r.code != http.StatusOK || st.Received != 3 || st.Complete {
		t.Fatalf("status: %d %s", r.code, r.body)
	}
	if r := put(vtt, 1, len(vttBody), vttSum, vttBody[1:], gen); r.code != http.StatusConflict || r.errCode() != workerproto.CodeOffsetMismatch {
		t.Fatalf("offset mismatch: %d %s", r.code, r.body)
	}
	if r := put(vtt, 3, len(vttBody), vttSum, vttBody[3:], gen); r.code != http.StatusOK {
		t.Fatalf("rest: %d %s", r.code, r.body)
	}

	result := workerproto.PhaseResult{Sprites: &workerproto.SpritesResult{SpriteSheets: []string{sheet}, VTT: vtt, SheetCount: 1, FrameInterval: 5}}
	sheetBody := []byte("webp")
	sheetSum, _, _ := workerproto.SHA256Reader(bytes.NewReader(sheetBody))
	complete := workerproto.CompleteRequest{Generation: a.Lease.Generation, Phase: workerproto.PhaseSprites, Result: result,
		Artifacts: []workerproto.ArtifactRef{{Name: sheet, Size: 4, SHA256: sheetSum}, {Name: vtt, Size: int64(len(vttBody)), SHA256: vttSum}}}
	r = x.do("POST", workerproto.JobPath(workerproto.PathJobComplete, a.JobID), token, complete, nil)
	var er workerproto.ErrorResponse
	json.Unmarshal(r.body, &er)
	if r.code != http.StatusUnprocessableEntity || er.Code != workerproto.CodeArtifactMissing || er.Details[sheet] != "missing" {
		t.Fatalf("complete without the sheet: %d %s", r.code, r.body)
	}

	// Signed media URL: full GET, Range, HEAD, tampered signature.
	r = x.do("GET", strings.TrimPrefix(a.Input.URL, "http://example.com"), "", nil, nil)
	if !strings.HasPrefix(a.Input.URL, "http://example.com/api/v1/workers/media/") {
		t.Fatalf("media URL %s", a.Input.URL)
	}
	if r.code != http.StatusOK || string(r.body) != "0123456789abcdefghij" || r.hdr.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("media GET: %d %q %v", r.code, r.body, r.hdr)
	}
	u, _ := url.Parse(a.Input.URL)
	r = x.do("GET", u.RequestURI(), "", nil, map[string]string{"Range": "bytes=10-14"})
	if r.code != http.StatusPartialContent || string(r.body) != "abcde" || r.hdr.Get("Content-Range") != "bytes 10-14/20" {
		t.Fatalf("media Range: %d %q %v", r.code, r.body, r.hdr)
	}
	r = x.do("GET", u.RequestURI(), "", nil, map[string]string{"Range": "bytes=50-60"})
	if r.code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("media bad range: %d", r.code)
	}
	r = x.do("HEAD", u.RequestURI(), "", nil, nil)
	if r.code != http.StatusOK || r.hdr.Get("Content-Length") != "20" || len(r.body) != 0 {
		t.Fatalf("media HEAD: %d %v", r.code, r.hdr)
	}
	q := u.Query()
	q.Set("sig", strings.Repeat("a", 64))
	if r := x.do("GET", u.Path+"?"+q.Encode(), "", nil, nil); r.code != http.StatusForbidden {
		t.Fatalf("tampered media URL: %d", r.code)
	}
	if r := x.do("POST", u.Path, "", nil, nil); r.code == http.StatusOK {
		t.Fatal("the media path accepts POST")
	}

	// Upload the sheet, complete, then the duplicate complete.
	if r := put(sheet, 0, 4, sheetSum, sheetBody, gen); r.code != http.StatusOK {
		t.Fatalf("sheet: %d %s", r.code, r.body)
	}
	r = x.do("POST", workerproto.JobPath(workerproto.PathJobComplete, a.JobID), token, complete, nil)
	if r.code != http.StatusOK {
		t.Fatalf("complete: %d %s", r.code, r.body)
	}
	r = x.do("POST", workerproto.JobPath(workerproto.PathJobComplete, a.JobID), token, complete, nil)
	var cr workerproto.CompleteResponse
	json.Unmarshal(r.body, &cr)
	if r.code != http.StatusOK || cr.Status != workerproto.CompleteAlreadyCommitted {
		t.Fatalf("second complete: %d %s", r.code, r.body)
	}
	// The media URL of a completed job is no longer valid.
	if r := x.do("GET", u.RequestURI(), "", nil, nil); r.code != http.StatusConflict {
		t.Fatalf("media of a completed job: %d", r.code)
	}
}

func TestWorkerAPIReleaseFailAndMediaURL(t *testing.T) {
	x := newWorkerAPITest(t, true)
	_, token := x.createWorker()
	session := x.register(token)
	a := x.leasedJob(token, session)
	if r := x.do("POST", workerproto.JobPath(workerproto.PathJobMediaURL, a.JobID), token, workerproto.MediaURLRequest{Generation: a.Lease.Generation}, nil); r.code != http.StatusOK {
		t.Fatalf("media-url: %d %s", r.code, r.body)
	}
	if r := x.do("POST", workerproto.JobPath(workerproto.PathJobRelease, a.JobID), token, workerproto.ReleaseRequest{Generation: a.Lease.Generation, Reason: "bogus"}, nil); r.code != http.StatusBadRequest {
		t.Fatalf("bad reason: %d", r.code)
	}
	if r := x.do("POST", workerproto.JobPath(workerproto.PathJobRelease, a.JobID), token, workerproto.ReleaseRequest{Generation: a.Lease.Generation, Reason: "shutdown"}, nil); r.code != http.StatusOK {
		t.Fatalf("release: %d %s", r.code, r.body)
	}
	if r := x.do("POST", workerproto.JobPath(workerproto.PathJobFail, a.JobID), token, workerproto.FailRequest{Generation: a.Lease.Generation, Error: "x"}, nil); r.code != http.StatusConflict || r.errCode() != workerproto.CodeLeaseLost {
		t.Fatalf("fail after release: %d %s", r.code, r.body)
	}
	if r := x.do("POST", workerproto.JobPath(workerproto.PathJobFail, "no-such-job"), token, workerproto.FailRequest{Generation: 1}, nil); r.code != http.StatusNotFound {
		t.Fatalf("fail of an unknown job: %d", r.code)
	}
	if r := x.do("POST", "/api/v1/workers/release-all", token, workerproto.ReleaseAllRequest{SessionID: session}, nil); r.code != http.StatusOK {
		t.Fatalf("release-all: %d", r.code)
	}
	big := bytes.Repeat([]byte(" "), 5<<20)
	if r := x.do("POST", "/api/v1/workers/heartbeat", token, big, nil); r.code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: %d", r.code)
	}
}

func TestWorkerAdminAPI(t *testing.T) {
	x := newWorkerAPITest(t, true)
	id, token := x.createWorker()
	if r := x.do("POST", "/api/v1/admin/workers", "", map[string]any{"name": ""}, nil); r.code != http.StatusBadRequest {
		t.Fatalf("empty name: %d", r.code)
	}
	r := x.do("GET", "/api/v1/admin/workers", "", nil, nil)
	if r.code != http.StatusOK || strings.Contains(string(r.body), "token_hash") || strings.Contains(string(r.body), token) {
		t.Fatalf("list: %d %s", r.code, r.body)
	}
	var list struct {
		Enabled bool                 `json:"enabled"`
		Workers []workers.WorkerView `json:"workers"`
	}
	json.Unmarshal(r.body, &list)
	found := false
	for _, w := range list.Workers {
		if w.ID == id {
			found = w.Status == workers.StatusNever && w.MaxConcurrency == 2
		}
	}
	if !list.Enabled || !found {
		t.Fatalf("list content %s", r.body)
	}
	if r := x.do("PATCH", fmt.Sprintf("/api/v1/admin/workers/%d", id), "", map[string]any{"allowed_phases": []string{"thumbnail"}}, nil); r.code != http.StatusBadRequest {
		t.Fatalf("bad phase: %d", r.code)
	}
	if r := x.do("PATCH", "/api/v1/admin/workers/abc", "", map[string]any{}, nil); r.code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", r.code)
	}
	if r := x.do("PATCH", "/api/v1/admin/workers/999999999", "", map[string]any{"name": "x"}, nil); r.code != http.StatusNotFound {
		t.Fatalf("missing worker: %d", r.code)
	}
	r = x.do("POST", fmt.Sprintf("/api/v1/admin/workers/%d/token", id), "", nil, nil)
	var rotated workers.CreatedWorker
	json.Unmarshal(r.body, &rotated)
	if r.code != http.StatusOK || rotated.Token == "" || rotated.Token == token {
		t.Fatalf("rotate: %d %s", r.code, r.body)
	}
	if r := x.do("POST", "/api/v1/workers/register", token, workerproto.RegisterRequest{APIVersion: 1}, nil); r.code != http.StatusUnauthorized {
		t.Fatalf("old token after rotation: %d", r.code)
	}
	x.register(rotated.Token)

	r = x.do("PUT", "/api/v1/admin/worker-routing", "", map[string]any{"routing": map[string]string{"sprites": "remote"}}, nil)
	var routing workers.RoutingView
	json.Unmarshal(r.body, &routing)
	if r.code != http.StatusOK || routing.Effective["sprites"] != workers.RouteRemote {
		t.Fatalf("routing: %d %s", r.code, r.body)
	}
	if r := x.do("PUT", "/api/v1/admin/worker-routing", "", map[string]any{"routing": map[string]string{"metadata": "remote"}}, nil); r.code != http.StatusBadRequest {
		t.Fatalf("metadata routing: %d", r.code)
	}
	if r := x.do("GET", "/api/v1/admin/worker-routing", "", nil, nil); r.code != http.StatusOK {
		t.Fatalf("get routing: %d", r.code)
	}
	if r := x.do("DELETE", fmt.Sprintf("/api/v1/admin/workers/%d", id), "", nil, nil); r.code != http.StatusOK {
		t.Fatalf("delete: %d", r.code)
	}
	if r := x.do("DELETE", fmt.Sprintf("/api/v1/admin/workers/%d", id), "", nil, nil); r.code != http.StatusNotFound {
		t.Fatalf("second delete: %d", r.code)
	}
}
