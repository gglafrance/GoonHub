// Package fakeserver is an in-memory GoonHub worker API (docs/REMOTE_WORKERS.md) for the
// worker's end-to-end tests: queue, leases with generations and expiry, signed media,
// resumable staged uploads, verified complete, release, fail, push dispatch, plus fault
// injection (network down, cut uploads, server restart, zombie generations).
package fakeserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goonhub/pkg/workerproto"
)

// Job states.
const (
	Pending   = "pending"
	Running   = "running"
	Completed = "completed"
	Failed    = "failed"
	Cancelled = "cancelled"
)

// Job is one job of the fake queue.
type Job struct {
	ID         string
	Phase      workerproto.Phase
	SceneID    uint
	Status     string
	Generation int64
	Worker     string // "" or the worker id holding the lease ("other" for a foreign one)
	ExpiresAt  time.Time
	RetryCount int
	Source     string // local file served by the media URL
	Video      workerproto.VideoInfo
	Params     workerproto.PhaseParams
	ServerPath string
	StorageID  *uint
	RelPath    string

	CommittedGeneration int64
	Committed           *workerproto.CompleteRequest
	CommittedFiles      map[string][]byte
	Releases            []workerproto.ReleaseReason
	Failure             *workerproto.FailRequest
	Completes           int // complete calls that committed
}

type upload struct {
	size     int64
	sha      string
	data     []byte
	complete bool
}

// Server is the fake GoonHub.
type Server struct {
	HTTP *httptest.Server

	Token        string
	WorkerID     string
	Secret       []byte
	HeartbeatSec int
	LeaseSec     int
	ClaimMaxWait int
	ChunkBytes   int64
	MaxArtifact  int64
	// MediaETag adds an ETag to media answers.
	MediaETag bool

	mu       sync.Mutex
	jobs     map[string]*Job
	order    []string
	seq      int
	session  string
	sessions int
	pushURL  string
	pushTok  string
	staging  map[string]map[string]*upload // "<job>/g<gen>" -> name -> upload
	down     bool
	cutAfter int64 // >0: the next artifact PUT is cut after this many body bytes
	failComp int   // next complete calls answering 503

	// Counters.
	MediaBytes    int64
	MediaRequests int
	MediaRanges   int
	UploadBytes   int64
	UploadPuts    int
	RegisterCalls int
	Resumes       []workerproto.JobLeaseRef
	ReleaseAlls   int
	Heartbeats    int

	stop chan struct{}
	wg   sync.WaitGroup
}

// New starts a fake server with short timings (heartbeat 1 s, lease 3 s).
func New() *Server {
	s := &Server{
		Token:        "ghw_1_testtoken",
		WorkerID:     "1",
		Secret:       []byte("media-secret"),
		HeartbeatSec: 1,
		LeaseSec:     3,
		ClaimMaxWait: 1,
		ChunkBytes:   64 << 10,
		MaxArtifact:  1 << 30,
		MediaETag:    true,
		jobs:         map[string]*Job{},
		staging:      map[string]map[string]*upload{},
		stop:         make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+workerproto.PathRegister, s.auth(s.handleRegister))
	mux.HandleFunc("POST "+workerproto.PathClaim, s.auth(s.handleClaim))
	mux.HandleFunc("POST "+workerproto.PathHeartbeat, s.auth(s.handleHeartbeat))
	mux.HandleFunc("POST "+workerproto.PathReleaseAll, s.auth(s.handleReleaseAll))
	mux.HandleFunc("POST "+workerproto.PathJobComplete, s.auth(s.handleComplete))
	mux.HandleFunc("POST "+workerproto.PathJobFail, s.auth(s.handleFail))
	mux.HandleFunc("POST "+workerproto.PathJobRelease, s.auth(s.handleRelease))
	mux.HandleFunc("POST "+workerproto.PathJobMediaURL, s.auth(s.handleMediaURL))
	mux.HandleFunc("PUT "+workerproto.PathJobArtifact, s.auth(s.handlePut))
	mux.HandleFunc("GET "+workerproto.PathJobArtifact, s.auth(s.handleUploadStatus))
	mux.HandleFunc("GET "+workerproto.PathMedia, s.handleMedia)
	mux.HandleFunc("HEAD "+workerproto.PathMedia, s.handleMedia)
	s.HTTP = httptest.NewServer(s.faults(mux))
	s.wg.Add(1)
	go s.sweep()
	return s
}

