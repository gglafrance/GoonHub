package together

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/together/togethertest"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// --- fixtures ---

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type fakeAccess struct {
	mu    sync.Mutex
	perms map[uint][]string
}

func (f *fakeAccess) ResolveUser(id uint) (*core.UserAccess, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return core.NewUserAccess(id, "user", false, f.perms[id]...), nil
}

type fakeEmotes map[string]bool

func (f fakeEmotes) HasEmote(name string) bool { return f[name] }

const (
	alice uint = 1 // host
	bob   uint = 2
	carol uint = 3
	mod   uint = 9
)

var names = map[uint]string{alice: "alice", bob: "bob", carol: "carol", mod: "mod"}

type env struct {
	t      *testing.T
	hub    *Hub
	store  *togethertest.Store
	clock  *fakeClock
	access *fakeAccess
}

func testConfig() config.Goon2getherConfig {
	return config.Goon2getherConfig{
		Enabled: true, MaxMembers: 20, MaxLobbiesPerUser: 5, MaxQueueItems: 50, MaxSuggestions: 10,
		MaxConnectionsPerUser: 4, ChatHistory: 50, ChatMaxLength: 100, ChatRateLimit: 3,
		ReactionRateLimit: 5, RateWindow: 10 * time.Second, DriftThreshold: 0.75, DriftNudge: true,
		HoldTimeout: 10 * time.Second, EndGrace: 1500 * time.Millisecond, HostGrace: 60 * time.Second,
		TickInterval: 500 * time.Millisecond, UnloadAfter: time.Minute, IdleTimeout: 24 * time.Hour,
		CleanupInterval: time.Minute, SendBuffer: 512,
	}
}

func newEnv(t *testing.T, mutate ...func(*config.Goon2getherConfig)) *env {
	t.Helper()
	cfg := testConfig()
	for _, m := range mutate {
		m(&cfg)
	}
	store := togethertest.NewStore()
	for id, n := range names {
		store.Users[id] = n
	}
	scenes := togethertest.Scenes{
		1: {ID: 1, Title: "One", Duration: 100},
		2: {ID: 2, Title: "Two", Duration: 200},
		3: {ID: 3, Title: "Three"}, // unknown duration
	}
	for id, s := range scenes {
		store.Scenes[id] = s
	}
	user := []string{core.PermTogetherUse, core.PermTogetherHost, core.PermScenesView}
	access := &fakeAccess{perms: map[uint][]string{
		alice: user, bob: user, carol: user,
		mod: append([]string{core.PermTogetherModerate}, user...),
	}}
	clock := &fakeClock{t: time.Date(2026, 11, 25, 20, 0, 0, 0, time.UTC)}
	h := NewHub(cfg, store, scenes, access, zap.NewNop())
	h.now = clock.now
	h.SetEmotes(fakeEmotes{"pog": true})
	return &env{t: t, hub: h, store: store, clock: clock, access: access}
}

func (e *env) actor(id uint) Actor {
	a, _ := e.access.ResolveUser(id)
	return Actor{ID: id, Username: names[id], Access: a}
}

// lobby creates a lobby hosted by alice with the given scenes queued.
func (e *env) lobby(in CreateInput, sceneIDs ...uint) uuid.UUID {
	e.t.Helper()
	for _, id := range sceneIDs {
		in.Items = append(in.Items, ItemInput{SceneID: id})
	}
	s, err := e.hub.CreateLobby(e.actor(alice), in)
	if err != nil {
		e.t.Fatalf("create lobby: %v", err)
	}
	return uuid.MustParse(s.ID)
}

func (e *env) connect(id uuid.UUID, user uint) *Client {
	e.t.Helper()
	c, err := e.hub.Connect(e.actor(user), id, false)
	if err != nil {
		e.t.Fatalf("connect %s: %v", names[user], err)
	}
	return c
}

func (e *env) send(c *Client, msg map[string]any) {
	e.t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		e.t.Fatal(err)
	}
	e.hub.Receive(c, b)
}

func drain(c *Client) []map[string]any {
	var out []map[string]any
	for {
		select {
		case b := <-c.Send():
			var m map[string]any
			if err := json.Unmarshal(b, &m); err == nil {
				out = append(out, m)
			}
		default:
			return out
		}
	}
}

func last(msgs []map[string]any, t string) map[string]any {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["t"] == t {
			return msgs[i]
		}
	}
	return nil
}

// expectError sends a message and checks it is refused with the code.
func (e *env) expectError(c *Client, msg map[string]any, code string) {
	e.t.Helper()
	drain(c)
	e.send(c, msg)
	got := last(drain(c), msgError)
	if got == nil || got["code"] != code {
		e.t.Fatalf("%v: expected error %q, got %v", msg, code, got)
	}
}

func (e *env) loaded(id uuid.UUID) *lobby {
	e.t.Helper()
	e.hub.mu.Lock()
	defer e.hub.mu.Unlock()
	l := e.hub.lobbies[id]
	if l == nil {
		e.t.Fatalf("lobby %s not loaded", id)
	}
	return l
}

