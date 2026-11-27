package together

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/together/togethertest"

	"github.com/google/uuid"
)

func withHistory(n int) func(*config.Goon2getherConfig) {
	return func(c *config.Goon2getherConfig) { c.HistoryMax = n }
}

// history gives the in-memory history of a lobby, oldest first, as "scene:outcome".
func (e *env) history(id uuid.UUID) []string {
	l := e.loaded(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.history))
	for _, h := range l.history {
		out = append(out, fmt.Sprintf("%d:%s", h.it.sceneID, h.outcome))
	}
	return out
}

// storedHistory gives the stored history of a lobby, oldest first, as "scene:outcome".
func (e *env) storedHistory(id uuid.UUID) []string {
	var out []string
	for _, h := range e.store.History(e.rowID(id)) {
		out = append(out, fmt.Sprintf("%d:%s", h.SceneID, h.Outcome))
	}
	return out
}

func expectHistory(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s: history %v, want %v", what, got, want)
	}
}

func TestHistoryRecordsEndsAndSkips(t *testing.T) {
	e := newEnv(t, withHistory(10))
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2, 3)
	host := e.connect(id, alice)
	start := e.clock.now()

	// Item 1 plays to its end: completed, broadcast to the room
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(102 * time.Second)
	drain(host)
	e.hub.Tick()
	added := last(drain(host), msgHistoryAdd)
	if added == nil {
		t.Fatal("no history_add at the end of an item")
	}
	entry := added["entry"].(map[string]any)
	if entry["scene_id"] != float64(1) || entry["title"] != "One" || entry["outcome"] != data.HistoryCompleted ||
		entry["available"] != true || entry["started_at"] != float64(start.UnixMilli()) ||
		entry["ended_at"] != float64(e.clock.now().UnixMilli()) || entry["added_by"].(map[string]any)["username"] != "alice" {
		t.Fatalf("history entry: %v", entry)
	}

	// Item 2 is skipped while playing
	e.clock.advance(10 * time.Second)
	e.send(host, map[string]any{"t": "skip"})
	expectHistory(t, "after a skip", e.history(id), "1:completed", "2:skipped")
	expectHistory(t, "stored", e.storedHistory(id), "1:completed", "2:skipped")

	// A late joiner gets it newest first, and the history size in the config
	w := last(drain(e.connect(id, bob)), msgWelcome)
	hist := w["history"].([]any)
	if len(hist) != 2 || hist[0].(map[string]any)["scene_id"] != float64(2) || hist[1].(map[string]any)["scene_id"] != float64(1) {
		t.Fatalf("welcome history: %v", hist)
	}
	if w["config"].(map[string]any)["history_max"] != float64(10) {
		t.Fatalf("welcome config: %v", w["config"])
	}

	// The last item ends: the queue is finished, the item is recorded
	e.send(host, map[string]any{"t": "ended", "item_id": e.state(id).itemID})
	expectHistory(t, "queue finished", e.history(id), "1:completed", "2:skipped", "3:completed")
	if e.state(id).itemID != 0 {
		t.Fatal("the queue did not finish")
	}
}

func TestHistorySkipsItemsNeverPlayed(t *testing.T) {
	e := newEnv(t, withHistory(10), func(c *config.Goon2getherConfig) { c.HistoryMinPlay = 3 * time.Second })
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2, 3)
	host := e.connect(id, alice)
	q, _ := e.queue(id)

	// The first item is current but paused: removing it does not record it (2 plays)
	e.send(host, map[string]any{"t": "queue_remove", "item_id": q[0]})
	expectHistory(t, "never played", e.history(id))
	// Skipped at once (shorter than history_min_play): not recorded (3 plays)
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "skip"})
	expectHistory(t, "skipped through", e.history(id))

	// Played, then paused, then removed: recorded as skipped
	e.clock.advance(5 * time.Second)
	e.send(host, map[string]any{"t": "pause"})
	e.send(host, map[string]any{"t": "queue_remove", "item_id": q[2]})
	expectHistory(t, "removed while paused", e.history(id), "3:skipped")
}

func TestHistoryPlayNowAndPlayItem(t *testing.T) {
	e := newEnv(t, withHistory(10))
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2, 3)
	host := e.connect(id, alice)
	q, _ := e.queue(id)

	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(5 * time.Second)
	// Starting the item playing over is the same play
	e.send(host, map[string]any{"t": "play_item", "item_id": q[0]})
	expectHistory(t, "restart", e.history(id))
	// Playing another item ends the play
	e.send(host, map[string]any{"t": "play_item", "item_id": q[2]})
	expectHistory(t, "play_item", e.history(id), "1:skipped")
	// play_now replaces the item playing
	e.clock.advance(5 * time.Second)
	e.send(host, map[string]any{"t": "queue_add", "play_now": true, "items": []map[string]any{{"scene_id": 1}}})
	expectHistory(t, "play_now", e.history(id), "1:skipped", "3:skipped")
	if st := e.state(id); !st.playing {
		t.Fatal("play_now did not play")
	}
}