// URL is the server base URL.
func (s *Server) URL() string { return s.HTTP.URL }

// Close stops the server.
func (s *Server) Close() {
	close(s.stop)
	s.wg.Wait()
	s.HTTP.CloseClientConnections()
	s.HTTP.Close()
}

// ---------------------------------------------------------------------------------------
// Test controls
// ---------------------------------------------------------------------------------------

// AddJob queues a job; source is the local file the media URL serves.
func (s *Server) AddJob(phase workerproto.Phase, sceneID uint, source string, video workerproto.VideoInfo, params workerproto.PhaseParams) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := fmt.Sprintf("job-%d", s.seq)
	sid := uint(1)
	j := &Job{ID: id, Phase: phase, SceneID: sceneID, Status: Pending, Source: source, Video: video, Params: params,
		ServerPath: "/media/stash/" + filepath.Base(source), StorageID: &sid, RelPath: filepath.Base(source)}
	s.jobs[id] = j
	s.order = append(s.order, id)
	return id
}

// Job returns a copy of a job.
func (s *Server) Job(id string) Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := *s.jobs[id]
	j.Releases = append([]workerproto.ReleaseReason(nil), j.Releases...)
	return j
}

// WaitJob polls until cond holds for the job or the timeout passes.
func (s *Server) WaitJob(id string, timeout time.Duration, cond func(Job) bool) (Job, bool) {
	deadline := time.Now().Add(timeout)
	for {
		j := s.Job(id)
		if cond(j) {
			return j, true
		}
		if time.Now().After(deadline) {
			return j, false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// SetDown makes every API call fail at the connection level (network cut).
func (s *Server) SetDown(down bool) {
	s.mu.Lock()
	s.down = down
	s.mu.Unlock()
	if down {
		s.HTTP.CloseClientConnections()
	}
}

// CutNextUpload makes the next artifact PUT break after n body bytes (kept by the server).
func (s *Server) CutNextUpload(n int64) {
	s.mu.Lock()
	s.cutAfter = n
	s.mu.Unlock()
}

// FailNextCompletes answers 503 commit_failed to the next n complete calls.
func (s *Server) FailNextCompletes(n int) {
	s.mu.Lock()
	s.failComp = n
	s.mu.Unlock()
}

// Restart simulates a server restart that lost the leases and the sessions: running jobs
// go back to pending with a new generation.
func (s *Server) Restart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = ""
	for _, j := range s.jobs {
		if j.Status == Running {
			j.Status, j.Worker, j.Generation = Pending, "", j.Generation+1
		}
	}
}

// Steal gives a running job to another worker (new generation): the current holder is a
// zombie from now on.
func (s *Server) Steal(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	j.Generation++
	j.Worker = "other"
	j.ExpiresAt = time.Now().Add(time.Hour)
}

// Cancel cancels a job (user action).
func (s *Server) Cancel(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	j.Status = Cancelled
	j.Generation++
	j.Worker = ""
}

// Counters returns a snapshot of the counters.
func (s *Server) Counters() (mediaBytes int64, mediaRequests int, uploadBytes int64, puts int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.MediaBytes, s.MediaRequests, s.UploadBytes, s.UploadPuts
}

// Snapshot runs fn under the server lock.
func (s *Server) Snapshot(fn func(s *Server)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s)
}

// ---------------------------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------------------------

func writeJSON(rw http.ResponseWriter, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(v)
}

func writeErr(rw http.ResponseWriter, status int, code, msg string, details map[string]string) {
	writeJSON(rw, status, workerproto.ErrorResponse{Error: msg, Code: code, Details: details})
}

// faults applies the network cut.
func (s *Server) faults(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		down := s.down
		s.mu.Unlock()
		if down {
			hijackClose(rw)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

func hijackClose(rw http.ResponseWriter) {
	if hj, ok := rw.(http.Hijacker); ok {
		if c, _, err := hj.Hijack(); err == nil {
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.SetLinger(0)
			}
			_ = c.Close()
			return
		}
	}
	rw.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Header.Get(workerproto.HeaderAuthorization) != "Bearer "+s.Token {
			writeErr(rw, http.StatusUnauthorized, workerproto.CodeUnauthorized, "bad token", nil)
			return
		}
		if r.Header.Get(workerproto.HeaderAPIVersion) != "1" {
			writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, "missing api version", nil)
			return
		}
		h(rw, r)
	}
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *Server) leaseLocked(j *Job) {
	j.ExpiresAt = time.Now().Add(time.Duration(s.LeaseSec) * time.Second)
}

