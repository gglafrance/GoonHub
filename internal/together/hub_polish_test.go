package together

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/together/togethertest"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// restart replaces the hub with a new one on the same store and clock (a server restart).
func (e *env) restart() {
	e.t.Helper()
	cfg := e.hub.cfg
	e.hub.Stop()
	h := NewHub(cfg, e.store, togethertest.Scenes(e.store.Scenes), e.access, zap.NewNop())
	h.now = e.clock.now
	h.SetEmotes(fakeEmotes{"pog": true})
	e.hub = h
}

func (e *env) isLoaded(id uuid.UUID) bool {
	e.hub.mu.Lock()
	defer e.hub.mu.Unlock()
	_, ok := e.hub.lobbies[id]
	return ok
}

// rowID gives the database ID of a lobby (loading it).
func (e *env) rowID(id uuid.UUID) uint {
	e.t.Helper()
	row, err := e.store.GetByUUID(id)
	if err != nil {
		e.t.Fatalf("lobby %s not stored: %v", id, err)
	}
	return row.ID
}

// drainRaw gives the raw frames queued for a client.
func drainRaw(c *Client) []string {
	var out []string
	for {
		select {
		case b := <-c.Send():
			out = append(out, string(b))
		default:
			return out
		}
	}
}

func all(msgs []map[string]any, t string) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		if m["t"] == t {
			out = append(out, m)
		}
	}
	return out
}

// --- auto-close ---

func TestEmptyLobbyClosesAfterEmptyCloseAfter(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.EmptyCloseAfter = 15 * time.Minute })
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	rowID := e.rowID(id)
	code := e.store.Lobby(rowID).InviteCode
	c := e.connect(id, alice)
	e.hub.Disconnect(c)

	// Unloaded after unload_after, still stored
	e.clock.advance(2 * time.Minute)
	e.hub.Tick()
	if e.isLoaded(id) {
		t.Fatal("the empty lobby was not unloaded")
	}
	e.clock.advance(12 * time.Minute)
	e.hub.Tick()
	if e.store.Lobby(rowID) == nil {
		t.Fatal("the empty lobby closed before empty_close_after")
	}
	// Unloading does not extend the empty time: 15 min after the last member left, it closes
	e.clock.advance(90 * time.Second)
	e.hub.Tick()
	if e.store.Lobby(rowID) != nil {
		t.Fatal("the empty lobby was not closed after empty_close_after")
	}
	if _, err := e.hub.GetLobby(e.actor(alice), id); !apperrors.IsNotFound(err) {
		t.Fatalf("the closed lobby is still there: %v", err)
	}
	if _, err := e.hub.JoinByCode(e.actor(bob), code); !apperrors.IsNotFound(err) {
		t.Fatalf("the invite code of a closed lobby still works: %v", err)
	}
	if list, _ := e.hub.ListLobbies(e.actor(bob), false); len(list) != 0 {
		t.Fatalf("the closed lobby is still listed: %+v", list)
	}
}

func TestEmptyLoadedLobbyClosesInTick(t *testing.T) {
	// unload_after longer than empty_close_after: the loaded lobby closes itself
	e := newEnv(t, func(c *config.Goon2getherConfig) {
		c.EmptyCloseAfter = 15 * time.Minute
		c.UnloadAfter = time.Hour
	})
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	rowID := e.rowID(id)
	e.hub.Disconnect(e.connect(id, alice))
	e.clock.advance(14 * time.Minute)
	e.hub.Tick()
	if !e.isLoaded(id) || e.store.Lobby(rowID) == nil {
		t.Fatal("closed before empty_close_after")
	}
	e.clock.advance(time.Minute)
	e.hub.Tick()
	if e.isLoaded(id) || e.store.Lobby(rowID) != nil {
		t.Fatal("the empty loaded lobby was not closed")
	}
}