func (e *env) state(id uuid.UUID) playback {
	l := e.loaded(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state
}

func (e *env) pos(id uuid.UUID) float64 {
	l := e.loaded(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.posNow(e.hub.now())
}

func (e *env) queue(id uuid.UUID) (queue, suggestions []uint) {
	l := e.loaded(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	return itemIDs(l.queue), itemIDs(l.suggestions)
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// --- tests ---

func TestCommandsNeedControl(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	member := e.connect(id, bob)

	for _, msg := range []map[string]any{
		{"t": "play"}, {"t": "pause"}, {"t": "seek", "position": 10}, {"t": "rate", "rate": 2},
		{"t": "skip"}, {"t": "settings", "name": "x"}, {"t": "grant_control", "user_id": bob},
	} {
		e.expectError(member, msg, ErrCodeNotAllowed)
	}
	if s := e.state(id); s.playing || s.position != 0 || s.rate != 1 {
		t.Fatalf("refused commands changed the state: %+v", s)
	}

	e.send(host, map[string]any{"t": "grant_control", "user_id": bob})
	e.send(member, map[string]any{"t": "seek", "position": 42})
	if s := e.state(id); !near(s.position, 42) {
		t.Fatalf("a controller's seek was not applied: %+v", s)
	}
	e.send(host, map[string]any{"t": "revoke_control", "user_id": bob})
	e.expectError(member, map[string]any{"t": "play"}, ErrCodeNotAllowed)

	// Control open to everyone
	e.send(host, map[string]any{"t": "settings", "control_open": true})
	drain(member)
	e.send(member, map[string]any{"t": "play"})
	if !e.state(id).playing {
		t.Fatal("control_open did not let the member play")
	}
	// Still host only
	e.expectError(member, map[string]any{"t": "settings", "name": "mine"}, ErrCodeNotAllowed)
}

func TestPositionMathWithFakeClock(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{}, 1)
	host := e.connect(id, alice)

	e.send(host, map[string]any{"t": "play", "position": 10})
	e.clock.advance(5 * time.Second)
	if p := e.pos(id); !near(p, 15) {
		t.Fatalf("position after 5 s: %v, want 15", p)
	}
	e.send(host, map[string]any{"t": "rate", "rate": 2})
	e.clock.advance(2 * time.Second)
	if p := e.pos(id); !near(p, 19) {
		t.Fatalf("position at rate 2: %v, want 19", p)
	}
	e.send(host, map[string]any{"t": "pause"})
	e.clock.advance(time.Minute)
	if p := e.pos(id); !near(p, 19) {
		t.Fatalf("paused position moved: %v", p)
	}
	// Seeks are clamped to the item
	e.send(host, map[string]any{"t": "seek", "position": 500})
	if p := e.pos(id); !near(p, 100) {
		t.Fatalf("seek past the end: %v, want 100", p)
	}
	e.expectError(host, map[string]any{"t": "rate", "rate": 10}, ErrCodeInvalid)

	// The sent state lets a client compute the same position
	drain(host)
	e.send(host, map[string]any{"t": "play", "position": 30})
	st := last(drain(host), msgState)["state"].(map[string]any)
	e.clock.advance(1500 * time.Millisecond)
	elapsed := float64(e.hub.now().UnixMilli()-int64(st["server_time"].(float64))) / 1000
	client := st["position"].(float64) + elapsed*st["rate"].(float64)
	if !near(client, e.pos(id)) {
		t.Fatalf("client position %v, server %v", client, e.pos(id))
	}
	// The playback is saved on pause and seek
	e.send(host, map[string]any{"t": "pause"})
	l := e.loaded(id)
	if row := e.store.Lobby(l.row.ID); row == nil || !near(row.PositionS, 33) || row.PlaybackRate != 2 {
		t.Fatalf("saved playback: %+v", row)
	}
}

func TestAutoAdvanceAndEndedReports(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2, 3)
	host := e.connect(id, alice)
	member := e.connect(id, bob)
	q, _ := e.queue(id)

	// Item 1 (100 s): the server advances after the duration plus end_grace
	e.send(host, map[string]any{"t": "play", "position": 99})
	e.clock.advance(2 * time.Second)
	e.hub.Tick()
	if s := e.state(id); s.itemID != q[0] {
		t.Fatalf("advanced before the end grace: %+v", s)
	}
	e.clock.advance(600 * time.Millisecond)
	e.hub.Tick()
	if s := e.state(id); s.itemID != q[1] || !s.playing || s.position != 0 {
		t.Fatalf("expected item 2 playing from 0, got %+v", s)
	}
	if got := e.store.Queue(e.loaded(id).row.ID); len(got) != 2 {
		t.Fatalf("the finished item was not removed: %v", got)
	}

	// Item 2 (200 s): an early ended report is ignored, a late one (stale item) too
	e.send(member, map[string]any{"t": "ended", "item_id": q[1]})
	if e.state(id).itemID != q[1] {
		t.Fatal("an ended report long before the end skipped the item")
	}
	e.send(host, map[string]any{"t": "seek", "position": 197})
	e.send(member, map[string]any{"t": "ended", "item_id": q[1]})
	if s := e.state(id); s.itemID != q[2] {
		t.Fatalf("an ended report near the end did not advance: %+v", s)
	}
	e.send(member, map[string]any{"t": "ended", "item_id": q[1]})
	if s := e.state(id); s.itemID != q[2] {
		t.Fatalf("a stale ended report advanced again: %+v", s)
	}

	// Item 3 (unknown duration): reports from half of the online members advance it
	carolC := e.connect(id, carol)
	_ = carolC
	e.send(member, map[string]any{"t": "ended", "item_id": q[2]})
	if e.state(id).itemID != q[2] {
		t.Fatal("one report of three members advanced")
	}
	e.send(carolC, map[string]any{"t": "ended", "item_id": q[2]})
	if s := e.state(id); s.itemID != 0 || s.playing {
		t.Fatalf("expected the queue finished, got %+v", s)
	}
	if m := last(drain(host), msgChat); m == nil || m["message"].(map[string]any)["text"] != "The queue is finished" {
		t.Fatalf("expected the queue finished system message, got %v", m)
	}
}

func TestRangeItemAdvancesAtItsEnd(t *testing.T) {
	e := newEnv(t)
	start, end := 10.0, 20.0
	label := "Marker"
	s, err := e.hub.CreateLobby(e.actor(alice), CreateInput{Items: []ItemInput{{SceneID: 2, Start: &start, End: &end, Label: &label}, {SceneID: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(s.ID)
	host := e.connect(id, alice)
	if p := e.pos(id); !near(p, 10) {
		t.Fatalf("a range item starts at its start: %v", p)
	}
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(10 * time.Second)
	e.hub.Tick()
	if st := e.state(id); st.itemID == 0 || e.loaded(id).queue[0].sceneID != 1 {
		t.Fatalf("the range item did not end at its end: %+v", st)
	}
}

func TestQueueOpsAndSuggestions(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic})
	host := e.connect(id, alice)
	member := e.connect(id, bob)

	// Adding to an empty queue makes the first item current, paused
	e.send(host, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 1}, {"scene_id": 2}, {"scene_id": 3}}})
	q, _ := e.queue(id)
	if len(q) != 3 || e.state(id).itemID != q[0] || e.state(id).playing {
		t.Fatalf("queue %v state %+v", q, e.state(id))
	}
	e.expectError(host, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 99}}}, ErrCodeNotFound)
	e.expectError(host, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 1, "start": 5, "end": 2}}}, ErrCodeInvalid)

	// A member without queue rights suggests
	e.send(member, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 2}}})
	_, sug := e.queue(id)
	if len(sug) != 1 {
		t.Fatalf("expected one suggestion, got %v", sug)
	}
	e.expectError(member, map[string]any{"t": "suggestion_accept", "item_id": sug[0]}, ErrCodeNotAllowed)
	e.send(host, map[string]any{"t": "suggestion_accept", "item_id": sug[0]})
	q, sug = e.queue(id)
	if len(q) != 4 || len(sug) != 0 {
		t.Fatalf("accept: queue %v suggestions %v", q, sug)
	}
	// One's own suggestion can be withdrawn
	e.send(member, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 3}}})
	_, sug = e.queue(id)
	e.send(member, map[string]any{"t": "suggestion_reject", "item_id": sug[0]})
	if _, sug = e.queue(id); len(sug) != 0 {
		t.Fatalf("own suggestion not withdrawn: %v", sug)
	}

	// Move the last upcoming item first (index 0 = right after the current one)
	e.send(host, map[string]any{"t": "queue_move", "item_id": q[3], "index": 0})
	want := []uint{q[0], q[3], q[1], q[2]}
	if got, _ := e.queue(id); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("move: got %v want %v", got, want)
	}
	if got := e.store.Queue(e.loaded(id).row.ID); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stored order %v want %v", got, want)
	}
	e.expectError(host, map[string]any{"t": "queue_move", "item_id": q[0], "index": 2}, ErrCodeNotFound)

	// Remove an upcoming item, then jump to another one (the current one is dropped)
	e.send(host, map[string]any{"t": "queue_remove", "item_id": q[1]})
	e.send(host, map[string]any{"t": "play_item", "item_id": q[2]})
	got, _ := e.queue(id)
	if fmt.Sprint(got) != fmt.Sprint([]uint{q[2], q[3]}) || e.state(id).itemID != q[2] || !e.state(id).playing {
		t.Fatalf("play_item: queue %v state %+v", got, e.state(id))
	}
	e.expectError(member, map[string]any{"t": "queue_remove", "item_id": q[3]}, ErrCodeNotAllowed)

	// Queue open to all: members add directly
	e.send(host, map[string]any{"t": "settings", "queue_open": true})
	e.send(member, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 1}}})
	if got, sug := e.queue(id); len(got) != 3 || len(sug) != 0 {
		t.Fatalf("queue_open: queue %v suggestions %v", got, sug)
	}

	// Members without scenes:view cannot add
	e.access.mu.Lock()
	e.access.perms[bob] = []string{core.PermTogetherUse}
	e.access.mu.Unlock()
	e.expectError(member, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 1}}}, ErrCodeNotAllowed)
}