func (s *Server) sweep() {
	defer s.wg.Done()
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
		}
		s.mu.Lock()
		now := time.Now()
		for _, j := range s.jobs {
			if j.Status == Running && j.Worker == s.WorkerID && now.After(j.ExpiresAt) {
				j.Status, j.Worker, j.Generation = Pending, "", j.Generation+1
				j.RetryCount++
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) assignmentLocked(j *Job) workerproto.Assignment {
	exp := time.Now().Add(time.Hour)
	return workerproto.Assignment{
		JobID:   j.ID,
		Phase:   j.Phase,
		SceneID: j.SceneID,
		Attempt: j.RetryCount,
		Lease:   workerproto.Lease{Generation: j.Generation, ExpiresAt: j.ExpiresAt.UTC(), DurationSec: s.LeaseSec},
		Input: workerproto.InputSpec{
			ServerPath:    j.ServerPath,
			StoragePathID: j.StorageID,
			RelativePath:  j.RelPath,
			Size:          fileSize(j.Source),
			FileName:      filepath.Base(j.Source),
			URL:           workerproto.MediaURL(s.HTTP.URL, s.Secret, j.ID, j.Generation, exp),
			URLExpiresAt:  exp.UTC(),
			Video:         j.Video,
		},
		Params:     j.Params,
		AssignedAt: time.Now().UTC(),
	}
}

func fileSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}

// guard checks a lease-guarded call; it writes the error and returns nil when refused.
func (s *Server) guardLocked(rw http.ResponseWriter, jobID string, gen int64) *Job {
	j := s.jobs[jobID]
	if j == nil {
		writeErr(rw, http.StatusNotFound, workerproto.CodeJobNotFound, "no such job", nil)
		return nil
	}
	if j.Status == Cancelled {
		writeErr(rw, http.StatusConflict, workerproto.CodeJobCancelled, "job cancelled", nil)
		return nil
	}
	if j.Status != Running || j.Generation != gen || j.Worker != s.WorkerID {
		writeErr(rw, http.StatusConflict, workerproto.CodeLeaseLost, "lease lost", nil)
		return nil
	}
	return j
}

// ---------------------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------------------