func TestEmptyCloseSurvivesRestart(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) { c.EmptyCloseAfter = 15 * time.Minute })

	// A lobby with a member connected when the server stops: the restart does not count as
	// empty time, and a lobby restored with nobody connected still closes
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	rowID := e.rowID(id)
	e.connect(id, alice)
	e.clock.advance(30 * time.Minute) // the cleanup did not run (no tick): last_active_at is old
	e.restart()
	if row := e.store.Lobby(rowID); row == nil || !row.LastActiveAt.Equal(e.clock.now().UTC()) {
		t.Fatalf("Stop did not touch the connected lobby: %+v", row)
	}
	e.clock.advance(time.Minute)
	e.hub.Tick()
	if e.store.Lobby(rowID) == nil {
		t.Fatal("the lobby was closed right after the restart")
	}
	// Restored by a REST call, nobody connects
	if _, err := e.hub.GetLobby(e.actor(alice), id); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		e.clock.advance(time.Minute)
		e.hub.Tick()
	}
	if e.store.Lobby(rowID) != nil {
		t.Fatal("a lobby restored after a restart with nobody connected did not close")
	}

	// A lobby left empty long before the restart is deleted by the first cleanup
	id2 := e.lobby(CreateInput{}, 1)
	rowID2 := e.rowID(id2)
	e.hub.Disconnect(e.connect(id2, alice))
	e.restart()
	e.clock.advance(20 * time.Minute)
	e.hub.Tick()
	if e.store.Lobby(rowID2) != nil {
		t.Fatal("an empty lobby was not closed after the restart")
	}
}

func TestInactiveLobbyWarnsThenCloses(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) {
		c.InactiveCloseAfter = time.Hour
		c.CloseWarning = 5 * time.Minute
	})
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	rowID := e.rowID(id)
	code := e.store.Lobby(rowID).InviteCode
	host := e.connect(id, alice)
	member := e.connect(id, bob)
	start := e.clock.now()

	e.clock.advance(54 * time.Minute)
	e.hub.Tick()
	if m := last(drain(member), msgClosing); m != nil {
		t.Fatalf("warned too early: %v", m)
	}
	e.clock.advance(time.Minute)
	e.hub.Tick()
	msgs := drain(member)
	closing := last(msgs, msgClosing)
	if closing == nil || int64(closing["at"].(float64)) != start.Add(time.Hour).UnixMilli() || closing["reason"] != "inactive" {
		t.Fatalf("expected the closing announce, got %v", closing)
	}
	chat := last(msgs, msgChat)
	if chat == nil || !strings.Contains(chat["message"].(map[string]any)["text"].(string), "closes in 5 minutes") {
		t.Fatalf("expected the warning system message, got %v", chat)
	}
	drain(host)
	// A late joiner sees the announce in the welcome
	carolC := e.connect(id, carol)
	if w := last(drain(carolC), msgWelcome); w == nil || w["closing_at"] == nil {
		t.Fatalf("welcome without closing_at: %v", w)
	}
	// Clock sync and buffering reports are not activity
	e.send(member, map[string]any{"t": "ping", "id": 1, "c": 1})
	e.send(member, map[string]any{"t": "status", "buffering": false, "seq": 1})
	if m := last(drain(host), msgClosing); m != nil {
		t.Fatalf("an automatic message called the close off: %v", m)
	}
	// Any activity calls it off
	e.send(member, map[string]any{"t": "chat", "text": "still here"})
	if m := last(drain(host), msgClosing); m == nil || m["at"] != nil {
		t.Fatalf("expected the announce to be called off, got %v", m)
	}
	e.clock.advance(10 * time.Minute)
	e.hub.Tick()
	if e.store.Lobby(rowID) == nil {
		t.Fatal("closed although there was activity")
	}

	// Nothing again: warned 55 min after the chat, closed at 60 min
	e.clock.advance(45 * time.Minute)
	e.hub.Tick()
	if m := last(drain(host), msgClosing); m == nil || m["at"] == nil {
		t.Fatalf("expected a second announce, got %v", m)
	}
	e.clock.advance(5 * time.Minute)
	e.hub.Tick()
	if bye := last(drain(member), msgBye); bye == nil || bye["reason"] != ByeInactive {
		t.Fatalf("expected a bye inactive, got %v", bye)
	}
	select {
	case <-member.Done():
		if code, _ := member.CloseStatus(); code != CloseClosed {
			t.Fatalf("close code %d", code)
		}
	default:
		t.Fatal("the connection of an inactive lobby stayed open")
	}
	if e.store.Lobby(rowID) != nil || e.isLoaded(id) {
		t.Fatal("the inactive lobby was not deleted")
	}
	if _, err := e.hub.JoinByCode(e.actor(carol), code); !apperrors.IsNotFound(err) {
		t.Fatalf("the invite code of an inactive lobby still works: %v", err)
	}
}