func TestHistoryAutoNextModes(t *testing.T) {
	t.Run("repeat_one counts once", func(t *testing.T) {
		e := newEnv(t, withHistory(10))
		start, end := 10.0, 20.0
		s, err := e.hub.CreateLobby(e.actor(alice), CreateInput{AutoNext: data.AutoNextRepeatOne, Items: []ItemInput{
			{SceneID: 2, Start: &start, End: &end}, {SceneID: 1},
		}})
		if err != nil {
			t.Fatal(err)
		}
		id := uuid.MustParse(s.ID)
		host := e.connect(id, alice)
		e.send(host, map[string]any{"t": "play"})
		for i := 0; i < 3; i++ {
			e.clock.advance(10 * time.Second)
			e.hub.Tick()
		}
		expectHistory(t, "loops", e.history(id))
		e.send(host, map[string]any{"t": "skip"})
		expectHistory(t, "skip after loops", e.history(id), "2:completed")
		l := e.loaded(id)
		l.mu.Lock()
		got := l.history[0]
		l.mu.Unlock()
		if got.it.start == nil || *got.it.start != 10 || got.it.end == nil || *got.it.end != 20 {
			t.Fatalf("the range was not kept: %+v", got.it)
		}
	})

	t.Run("repeat_queue records each play", func(t *testing.T) {
		e := newEnv(t, withHistory(10))
		id := e.lobby(CreateInput{AutoNext: data.AutoNextRepeatQueue}, 1, 2)
		host := e.connect(id, alice)
		e.send(host, map[string]any{"t": "play", "position": 99})
		e.clock.advance(3 * time.Second)
		e.hub.Tick()                              // 1 ends, 2 plays
		e.send(host, map[string]any{"t": "skip"}) // 2 skipped, 1 plays again
		e.send(host, map[string]any{"t": "play", "position": 99})
		e.clock.advance(3 * time.Second)
		e.hub.Tick() // 1 ends again
		expectHistory(t, "repeat_queue", e.history(id), "1:completed", "2:skipped", "1:completed")
		// A single item looping is one play
		q, _ := e.queue(id) // 2 (playing), 1
		e.send(host, map[string]any{"t": "queue_remove", "item_id": q[0]})
		expectHistory(t, "remove", e.history(id), "1:completed", "2:skipped", "1:completed", "2:skipped")
		for i := 0; i < 2; i++ {
			e.send(host, map[string]any{"t": "play", "position": 99})
			e.clock.advance(3 * time.Second)
			e.hub.Tick()
		}
		expectHistory(t, "single item loops", e.history(id), "1:completed", "2:skipped", "1:completed", "2:skipped")
	})

	t.Run("stop records when the item changes", func(t *testing.T) {
		e := newEnv(t, withHistory(10))
		id := e.lobby(CreateInput{AutoNext: data.AutoNextStop}, 1, 2)
		host := e.connect(id, alice)
		e.send(host, map[string]any{"t": "play", "position": 99})
		e.clock.advance(3 * time.Second)
		e.hub.Tick() // paused at the end
		expectHistory(t, "stopped", e.history(id))
		e.send(host, map[string]any{"t": "play", "position": 100}) // starts over
		e.clock.advance(time.Second)
		e.send(host, map[string]any{"t": "skip"})
		expectHistory(t, "skip after the end", e.history(id), "1:completed")
	})
}

func TestHistoryIsBounded(t *testing.T) {
	e := newEnv(t, withHistory(2))
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2, 3, 1)
	host := e.connect(id, alice)
	for i := 0; i < 3; i++ {
		e.send(host, map[string]any{"t": "play"})
		e.clock.advance(time.Second)
		e.send(host, map[string]any{"t": "skip"})
	}
	expectHistory(t, "memory", e.history(id), "2:skipped", "3:skipped")
	expectHistory(t, "stored", e.storedHistory(id), "2:skipped", "3:skipped")
	if w := last(drain(e.connect(id, bob)), msgWelcome); len(w["history"].([]any)) != 2 {
		t.Fatalf("welcome history: %v", w["history"])
	}
}

func TestHistoryOff(t *testing.T) {
	e := newEnv(t) // history_max 0
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2)
	host := e.connect(id, alice)
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "skip"})
	if got := e.storedHistory(id); len(got) != 0 {
		t.Fatalf("history recorded while off: %v", got)
	}
	w := last(drain(e.connect(id, bob)), msgWelcome)
	if h, ok := w["history"].([]any); !ok || len(h) != 0 || w["config"].(map[string]any)["history_max"] != float64(0) {
		t.Fatalf("welcome without history: %v %v", w["history"], w["config"])
	}
}

