package workers

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/pkg/workerproto"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Worker statuses shown to the admin.
const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusDisabled = "disabled"
	StatusRevoked  = "revoked"
	StatusNever    = "never_connected"
)

// WorkerJobView is a job a worker holds.
type WorkerJobView struct {
	JobID          string     `json:"job_id"`
	Phase          string     `json:"phase"`
	SceneID        uint       `json:"scene_id"`
	SceneTitle     string     `json:"scene_title"`
	Progress       int        `json:"progress"`
	Stage          string     `json:"stage"`
	InputMode      string     `json:"input_mode"`
	Attempt        int        `json:"attempt"`
	Generation     int64      `json:"generation"`
	StartedAt      time.Time  `json:"started_at"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
}

// WorkerView is a worker as the admin API shows it.
type WorkerView struct {
	data.Worker
	Status       string                    `json:"status"`
	HasToken     bool                      `json:"has_token"`
	Capabilities workerproto.Capabilities  `json:"capabilities"`
	PathMappings []workerproto.PathMapping `json:"path_mappings"`
	Jobs         []WorkerJobView           `json:"jobs"`
}

// CreatedWorker is the answer of a creation or a token rotation: the token is shown once.
type CreatedWorker struct {
	Worker WorkerView `json:"worker"`
	Token  string     `json:"token"`
}

// WorkerInput are the admin-editable fields of a worker (nil = unchanged).
type WorkerInput struct {
	Name           *string  `json:"name"`
	Enabled        *bool    `json:"enabled"`
	Draining       *bool    `json:"draining"`
	ConnectionMode *string  `json:"connection_mode"`
	PushURL        *string  `json:"push_url"`
	AllowedPhases  []string `json:"allowed_phases"`
	MaxConcurrency *int     `json:"max_concurrency"`
	RevokeToken    bool     `json:"revoke_token"`
}

func (s *Service) status(w *data.Worker) string {
	switch {
	case w.TokenHash == nil:
		return StatusRevoked
	case !w.Enabled:
		return StatusDisabled
	case w.LastSeenAt == nil:
		return StatusNever
	case s.now().Sub(*w.LastSeenAt) <= s.cfg.OfflineAfter:
		return StatusOnline
	}
	return StatusOffline
}

func (s *Service) view(w *data.Worker, held []data.JobHistory) WorkerView {
	v := WorkerView{
		Worker:       *w,
		Status:       s.status(w),
		HasToken:     w.TokenHash != nil,
		Capabilities: workerCaps(w),
		PathMappings: workerMappings(w),
		Jobs:         []WorkerJobView{},
	}
	if v.PathMappings == nil {
		v.PathMappings = []workerproto.PathMapping{}
	}
	if v.AllowedPhases == nil {
		v.AllowedPhases = pq.StringArray{}
	}
	for _, j := range held {
		jv := WorkerJobView{
			JobID: j.JobID, Phase: j.Phase, SceneID: j.SceneID, SceneTitle: j.SceneTitle,
			Progress: j.Progress, Attempt: j.RetryCount, Generation: j.LeaseGeneration,
			StartedAt: j.StartedAt, LeaseExpiresAt: j.LeaseExpiresAt,
		}
		if j.WorkerStage != nil {
			jv.Stage = *j.WorkerStage
		}
		if j.InputMode != nil {
			jv.InputMode = *j.InputMode
		}
		v.Jobs = append(v.Jobs, jv)
	}
	return v
}

// ListWorkers returns every worker with its status and current jobs.
func (s *Service) ListWorkers() ([]WorkerView, error) {
	list, err := s.Workers.List()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list workers", err)
	}
	running, err := s.Leases.ListRemoteRunning(0)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list remote jobs", err)
	}
	byWorker := map[uint][]data.JobHistory{}
	for _, j := range running {
		if j.WorkerID != nil {
			byWorker[*j.WorkerID] = append(byWorker[*j.WorkerID], j)
		}
	}
	out := make([]WorkerView, 0, len(list))
	for i := range list {
		out = append(out, s.view(&list[i], byWorker[list[i].ID]))
	}
	return out, nil
}

// GetWorker returns one worker.
func (s *Service) GetWorker(id uint) (*WorkerView, error) {
	w, err := s.Workers.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("worker", id)
		}
		return nil, apperrors.NewInternalError("failed to load worker", err)
	}
	held, err := s.Leases.ListRemoteRunning(id)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list worker jobs", err)
	}
	v := s.view(w, held)
	return &v, nil
}

func validatePhases(phases []string) (pq.StringArray, error) {
	seen := map[string]bool{}
	out := pq.StringArray{}
	for _, p := range phases {
		if !workerproto.Phase(p).Valid() {
			return nil, apperrors.NewValidationErrorWithField("allowed_phases", fmt.Sprintf("phase %q cannot be processed remotely", p))
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func validatePushURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", apperrors.NewValidationErrorWithField("push_url", "push URL must be an http(s) URL")
	}
	return raw, nil
}

// CreateWorker creates a worker and returns its token (shown once).
func (s *Service) CreateWorker(in WorkerInput) (*CreatedWorker, error) {
	name := ""
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if name == "" || len(name) > 100 {
		return nil, apperrors.NewValidationErrorWithField("name", "name is required (100 characters at most)")
	}
	w := &data.Worker{Name: name, Enabled: true, ConnectionMode: string(workerproto.ConnPull),
		AllowedPhases: pq.StringArray{"animated_thumbnails", "fingerprint", "sprites"},
		Capabilities:  []byte("{}"), PathMappings: []byte("[]")}
	if in.ConnectionMode != nil {
		if !workerproto.ConnectionMode(*in.ConnectionMode).Valid() {
			return nil, apperrors.NewValidationErrorWithField("connection_mode", "connection mode must be pull or push")
		}
		w.ConnectionMode = *in.ConnectionMode
	}
	if in.PushURL != nil {
		u, err := validatePushURL(*in.PushURL)
		if err != nil {
			return nil, err
		}
		w.PushURL = u
	}
	if in.AllowedPhases != nil {
		phases, err := validatePhases(in.AllowedPhases)
		if err != nil {
			return nil, err
		}
		w.AllowedPhases = phases
	}
	if in.MaxConcurrency != nil {
		if *in.MaxConcurrency < 0 || *in.MaxConcurrency > 64 {
			return nil, apperrors.NewValidationErrorWithField("max_concurrency", "max concurrency must be between 0 (no limit) and 64")
		}
		w.MaxConcurrency = *in.MaxConcurrency
	}
	if err := s.Workers.Create(w); err != nil {
		return nil, apperrors.NewInternalError("failed to create worker", err)
	}
	token, hash, err := NewToken(w.ID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create worker token", err)
	}
	if err := s.Workers.Update(w.ID, map[string]any{"token_hash": hash}); err != nil {
		return nil, apperrors.NewInternalError("failed to store worker token", err)
	}
	v, err := s.GetWorker(w.ID)
	if err != nil {
		return nil, err
	}
	return &CreatedWorker{Worker: *v, Token: token}, nil
}

// UpdateWorker edits a worker. Disabling it or revoking its token releases its leases at once.
func (s *Service) UpdateWorker(id uint, in WorkerInput) (*WorkerView, error) {
	current, err := s.GetWorker(id)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > 100 {
			return nil, apperrors.NewValidationErrorWithField("name", "name is required (100 characters at most)")
		}
		fields["name"] = name
	}
	if in.Enabled != nil {
		fields["enabled"] = *in.Enabled
	}
	if in.Draining != nil {
		fields["draining"] = *in.Draining
	}
	if in.ConnectionMode != nil {
		if !workerproto.ConnectionMode(*in.ConnectionMode).Valid() {
			return nil, apperrors.NewValidationErrorWithField("connection_mode", "connection mode must be pull or push")
		}
		fields["connection_mode"] = *in.ConnectionMode
	}
	if in.PushURL != nil {
		u, err := validatePushURL(*in.PushURL)
		if err != nil {
			return nil, err
		}
		fields["push_url"] = u
	}
	if in.AllowedPhases != nil {
		phases, err := validatePhases(in.AllowedPhases)
		if err != nil {
			return nil, err
		}
		fields["allowed_phases"] = phases
	}
	if in.MaxConcurrency != nil {
		if *in.MaxConcurrency < 0 || *in.MaxConcurrency > 64 {
			return nil, apperrors.NewValidationErrorWithField("max_concurrency", "max concurrency must be between 0 (no limit) and 64")
		}
		fields["max_concurrency"] = *in.MaxConcurrency
	}
	if in.RevokeToken {
		fields["token_hash"] = nil
		fields["session_id"] = ""
	}
	if err := s.Workers.Update(id, fields); err != nil {
		return nil, apperrors.NewInternalError("failed to update worker", err)
	}
	disabled := in.Enabled != nil && !*in.Enabled && current.Enabled
	if disabled || in.RevokeToken {
		if _, err := s.ReleaseWorker(id); err != nil {
			return nil, apperrors.NewInternalError("failed to release the worker jobs", err)
		}
	}
	return s.GetWorker(id)
}

// RotateToken replaces the token of a worker (the old one stops working at once) and
// returns the new one. The worker must register again with it.
func (s *Service) RotateToken(id uint) (*CreatedWorker, error) {
	if _, err := s.GetWorker(id); err != nil {
		return nil, err
	}
	token, hash, err := NewToken(id)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create worker token", err)
	}
	if err := s.Workers.Update(id, map[string]any{"token_hash": hash, "session_id": ""}); err != nil {
		return nil, apperrors.NewInternalError("failed to store worker token", err)
	}
	if _, err := s.ReleaseWorker(id); err != nil {
		return nil, apperrors.NewInternalError("failed to release the worker jobs", err)
	}
	v, err := s.GetWorker(id)
	if err != nil {
		return nil, err
	}
	return &CreatedWorker{Worker: *v, Token: token}, nil
}

// DeleteWorker releases the jobs of a worker, then deletes it (its job history is kept).
func (s *Service) DeleteWorker(id uint) error {
	if _, err := s.GetWorker(id); err != nil {
		return err
	}
	if _, err := s.ReleaseWorker(id); err != nil {
		return apperrors.NewInternalError("failed to release the worker jobs", err)
	}
	if err := s.Workers.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete worker", err)
	}
	return nil
}