func TestPlaybackKeepsTheLobbyActive(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) {
		c.InactiveCloseAfter = time.Hour
		c.CloseWarning = 5 * time.Minute
	})
	id := e.lobby(CreateInput{}, 3) // unknown duration: plays until paused
	rowID := e.rowID(id)
	host := e.connect(id, alice)
	e.send(host, map[string]any{"t": "play"})
	for i := 0; i < 12; i++ {
		e.clock.advance(10 * time.Minute)
		e.hub.Tick()
	}
	if e.store.Lobby(rowID) == nil || last(drain(host), msgClosing) != nil {
		t.Fatal("a lobby playing was warned or closed")
	}
	e.send(host, map[string]any{"t": "pause"})
	e.clock.advance(59 * time.Minute)
	e.hub.Tick()
	if e.store.Lobby(rowID) == nil {
		t.Fatal("closed before inactive_close_after after the pause")
	}
	e.clock.advance(time.Minute)
	e.hub.Tick()
	if e.store.Lobby(rowID) != nil {
		t.Fatal("not closed after inactive_close_after")
	}
}

// --- auto-next ---

func TestAutoNextStop(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{AutoNext: data.AutoNextStop}, 1, 2)
	host := e.connect(id, alice)
	q, _ := e.queue(id)
	e.send(host, map[string]any{"t": "play", "position": 99})
	e.clock.advance(3 * time.Second)
	e.hub.Tick()
	if s := e.state(id); s.itemID != q[0] || s.playing || !near(e.pos(id), 100) {
		t.Fatalf("stop: expected item 1 paused at its end, got %+v", s)
	}
	if got, _ := e.queue(id); len(got) != 2 {
		t.Fatalf("stop dropped the item: %v", got)
	}
	// Playing again at the end starts the item over
	e.send(host, map[string]any{"t": "play", "position": 100})
	if s := e.state(id); s.itemID != q[0] || !s.playing || !near(s.position, 0) {
		t.Fatalf("play at the end did not start over: %+v", s)
	}
	// Skipping still goes to the next item
	e.send(host, map[string]any{"t": "skip"})
	if s := e.state(id); s.itemID != q[1] {
		t.Fatalf("skip: %+v", s)
	}
}

func TestAutoNextRepeatOne(t *testing.T) {
	e := newEnv(t)
	start, end := 10.0, 20.0
	s, err := e.hub.CreateLobby(e.actor(alice), CreateInput{AutoNext: data.AutoNextRepeatOne, Items: []ItemInput{
		{SceneID: 2, Start: &start, End: &end}, {SceneID: 3}, {SceneID: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(s.ID)
	host := e.connect(id, alice)
	q, _ := e.queue(id)
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(10 * time.Second)
	e.hub.Tick()
	if st := e.state(id); st.itemID != q[0] || !st.playing || !near(e.pos(id), 10) {
		t.Fatalf("repeat_one: the range did not start over: %+v", st)
	}
	if got, _ := e.queue(id); len(got) != 3 {
		t.Fatalf("repeat_one dropped the item: %v", got)
	}

	// Unknown duration: the controller's ended report loops it; a stale report of the
	// previous loop does not loop it again
	e.send(host, map[string]any{"t": "skip"})
	if st := e.state(id); st.itemID != q[1] {
		t.Fatalf("skip in repeat_one: %+v", st)
	}
	e.clock.advance(30 * time.Second)
	e.send(host, map[string]any{"t": "ended", "item_id": q[1]})
	seq := e.state(id).seq
	if !near(e.pos(id), 0) {
		t.Fatalf("ended did not loop: %v", e.pos(id))
	}
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "ended", "item_id": q[1]})
	if e.state(id).seq != seq {
		t.Fatal("a stale ended report looped again")
	}
	e.clock.advance(5 * time.Second)
	e.send(host, map[string]any{"t": "ended", "item_id": q[1]})
	if e.state(id).seq == seq {
		t.Fatal("a real ended report after the grace did not loop")
	}
}

func TestAutoNextRepeatQueue(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{AutoNext: data.AutoNextRepeatQueue}, 1, 2, 3)
	host := e.connect(id, alice)
	q, _ := e.queue(id)
	rowID := e.rowID(id)

	e.send(host, map[string]any{"t": "play", "position": 99})
	e.clock.advance(3 * time.Second)
	e.hub.Tick()
	want := []uint{q[1], q[2], q[0]}
	if got, _ := e.queue(id); fmt.Sprint(got) != fmt.Sprint(want) || e.state(id).itemID != q[1] {
		t.Fatalf("repeat_queue end: got %v (playing %d), want %v", got, e.state(id).itemID, want)
	}
	if got := e.store.Queue(rowID); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stored order %v want %v", got, want)
	}
	// Skip keeps the item too
	e.send(host, map[string]any{"t": "skip"})
	want = []uint{q[2], q[0], q[1]}
	if got, _ := e.queue(id); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("repeat_queue skip: got %v want %v", got, want)
	}
	// Playing an item turns the queue (nothing dropped)
	e.send(host, map[string]any{"t": "play_item", "item_id": q[1]})
	want = []uint{q[1], q[2], q[0]}
	if got, _ := e.queue(id); fmt.Sprint(got) != fmt.Sprint(want) || e.state(id).itemID != q[1] {
		t.Fatalf("repeat_queue play_item: got %v want %v", got, want)
	}
	// Removing the current item drops it
	e.send(host, map[string]any{"t": "queue_remove", "item_id": q[1]})
	want = []uint{q[2], q[0]}
	if got, _ := e.queue(id); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("repeat_queue remove: got %v want %v", got, want)
	}
	// A single item loops
	e.send(host, map[string]any{"t": "queue_remove", "item_id": q[2]})
	e.send(host, map[string]any{"t": "play", "position": 99})
	e.clock.advance(3 * time.Second)
	e.hub.Tick()
	if st := e.state(id); st.itemID != q[0] || !st.playing || !near(e.pos(id), 0) {
		t.Fatalf("repeat_queue single item: %+v", st)
	}
}

