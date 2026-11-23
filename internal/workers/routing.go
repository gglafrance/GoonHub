package workers

import (
	"encoding/json"
	"fmt"
	"sync"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/pkg/workerproto"
)

// RoutingMode says who processes the jobs of a remotable phase.
type RoutingMode string

const (
	RouteLocal  RoutingMode = "local"  // the server's own pools only
	RouteRemote RoutingMode = "remote" // remote workers only (the local pool does not claim)
	RouteBoth   RoutingMode = "both"   // both compete on the same pending rows
)

func (m RoutingMode) valid() bool {
	return m == RouteLocal || m == RouteRemote || m == RouteBoth
}

// PhaseRouting is the routing of each remotable phase.
type PhaseRouting map[string]RoutingMode

// Router decides, per phase, whether the local pools and the remote workers may claim its
// jobs. Phases that are not remotable (metadata, thumbnail) are always local. With the worker
// API disabled every phase is local: the server behaves exactly as without remote workers.
//
// The routing of a phase is the admin override (worker_settings.phase_routing) when set, else
// the config: a phase in workers.local_phases is "both", any other is "remote".
type Router struct {
	enabled  bool
	defaults PhaseRouting
	repo     data.WorkerRepository

	mu        sync.RWMutex
	overrides PhaseRouting
}

// NewRouter creates a Router; call Load to read the admin overrides.
func NewRouter(enabled bool, localPhases []string, repo data.WorkerRepository) *Router {
	local := make(map[string]bool, len(localPhases))
	for _, p := range localPhases {
		local[p] = true
	}
	defaults := PhaseRouting{}
	for _, p := range workerproto.RemotePhases {
		if local[string(p)] {
			defaults[string(p)] = RouteBoth
		} else {
			defaults[string(p)] = RouteRemote
		}
	}
	return &Router{enabled: enabled, defaults: defaults, repo: repo, overrides: PhaseRouting{}}
}

// Load reads the admin overrides from the database.
func (r *Router) Load() error {
	if r.repo == nil {
		return nil
	}
	s, err := r.repo.GetSettings()
	if err != nil {
		return fmt.Errorf("failed to load worker settings: %w", err)
	}
	overrides := PhaseRouting{}
	if len(s.PhaseRouting) > 0 {
		if err := json.Unmarshal(s.PhaseRouting, &overrides); err != nil {
			return fmt.Errorf("invalid worker phase routing: %w", err)
		}
	}
	r.mu.Lock()
	r.overrides = sanitizeRouting(overrides)
	r.mu.Unlock()
	return nil
}

func sanitizeRouting(in PhaseRouting) PhaseRouting {
	out := PhaseRouting{}
	for phase, mode := range in {
		if workerproto.Phase(phase).Valid() && mode.valid() {
			out[phase] = mode
		}
	}
	return out
}

// Enabled reports whether the worker API is enabled.
func (r *Router) Enabled() bool { return r.enabled }

// Mode returns the routing of a phase.
func (r *Router) Mode(phase string) RoutingMode {
	if !workerproto.Phase(phase).Valid() || !r.enabled {
		return RouteLocal
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.overrides[phase]; ok {
		return m
	}
	return r.defaults[phase]
}

// LocalAllowed reports whether the local pool of a phase may claim its jobs.
func (r *Router) LocalAllowed(phase string) bool { return r.Mode(phase) != RouteRemote }

// RemoteAllowed reports whether remote workers may claim the jobs of a phase.
func (r *Router) RemoteAllowed(phase string) bool { return r.Mode(phase) != RouteLocal }

// RemoteOnlyPhases lists the phases only remote workers process.
func (r *Router) RemoteOnlyPhases() []string {
	var out []string
	for _, p := range workerproto.RemotePhases {
		if r.Mode(string(p)) == RouteRemote {
			out = append(out, string(p))
		}
	}
	return out
}

// RoutingView is the routing of every remotable phase, with where it comes from.
type RoutingView struct {
	Enabled   bool         `json:"enabled"`
	Effective PhaseRouting `json:"effective"`
	Defaults  PhaseRouting `json:"defaults"`
	Overrides PhaseRouting `json:"overrides"`
}

// View returns the current routing.
func (r *Router) View() RoutingView {
	v := RoutingView{Enabled: r.enabled, Effective: PhaseRouting{}, Defaults: PhaseRouting{}, Overrides: PhaseRouting{}}
	r.mu.RLock()
	for k, m := range r.overrides {
		v.Overrides[k] = m
	}
	r.mu.RUnlock()
	for _, p := range workerproto.RemotePhases {
		v.Defaults[string(p)] = r.defaults[string(p)]
		v.Effective[string(p)] = r.Mode(string(p))
	}
	return v
}

// Update replaces the admin overrides (an empty mode removes the override of a phase).
func (r *Router) Update(routing map[string]string) (RoutingView, error) {
	next := PhaseRouting{}
	for phase, mode := range routing {
		if !workerproto.Phase(phase).Valid() {
			return RoutingView{}, apperrors.NewValidationErrorWithField("routing", fmt.Sprintf("phase %q cannot be processed remotely", phase))
		}
		if mode == "" {
			continue
		}
		if !RoutingMode(mode).valid() {
			return RoutingView{}, apperrors.NewValidationErrorWithField("routing", fmt.Sprintf("routing of %s must be local, remote or both", phase))
		}
		next[phase] = RoutingMode(mode)
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return RoutingView{}, apperrors.NewInternalError("failed to encode phase routing", err)
	}
	if r.repo != nil {
		if err := r.repo.UpdatePhaseRouting(raw); err != nil {
			return RoutingView{}, apperrors.NewInternalError("failed to save phase routing", err)
		}
	}
	r.mu.Lock()
	r.overrides = next
	r.mu.Unlock()
	return r.View(), nil
}