func TestQueueAddPlayNow(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2)
	host := e.connect(id, alice)
	member := e.connect(id, bob)
	q, _ := e.queue(id)

	// Without control, play_now is refused and nothing is added (not even as a suggestion)
	e.expectError(member, map[string]any{"t": "queue_add", "play_now": true, "items": []map[string]any{{"scene_id": 3}}}, ErrCodeNotAllowed)
	if got, sug := e.queue(id); len(got) != 2 || len(sug) != 0 {
		t.Fatalf("refused play_now changed the queue: %v %v", got, sug)
	}
	// queue_open lets members add, not play now
	e.send(host, map[string]any{"t": "settings", "queue_open": true})
	e.expectError(member, map[string]any{"t": "queue_add", "play_now": true, "items": []map[string]any{{"scene_id": 3}}}, ErrCodeNotAllowed)

	// The added items go first and the first one plays; the item playing is dropped, the
	// upcoming ones follow
	e.send(host, map[string]any{"t": "queue_add", "play_now": true, "items": []map[string]any{
		{"scene_id": 2, "start": 20, "end": 40, "label": "Kiss"}, {"scene_id": 3},
	}})
	got, _ := e.queue(id)
	if len(got) != 3 || got[2] != q[1] {
		t.Fatalf("play_now order: got %v (old queue %v)", got, q)
	}
	st := e.state(id)
	if st.itemID != got[0] || !st.playing || !near(e.pos(id), 20) {
		t.Fatalf("play_now did not start the first added item: %+v", st)
	}
	if stored := e.store.Queue(e.loaded(id).row.ID); fmt.Sprint(stored) != fmt.Sprint(got) {
		t.Fatalf("stored order %v want %v", stored, got)
	}

	// Into an empty queue, play_now starts playing (a plain add stays paused)
	id2 := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic})
	host2 := e.connect(id2, alice)
	e.send(host2, map[string]any{"t": "queue_add", "play_now": true, "items": []map[string]any{{"scene_id": 1}}})
	if q2, _ := e.queue(id2); len(q2) != 1 || e.state(id2).itemID != q2[0] || !e.state(id2).playing {
		t.Fatalf("play_now into an empty queue: %v %+v", q2, e.state(id2))
	}
}