func TestAutoNextCommand(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	member := e.connect(id, bob)
	if w := e.loaded(id).info(); w.AutoNext != data.AutoNextNext {
		t.Fatalf("default mode %q", w.AutoNext)
	}
	e.expectError(member, map[string]any{"t": "auto_next", "mode": "stop"}, ErrCodeNotAllowed)
	e.expectError(host, map[string]any{"t": "auto_next", "mode": "shuffle"}, ErrCodeInvalid)

	// Whoever has control changes it, everyone gets it, it is saved
	e.send(host, map[string]any{"t": "grant_control", "user_id": bob})
	drain(host)
	e.send(member, map[string]any{"t": "auto_next", "mode": "repeat_one"})
	lobbyMsg := last(drain(host), msgLobby)
	if lobbyMsg == nil || lobbyMsg["lobby"].(map[string]any)["auto_next"] != "repeat_one" {
		t.Fatalf("lobby message %v", lobbyMsg)
	}
	if row := e.store.Lobby(e.rowID(id)); row.AutoNext != data.AutoNextRepeatOne {
		t.Fatalf("mode not saved: %q", row.AutoNext)
	}
	if s, _ := e.hub.GetLobby(e.actor(carol), id); s.AutoNext != data.AutoNextRepeatOne {
		t.Fatalf("summary mode %q", s.AutoNext)
	}
	if _, err := e.hub.CreateLobby(e.actor(alice), CreateInput{AutoNext: "shuffle"}); !apperrors.IsValidation(err) {
		t.Fatalf("an unknown mode was accepted: %v", err)
	}
}

// --- anonymous members ---

// leaks reports the frames that carry bob's identity.
func leaks(frames []string) []string {
	var out []string
	for _, f := range frames {
		if strings.Contains(f, `"bob"`) || strings.Contains(f, "(bob)") ||
			strings.Contains(f, `"user_id":2,`) || strings.Contains(f, `"user_id":2}`) ||
			strings.Contains(f, `"added_by":{"id":2`) {
			out = append(out, f)
		}
	}
	return out
}