func (s *Server) handleRegister(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.RegisterRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	if req.APIVersion != workerproto.APIVersion {
		writeErr(rw, http.StatusUpgradeRequired, workerproto.CodeUnsupportedAPIVersion, "unsupported", nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RegisterCalls++
	s.sessions++
	s.session = fmt.Sprintf("sess-%d", s.sessions)
	s.pushURL, s.pushTok = req.PushURL, req.PushToken
	s.Resumes = append(s.Resumes, req.Resume...)
	resp := workerproto.RegisterResponse{
		WorkerID: s.WorkerID, SessionID: s.session, APIVersion: 1, ServerTime: time.Now().UTC(),
		HeartbeatIntervalSec: s.HeartbeatSec, LeaseDurationSec: s.LeaseSec, ClaimMaxWaitSec: s.ClaimMaxWait,
		MaxArtifactBytes: s.MaxArtifact, UploadChunkBytes: s.ChunkBytes,
	}
	resumed := map[string]bool{}
	for _, ref := range req.Resume {
		j := s.jobs[ref.JobID]
		st := workerproto.JobLeaseStatus{JobID: ref.JobID, Generation: ref.Generation, Status: workerproto.LeaseLost}
		switch {
		case j == nil:
		case j.Status == Completed && j.CommittedGeneration == ref.Generation:
			st.Status = workerproto.LeaseCommitted
		case j.Status == Cancelled:
			st.Status = workerproto.LeaseCancelled
		case j.Status == Running && j.Generation == ref.Generation && j.Worker == s.WorkerID:
			st.Status = workerproto.LeaseOK
			s.leaseLocked(j)
			exp := j.ExpiresAt.UTC()
			st.LeaseExpiresAt = &exp
			resumed[j.ID] = true
		}
		resp.Resumed = append(resp.Resumed, st)
	}
	// Every other lease of this worker is released (retry_count + 1).
	for _, j := range s.jobs {
		if j.Status == Running && j.Worker == s.WorkerID && !resumed[j.ID] {
			j.Status, j.Worker, j.Generation = Pending, "", j.Generation+1
			j.RetryCount++
		}
	}
	writeJSON(rw, http.StatusOK, resp)
}

func (s *Server) claimLocked(slots map[workerproto.Phase]int) []workerproto.Assignment {
	out := []workerproto.Assignment{}
	for _, id := range s.order {
		j := s.jobs[id]
		if j.Status != Pending || slots[j.Phase] <= 0 {
			continue
		}
		slots[j.Phase]--
		j.Status, j.Worker = Running, s.WorkerID
		j.Generation++
		s.leaseLocked(j)
		out = append(out, s.assignmentLocked(j))
	}
	return out
}

func (s *Server) handleClaim(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.ClaimRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	wait := min(req.WaitSec, s.ClaimMaxWait)
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for {
		s.mu.Lock()
		if req.SessionID != s.session || s.session == "" {
			s.mu.Unlock()
			writeErr(rw, http.StatusConflict, workerproto.CodeUnknownSession, "unknown session", nil)
			return
		}
		slots := map[workerproto.Phase]int{}
		for p, n := range req.Slots {
			slots[p] = n
		}
		got := s.claimLocked(slots)
		s.mu.Unlock()
		if len(got) > 0 || time.Now().After(deadline) {
			writeJSON(rw, http.StatusOK, workerproto.ClaimResponse{Assignments: got})
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Millisecond):
		}
	}
}

func (s *Server) handleHeartbeat(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.HeartbeatRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.SessionID != s.session || s.session == "" {
		writeErr(rw, http.StatusConflict, workerproto.CodeUnknownSession, "unknown session", nil)
		return
	}
	s.Heartbeats++
	resp := workerproto.HeartbeatResponse{ServerTime: time.Now().UTC(), Jobs: []workerproto.JobLeaseStatus{}}
	for _, hb := range req.Jobs {
		j := s.jobs[hb.JobID]
		st := workerproto.JobLeaseStatus{JobID: hb.JobID, Generation: hb.Generation, Status: workerproto.LeaseLost}
		switch {
		case j == nil:
		case j.Status == Completed && j.CommittedGeneration == hb.Generation:
			st.Status = workerproto.LeaseCommitted
		case j.Status == Cancelled:
			st.Status = workerproto.LeaseCancelled
		case j.Status == Running && j.Generation == hb.Generation && j.Worker == s.WorkerID:
			st.Status = workerproto.LeaseOK
			s.leaseLocked(j)
			exp := j.ExpiresAt.UTC()
			st.LeaseExpiresAt = &exp
		}
		resp.Jobs = append(resp.Jobs, st)
	}
	writeJSON(rw, http.StatusOK, resp)
}

func (s *Server) handleReleaseAll(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.ReleaseAllRequest
	_ = decode(r, &req)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ReleaseAlls++
	n := 0
	for _, j := range s.jobs {
		if j.Status == Running && j.Worker == s.WorkerID {
			j.Status, j.Worker, j.Generation = Pending, "", j.Generation+1
			j.Releases = append(j.Releases, req.Reason)
			n++
		}
	}
	writeJSON(rw, http.StatusOK, workerproto.ReleaseResponse{Released: n})
}

func (s *Server) handleRelease(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.ReleaseRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.guardLocked(rw, r.PathValue("job_id"), req.Generation)
	if j == nil {
		return
	}
	j.Status, j.Worker, j.Generation = Pending, "", j.Generation+1
	if req.Reason.CountsAsAttempt() {
		j.RetryCount++
	}
	j.Releases = append(j.Releases, req.Reason)
	writeJSON(rw, http.StatusOK, workerproto.ReleaseResponse{Released: 1})
}

func (s *Server) handleFail(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.FailRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.guardLocked(rw, r.PathValue("job_id"), req.Generation)
	if j == nil {
		return
	}
	j.Status, j.Worker, j.Generation = Failed, "", j.Generation+1
	j.RetryCount++
	j.Failure = &req
	writeJSON(rw, http.StatusOK, map[string]any{})
}

func (s *Server) handleMediaURL(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.MediaURLRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.guardLocked(rw, r.PathValue("job_id"), req.Generation)
	if j == nil {
		return
	}
	exp := time.Now().Add(time.Hour)
	writeJSON(rw, http.StatusOK, workerproto.MediaURLResponse{URL: workerproto.MediaURL(s.HTTP.URL, s.Secret, j.ID, j.Generation, exp), ExpiresAt: exp.UTC()})
}

// countingWriter counts the body bytes of media answers.
type countingWriter struct {
	http.ResponseWriter
	s *Server
}

func (c *countingWriter) Write(b []byte) (int, error) {
	n, err := c.ResponseWriter.Write(b)
	c.s.mu.Lock()
	c.s.MediaBytes += int64(n)
	c.s.mu.Unlock()
	return n, err
}

func (s *Server) handleMedia(rw http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	gen, err := workerproto.VerifyMedia(s.Secret, jobID, r.URL.Query(), time.Now())
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			writeErr(rw, http.StatusForbidden, workerproto.CodeURLExpired, "url expired", nil)
			return
		}
		writeErr(rw, http.StatusForbidden, "forbidden", err.Error(), nil)
		return
	}
	s.mu.Lock()
	j := s.jobs[jobID]
	if j == nil || j.Status != Running || j.Generation != gen {
		s.mu.Unlock()
		writeErr(rw, http.StatusConflict, workerproto.CodeLeaseLost, "lease lost", nil)
		return
	}
	src := j.Source
	if r.Method == http.MethodGet {
		s.MediaRequests++
		if r.Header.Get("Range") != "" {
			s.MediaRanges++
		}
	}
	etag := s.MediaETag
	s.mu.Unlock()
	f, err := os.Open(src)
	if err != nil {
		writeErr(rw, http.StatusNotFound, "not_found", err.Error(), nil)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	if etag {
		rw.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, st.ModTime().UnixNano(), st.Size()))
	}
	http.ServeContent(&countingWriter{ResponseWriter: rw, s: s}, r, filepath.Base(src), st.ModTime(), f)
}