func TestHistoryAnonymousAdderIsRedacted(t *testing.T) {
	e := newEnv(t, withHistory(10))
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic, QueueOpen: true}, 1)
	host := e.connect(id, alice)
	modC := e.connect(id, mod)
	bobC, err := e.hub.Connect(e.actor(bob), id, true)
	if err != nil {
		t.Fatal(err)
	}
	e.send(bobC, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 2}}})
	q, _ := e.queue(id)
	e.send(host, map[string]any{"t": "play_item", "item_id": q[1]})
	drainRaw(host)
	drainRaw(modC)
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "skip"})

	frames := drainRaw(host)
	if l := leaks(frames); len(l) > 0 {
		t.Fatalf("the host got bob's identity: %v", l)
	}
	joined := strings.Join(frames, "\n")
	if !strings.Contains(joined, `"t":"history_add"`) || !strings.Contains(joined, `"added_by":{"id":null,"username":"Anonymous #1","anonymous":true,"anon_number":1}`) {
		t.Fatalf("redacted history entry not sent: %s", joined)
	}
	if mf := strings.Join(drainRaw(modC), "\n"); !strings.Contains(mf, `"added_by":{"id":2,"username":"bob","anonymous":true,"anon_number":1}`) {
		t.Fatalf("the moderator did not get the real adder: %s", mf)
	}
	// Welcomes too, also after a reload from the database
	if l := leaks(drainRaw(e.connect(id, carol))); len(l) > 0 {
		t.Fatalf("a welcome leaks bob: %v", l)
	}
	e.hub.Disconnect(bobC)
	e.restart()
	frames = drainRaw(e.connect(id, carol))
	if l := leaks(frames); len(l) > 0 {
		t.Fatalf("a welcome after a reload leaks bob: %v", l)
	}
	if !strings.Contains(strings.Join(frames, "\n"), `"username":"Anonymous #1","anonymous":true,"anon_number":1}`) {
		t.Fatalf("the reloaded history lost the anonymous adder: %v", frames)
	}
}

func TestHistorySurvivesRestartAndUnload(t *testing.T) {
	e := newEnv(t, withHistory(10))
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2, 3)
	host := e.connect(id, alice)
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "skip"}) // 1 recorded
	playStart := e.clock.now()
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(30 * time.Second)

	// Restart while item 2 plays: the play goes on, it is recorded once, with its start
	e.restart()
	host = e.connect(id, alice)
	expectHistory(t, "reloaded", e.history(id), "1:skipped")
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "skip"})
	expectHistory(t, "after the restart", e.history(id), "1:skipped", "2:skipped")
	l := e.loaded(id)
	l.mu.Lock()
	got := l.history[1].startedAt
	l.mu.Unlock()
	if !got.Equal(playStart) {
		t.Fatalf("play start %v, want %v", got, playStart)
	}

	// Item 3 reaches its end (unknown duration: the controller reports it) but stop mode
	// keeps it; an unload keeps the completed flag
	e.send(host, map[string]any{"t": "auto_next", "mode": "stop"})
	e.send(host, map[string]any{"t": "play"})
	e.send(host, map[string]any{"t": "ended", "item_id": e.state(id).itemID})
	e.hub.Disconnect(host)
	e.clock.advance(2 * time.Minute)
	e.hub.Tick()
	if e.isLoaded(id) {
		t.Fatal("not unloaded")
	}
	host = e.connect(id, alice)
	e.send(host, map[string]any{"t": "skip"})
	expectHistory(t, "after an unload", e.history(id), "1:skipped", "2:skipped", "3:completed")
}

func TestHistoryAvailabilityAndClose(t *testing.T) {
	e := newEnv(t, withHistory(10))
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1, 2)
	host := e.connect(id, alice)
	e.send(host, map[string]any{"t": "play"})
	e.clock.advance(time.Second)
	e.send(host, map[string]any{"t": "skip"})

	// Scene 1 goes to the trash: the next snapshot shows it unavailable, requeueing it fails
	delete(e.hub.scenes.(togethertest.Scenes), 1)
	w := last(drain(e.connect(id, bob)), msgWelcome)
	if h := w["history"].([]any); len(h) != 1 || h[0].(map[string]any)["available"] != false {
		t.Fatalf("trashed scene still available: %v", h)
	}
	e.expectError(host, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 1}}}, ErrCodeNotFound)

	// Closing the lobby deletes its history
	rowID := e.rowID(id)
	if err := e.hub.CloseLobby(e.actor(alice), id); err != nil {
		t.Fatal(err)
	}
	if got := e.store.History(rowID); len(got) != 0 {
		t.Fatalf("history left after the close: %+v", got)
	}
}
