package core

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// fakeReactions is an in-memory message_reactions table behind the mock repository, with the
// semantics of the SQL (caps, deleted messages).
type fakeReactions struct {
	mu        sync.Mutex
	rows      []data.MessageReactionRow
	deleted   map[uint]bool
	requested [][]uint
	clock     time.Time
}

func newFakeReactions(repo *mocks.MockMessageRepository) *fakeReactions {
	f := &fakeReactions{deleted: map[uint]bool{}, clock: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	repo.EXPECT().ListReactions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ids []uint) ([]data.MessageReactionRow, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requested = append(f.requested, append([]uint(nil), ids...))
		want := map[uint]bool{}
		for _, id := range ids {
			want[id] = true
		}
		var out []data.MessageReactionRow
		for _, r := range f.rows {
			if want[r.MessageID] {
				out = append(out, r)
			}
		}
		return out, nil
	}).AnyTimes()
	repo.EXPECT().AddReaction(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, messageID, userID uint, emoji string, maxKinds, maxPerUser int) (data.ReactionAddResult, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.deleted[messageID] {
				return data.ReactionMessageGone, nil
			}
			kinds := map[string]bool{}
			mine := 0
			for _, r := range f.rows {
				if r.MessageID != messageID {
					continue
				}
				kinds[r.Emoji] = true
				if r.UserID == userID {
					if r.Emoji == emoji {
						return data.ReactionExists, nil
					}
					mine++
				}
			}
			if maxKinds > 0 && !kinds[emoji] && len(kinds) >= maxKinds {
				return data.ReactionTooManyKinds, nil
			}
			if maxPerUser > 0 && mine >= maxPerUser {
				return data.ReactionTooManyForUser, nil
			}
			f.clock = f.clock.Add(time.Second)
			f.rows = append(f.rows, data.MessageReactionRow{MessageID: messageID, UserID: userID, Emoji: emoji, CreatedAt: f.clock, Username: "u" + string(rune('0'+userID))})
			return data.ReactionAdded, nil
		}).AnyTimes()
	repo.EXPECT().RemoveReaction(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, messageID, userID uint, emoji string) (bool, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			for i, r := range f.rows {
				if r.MessageID == messageID && r.UserID == userID && r.Emoji == emoji {
					f.rows = append(f.rows[:i:i], f.rows[i+1:]...)
					return true, nil
				}
			}
			return false, nil
		}).AnyTimes()
	return f
}

type fakeEmoteNames map[string]bool

func (f fakeEmoteNames) HasEmote(name string) bool { return f[name] }

// expectMessages makes GetMessage answer these messages of conversation 7 (others: not found).
func (e *messageTestEnv) expectMessages(msgs ...data.Message) {
	byID := map[uint]data.Message{}
	for _, m := range msgs {
		byID[m.ID] = m
	}
	e.repo.EXPECT().GetMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id uint) (*data.Message, error) {
		m, ok := byID[id]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		return &m, nil
	}).AnyTimes()
}

func reactionSummary(list []MessageReactionView) map[string]int {
	out := map[string]int{}
	for _, r := range list {
		out[r.Emoji] = r.Count
	}
	return out
}