func TestQueueAddClampsEndToTheScene(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic})
	host := e.connect(id, alice)
	// Scene 1 lasts 100 s, scene 3 has no known duration
	e.send(host, map[string]any{"t": "queue_add", "items": []map[string]any{
		{"scene_id": 1, "start": 90, "end": 120},
		{"scene_id": 1, "start": 100, "end": 130},
		{"scene_id": 3, "start": 10, "end": 40},
		{"scene_id": 1, "start": 10, "end": 40},
	}})
	l := e.loaded(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	ends := make([]any, len(l.queue))
	for i, it := range l.queue {
		if it.end != nil {
			ends[i] = *it.end
		}
	}
	if fmt.Sprint(ends) != fmt.Sprint([]any{100.0, nil, 40.0, 40.0}) {
		t.Fatalf("ends: %v", ends)
	}
}

func TestQueueLimits(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.MaxQueueItems = 2; c.MaxSuggestions = 1 })
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2)
	host := e.connect(id, alice)
	member := e.connect(id, bob)
	e.expectError(host, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 3}}}, ErrCodeQueueFull)
	e.send(member, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 3}}})
	e.expectError(member, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 3}}}, ErrCodeQueueFull)
}

func TestHostTransfer(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	bobC := e.connect(id, bob)
	e.connect(id, carol)

	// Manual transfer, to an online member only
	e.send(host, map[string]any{"t": "transfer_host", "user_id": bob})
	if l := e.loaded(id); !l.isHost(bob) {
		t.Fatal("manual transfer failed")
	}
	e.expectError(host, map[string]any{"t": "transfer_host", "user_id": carol}, ErrCodeNotAllowed)

	// Automatic transfer after the grace period, to the member with control first
	e.send(bobC, map[string]any{"t": "grant_control", "user_id": carol})
	e.hub.Disconnect(bobC)
	e.clock.advance(59 * time.Second)
	e.hub.Tick()
	if l := e.loaded(id); !l.isHost(bob) {
		t.Fatal("the host role moved before the grace period")
	}
	e.clock.advance(2 * time.Second)
	e.hub.Tick()
	l := e.loaded(id)
	if !l.isHost(carol) {
		t.Fatalf("expected carol (control) to be the host, got %v", l.row.HostUserID)
	}
	if row := e.store.Lobby(l.row.ID); row.HostUserID == nil || *row.HostUserID != carol {
		t.Fatalf("host not saved: %+v", row.HostUserID)
	}

	// A host coming back in time keeps the role
	carolC := e.connect(id, carol)
	e.hub.Disconnect(carolC)
	e.clock.advance(30 * time.Second)
	carolC = e.connect(id, carol)
	e.clock.advance(60 * time.Second)
	e.hub.Tick()
	if !e.loaded(id).isHost(carol) {
		t.Fatal("a host who came back lost the role")
	}
}