func TestAnonymousMemberIsRedacted(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	carolC := e.connect(id, carol)
	modC := e.connect(id, mod)

	s, err := e.hub.JoinLobby(e.actor(bob), id, true)
	if err != nil || !s.Anonymous || !s.IsMember {
		t.Fatalf("anonymous join: %+v %v", s, err)
	}
	// Connecting afterwards (without asking) keeps the anonymity
	bobC := e.connect(id, bob)
	w := last(drain(bobC), msgWelcome)
	you := w["you"].(map[string]any)
	if you["anonymous"] != true || you["anon_number"] != float64(1) || you["user_id"] != float64(bob) {
		t.Fatalf("welcome you: %v", you)
	}
	e.send(bobC, map[string]any{"t": "chat", "text": "hi"})
	e.send(bobC, map[string]any{"t": "reaction", "emote": "🔥"})
	e.send(bobC, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 2}}}) // a suggestion
	e.send(host, map[string]any{"t": "grant_control", "anon": 1})

	for name, c := range map[string]*Client{"host": host, "carol": carolC} {
		frames := drainRaw(c)
		if l := leaks(frames); len(l) > 0 {
			t.Fatalf("%s got bob's identity: %v", name, l)
		}
		joined := strings.Join(frames, "\n")
		for _, want := range []string{"Anonymous #1 joined", `"username":"Anonymous #1"`, `"anon_number":1`} {
			if !strings.Contains(joined, want) {
				t.Fatalf("%s did not get %q: %s", name, want, joined)
			}
		}
	}
	// A moderator sees who it is, with the anonymous flag
	modFrames := strings.Join(drainRaw(modC), "\n")
	for _, want := range []string{"Anonymous #1 (bob) joined", `"user_id":2,"username":"bob","anonymous":true`, `"added_by":{"id":2,"username":"bob","anonymous":true`} {
		if !strings.Contains(modFrames, want) {
			t.Fatalf("the moderator did not get %q: %s", want, modFrames)
		}
	}
	// bob has control now (addressed by number)
	if !e.loaded(id).canControl(bob) {
		t.Fatal("grant_control by anon number failed")
	}
	// A late joiner's welcome is redacted too (members, chat, suggestions)
	if l := leaks(drainRaw(e.connect(id, carol))); len(l) > 0 {
		t.Fatalf("a welcome leaks bob: %v", l)
	}
	// Presence never shows an anonymous membership
	if active := e.hub.ActiveLobbiesForUser(bob); len(active) != 0 {
		t.Fatalf("an anonymous membership is active in presence: %+v", active)
	}
	if active := e.hub.ActiveLobbiesForUser(carol); len(active) != 1 {
		t.Fatalf("carol's active lobbies: %+v", active)
	}

	// bob leaves: the suggestion stays anonymous, also after a reload from the database
	e.hub.Disconnect(bobC)
	if err := e.hub.LeaveLobby(e.actor(bob), id); err != nil {
		t.Fatal(err)
	}
	e.restart()
	if l := leaks(drainRaw(e.connect(id, carol))); len(l) > 0 {
		t.Fatalf("a welcome after a reload leaks bob: %v", l)
	}
}

func TestAnonymousTargetsAreHidden(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	if _, err := e.hub.Connect(e.actor(bob), id, true); err != nil {
		t.Fatal(err)
	}
	modC := e.connect(id, mod)
	// The host cannot find out who is behind an anonymous member by trying user IDs: an
	// anonymous member named by user_id is "no such member", like a non-member
	for _, msg := range []map[string]any{
		{"t": "grant_control", "user_id": bob}, {"t": "transfer_host", "user_id": bob},
		{"t": "kick", "user_id": bob}, {"t": "kick", "user_id": bob, "ban": true},
		{"t": "kick", "user_id": carol, "ban": true},
	} {
		e.expectError(host, msg, ErrCodeNotFound)
	}
	e.expectError(host, map[string]any{"t": "transfer_host", "anon": 1}, ErrCodeNotAllowed)
	// Moderators can name them by user_id
	e.send(modC, map[string]any{"t": "kick", "user_id": bob})
	if e.loaded(id).members[bob] != nil {
		t.Fatal("the moderator could not kick the anonymous member")
	}
}

