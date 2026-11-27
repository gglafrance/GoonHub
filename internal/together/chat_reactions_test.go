package together

import (
	"strings"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
)

// chatID sends a chat message from c and returns its id (as received by the sender).
func (e *env) chatID(c *Client, text string) float64 {
	e.t.Helper()
	drain(c)
	e.send(c, map[string]any{"t": "chat", "text": text})
	m := last(drain(c), msgChat)
	if m == nil {
		e.t.Fatalf("chat %q was not sent", text)
	}
	return m["message"].(map[string]any)["id"].(float64)
}

// reactionsOf reads the reactions of a chat_reactions message as emote -> usernames.
func reactionsOf(t *testing.T, msg map[string]any) map[string][]string {
	t.Helper()
	if msg == nil {
		t.Fatal("no chat_reactions message")
	}
	out := map[string][]string{}
	list, ok := msg["reactions"].([]any)
	if !ok {
		t.Fatalf("reactions is not a list: %v", msg)
	}
	for _, r := range list {
		rm := r.(map[string]any)
		var names []string
		for _, u := range rm["users"].([]any) {
			names = append(names, u.(map[string]any)["username"].(string))
		}
		if int(rm["count"].(float64)) != len(names) {
			t.Fatalf("count %v for users %v", rm["count"], names)
		}
		out[rm["emote"].(string)] = names
	}
	return out
}

func TestChatReactionsToggleAndSnapshot(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	bobC := e.connect(id, bob)
	carolC := e.connect(id, carol)
	msgID := e.chatID(carolC, "hello")
	drain(host)
	drain(bobC)

	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "🔥"})
	for name, c := range map[string]*Client{"host": host, "bob": bobC, "carol": carolC} {
		got := last(drain(c), msgChatReactions)
		if got == nil || got["id"] != msgID {
			t.Fatalf("%s got %v", name, got)
		}
		if r := reactionsOf(t, got); len(r) != 1 || strings.Join(r["🔥"], ",") != "bob" {
			t.Fatalf("%s reactions %v", name, r)
		}
	}
	// Reacting again with the same key changes nothing and sends nothing
	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "🔥"})
	if got := last(drain(host), msgChatReactions); got != nil {
		t.Fatalf("a no-op reaction was broadcast: %v", got)
	}
	// Grouped in reaction order, a server emote too
	e.send(host, map[string]any{"t": "chat_react", "id": msgID, "emote": "🔥"})
	e.send(host, map[string]any{"t": "chat_react", "id": msgID, "emote": ":pog:"})
	r := reactionsOf(t, last(drain(carolC), msgChatReactions))
	if strings.Join(r["🔥"], ",") != "bob,alice" || strings.Join(r[":pog:"], ",") != "alice" {
		t.Fatalf("grouped reactions %v", r)
	}

	// Late joiners get the reactions in the chat snapshot
	w := last(drain(e.connect(id, mod)), msgWelcome)
	var snap map[string]any
	for _, m := range w["chat"].([]any) {
		if mm := m.(map[string]any); mm["id"] == msgID {
			snap = mm
		}
	}
	if r := reactionsOf(t, snap); strings.Join(r["🔥"], ",") != "bob,alice" || len(r) != 2 {
		t.Fatalf("welcome reactions %v", snap)
	}

	// Removing one's own reaction; the last reactor removes the key; a missing one is a no-op
	e.send(bobC, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "🔥"})
	if r := reactionsOf(t, last(drain(host), msgChatReactions)); strings.Join(r["🔥"], ",") != "alice" {
		t.Fatalf("after bob's removal %v", r)
	}
	e.send(bobC, map[string]any{"t": "chat_unreact", "id": msgID, "emote": ":pog:"})
	if got := last(drain(host), msgChatReactions); got != nil {
		t.Fatalf("removing someone else's reaction was broadcast: %v", got)
	}
	e.send(host, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "🔥"})
	e.send(host, map[string]any{"t": "chat_unreact", "id": msgID, "emote": ":pog:"})
	final := last(drain(carolC), msgChatReactions)
	if list, ok := final["reactions"].([]any); !ok || len(list) != 0 {
		t.Fatalf("no reaction left should be an empty list: %v", final)
	}

	// Refusals
	e.expectError(bobC, map[string]any{"t": "chat_react", "emote": "🔥"}, ErrCodeInvalid)
	e.expectError(bobC, map[string]any{"t": "chat_react", "id": msgID}, ErrCodeInvalid)
	e.expectError(bobC, map[string]any{"t": "chat_react", "id": 9999, "emote": "🔥"}, ErrCodeNotFound)
	e.expectError(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "lol"}, ErrCodeInvalid)
	e.expectError(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": ":nope:"}, ErrCodeNotFound)
	var systemID float64
	for _, m := range w["chat"].([]any) {
		if mm := m.(map[string]any); mm["system"] == true {
			systemID = mm["id"].(float64)
		}
	}
	if systemID == 0 {
		t.Fatal("no system message in the snapshot")
	}
	e.expectError(bobC, map[string]any{"t": "chat_react", "id": systemID, "emote": "🔥"}, ErrCodeNotAllowed)

	// A deleted message takes its reactions with it
	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "😂"})
	e.send(carolC, map[string]any{"t": "chat_delete", "id": msgID})
	e.expectError(bobC, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "😂"}, ErrCodeNotFound)
	for _, m := range last(drain(e.connect(id, carol)), msgWelcome)["chat"].([]any) {
		if m.(map[string]any)["id"] == msgID {
			t.Fatal("a deleted message is still in the snapshot")
		}
	}
}