func TestLeaveHandsOverOrCloses(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	other := e.connect(id, alice)
	if _, err := e.hub.JoinLobby(e.actor(bob), id, false); err != nil {
		t.Fatal(err)
	}
	if err := e.hub.LeaveLobby(e.actor(alice), id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-other.Done():
		if code, _ := other.CloseStatus(); code != CloseLeft {
			t.Fatalf("leaving closed the tab with %d", code)
		}
	default:
		t.Fatal("the leaving user's connection stayed open")
	}
	if !e.loaded(id).isHost(bob) {
		t.Fatal("the host role was not handed over")
	}
	if err := e.hub.LeaveLobby(e.actor(bob), id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hub.GetLobby(e.actor(bob), id); !apperrors.IsNotFound(err) {
		t.Fatalf("the empty lobby was not closed: %v", err)
	}
}

func TestKickBanAndModeration(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	bobC := e.connect(id, bob)
	carolC := e.connect(id, carol)

	e.expectError(bobC, map[string]any{"t": "kick", "user_id": carol}, ErrCodeNotAllowed)
	e.expectError(carolC, map[string]any{"t": "kick", "user_id": alice}, ErrCodeNotAllowed)

	e.send(host, map[string]any{"t": "kick", "user_id": bob})
	select {
	case <-bobC.Done():
	default:
		t.Fatal("the kicked client is still open")
	}
	if code, _ := bobC.CloseStatus(); code != CloseKicked {
		t.Fatalf("kick close code %d", code)
	}
	if m := last(drain(bobC), msgBye); m == nil || m["reason"] != ByeKicked {
		t.Fatalf("expected a bye before the close, got %v", m)
	}
	// A kicked user can come back to a public lobby, a banned one cannot
	bobC = e.connect(id, bob)
	modC := e.connect(id, mod)
	e.send(modC, map[string]any{"t": "kick", "user_id": bob, "ban": true})
	if code, _ := bobC.CloseStatus(); code != CloseForbidden {
		t.Fatalf("ban close code %d", code)
	}
	if _, err := e.hub.Connect(e.actor(bob), id, false); !apperrors.IsForbidden(err) {
		t.Fatalf("a banned user reconnected: %v", err)
	}
	if _, err := e.hub.JoinLobby(e.actor(bob), id, false); !apperrors.IsForbidden(err) {
		t.Fatalf("a banned user joined: %v", err)
	}
	list, err := e.hub.ListLobbies(e.actor(bob), false)
	if err != nil || len(list) != 0 {
		t.Fatalf("a banned user still lists the lobby: %v %v", list, err)
	}

	// Chat deletion: author, host and moderators only
	e.send(carolC, map[string]any{"t": "chat", "text": "hello"})
	msg := last(drain(host), msgChat)["message"].(map[string]any)
	msgID := msg["id"]
	if msg["text"] != "hello" || msg["username"] != "carol" {
		t.Fatalf("chat message %v", msg)
	}
	e.send(carolC, map[string]any{"t": "chat", "text": "second"})
	second := last(drain(host), msgChat)["message"].(map[string]any)["id"]
	other := e.connect(id, bob+100) // unknown user: no permission, just a member of a public lobby
	_ = other
	e.send(modC, map[string]any{"t": "chat_delete", "id": msgID})
	if m := last(drain(host), msgChatDeleted); m == nil || m["id"] != msgID {
		t.Fatalf("moderator delete: %v", m)
	}
	e.send(host, map[string]any{"t": "chat_delete", "id": second})
	if m := last(drain(host), msgChatDeleted); m == nil || m["id"] != second {
		t.Fatalf("host delete: %v", m)
	}
	e.send(carolC, map[string]any{"t": "chat", "text": "third"})
	third := last(drain(host), msgChat)["message"].(map[string]any)["id"]
	e.expectError(other, map[string]any{"t": "chat_delete", "id": third}, ErrCodeNotAllowed)
}

func TestChatLimitsAndReactions(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	c := e.connect(id, alice)
	long := make([]byte, 101)
	for i := range long {
		long[i] = 'a'
	}
	e.expectError(c, map[string]any{"t": "chat", "text": string(long)}, ErrCodeTooLong)
	e.expectError(c, map[string]any{"t": "chat", "text": "   "}, ErrCodeInvalid)
	e.expectError(c, map[string]any{"t": "chat", "text": "bell\x07"}, ErrCodeInvalid)
	for i := 0; i < 3; i++ {
		e.send(c, map[string]any{"t": "chat", "text": "spam"})
	}
	e.expectError(c, map[string]any{"t": "chat", "text": "spam"}, ErrCodeRateLimited)
	e.clock.advance(4 * time.Second) // one message per 10/3 s comes back
	drain(c)
	e.send(c, map[string]any{"t": "chat", "text": "ok again"})
	if m := last(drain(c), msgChat); m == nil {
		t.Fatal("the rate limit did not refill")
	}

	for _, ok := range []string{"\U0001F525", "❤️‍\U0001F525", ":pog:", "#️⃣"} {
		drain(c)
		e.send(c, map[string]any{"t": "reaction", "emote": ok})
		if m := last(drain(c), msgReaction); m == nil || m["emote"] != ok {
			t.Fatalf("reaction %q refused: %v", ok, m)
		}
	}
	e.expectError(c, map[string]any{"t": "reaction", "emote": ":nope:"}, ErrCodeNotFound)
	e.expectError(c, map[string]any{"t": "reaction", "emote": "hello"}, ErrCodeInvalid)
	e.expectError(c, map[string]any{"t": "reaction", "emote": "<b>\U0001F525</b>"}, ErrCodeInvalid)
	e.expectError(c, map[string]any{"t": "bogus"}, ErrCodeUnknownType)
	drain(c)
	e.hub.Receive(c, []byte("{not json"))
	if m := last(drain(c), msgError); m == nil || m["code"] != ErrCodeInvalid {
		t.Fatalf("invalid JSON: %v", m)
	}

	// Late joiners get the recent chat (bounded history)
	late := e.connect(id, bob)
	w := last(drain(late), msgWelcome)
	if w == nil || len(w["chat"].([]any)) == 0 {
		t.Fatalf("welcome without chat history: %v", w)
	}
}

func TestPingPong(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{}, 1)
	c := e.connect(id, alice)
	drain(c)
	e.send(c, map[string]any{"t": "ping", "id": 7, "c": 123456})
	p := last(drain(c), msgPong)
	if p == nil || p["id"] != float64(7) || p["c"] != float64(123456) || p["s"] != float64(e.hub.now().UnixMilli()) {
		t.Fatalf("pong %v", p)
	}
}