func stagingKey(jobID string, gen int64) string { return jobID + "/g" + strconv.FormatInt(gen, 10) }

func uploadStatus(name string, u *upload) workerproto.UploadStatus {
	if u == nil {
		return workerproto.UploadStatus{Name: name}
	}
	return workerproto.UploadStatus{Name: name, Size: u.size, Received: int64(len(u.data)), SHA256: u.sha, Complete: u.complete}
}

func (s *Server) handleUploadStatus(rw http.ResponseWriter, r *http.Request) {
	gen, _ := strconv.ParseInt(r.URL.Query().Get("generation"), 10, 64)
	name := r.PathValue("name")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.guardLocked(rw, r.PathValue("job_id"), gen) == nil {
		return
	}
	writeJSON(rw, http.StatusOK, uploadStatus(name, s.staging[stagingKey(r.PathValue("job_id"), gen)][name]))
}

func (s *Server) handlePut(rw http.ResponseWriter, r *http.Request) {
	jobID, name := r.PathValue("job_id"), r.PathValue("name")
	if err := workerproto.ValidateArtifactName(name); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidArtifactName, err.Error(), nil)
		return
	}
	gen, _ := strconv.ParseInt(r.Header.Get(workerproto.HeaderLeaseGeneration), 10, 64)
	offset, err1 := strconv.ParseInt(r.Header.Get(workerproto.HeaderUploadOffset), 10, 64)
	size, err2 := strconv.ParseInt(r.Header.Get(workerproto.HeaderArtifactSize), 10, 64)
	sha := r.Header.Get(workerproto.HeaderArtifactSHA256)
	if err1 != nil || err2 != nil || !workerproto.ValidSHA256(sha) {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, "bad upload headers", nil)
		return
	}
	s.mu.Lock()
	if s.guardLocked(rw, jobID, gen) == nil {
		s.mu.Unlock()
		return
	}
	if size > s.MaxArtifact {
		s.mu.Unlock()
		writeErr(rw, http.StatusRequestEntityTooLarge, workerproto.CodeArtifactTooLarge, "too large", nil)
		return
	}
	key := stagingKey(jobID, gen)
	if s.staging[key] == nil {
		s.staging[key] = map[string]*upload{}
	}
	u := s.staging[key][name]
	if offset == 0 && (u == nil || !(u.complete && u.size == size && u.sha == sha)) {
		u = &upload{size: size, sha: sha}
		s.staging[key][name] = u
	}
	if u == nil || u.size != size || u.sha != sha {
		st := uploadStatus(name, u)
		s.mu.Unlock()
		writeErr(rw, http.StatusConflict, workerproto.CodeOffsetMismatch, "declaration differs", map[string]string{"received": strconv.FormatInt(st.Received, 10)})
		return
	}
	if u.complete {
		st := uploadStatus(name, u)
		s.mu.Unlock()
		writeJSON(rw, http.StatusOK, st)
		return
	}
	if offset != int64(len(u.data)) {
		st := uploadStatus(name, u)
		s.mu.Unlock()
		writeErr(rw, http.StatusConflict, workerproto.CodeOffsetMismatch, "offset mismatch", map[string]string{"received": strconv.FormatInt(st.Received, 10)})
		return
	}
	cut := s.cutAfter
	s.cutAfter = 0
	s.UploadPuts++
	s.mu.Unlock()

	// Read the body outside the lock, keeping every byte received (even on a cut).
	var body io.Reader = io.LimitReader(r.Body, size-offset)
	if cut > 0 {
		body = io.LimitReader(body, cut)
	}
	buf, rerr := io.ReadAll(body)
	s.mu.Lock()
	// The lease may have changed while reading.
	if cur := s.jobs[jobID]; cur == nil || cur.Generation != gen || cur.Status != Running {
		s.mu.Unlock()
		writeErr(rw, http.StatusConflict, workerproto.CodeLeaseLost, "lease lost", nil)
		return
	}
	u.data = append(u.data, buf...)
	s.UploadBytes += int64(len(buf))
	if cut > 0 {
		s.mu.Unlock()
		hijackClose(rw)
		return
	}
	if rerr != nil {
		s.mu.Unlock()
		return
	}
	if int64(len(u.data)) == u.size {
		sum := sha256.Sum256(u.data)
		if hex.EncodeToString(sum[:]) != u.sha {
			delete(s.staging[key], name)
			s.mu.Unlock()
			writeErr(rw, http.StatusUnprocessableEntity, workerproto.CodeSHA256Mismatch, "sha256 mismatch", nil)
			return
		}
		u.complete = true
	}
	st := uploadStatus(name, u)
	s.mu.Unlock()
	writeJSON(rw, http.StatusOK, st)
}