func TestChatReactionCapsAndRateLimit(t *testing.T) {
	e := newEnv(t, func(c *config.Goon2getherConfig) {
		c.ChatMaxReactionKinds = 2
		c.ChatMaxReactionsPerUser = 2
		c.ChatReactionRateLimit = 4
	})
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	bobC := e.connect(id, bob)
	msgID := e.chatID(host, "react to me")

	if w := last(drain(e.connect(id, carol)), msgWelcome); w["config"].(map[string]any)["chat_max_reaction_kinds"] != float64(2) {
		t.Fatalf("welcome config %v", w["config"])
	}
	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "🔥"})
	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "😂"})
	// a third key: too many kinds, for everyone
	e.expectError(host, map[string]any{"t": "chat_react", "id": msgID, "emote": "👍"}, ErrCodeReactionLimit)
	// an existing key is fine
	e.send(host, map[string]any{"t": "chat_react", "id": msgID, "emote": "😂"})
	if r := reactionsOf(t, last(drain(bobC), msgChatReactions)); len(r["😂"]) != 2 {
		t.Fatalf("joining an existing key %v", r)
	}
	// bob has 2 reactions: refused, before the rate limit counts it
	e.send(host, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "😂"})
	e.expectError(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "👍"}, ErrCodeReactionLimit)

	// rate limit: bob used 2 of 4, two more pass, the next is refused until the window moves
	e.send(bobC, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "😂"})
	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "😂"})
	e.expectError(bobC, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "😂"}, ErrCodeRateLimited)
	e.clock.advance(10 * time.Second)
	drain(bobC)
	e.send(bobC, map[string]any{"t": "chat_unreact", "id": msgID, "emote": "😂"})
	if got := last(drain(bobC), msgError); got != nil {
		t.Fatalf("after the window: %v", got)
	}
	// the floating reactions keep their own limiter and message
	e.send(bobC, map[string]any{"t": "reaction", "emote": "🔥"})
	if got := last(drain(host), msgReaction); got == nil || got["emote"] != "🔥" {
		t.Fatalf("floating reaction %v", got)
	}
}

func TestChatReactionsRedactAnonymousReactors(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Visibility: data.LobbyVisibilityPublic}, 1)
	host := e.connect(id, alice)
	carolC := e.connect(id, carol)
	modC := e.connect(id, mod)
	if _, err := e.hub.JoinLobby(e.actor(bob), id, true); err != nil {
		t.Fatal(err)
	}
	bobC := e.connect(id, bob)
	msgID := e.chatID(carolC, "hi all")
	drainRaw(host)
	drainRaw(modC)

	e.send(bobC, map[string]any{"t": "chat_react", "id": msgID, "emote": "🔥"})
	for name, c := range map[string]*Client{"host": host, "carol": carolC} {
		frames := drainRaw(c)
		if l := leaks(frames); len(l) > 0 {
			t.Fatalf("%s got bob's identity: %v", name, l)
		}
		joined := strings.Join(frames, "\n")
		if !strings.Contains(joined, `"users":[{"id":null,"username":"Anonymous #1","anonymous":true,"anon_number":1}]`) {
			t.Fatalf("%s did not get the anonymous reactor: %s", name, joined)
		}
	}
	// the anonymous reactor recognizes their own reaction by number
	if own := strings.Join(drainRaw(bobC), "\n"); !strings.Contains(own, `"username":"Anonymous #1","anonymous":true,"anon_number":1`) {
		t.Fatalf("bob did not get his anonymous reaction: %s", own)
	}
	// moderators see who it is
	if mod := strings.Join(drainRaw(modC), "\n"); !strings.Contains(mod, `"users":[{"id":2,"username":"bob","anonymous":true,"anon_number":1}]`) {
		t.Fatalf("the moderator did not get the real reactor: %s", mod)
	}
	// late joiners too: redacted snapshot for members, full for moderators
	if l := leaks(drainRaw(e.connect(id, carol))); len(l) > 0 {
		t.Fatalf("a welcome leaks bob's reaction: %v", l)
	}
	if w := strings.Join(drainRaw(e.connect(id, mod)), ""); !strings.Contains(w, `"username":"bob","anonymous":true`) {
		t.Fatalf("the moderator's welcome hides the reactor: %s", w)
	}
	// a named member's reaction on a lobby with anonymous content: both variants are right
	e.send(host, map[string]any{"t": "chat_react", "id": msgID, "emote": "🔥"})
	if r := reactionsOf(t, last(drain(carolC), msgChatReactions)); strings.Join(r["🔥"], ",") != "Anonymous #1,alice" {
		t.Fatalf("mixed reactors for members %v", r)
	}
	if r := reactionsOf(t, last(drain(modC), msgChatReactions)); strings.Join(r["🔥"], ",") != "bob,alice" {
		t.Fatalf("mixed reactors for moderators %v", r)
	}
}