func TestReactionsToggleAndLiveEvents(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	e.svc.SetEmotes(fakeEmoteNames{"pog": true})
	alice, bob := actorWith(1, PermMessagesUse), actorWith(2, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	e.expectRelation(2, 1, data.UserRelation{Friend: true})
	bobID := uint(2)
	e.expectMessages(data.Message{ID: 10, ConversationID: 7, SenderID: &bobID, Body: "hi"})
	_, aliceCh := e.bus.SubscribeUser(1)
	_, bobCh := e.bus.SubscribeUser(2)
	_, carolCh := e.bus.SubscribeUser(3)

	r, err := e.svc.React(ctx, alice, 7, 10, " 🔥 ")
	if err != nil {
		t.Fatalf("react: %v", err)
	}
	if len(r.Reactions) != 1 || r.Reactions[0].Emoji != "🔥" || r.Reactions[0].Count != 1 || r.Reactions[0].Users[0].ID != 1 || r.MessageID != 10 || r.ConversationID != 7 {
		t.Fatalf("reactions = %+v", r)
	}
	for name, ch := range map[string]<-chan SceneEvent{"alice": aliceCh, "bob": bobCh} {
		evs := drainEvents(ch)
		if len(evs) != 1 || evs[0].Type != EventChatReaction {
			t.Fatalf("%s got %+v", name, evs)
		}
		d := evs[0].Data.(map[string]any)
		if d["conversation_id"] != uint(7) || d["message_id"] != uint(10) || len(d["reactions"].([]MessageReactionView)) != 1 {
			t.Fatalf("%s event data = %+v", name, d)
		}
	}
	if evs := drainEvents(carolCh); len(evs) != 0 {
		t.Fatalf("a non participant got %+v", evs)
	}

	// The same reaction again changes nothing and tells nobody
	if r, err := e.svc.React(ctx, alice, 7, 10, "🔥"); err != nil || reactionSummary(r.Reactions)["🔥"] != 1 {
		t.Fatalf("react again: %+v %v", r, err)
	}
	if evs := drainEvents(bobCh); len(evs) != 0 {
		t.Fatalf("a no-op reaction was published: %+v", evs)
	}

	// bob joins the same key, then an emote: grouped, in the order of the first reaction
	if _, err := e.svc.React(ctx, bob, 7, 10, "🔥"); err != nil {
		t.Fatalf("bob reacts: %v", err)
	}
	r, err = e.svc.React(ctx, bob, 7, 10, ":pog:")
	if err != nil {
		t.Fatalf("emote reaction: %v", err)
	}
	if len(r.Reactions) != 2 || r.Reactions[0].Emoji != "🔥" || r.Reactions[0].Count != 2 ||
		r.Reactions[0].Users[0].ID != 1 || r.Reactions[0].Users[1].ID != 2 || r.Reactions[1].Emoji != ":pog:" {
		t.Fatalf("grouped reactions = %+v", r.Reactions)
	}

	// Removing: only one's own, and a missing one changes nothing
	drainEvents(aliceCh)
	r, err = e.svc.Unreact(ctx, alice, 7, 10, "🔥")
	if err != nil || reactionSummary(r.Reactions)["🔥"] != 1 || r.Reactions[0].Users[0].ID != 2 {
		t.Fatalf("unreact: %+v %v", r, err)
	}
	if evs := drainEvents(aliceCh); len(evs) != 1 || evs[0].Type != EventChatReaction {
		t.Fatalf("unreact event: %+v", evs)
	}
	if _, err := e.svc.Unreact(ctx, alice, 7, 10, ":pog:"); err != nil {
		t.Fatalf("removing someone else's key: %v", err)
	}
	if evs := drainEvents(aliceCh); len(evs) != 0 {
		t.Fatalf("a no-op removal was published: %+v", evs)
	}
	if got := reactionSummary(e.reactions.rows2views(10)); got[":pog:"] != 1 {
		t.Fatalf("bob's emote was removed by alice: %v", got)
	}
}

// rows2views groups the fake rows of a message.
func (f *fakeReactions) rows2views(messageID uint) []MessageReactionView {
	f.mu.Lock()
	defer f.mu.Unlock()
	return groupReactions(f.rows)[messageID]
}

func TestReactionPermissions(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectConversation(8, 2, 3)
	bobID := uint(2)
	at := time.Now()
	e.expectMessages(
		data.Message{ID: 10, ConversationID: 7, SenderID: &bobID, Body: "hi"},
		data.Message{ID: 11, ConversationID: 8, SenderID: &bobID, Body: "elsewhere"},
		data.Message{ID: 12, ConversationID: 7, SenderID: &bobID, DeletedAt: &at},
	)
	// unfriended: the conversation is read-only for alice (default policy friends)
	e.expectRelation(1, 2, data.UserRelation{Following: true})

	_, err := e.svc.React(ctx, alice, 8, 11, "🔥")
	assertMessageCode(t, err, apperrors.CodeConversationNotFound, http.StatusNotFound)
	_, err = e.svc.Unreact(ctx, alice, 8, 11, "🔥")
	assertMessageCode(t, err, apperrors.CodeConversationNotFound, http.StatusNotFound)
	_, err = e.svc.React(ctx, alice, 7, 11, "🔥")
	assertMessageCode(t, err, apperrors.CodeMessageNotFound, http.StatusNotFound)
	_, err = e.svc.React(ctx, alice, 7, 12, "🔥")
	assertMessageCode(t, err, apperrors.CodeMessageNotFound, http.StatusNotFound)
	_, err = e.svc.React(ctx, alice, 7, 99, "🔥")
	assertMessageCode(t, err, apperrors.CodeMessageNotFound, http.StatusNotFound)

	// read-only: no new reaction, but removing one's own still works
	_, err = e.svc.React(ctx, alice, 7, 10, "🔥")
	assertMessageCode(t, err, apperrors.CodeMessagesClosed, http.StatusForbidden)
	e.reactions.rows = append(e.reactions.rows, data.MessageReactionRow{MessageID: 10, UserID: 1, Emoji: "🔥"})
	if r, err := e.svc.Unreact(ctx, alice, 7, 10, "🔥"); err != nil || len(r.Reactions) != 0 {
		t.Fatalf("unreact in a read-only conversation: %+v %v", r, err)
	}
	// without messages:use
	_, err = e.svc.React(ctx, actorWith(1), 7, 10, "🔥")
	if !apperrors.IsForbidden(err) {
		t.Fatalf("react without messages:use: %v", err)
	}

	// keys: emoji or an existing server emote only
	for _, key := range []string{"", "x", "<b>", "hello world", ":no such:", "🔥 🔥", string(make([]byte, 80))} {
		if _, err := e.svc.React(ctx, alice, 7, 10, key); !apperrors.IsValidation(err) {
			t.Fatalf("key %q: %v", key, err)
		}
	}
	// no emote service (Goon2gether disabled): emote keys are refused
	if _, err := e.svc.React(ctx, alice, 7, 10, ":pog:"); !apperrors.IsValidation(err) {
		t.Fatalf("emote without emotes: %v", err)
	}
	e.svc.SetEmotes(fakeEmoteNames{"pog": true})
	if _, err := e.svc.React(ctx, alice, 7, 10, ":nope:"); !apperrors.IsValidation(err) {
		t.Fatalf("unknown emote: %v", err)
	}
}

func TestReactionCapsAndRateLimit(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{MaxReactionKinds: 2, MaxReactionsPerUser: 2})
	alice, bob := actorWith(1, PermMessagesUse), actorWith(2, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	e.expectRelation(2, 1, data.UserRelation{Friend: true})
	bobID := uint(2)
	e.expectMessages(data.Message{ID: 10, ConversationID: 7, SenderID: &bobID, Body: "hi"})

	for _, key := range []string{"🔥", "😂"} {
		if _, err := e.svc.React(ctx, alice, 7, 10, key); err != nil {
			t.Fatalf("react %s: %v", key, err)
		}
	}
	// a third distinct key: too many kinds (also for bob)
	_, err := e.svc.React(ctx, bob, 7, 10, "👍")
	assertMessageCode(t, err, apperrors.CodeReactionLimit, http.StatusConflict)
	// an existing key is still fine for bob
	if _, err := e.svc.React(ctx, bob, 7, 10, "😂"); err != nil {
		t.Fatalf("joining an existing key: %v", err)
	}
	// alice has 2 reactions: no third one, even after a key frees up
	if _, err := e.svc.Unreact(ctx, bob, 7, 10, "😂"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Unreact(ctx, alice, 7, 10, "😂"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.React(ctx, alice, 7, 10, "👍"); err != nil {
		t.Fatalf("a freed kind: %v", err)
	}
	_, err = e.svc.React(ctx, alice, 7, 10, "🎉")
	assertMessageCode(t, err, apperrors.CodeReactionLimit, http.StatusConflict)

	// rate limit: 1 per minute after a burst of 2
	e2 := newMessageTestEnv(t, config.MessagesConfig{ReactionRatePerMinute: 1, ReactionRateBurst: 2})
	e2.expectConversation(7, 1, 2)
	e2.expectRelation(1, 2, data.UserRelation{Friend: true})
	e2.expectMessages(data.Message{ID: 10, ConversationID: 7, SenderID: &bobID, Body: "hi"})
	if _, err := e2.svc.React(ctx, alice, 7, 10, "🔥"); err != nil {
		t.Fatal(err)
	}
	if _, err := e2.svc.Unreact(ctx, alice, 7, 10, "🔥"); err != nil {
		t.Fatal(err)
	}
	_, err = e2.svc.React(ctx, alice, 7, 10, "🔥")
	assertMessageCode(t, err, apperrors.CodeReactionRateLimited, http.StatusTooManyRequests)
}

func TestHistoryCarriesReactionsAndDeletedMessagesLoseThem(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	at := time.Now()
	e.reactions.rows = []data.MessageReactionRow{
		{MessageID: 5, UserID: 2, Emoji: "🔥", Username: "bob"},
		{MessageID: 6, UserID: 1, Emoji: "😂", Username: "alice"}, // stale row of a deleted message
	}
	e.repo.EXPECT().ListMessages(gomock.Any(), uint(7), uint(0), gomock.Any()).Return([]data.Message{
		{ID: 6, ConversationID: 7, DeletedAt: &at},
		{ID: 5, ConversationID: 7, Body: "hi"},
		{ID: 4, ConversationID: 7, Body: "no reaction"},
	}, nil)
	h, err := e.svc.History(ctx, alice, 7, 0, 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	got := map[uint][]MessageReactionView{}
	for _, m := range h.Messages {
		if m.Reactions == nil {
			t.Fatalf("message %d has null reactions (want an empty list)", m.ID)
		}
		got[m.ID] = m.Reactions
	}
	if len(got[5]) != 1 || got[5][0].Users[0].Username != "bob" || len(got[4]) != 0 || len(got[6]) != 0 {
		t.Fatalf("history reactions = %+v", got)
	}
	// one query for the page, without the deleted message
	if len(e.reactions.requested) != 1 || len(e.reactions.requested[0]) != 2 {
		t.Fatalf("reaction queries = %v", e.reactions.requested)
	}

	// Deleting a message: the repository clears the reactions with it, the event has none
	sender := uint(1)
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(4)).Return(&data.Message{ID: 4, ConversationID: 7, SenderID: &sender, Body: "x"}, nil)
	e.repo.EXPECT().SoftDeleteMessage(gomock.Any(), uint(4), gomock.Any()).Return(true, nil)
	_, bobCh := e.bus.SubscribeUser(2)
	if err := e.svc.Delete(ctx, alice, 7, 4); err != nil {
		t.Fatalf("delete: %v", err)
	}
	evs := drainEvents(bobCh)
	if len(evs) != 1 {
		t.Fatalf("delete events: %+v", evs)
	}
	if m := evs[0].Data.(map[string]any)["message"].(MessageView); !m.Deleted || len(m.Reactions) != 0 {
		t.Fatalf("deleted message event = %+v", m)
	}
}

func TestReactionKeys(t *testing.T) {
	emotes := fakeEmoteNames{"pog": true}
	for key, want := range map[string]ReactionKeyError{
		"🔥": ReactionKeyOK, "👍🏽": ReactionKeyOK, "👨‍👩‍👧": ReactionKeyOK, "1️⃣": ReactionKeyOK, ":pog:": ReactionKeyOK,
		":nope:": ReactionKeyUnknownEmote, "": ReactionKeyInvalid, "a": ReactionKeyInvalid, "::": ReactionKeyInvalid,
		":a b:": ReactionKeyInvalid, "<img>": ReactionKeyInvalid, "🔥\n": ReactionKeyInvalid,
	} {
		if got := CheckReactionKey(key, emotes); got != want {
			t.Fatalf("CheckReactionKey(%q) = %v, want %v", key, got, want)
		}
	}
	if CheckReactionKey(":pog:", nil) != ReactionKeyUnknownEmote {
		t.Fatal("an emote key without emotes was accepted")
	}
}