func (s *Server) handleComplete(rw http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	var req workerproto.CompleteRequest
	if err := decode(r, &req); err != nil {
		writeErr(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error(), nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[jobID]; j != nil && j.Status == Completed && j.CommittedGeneration == req.Generation {
		writeJSON(rw, http.StatusOK, workerproto.CompleteResponse{Status: workerproto.CompleteAlreadyCommitted})
		return
	}
	j := s.guardLocked(rw, jobID, req.Generation)
	if j == nil {
		return
	}
	if s.failComp > 0 {
		s.failComp--
		writeErr(rw, http.StatusServiceUnavailable, workerproto.CodeCommitFailed, "db down", nil)
		return
	}
	if err := req.Result.Validate(j.Phase); err != nil {
		writeErr(rw, http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, err.Error(), nil)
		return
	}
	if err := workerproto.ValidateArtifacts(req.Artifacts, req.Result); err != nil {
		writeErr(rw, http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, err.Error(), nil)
		return
	}
	var verr error
	switch j.Phase {
	case workerproto.PhaseSprites:
		verr = req.Result.Sprites.Validate(j.SceneID)
	case workerproto.PhaseAnimatedThumbnails:
		verr = req.Result.AnimatedThumbnails.Validate(j.Params.AnimatedThumbnails)
	case workerproto.PhaseFingerprint:
		verr = req.Result.Fingerprint.Validate()
	}
	if verr != nil {
		writeErr(rw, http.StatusUnprocessableEntity, workerproto.CodeInvalidResult, verr.Error(), nil)
		return
	}
	staged := s.staging[stagingKey(jobID, req.Generation)]
	missing := map[string]string{}
	for _, ref := range req.Artifacts {
		u := staged[ref.Name]
		switch {
		case u == nil:
			missing[ref.Name] = "missing"
		case !u.complete:
			missing[ref.Name] = "partial"
		case u.sha != ref.SHA256 || u.size != ref.Size:
			missing[ref.Name] = "sha256"
		}
	}
	if len(missing) > 0 {
		writeErr(rw, http.StatusUnprocessableEntity, workerproto.CodeArtifactMissing, "artifacts missing", missing)
		return
	}
	j.Status = Completed
	j.CommittedGeneration = req.Generation
	j.Committed = &req
	j.Completes++
	j.CommittedFiles = map[string][]byte{}
	for _, ref := range req.Artifacts {
		j.CommittedFiles[ref.Name] = bytes.Clone(staged[ref.Name].data)
	}
	delete(s.staging, stagingKey(jobID, req.Generation))
	writeJSON(rw, http.StatusOK, workerproto.CompleteResponse{Status: workerproto.CompleteCommitted})
}

// ---------------------------------------------------------------------------------------
// Push dispatcher
// ---------------------------------------------------------------------------------------

// PushDispatch does one dispatcher pass for the registered push worker: GET /v1/status,
// claim jobs for its free slots, POST /v1/assign each (releasing on failure). It returns
// the assigned job ids.
func (s *Server) PushDispatch(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	base, tok, sess := s.pushURL, s.pushTok, s.session
	s.mu.Unlock()
	if base == "" || sess == "" {
		return nil, errors.New("no push worker registered")
	}
	var st workerproto.PushStatus
	if err := pushCall(ctx, http.MethodGet, base+workerproto.PathPushStatus, tok, nil, &st); err != nil {
		return nil, err
	}
	if !st.Accepting {
		return nil, nil
	}
	s.mu.Lock()
	slots := map[workerproto.Phase]int{}
	for p, n := range st.FreeSlots {
		slots[p] = n
	}
	assignments := s.claimLocked(slots)
	s.mu.Unlock()
	var ids []string
	for _, a := range assignments {
		err := pushCall(ctx, http.MethodPost, base+workerproto.PathPushAssign, tok, workerproto.PushAssignRequest{Assignment: a}, nil)
		if err != nil {
			s.mu.Lock()
			if j := s.jobs[a.JobID]; j.Generation == a.Lease.Generation && j.Status == Running {
				j.Status, j.Worker, j.Generation = Pending, "", j.Generation+1
				j.Releases = append(j.Releases, workerproto.ReleaseOverloaded)
			}
			s.mu.Unlock()
			continue
		}
		ids = append(ids, a.JobID)
	}
	sort.Strings(ids)
	return ids, nil
}

// PushCancel sends POST /v1/cancel to the push worker.
func (s *Server) PushCancel(ctx context.Context, jobID string, gen int64) error {
	s.mu.Lock()
	base, tok := s.pushURL, s.pushTok
	s.mu.Unlock()
	return pushCall(ctx, http.MethodPost, base+workerproto.PathPushCancel, tok, workerproto.PushCancelRequest{JobID: jobID, Generation: gen}, nil)
}

// PushToken returns the push token the worker registered.
func (s *Server) PushToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pushTok
}

func pushCall(ctx context.Context, method, url, tok string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set(workerproto.HeaderAuthorization, "Bearer "+tok)
	req.Header.Set(workerproto.HeaderAPIVersion, "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push %s: %d %s", url, resp.StatusCode, b)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