func TestWaitForEveryoneHold(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{WaitForAll: true, Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	member := e.connect(id, bob)
	itemID := e.state(id).itemID

	// A seek is a sync point: playing holds until every connection is ready for it
	e.send(host, map[string]any{"t": "play"})
	e.send(host, map[string]any{"t": "seek", "position": 30})
	st := e.state(id)
	if !st.hold {
		t.Fatalf("expected a hold after the seek: %+v", st)
	}
	e.clock.advance(3 * time.Second)
	if p := e.pos(id); !near(p, 30) {
		t.Fatalf("the position moved during the hold: %v", p)
	}
	e.send(host, map[string]any{"t": "status", "buffering": false, "seq": st.seq, "item_id": itemID})
	if !e.state(id).hold {
		t.Fatal("the hold ended with one member not ready")
	}
	e.send(member, map[string]any{"t": "status", "buffering": false, "seq": st.seq, "item_id": itemID})
	st = e.state(id)
	if st.hold || !st.playing {
		t.Fatalf("the hold did not end once everyone was ready: %+v", st)
	}
	e.clock.advance(2 * time.Second)
	if p := e.pos(id); !near(p, 32) {
		t.Fatalf("position after the hold: %v, want 32", p)
	}

	// Buffering while playing holds; the hold times out and does not come back
	e.send(member, map[string]any{"t": "status", "buffering": true, "seq": st.seq, "item_id": itemID})
	if !e.state(id).hold {
		t.Fatal("buffering did not hold")
	}
	e.clock.advance(10 * time.Second)
	e.hub.Tick()
	if e.state(id).hold {
		t.Fatal("the hold did not time out")
	}
	e.send(member, map[string]any{"t": "status", "buffering": true, "seq": e.state(id).seq, "item_id": itemID})
	if e.state(id).hold {
		t.Fatal("a timed out member held again before the next sync point")
	}

	// A late joiner does not hold the lobby
	e.send(member, map[string]any{"t": "status", "buffering": false, "seq": e.state(id).seq, "item_id": itemID})
	e.connect(id, carol)
	e.send(member, map[string]any{"t": "status", "buffering": false, "seq": e.state(id).seq, "item_id": itemID})
	if e.state(id).hold {
		t.Fatal("a late joiner held the lobby")
	}

	// Without wait_for_all, buffering never holds
	e.send(host, map[string]any{"t": "settings", "wait_for_all": false})
	e.send(host, map[string]any{"t": "seek", "position": 50})
	if e.state(id).hold {
		t.Fatal("held without wait_for_all")
	}
}

func TestVisibilityAndInviteCodes(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Name: "Secret"}, 1)
	if _, err := e.hub.GetLobby(e.actor(bob), id); !apperrors.IsNotFound(err) {
		t.Fatalf("a private lobby is visible to non-members: %v", err)
	}
	if _, err := e.hub.Connect(e.actor(bob), id, false); !apperrors.IsNotFound(err) {
		t.Fatalf("a non-member connected to a private lobby: %v", err)
	}
	if s, err := e.hub.GetLobby(e.actor(mod), id); err != nil || s.InviteCode == "" {
		t.Fatalf("a moderator does not see the private lobby: %v %v", s, err)
	}
	owner, _ := e.hub.GetLobby(e.actor(alice), id)
	code := owner.InviteCode
	if len(code) != inviteCodeLength {
		t.Fatalf("invite code %q", code)
	}
	s, err := e.hub.JoinByCode(e.actor(bob), " "+code[:4]+"-"+code[4:]+" ")
	if err != nil || !s.IsMember || s.ID != id.String() {
		t.Fatalf("join by code: %v %v", s, err)
	}
	newCode, err := e.hub.RotateInviteCode(e.actor(alice), id)
	if err != nil || newCode == code {
		t.Fatalf("rotate: %q %v", newCode, err)
	}
	if _, err := e.hub.JoinByCode(e.actor(carol), code); !apperrors.IsNotFound(err) {
		t.Fatalf("the old code still works: %v", err)
	}
	if _, err := e.hub.RotateInviteCode(e.actor(bob), id); !apperrors.IsForbidden(err) {
		t.Fatalf("a member rotated the code: %v", err)
	}
	if err := e.hub.CloseLobby(e.actor(bob), id); !apperrors.IsForbidden(err) {
		t.Fatalf("a member closed the lobby: %v", err)
	}

	// The invite hook: a member pre-authorizes a user
	if err := e.hub.InviteUser(id, bob, carol, "carol"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hub.Connect(e.actor(carol), id, false); err != nil {
		t.Fatalf("an invited user cannot connect: %v", err)
	}
	active := e.hub.ActiveLobbiesForUser(carol)
	if len(active) != 1 || active[0].ID != id.String() || active[0].SceneID != 1 {
		t.Fatalf("active lobbies: %+v", active)
	}
	if err := e.hub.CloseLobby(e.actor(mod), id); err != nil {
		t.Fatalf("a moderator could not close: %v", err)
	}
	if len(e.hub.ActiveLobbiesForUser(carol)) != 0 {
		t.Fatal("a closed lobby is still active")
	}
}