func TestAnonymousJoinRules(t *testing.T) {
	e := newEnv(t)
	priv := e.lobby(CreateInput{Name: "Private"}, 1)
	pub := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	if _, err := e.hub.JoinLobby(e.actor(mod), priv, true); !apperrors.IsValidation(err) {
		t.Fatalf("anonymous join of a private lobby: %v", err)
	}
	if _, err := e.hub.Connect(e.actor(mod), priv, true); !apperrors.IsValidation(err) {
		t.Fatalf("anonymous connect to a private lobby: %v", err)
	}
	// The creator cannot hide, even after leaving
	if _, err := e.hub.JoinLobby(e.actor(bob), pub, false); err != nil {

		t.Fatal(err)
	}
	if err := e.hub.LeaveLobby(e.actor(alice), pub); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hub.JoinLobby(e.actor(alice), pub, true); !apperrors.IsValidation(err) {
		t.Fatalf("the creator joined anonymously: %v", err)
	}
	// A named member cannot turn anonymous; an anonymous one stays anonymous
	if _, err := e.hub.JoinLobby(e.actor(bob), pub, true); !apperrors.IsConflict(err) {
		t.Fatalf("a named member turned anonymous: %v", err)
	}
	if s, err := e.hub.JoinLobby(e.actor(carol), pub, true); err != nil || !s.Anonymous {
		t.Fatalf("anonymous join: %+v %v", s, err)
	}
	if s, err := e.hub.JoinLobby(e.actor(carol), pub, false); err != nil || !s.Anonymous {
		t.Fatalf("an anonymous member was revealed by joining again: %+v %v", s, err)
	}
	// Numbers are not reused within a lobby
	if err := e.hub.LeaveLobby(e.actor(carol), pub); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hub.JoinLobby(e.actor(carol), pub, true); err != nil {
		t.Fatal(err)
	}
	if m := e.loaded(pub).members[carol]; m == nil || m.anon != 2 {
		t.Fatalf("expected Anonymous #2, got %+v", m)
	}
	list, _ := e.hub.ListLobbies(e.actor(carol), false)
	if len(list) != 1 || !list[0].Anonymous {
		t.Fatalf("list: %+v", list)
	}
}

func TestHostTransferSkipsAnonymousMembers(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	bobC, err := e.hub.Connect(e.actor(bob), id, true)
	if err != nil {
		t.Fatal(err)
	}
	e.send(host, map[string]any{"t": "grant_control", "anon": 1})
	carolC := e.connect(id, carol)

	// Automatic transfer: bob has control but is anonymous, carol gets it
	e.hub.Disconnect(host)
	e.clock.advance(61 * time.Second)
	e.hub.Tick()
	if l := e.loaded(id); !l.isHost(carol) {
		t.Fatalf("expected carol to be the host, got %v", l.row.HostUserID)
	}
	// Only anonymous members left: no host, every member controls
	e.hub.Disconnect(carolC)
	if err := e.hub.LeaveLobby(e.actor(carol), id); err != nil {
		t.Fatal(err)
	}
	if err := e.hub.LeaveLobby(e.actor(alice), id); err != nil {
		t.Fatal(err)
	}
	if l := e.loaded(id); l.row.HostUserID != nil {
		t.Fatalf("an anonymous member became the host: %v", *l.row.HostUserID)
	}
	e.send(bobC, map[string]any{"t": "play"})
	if !e.state(id).playing {
		t.Fatal("a member of a lobby without host could not play")
	}
	// A named member arriving takes the role
	e.connect(id, carol)
	e.hub.Tick()
	if !e.loaded(id).isHost(carol) {
		t.Fatal("the named member did not become the host")
	}
	if msgs := all(drain(bobC), msgLobby); len(msgs) == 0 {
		t.Fatal("no lobby message about the host")
	}
}

func TestKeepOpenCallsTheCloseOff(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) {
		c.InactiveCloseAfter = 10 * time.Minute
		c.CloseWarning = 2 * time.Minute
	})
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	member := e.connect(id, bob) // no control: keep_open needs none
	e.clock.advance(8 * time.Minute)
	e.hub.Tick()
	if m := last(drain(host), msgClosing); m == nil || m["at"] == nil {
		t.Fatalf("expected the announce, got %v", m)
	}
	e.send(member, map[string]any{"t": "keep_open"})
	if m := last(drain(host), msgClosing); m == nil || m["at"] != nil {
		t.Fatalf("keep_open did not call the close off: %v", m)
	}
	e.clock.advance(9 * time.Minute)
	e.hub.Tick()
	if !e.isLoaded(id) {
		t.Fatal("closed despite keep_open")
	}
}
