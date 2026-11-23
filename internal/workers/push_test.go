package workers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"goonhub/internal/data"
	"goonhub/pkg/workerproto"
)

// fakePushWorker is an httptest push worker.
type fakePushWorker struct {
	mu        sync.Mutex
	token     string
	accepting bool
	free      map[workerproto.Phase]int
	assignOK  bool
	assigned  []workerproto.Assignment
	cancelled []workerproto.PushCancelRequest
	badAuth   int
}

func (f *fakePushWorker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get(workerproto.HeaderAuthorization) != "Bearer "+f.token || r.Header.Get(workerproto.HeaderAPIVersion) != "1" {
		f.badAuth++
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case workerproto.PathPushStatus:
		json.NewEncoder(w).Encode(workerproto.PushStatus{APIVersion: 1, Accepting: f.accepting, FreeSlots: f.free})
	case workerproto.PathPushAssign:
		var req workerproto.PushAssignRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !f.assignOK {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(workerproto.ErrorResponse{Error: "busy", Code: workerproto.CodeBusy})
			return
		}
		f.assigned = append(f.assigned, req.Assignment)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(workerproto.PushAssignResponse{Accepted: true})
	case workerproto.PathPushCancel:
		var req workerproto.PushCancelRequest
		json.NewDecoder(r.Body).Decode(&req)
		f.cancelled = append(f.cancelled, req)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (e *testEnv) pushWorker(fake *fakePushWorker, url string) *data.Worker {
	e.t.Helper()
	w, token := e.worker(workerproto.Capabilities{})
	if _, err := e.svc.Register(w, workerproto.RegisterRequest{APIVersion: 1, ConnectionMode: workerproto.ConnPush,
		PushURL: url, PushToken: fake.token, Capabilities: defaultCaps()}, "http://127.0.0.1:8080"); err != nil {
		e.t.Fatalf("register push worker: %v", err)
	}
	return e.auth(token)
}

func TestPushDispatcherAssignsToFreeSlots(t *testing.T) {
	e := newTestEnv(t)
	fake := &fakePushWorker{token: "push-secret", accepting: true, assignOK: true,
		free: map[workerproto.Phase]int{"sprites": 1}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	w := e.pushWorker(fake, srv.URL)
	j := e.pending(e.scene(""), workerproto.PhaseSprites)

	d := NewPushDispatcher(e.svc, srv.Client())
	for i := 0; i < 5 && e.job(j.JobID).Status == data.JobStatusPending; i++ {
		d.DispatchOnce(context.Background())
	}
	got := e.job(j.JobID)
	if got.Status != data.JobStatusRunning || got.WorkerID == nil || *got.WorkerID != w.ID {
		t.Fatalf("job %+v", got)
	}
	fake.mu.Lock()
	var mine *workerproto.Assignment
	for i := range fake.assigned {
		if fake.assigned[i].JobID == j.JobID {
			mine = &fake.assigned[i]
		}
	}
	fake.mu.Unlock()
	if mine == nil || mine.Lease.Generation != got.LeaseGeneration || mine.Params.Sprites == nil {
		t.Fatalf("assignment not posted: %+v", mine)
	}
	if fake.badAuth != 0 {
		t.Fatal("push call without the push token")
	}
	// After the push, the job runs exactly as in pull mode.
	hb, err := e.svc.Heartbeat(w, workerproto.HeartbeatRequest{SessionID: w.SessionID,
		Jobs: []workerproto.JobHeartbeat{{JobID: j.JobID, Generation: mine.Lease.Generation}}})
	if err != nil || hb.Jobs[0].Status != workerproto.LeaseOK {
		t.Fatalf("heartbeat %+v %v", hb, err)
	}
	if resp, err := e.svc.Complete(w, j.JobID, e.uploadSprites(*mine)); err != nil || resp.Status != workerproto.CompleteCommitted {
		t.Fatalf("complete %+v %v", resp, err)
	}
}

// Push assign lost / refused: the lease is released at once, no attempt counted.
func TestPushDispatcherReleasesRefusedAssignments(t *testing.T) {
	e := newTestEnv(t)
	fake := &fakePushWorker{token: "push-secret", accepting: true, assignOK: false,
		free: map[workerproto.Phase]int{"sprites": 2}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	e.pushWorker(fake, srv.URL)
	j := e.pending(e.scene(""), workerproto.PhaseSprites)

	d := NewPushDispatcher(e.svc, srv.Client())
	d.DispatchOnce(context.Background())
	got := e.job(j.JobID)
	if got.Status != data.JobStatusPending || got.WorkerID != nil || got.RetryCount != 0 {
		t.Fatalf("refused assignment kept: %+v", got)
	}
	if got.LeaseGeneration < 2 {
		t.Fatal("the refused generation was not fenced off")
	}
}

func TestPushDispatcherSkipsUnreachableAndNotAcceptingWorkers(t *testing.T) {
	e := newTestEnv(t)
	fake := &fakePushWorker{token: "push-secret", accepting: false, assignOK: true,
		free: map[workerproto.Phase]int{"sprites": 2}}
	srv := httptest.NewServer(fake)
	w := e.pushWorker(fake, srv.URL)
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	d := NewPushDispatcher(e.svc, srv.Client())
	d.svc.cfg.PushTimeout = 500 * time.Millisecond

	d.DispatchOnce(context.Background())
	if e.job(j.JobID).Status != data.JobStatusPending {
		t.Fatal("assigned to a worker that is not accepting")
	}
	srv.Close() // the worker stops answering
	d.DispatchOnce(context.Background())
	if e.job(j.JobID).Status != data.JobStatusPending {
		t.Fatal("assigned to an unreachable worker")
	}
	ww, _ := e.svc.Workers.GetByID(w.ID)
	if ww.LastError == "" {
		t.Fatal("unreachable push worker not recorded")
	}
}

func TestPushCancelReachesTheWorker(t *testing.T) {
	e := newTestEnv(t)
	fake := &fakePushWorker{token: "push-secret", accepting: true, assignOK: true,
		free: map[workerproto.Phase]int{"sprites": 1}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	e.pushWorker(fake, srv.URL)
	j := e.pending(e.scene(""), workerproto.PhaseSprites)
	d := NewPushDispatcher(e.svc, srv.Client())
	for i := 0; i < 5 && e.job(j.JobID).Status == data.JobStatusPending; i++ {
		d.DispatchOnce(context.Background())
	}
	gen := e.job(j.JobID).LeaseGeneration
	if ok, err := e.svc.CancelJob(j.JobID); err != nil || !ok {
		t.Fatalf("cancel %v %v", ok, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		fake.mu.Lock()
		n := len(fake.cancelled)
		var got workerproto.PushCancelRequest
		if n > 0 {
			got = fake.cancelled[0]
		}
		fake.mu.Unlock()
		if n > 0 {
			if got.JobID != j.JobID || got.Generation != gen {
				t.Fatalf("cancel %+v, want generation %d", got, gen)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the push worker was not told about the cancel")
}