func TestListLobbies(t *testing.T) {
	e := newEnv(t)
	pub := e.lobby(CreateInput{Name: "Public", Visibility: data.LobbyVisibilityPublic}, 1)
	e.lobby(CreateInput{Name: "Private"}, 2)
	e.connect(pub, alice)

	list, err := e.hub.ListLobbies(e.actor(bob), false)
	if err != nil || len(list) != 1 || list[0].ID != pub.String() {
		t.Fatalf("bob's list: %+v %v", list, err)
	}
	if l := list[0]; l.OnlineCount != 1 || l.NowPlaying == nil || l.NowPlaying.Title != "One" || l.InviteCode != "" || l.IsMember {
		t.Fatalf("summary: %+v", l)
	}
	if list, _ := e.hub.ListLobbies(e.actor(alice), false); len(list) != 2 {
		t.Fatalf("alice's list: %+v", list)
	}
	if list, _ := e.hub.ListLobbies(e.actor(mod), true); len(list) != 2 {
		t.Fatalf("moderator's full list: %+v", list)
	}
	if list, _ := e.hub.ListLobbies(e.actor(bob), true); len(list) != 1 {
		t.Fatalf("all=true without moderation: %+v", list)
	}
	// Without scenes:view, no title
	e.access.mu.Lock()
	e.access.perms[carol] = []string{core.PermTogetherUse}
	e.access.mu.Unlock()
	if list, _ := e.hub.ListLobbies(e.actor(carol), false); len(list) != 1 || list[0].NowPlaying.Title != "" {
		t.Fatalf("title leaked without scenes:view: %+v", list)
	}
}

func TestCreateValidation(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.MaxLobbiesPerUser = 1 })
	if _, err := e.hub.CreateLobby(e.actor(alice), CreateInput{Visibility: "secret"}); !apperrors.IsValidation(err) {
		t.Fatalf("bad visibility: %v", err)
	}
	if _, err := e.hub.CreateLobby(e.actor(alice), CreateInput{MaxMembers: 99}); !apperrors.IsValidation(err) {
		t.Fatalf("max members over the server limit: %v", err)
	}
	s, err := e.hub.CreateLobby(e.actor(alice), CreateInput{})
	if err != nil || s.Name != "alice's lobby" || s.Visibility != data.LobbyVisibilityPrivate {
		t.Fatalf("defaults: %+v %v", s, err)
	}
	if _, err := e.hub.CreateLobby(e.actor(alice), CreateInput{}); !apperrors.IsValidation(err) {
		t.Fatalf("lobby limit: %v", err)
	}
}

