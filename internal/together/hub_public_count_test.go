package together

import (
	"sync"
	"testing"
	"time"

	"goonhub/internal/data"
)

// countRecorder records the public lobby counts reported by the hub.
type countRecorder struct {
	mu     sync.Mutex
	counts []int64
}

func (r *countRecorder) record(n int64) {
	r.mu.Lock()
	r.counts = append(r.counts, n)
	r.mu.Unlock()
}

func (r *countRecorder) all() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.counts...)
}

// expect ticks the hub and checks the counts reported so far.
func (r *countRecorder) expect(t *testing.T, e *env, want ...int64) {
	t.Helper()
	e.hub.Tick()
	got := r.all()
	if len(got) != len(want) {
		t.Fatalf("reported counts %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reported counts %v, want %v", got, want)
		}
	}
}

func TestPublicLobbyCountReports(t *testing.T) {
	e := newEnv(t)
	rec := &countRecorder{}
	e.hub.OnPublicCountChange(rec.record)

	rec.expect(t, e) // nothing changed: nothing reported

	// A private lobby does not count
	e.lobby(CreateInput{Name: "Private"}, 1)
	rec.expect(t, e)

	pub := e.lobby(CreateInput{Name: "Public", Visibility: data.LobbyVisibilityPublic}, 1)
	rec.expect(t, e, 1)
	rec.expect(t, e, 1) // reported once
	if n, err := e.hub.PublicLobbyCount(); err != nil || n != 1 {
		t.Fatalf("public count: %d %v", n, err)
	}

	// Visibility changes in the lobby settings
	host := e.connect(pub, alice)
	e.send(host, map[string]any{"t": "settings", "visibility": data.LobbyVisibilityPrivate})
	rec.expect(t, e, 1, 0)
	e.send(host, map[string]any{"t": "settings", "name": "Renamed"})
	rec.expect(t, e, 1, 0) // other settings: no report
	e.send(host, map[string]any{"t": "settings", "visibility": data.LobbyVisibilityPublic})
	rec.expect(t, e, 1, 0, 1)

	// Closing it
	if err := e.hub.CloseLobby(e.actor(alice), pub); err != nil {
		t.Fatal(err)
	}
	rec.expect(t, e, 1, 0, 1, 0)

	// An unloaded public lobby deleted by the idle cleanup
	e.lobby(CreateInput{Name: "Idle", Visibility: data.LobbyVisibilityPublic}, 1)
	rec.expect(t, e, 1, 0, 1, 0, 1)
	e.clock.advance(2 * time.Minute)
	e.hub.Tick() // unloads it
	e.clock.advance(25 * time.Hour)
	rec.expect(t, e, 1, 0, 1, 0, 1, 0)
	if n, err := e.hub.PublicLobbyCount(); err != nil || n != 0 {
		t.Fatalf("public count after the cleanup: %d %v", n, err)
	}
}

func TestPublicLobbyCountWithoutListener(t *testing.T) {
	e := newEnv(t)
	e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	e.hub.Tick() // no listener set: must not panic
	rec := &countRecorder{}
	e.hub.OnPublicCountChange(rec.record)
	e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 2)
	rec.expect(t, e, 2)
}
