package worker

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"goonhub/pkg/workerproto"
)

// pushServer is the push-mode endpoint (section 8): the server assigns jobs to it; the
// execution path afterwards is the same as pull.
type pushServer struct {
	srv  *http.Server
	ln   net.Listener
	done chan struct{}
}

// startPush listens on push.listen (or the given listener).
func (w *Worker) startPush() (*pushServer, error) {
	ln := w.pushListener
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", w.cfg.Push.Listen)
		if err != nil {
			return nil, fmt.Errorf("push mode: cannot listen on %s: %w", w.cfg.Push.Listen, err)
		}
	}
	var allowed []netip.Prefix
	for _, c := range w.cfg.Push.AllowedCIDRs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("push.allowed_cidrs %q: %w", c, err)
		}
		allowed = append(allowed, p)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+workerproto.PathPushStatus, w.handlePushStatus)
	mux.HandleFunc("POST "+workerproto.PathPushAssign, w.handlePushAssign)
	mux.HandleFunc("POST "+workerproto.PathPushCancel, w.handlePushCancel)
	h := w.pushAuth(allowed, mux)
	ps := &pushServer{
		srv: &http.Server{
			Handler:           h,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			MaxHeaderBytes:    64 << 10,
		},
		ln:   ln,
		done: make(chan struct{}),
	}
	go func() {
		defer close(ps.done)
		if err := ps.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			w.log.Error("push endpoint stopped", "error", err)
		}
	}()
	w.log.Info("push endpoint listening", "listen", ln.Addr().String(), "url", w.cfg.Push.URL, "allowed_cidrs", w.cfg.Push.AllowedCIDRs)
	return ps, nil
}

func (ps *pushServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ps.srv.Shutdown(ctx)
	<-ps.done
}

func writeJSON(rw http.ResponseWriter, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(v)
}

func writeError(rw http.ResponseWriter, status int, code, msg string) {
	writeJSON(rw, status, workerproto.ErrorResponse{Error: msg, Code: code})
}

// pushAuth checks the source address, the API version and the push token.
func (w *Worker) pushAuth(allowed []netip.Prefix, next http.Handler) http.Handler {
	want := []byte("Bearer " + w.pushToken)
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		addr, err := netip.ParseAddr(host)
		if err != nil || !inPrefixes(addr.Unmap(), allowed) {
			w.log.Warn("push call refused: source address not in push.allowed_cidrs", "remote", r.RemoteAddr, "path", r.URL.Path)
			writeError(rw, http.StatusForbidden, workerproto.CodeAddressNotAllowed, "source address not allowed")
			return
		}
		if v := r.Header.Get(workerproto.HeaderAPIVersion); v != "" && v != strconv.Itoa(workerproto.APIVersion) {
			writeError(rw, http.StatusUpgradeRequired, workerproto.CodeUnsupportedAPIVersion, "unsupported worker API version "+v)
			return
		}
		got := []byte(r.Header.Get(workerproto.HeaderAuthorization))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			w.log.Warn("push call refused: wrong push token (another server, or a stale registration)", "remote", r.RemoteAddr, "path", r.URL.Path)
			writeError(rw, http.StatusUnauthorized, workerproto.CodeUnauthorized, "invalid push token")
			return
		}
		r.Body = http.MaxBytesReader(rw, r.Body, 8<<20)
		next.ServeHTTP(rw, r)
	})
}

func inPrefixes(a netip.Addr, ps []netip.Prefix) bool {
	for _, p := range ps {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func (w *Worker) handlePushStatus(rw http.ResponseWriter, _ *http.Request) {
	w.mu.Lock()
	st := workerproto.PushStatus{
		WorkerID:   w.sessionSnapshot().workerID,
		APIVersion: workerproto.APIVersion,
		Accepting:  w.acceptingLocked(),
		FreeSlots:  w.freeSlotsPerPhaseLocked(),
		Jobs:       make([]workerproto.PushJobStatus, 0, len(w.jobs)),
		Accel:      w.env.Accel.Label(),
	}
	for _, j := range w.jobs {
		hb := j.heartbeat()
		st.Jobs = append(st.Jobs, workerproto.PushJobStatus{JobID: j.id, Generation: j.gen, Phase: j.phase, Stage: hb.Stage, Progress: hb.Progress})
	}
	w.mu.Unlock()
	writeJSON(rw, http.StatusOK, st)
}

func (w *Worker) handlePushAssign(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.PushAssignRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, "invalid body: "+err.Error())
		return
	}
	err := w.accept(req.Assignment)
	var inv *errInvalidAssignment
	switch {
	case err == nil:
		writeJSON(rw, http.StatusAccepted, workerproto.PushAssignResponse{Accepted: true})
	case errors.As(err, &inv):
		w.log.Warn("push assignment refused", "job_id", req.Assignment.JobID, "error", err)
		writeError(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, err.Error())
	case errors.Is(err, errBusy), errors.Is(err, errStale):
		writeError(rw, http.StatusConflict, workerproto.CodeBusy, "no free slot for "+string(req.Assignment.Phase))
	default:
		w.log.Error("push assignment: cannot take it", "job_id", req.Assignment.JobID, "error", err)
		writeError(rw, http.StatusConflict, workerproto.CodeBusy, "cannot take the job: "+err.Error())
	}
}

func (w *Worker) handlePushCancel(rw http.ResponseWriter, r *http.Request) {
	var req workerproto.PushCancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.JobID) == "" {
		writeError(rw, http.StatusBadRequest, workerproto.CodeInvalidRequest, "invalid body")
		return
	}
	w.mu.Lock()
	j := w.jobs[req.JobID]
	w.mu.Unlock()
	if j != nil && (req.Generation == 0 || req.Generation >= j.gen) {
		j.log.Info("server cancelled the job (push cancel)")
		w.discard(j)
	}
	writeJSON(rw, http.StatusOK, map[string]any{})
}