func TestMemberLimits(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.MaxConnectionsPerUser = 2 })
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic, MaxMembers: 2}, 1)
	e.connect(id, alice)
	e.connect(id, bob)
	if _, err := e.hub.Connect(e.actor(carol), id, false); err != ErrLobbyFull {
		t.Fatalf("a third member got in: %v", err)
	}
	e.connect(id, bob)
	if _, err := e.hub.Connect(e.actor(bob), id, false); err != ErrLobbyFull {
		t.Fatalf("a third connection of a user got in: %v", err)
	}
}

func TestUnloadReloadAndIdleCleanup(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{}, 1, 2)
	c := e.connect(id, alice)
	e.send(c, map[string]any{"t": "seek", "position": 12})
	cur := e.state(id).itemID
	e.hub.Disconnect(c)

	e.clock.advance(59 * time.Second)
	e.hub.Tick()
	e.hub.mu.Lock()
	_, still := e.hub.lobbies[id]
	e.hub.mu.Unlock()
	if !still {
		t.Fatal("unloaded before unload_after")
	}
	e.clock.advance(2 * time.Second)
	e.hub.Tick()
	e.hub.mu.Lock()
	_, still = e.hub.lobbies[id]
	e.hub.mu.Unlock()
	if still {
		t.Fatal("not unloaded after unload_after")
	}

	// Loaded again from the database, paused where it was
	c = e.connect(id, alice)
	if st := e.state(id); st.itemID != cur || !near(st.position, 12) || st.playing {
		t.Fatalf("reloaded state %+v", st)
	}
	w := last(drain(c), msgWelcome)
	if w == nil || len(w["queue"].([]any)) != 2 {
		t.Fatalf("welcome after reload: %v", w)
	}
	e.hub.Disconnect(c)

	// Idle cleanup deletes lobbies nobody connected to for idle_timeout
	e.clock.advance(2 * time.Minute)
	e.hub.Tick() // unloads it
	e.clock.advance(25 * time.Hour)
	e.hub.Tick()
	if len(e.store.DeleteIdleCalls) == 0 {
		t.Fatal("no idle cleanup ran")
	}
	if _, err := e.hub.GetLobby(e.actor(alice), id); !apperrors.IsNotFound(err) {
		t.Fatalf("the idle lobby was not deleted: %v", err)
	}
}

func TestSlowConsumerIsClosed(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.SendBuffer = 4 })
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	slow := e.connect(id, bob) // never read
	host := e.connect(id, alice)
	for i := 0; i < 10; i++ {
		e.send(host, map[string]any{"t": "seek", "position": i})
		drain(host)
	}
	select {
	case <-slow.Done():
		if code, _ := slow.CloseStatus(); code != ClosePolicy {
			t.Fatalf("slow consumer close code %d", code)
		}
	default:
		t.Fatal("the slow consumer was not closed")
	}
	// The other clients are not affected
	select {
	case <-host.Done():
		t.Fatal("a reading client was closed")
	default:
	}
	e.hub.Disconnect(slow)
}

func TestConcurrentClientsRace(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.SendBuffer = 8; c.ChatRateLimit = 1000 })
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic, ControlOpen: true}, 1, 2, 3)
	var wg sync.WaitGroup
	for _, u := range []uint{alice, bob, carol, mod} {
		wg.Add(1)
		go func(u uint) {
			defer wg.Done()
			for round := 0; round < 5; round++ {
				c, err := e.hub.Connect(e.actor(u), id, false)
				if err != nil {
					t.Errorf("connect: %v", err)
					return
				}
				done := make(chan struct{})
				go func() { // reader
					defer close(done)
					for {
						select {
						case <-c.Send():
						case <-c.Done():
							return
						}
					}
				}()
				for i := 0; i < 20; i++ {
					for _, m := range []string{
						`{"t":"play"}`, `{"t":"seek","position":5}`, `{"t":"chat","text":"hi"}`,
						`{"t":"status","buffering":false,"seq":1}`, `{"t":"ping","id":1,"c":1}`,
						`{"t":"queue_add","items":[{"scene_id":1}]}`,
					} {
						e.hub.Receive(c, []byte(m))
					}
				}
				e.hub.Disconnect(c)
				<-done
			}
		}(u)
	}
	stop := make(chan struct{})
	var tickWG sync.WaitGroup
	tickWG.Add(1)
	go func() {
		defer tickWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
				e.hub.Tick()
				_, _ = e.hub.ListLobbies(e.actor(alice), false)
				e.hub.ActiveLobbiesForUser(alice)
			}
		}
	}()
	wg.Wait()
	close(stop)
	tickWG.Wait()
}

func TestStopClosesConnections(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{}, 1)
	c := e.connect(id, alice)
	e.hub.Start()
	e.hub.Stop()
	select {
	case <-c.Done():
	default:
		t.Fatal("Stop left a connection open")
	}
}
