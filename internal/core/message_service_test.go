package core

import (
	"context"
	"net/http"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// fakeMessageAccess resolves the access of the recipients of live events.
type fakeMessageAccess map[uint]*UserAccess

func (f fakeMessageAccess) ResolveUser(userID uint) (*UserAccess, error) {
	if a, ok := f[userID]; ok {
		return a, nil
	}
	return NewUserAccess(userID, "user", false), nil
}

// fakeLobbies stands in for the Goon2gether hub.
type fakeLobbies struct {
	members  map[uuid.UUID]map[uint]bool
	public   map[uuid.UUID]bool
	banned   map[uuid.UUID]map[uint]bool
	accepted []uint
}

func (f *fakeLobbies) CanInvite(id uuid.UUID, inviterID uint) error {
	if !f.members[id][inviterID] {
		return apperrors.NewForbiddenError("only members can invite")
	}
	return nil
}

func (f *fakeLobbies) InviteCard(id uuid.UUID, inviterID uint, viewer *UserAccess) (*LobbyInviteCard, error) {
	if f.members[id] == nil {
		return nil, nil
	}
	return &LobbyInviteCard{
		ID:       id.String(),
		Name:     "Friday",
		IsMember: f.members[id][viewer.UserID],
		Joinable: !f.banned[id][viewer.UserID] && (f.public[id] || f.members[id][inviterID]),
	}, nil
}

func (f *fakeLobbies) AcceptInvite(id uuid.UUID, inviterID, userID uint, username string) error {
	if !f.public[id] && !f.members[id][inviterID] {
		return apperrors.ErrShareNotAvailable("gone")
	}
	f.accepted = append(f.accepted, userID)
	return nil
}

type messageTestEnv struct {
	svc     *MessageService
	repo    *mocks.MockMessageRepository
	social  *mocks.MockSocialRepository
	bus     *EventBus
	lobbies *fakeLobbies
	access  fakeMessageAccess
	nextID  uint
	// reactions answered by ListReactions (a fake table, see message_reactions_test.go)
	reactions *fakeReactions
}

func newMessageTestEnv(t *testing.T, cfg config.MessagesConfig) *messageTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := &messageTestEnv{
		repo:    mocks.NewMockMessageRepository(ctrl),
		social:  mocks.NewMockSocialRepository(ctrl),
		bus:     NewEventBus(zap.NewNop()),
		lobbies: &fakeLobbies{members: map[uuid.UUID]map[uint]bool{}, public: map[uuid.UUID]bool{}, banned: map[uuid.UUID]map[uint]bool{}},
		access:  fakeMessageAccess{},
		nextID:  100,
	}
	e.svc = NewMessageService(e.repo, e.social, nil, e.access, e.lobbies, e.bus, cfg, zap.NewNop())
	e.reactions = newFakeReactions(e.repo)
	return e
}

func actorWith(id uint, perms ...string) MessageActor {
	return MessageActor{ID: id, Username: "u" + string(rune('0'+id)), Access: NewUserAccess(id, "user", false, perms...)}
}

// expectRelation makes Relations(viewer, [target]) answer the row.
func (e *messageTestEnv) expectRelation(viewer, target uint, row data.UserRelation) {
	row.UserID = target
	e.social.EXPECT().Relations(gomock.Any(), viewer, []uint{target}).Return([]data.UserRelation{row}, nil).AnyTimes()
}

func (e *messageTestEnv) expectUsers(ids ...uint) {
	for _, id := range ids {
		e.social.EXPECT().GetUser(gomock.Any(), id).Return(&data.SocialUser{ID: id, Username: "u"}, nil).AnyTimes()
	}
}

// expectInserts saves the inserted messages (ids from 100).
func (e *messageTestEnv) expectInserts(saved *[]data.Message) {
	e.repo.EXPECT().InsertMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m *data.Message) error {
		e.nextID++
		m.ID = e.nextID
		if saved != nil {
			*saved = append(*saved, *m)
		}
		return nil
	}).AnyTimes()
}

func (e *messageTestEnv) expectConversation(conversationID uint, participants ...uint) {
	e.repo.EXPECT().ParticipantIDs(gomock.Any(), conversationID).Return(participants, nil).AnyTimes()
}

func assertMessageCode(t *testing.T, err error, code string, status int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", code)
	}
	if apperrors.GetCode(err) != code || apperrors.GetHTTPStatus(err) != status {
		t.Fatalf("err = %v (%s, %d), want %s %d", err, apperrors.GetCode(err), apperrors.GetHTTPStatus(err), code, status)
	}
}

// drainEvents returns the events received on a stream within a short wait.
func drainEvents(ch <-chan SceneEvent) []SceneEvent {
	var out []SceneEvent
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-time.After(100 * time.Millisecond):
			return out
		}
	}
}

func TestMessagePolicyAllows(t *testing.T) {
	friend := Relationship{Friend: true}
	mutual := Relationship{Mutual: true, Following: true, FollowedBy: true}
	follower := Relationship{Following: true}
	none := Relationship{}
	tests := []struct {
		policy string
		rel    Relationship
		want   bool
	}{
		{data.MessagePolicyEveryone, none, true},
		{data.MessagePolicyMutuals, friend, true},
		{data.MessagePolicyMutuals, mutual, true},
		{data.MessagePolicyMutuals, follower, false},
		{data.MessagePolicyFriends, friend, true},
		{data.MessagePolicyFriends, mutual, false},
		{"", friend, true},
		{"", mutual, false},
		{data.MessagePolicyNobody, friend, false},
	}
	for _, tc := range tests {
		if got := messagePolicyAllows(tc.policy, tc.rel); got != tc.want {
			t.Fatalf("policy %q, relation %+v: got %v, want %v", tc.policy, tc.rel, got, tc.want)
		}
	}
	// relationshipFrom fills CanMessage from the row
	if r := relationshipFrom(data.UserRelation{Friend: true}); !r.CanMessage {
		t.Fatal("a friend cannot message with the default policy")
	}
	if r := relationshipFrom(data.UserRelation{MessagePolicy: data.MessagePolicyNobody, Friend: true}); r.CanMessage {
		t.Fatal("nobody policy lets a friend write")
	}
}

func TestSendDirectChecksThePolicyAtSendTime(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	alice := actorWith(1, PermMessagesUse, PermScenesView)

	e.expectUsers(2, 3)
	e.expectRelation(1, 3, data.UserRelation{Following: true, FollowedBy: true}) // mutual, default policy friends
	_, _, err := e.svc.SendDirect(ctx, alice, 3, SendMessageInput{Body: "hi"})
	assertMessageCode(t, err, apperrors.CodeMessagesClosed, http.StatusForbidden)

	// without messages:use
	_, _, err = e.svc.SendDirect(ctx, actorWith(1), 2, SendMessageInput{Body: "hi"})
	if !apperrors.IsForbidden(err) {
		t.Fatalf("send without messages:use: %v", err)
	}
	// to oneself
	if _, _, err := e.svc.SendDirect(ctx, alice, 1, SendMessageInput{Body: "hi"}); !apperrors.IsValidation(err) {
		t.Fatalf("send to oneself: %v", err)
	}

	// friends: the conversation is created on the first message, every participant is told
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	e.social.EXPECT().Relations(gomock.Any(), uint(2), gomock.Any()).Return(nil, nil).AnyTimes()
	e.repo.EXPECT().FindOrCreateDirect(gomock.Any(), uint(1), uint(2)).Return(&data.Conversation{ID: 7, Kind: data.ConversationKindDirect}, true, nil)
	var saved []data.Message
	e.expectInserts(&saved)
	other := uint(2)
	e.repo.EXPECT().GetConversationRow(gomock.Any(), uint(1), uint(7)).Return(&data.ConversationRow{ID: 7, Kind: "direct", OtherUserID: &other, OtherUsername: "bob"}, nil)

	_, aliceCh := e.bus.SubscribeUser(1)
	_, bobCh := e.bus.SubscribeUser(2)
	_, carolCh := e.bus.SubscribeUser(3)
	_, anonCh := e.bus.Subscribe()

	if _, _, err := e.svc.SendDirect(ctx, alice, 2, SendMessageInput{Body: "   "}); !apperrors.IsValidation(err) {
		t.Fatalf("empty message: %v", err)
	}
	conv, msg, err := e.svc.SendDirect(ctx, alice, 2, SendMessageInput{Body: "  hello  "})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if conv.ID != 7 || !conv.CanSend || msg.Body != "hello" || len(saved) != 1 || saved[0].ConversationID != 7 || *saved[0].SenderID != 1 {
		t.Fatalf("conv %+v msg %+v saved %+v", conv, msg, saved)
	}
	for name, ch := range map[string]<-chan SceneEvent{"alice": aliceCh, "bob": bobCh} {
		evs := drainEvents(ch)
		if len(evs) != 1 || evs[0].Type != EventChatMessage {
			t.Fatalf("%s got %+v", name, evs)
		}
	}
	if evs := drainEvents(carolCh); len(evs) != 0 {
		t.Fatalf("a non participant got %+v", evs)
	}
	if evs := drainEvents(anonCh); len(evs) != 0 {
		t.Fatalf("a stream without user got %+v", evs)
	}
}

func TestSendLimitsAndPolicies(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{MaxLength: 5, RatePerMinute: 1, RateBurst: 1})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectRelation(1, 2, data.UserRelation{MessagePolicy: data.MessagePolicyEveryone})
	e.expectInserts(nil)

	if _, err := e.svc.Send(ctx, alice, 7, SendMessageInput{Body: "too long"}); !apperrors.IsValidation(err) {
		t.Fatalf("too long: %v", err)
	}
	if _, err := e.svc.Send(ctx, alice, 7, SendMessageInput{Body: "héllo"}); err != nil {
		t.Fatalf("five characters (not bytes): %v", err)
	}
	_, err := e.svc.Send(ctx, alice, 7, SendMessageInput{Body: "again"})
	assertMessageCode(t, err, apperrors.CodeMessageRateLimited, http.StatusTooManyRequests)

	// not a participant: the same answer as an unknown conversation
	e.expectConversation(8, 2, 3)
	_, err = e.svc.Send(ctx, alice, 8, SendMessageInput{Body: "x"})
	assertMessageCode(t, err, apperrors.CodeConversationNotFound, http.StatusNotFound)
	_, err = e.svc.History(ctx, alice, 8, 0, 10)
	assertMessageCode(t, err, apperrors.CodeConversationNotFound, http.StatusNotFound)
}

func TestUnfriendMakesTheConversationReadOnly(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	// no longer friends, default policy (friends)
	e.expectRelation(1, 2, data.UserRelation{Following: true})

	_, err := e.svc.Send(ctx, alice, 7, SendMessageInput{Body: "still there?"})
	assertMessageCode(t, err, apperrors.CodeMessagesClosed, http.StatusForbidden)

	other := uint(2)
	last := uint(50)
	at := time.Now()
	e.repo.EXPECT().GetConversationRow(gomock.Any(), uint(1), uint(7)).Return(&data.ConversationRow{ID: 7, Kind: "direct", OtherUserID: &other, UnreadCount: 2, LastID: &last, LastCreatedAt: &at, LastBody: "old"}, nil)
	v, err := e.svc.Conversation(ctx, alice, 7)
	if err != nil {
		t.Fatalf("conversation: %v", err)
	}
	if v.CanSend || v.ReadOnlyReason != ReadOnlyPolicy || v.UnreadCount != 2 || v.LastMessage == nil || v.LastMessage.Body != "old" {
		t.Fatalf("view = %+v", v)
	}

	// the history stays readable, and one's own messages can still be deleted
	sender := uint(1)
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(50)).Return(&data.Message{ID: 50, ConversationID: 7, SenderID: &sender, Body: "old"}, nil)
	e.repo.EXPECT().SoftDeleteMessage(gomock.Any(), uint(50), gomock.Any()).Return(true, nil)
	if err := e.svc.Delete(ctx, alice, 7, 50); err != nil {
		t.Fatalf("delete in a read-only conversation: %v", err)
	}
	// but not edited
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(51)).Return(&data.Message{ID: 51, ConversationID: 7, SenderID: &sender, Body: "old"}, nil)
	_, err = e.svc.Edit(ctx, alice, 7, 51, "new")
	assertMessageCode(t, err, apperrors.CodeMessagesClosed, http.StatusForbidden)

	// a deleted other user: read-only too
	e.repo.EXPECT().GetConversationRow(gomock.Any(), uint(1), uint(9)).Return(&data.ConversationRow{ID: 9, Kind: "direct"}, nil)
	if v, err := e.svc.Conversation(ctx, alice, 9); err != nil || v.CanSend || v.ReadOnlyReason != ReadOnlyDeletedUser || v.Other != nil {
		t.Fatalf("deleted user view = %+v %v", v, err)
	}
	e.expectConversation(9, 1)
	_, err = e.svc.Send(ctx, alice, 9, SendMessageInput{Body: "x"})
	assertMessageCode(t, err, apperrors.CodeMessagesClosed, http.StatusForbidden)
}

func TestEditAndDeleteOwnMessagesOnly(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	bob, me := uint(2), uint(1)
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(10)).Return(&data.Message{ID: 10, ConversationID: 7, SenderID: &bob, Body: "bob"}, nil).AnyTimes()
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(11)).Return(&data.Message{ID: 11, ConversationID: 8, SenderID: &me, Body: "elsewhere"}, nil).AnyTimes()
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(12)).Return(nil, gorm.ErrRecordNotFound).AnyTimes()

	_, err := e.svc.Edit(ctx, alice, 7, 10, "mine now")
	assertMessageCode(t, err, apperrors.CodeNotMessageAuthor, http.StatusForbidden)
	assertMessageCode(t, e.svc.Delete(ctx, alice, 7, 10), apperrors.CodeNotMessageAuthor, http.StatusForbidden)
	_, err = e.svc.Edit(ctx, alice, 7, 11, "x")
	assertMessageCode(t, err, apperrors.CodeMessageNotFound, http.StatusNotFound)
	assertMessageCode(t, e.svc.Delete(ctx, alice, 7, 12), apperrors.CodeMessageNotFound, http.StatusNotFound)

	// own message: edited, the participants are told
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(13)).Return(&data.Message{ID: 13, ConversationID: 7, SenderID: &me, Body: "tpyo"}, nil)
	e.repo.EXPECT().UpdateMessageBody(gomock.Any(), uint(13), "typo", gomock.Any()).Return(true, nil)
	_, bobCh := e.bus.SubscribeUser(2)
	v, err := e.svc.Edit(ctx, alice, 7, 13, " typo ")
	if err != nil || v.Body != "typo" || v.EditedAt == nil {
		t.Fatalf("edit: %+v %v", v, err)
	}
	evs := drainEvents(bobCh)
	if len(evs) != 1 || evs[0].Type != EventChatMessageUpdated {
		t.Fatalf("bob got %+v", evs)
	}
	// an edit cannot empty a text message
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(14)).Return(&data.Message{ID: 14, ConversationID: 7, SenderID: &me, Body: "text"}, nil)
	if _, err := e.svc.Edit(ctx, alice, 7, 14, "  "); !apperrors.IsValidation(err) {
		t.Fatalf("emptying edit: %v", err)
	}
}

func TestShareCardsAreResolvedForTheViewer(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	kind := func(k string) *string { return &k }
	id := func(v uint) *uint { return &v }
	sender := uint(2)
	lobbyOK, lobbyLeft, lobbyGone := uuid.New(), uuid.New(), uuid.New()
	e.lobbies.members[lobbyOK] = map[uint]bool{2: true}
	e.lobbies.members[lobbyLeft] = map[uint]bool{5: true}
	deletedAt := time.Now()
	msgs := []data.Message{
		{ID: 1, SenderID: &sender, AttachmentKind: kind(data.AttachmentScene), AttachmentSceneID: id(10)},
		{ID: 2, SenderID: &sender, AttachmentKind: kind(data.AttachmentScene), AttachmentSceneID: id(11)},
		{ID: 3, SenderID: &sender, AttachmentKind: kind(data.AttachmentScene)}, // the scene was deleted (SET NULL)
		{ID: 4, SenderID: &sender, AttachmentKind: kind(data.AttachmentMarker), AttachmentMarkerID: id(20)},
		{ID: 5, SenderID: &sender, AttachmentKind: kind(data.AttachmentPlaylist), AttachmentPlaylistID: id(30)},
		{ID: 6, SenderID: &sender, AttachmentKind: kind(data.AttachmentPlaylist), AttachmentPlaylistID: id(31)},
		{ID: 7, SenderID: &sender, AttachmentKind: kind(data.AttachmentPlaylist), AttachmentPlaylistID: id(32)},
		{ID: 8, SenderID: &sender, AttachmentKind: kind(data.AttachmentLobby), AttachmentLobbyUUID: &lobbyOK},
		{ID: 9, SenderID: &sender, AttachmentKind: kind(data.AttachmentLobby), AttachmentLobbyUUID: &lobbyLeft},
		{ID: 10, SenderID: &sender, AttachmentKind: kind(data.AttachmentLobby), AttachmentLobbyUUID: &lobbyGone},
		{ID: 11, SenderID: &sender, DeletedAt: &deletedAt},
		{ID: 12, SenderID: &sender, Body: "plain text"},
	}
	trashed := time.Now()
	e.repo.EXPECT().SceneCards(gomock.Any(), gomock.Any()).Return([]data.ShareSceneRow{
		{ID: 10, Title: "Ten", Duration: 600, ThumbnailPath: "t.jpg", UpdatedAt: time.UnixMilli(1000)},
		{ID: 11, Title: "Eleven", TrashedAt: &trashed},
	}, nil).AnyTimes()
	end := 40
	// bob's marker: bob does not share his markers, the recipient still sees the shared one
	e.repo.EXPECT().MarkerCards(gomock.Any(), gomock.Any()).Return([]data.ShareMarkerRow{
		{ID: 20, UserID: 2, OwnerUsername: "bob", SceneID: 10, Timestamp: 10, EndTimestamp: &end, Label: "Kiss", SceneTitle: "Ten", ThumbnailPath: "m.webp", UpdatedAt: time.UnixMilli(2000)},
	}, nil).AnyTimes()
	e.repo.EXPECT().PlaylistCards(gomock.Any(), gomock.Any()).Return([]data.SharePlaylistRow{
		{ID: 30, UUID: uuid.New(), Name: "Bob private", Visibility: "private", UserID: 2},
		{ID: 31, UUID: uuid.New(), Name: "Bob public", Visibility: "public", UserID: 2, ItemCount: 3},
		{ID: 32, UUID: uuid.New(), Name: "Mine", Visibility: "private", UserID: 1},
	}, nil).AnyTimes()

	full := NewUserAccess(1, "user", false, PermScenesView, PermPlaylistsPublic, PermTogetherUse)
	got, err := e.svc.views(ctx, msgs, full, false)
	if err != nil {
		t.Fatalf("views: %v", err)
	}
	want := map[uint]string{1: "", 2: ShareReasonTrashed, 3: ShareReasonNotFound, 4: "", 5: ShareReasonPrivate, 6: "", 7: "", 8: "", 9: ShareReasonNotJoinable, 10: ShareReasonNotFound}
	for _, v := range got {
		reason, has := want[v.ID]
		if !has {
			if v.Attachment != nil {
				t.Fatalf("message %d has an attachment: %+v", v.ID, v.Attachment)
			}
			continue
		}
		if v.Attachment == nil || v.Attachment.Available != (reason == "") || v.Attachment.Reason != reason {
			t.Fatalf("message %d: %+v, want reason %q", v.ID, v.Attachment, reason)
		}
	}
	if s := got[0].Attachment.Scene; s == nil || s.Title != "Ten" || s.ThumbnailURL != "/thumbnails/10?v=1000" {
		t.Fatalf("scene card = %+v", s)
	}
	if m := got[3].Attachment.Marker; m == nil || m.Start != 10 || m.End == nil || *m.End != 40 || m.ThumbnailURL != "/marker-thumbnails/20?v=2000" {
		t.Fatalf("marker card = %+v", m)
	}
	if !got[10].Deleted || got[10].Body != "" {
		t.Fatalf("deleted message = %+v", got[10])
	}

	// Without scenes:view nor the other permissions: nothing is read, nothing is shown
	bare := NewUserAccess(1, "user", false, PermMessagesUse)
	got, err = e.svc.views(ctx, msgs[:10], bare, false)
	if err != nil {
		t.Fatalf("views: %v", err)
	}
	for _, v := range got {
		if v.Attachment == nil || v.Attachment.Available {
			t.Fatalf("message %d visible without permission: %+v", v.ID, v.Attachment)
		}
		if v.Attachment.Scene != nil || v.Attachment.Marker != nil || v.Attachment.Playlist != nil || v.Attachment.Lobby != nil {
			t.Fatalf("message %d leaks a card: %+v", v.ID, v.Attachment)
		}
	}
	// a public playlist of someone else needs playlists:view_public
	noPublic := NewUserAccess(1, "user", false, PermScenesView)
	got, _ = e.svc.views(ctx, msgs[5:6], noPublic, false)
	if got[0].Attachment.Available || got[0].Attachment.Reason != ShareReasonNoPermission {
		t.Fatalf("public playlist without view_public: %+v", got[0].Attachment)
	}
}

func TestLiveEventsAreResolvedForEachRecipient(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	alice := actorWith(1, PermMessagesUse, PermScenesView)
	// bob may not see scenes
	e.access[2] = NewUserAccess(2, "user", false, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	e.expectInserts(nil)
	e.repo.EXPECT().SceneCards(gomock.Any(), []uint{10}).Return([]data.ShareSceneRow{{ID: 10, Title: "Ten"}}, nil).AnyTimes()

	_, aliceCh := e.bus.SubscribeUser(1)
	_, bobCh := e.bus.SubscribeUser(2)
	at := 42.0
	v, err := e.svc.Send(ctx, alice, 7, SendMessageInput{Attachment: &AttachmentInput{Kind: data.AttachmentScene, SceneID: 10, Time: &at}})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if v.Attachment == nil || !v.Attachment.Available || v.Attachment.Time == nil || *v.Attachment.Time != 42 {
		t.Fatalf("sender view = %+v", v.Attachment)
	}
	view := func(ch <-chan SceneEvent) MessageView {
		evs := drainEvents(ch)
		if len(evs) != 1 {
			t.Fatalf("events = %+v", evs)
		}
		return evs[0].Data.(map[string]any)["message"].(MessageView)
	}
	if a := view(aliceCh).Attachment; a == nil || !a.Available {
		t.Fatalf("alice's event = %+v", a)
	}
	if b := view(bobCh).Attachment; b == nil || b.Available || b.Scene != nil || b.Reason != ShareReasonNoPermission {
		t.Fatalf("bob's event = %+v", b)
	}

	// sharing an item the sender may not see is refused
	e.repo.EXPECT().MarkerVisibleTo(gomock.Any(), uint(20), uint(1)).Return(false, nil)
	_, err = e.svc.Send(ctx, alice, 7, SendMessageInput{Attachment: &AttachmentInput{Kind: data.AttachmentMarker, MarkerID: 20}})
	assertMessageCode(t, err, apperrors.CodeShareNotAvailable, http.StatusBadRequest)
	// a lobby the sender is not a member of
	_, err = e.svc.Send(ctx, actorWith(1, PermMessagesUse, PermTogetherUse), 7, SendMessageInput{Attachment: &AttachmentInput{Kind: data.AttachmentLobby, LobbyID: uuid.NewString()}})
	if !apperrors.IsForbidden(err) {
		t.Fatalf("invite to a lobby one is not in: %v", err)
	}
	if _, err := e.svc.Send(ctx, alice, 7, SendMessageInput{Attachment: &AttachmentInput{Kind: "file"}}); !apperrors.IsValidation(err) {
		t.Fatalf("unknown kind: %v", err)
	}
}

func TestHistoryPagination(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{PageSize: 2, MaxPageSize: 3})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	// newest first from the repository, one more than the page
	e.repo.EXPECT().ListMessages(gomock.Any(), uint(7), uint(0), 3).Return([]data.Message{{ID: 9}, {ID: 8}, {ID: 7}}, nil)
	h, err := e.svc.History(ctx, alice, 7, 0, 0)
	if err != nil || !h.HasMore || len(h.Messages) != 2 || h.Messages[0].ID != 8 || h.Messages[1].ID != 9 {
		t.Fatalf("first page: %+v %v", h, err)
	}
	// the limit is capped; the last page has no more
	e.repo.EXPECT().ListMessages(gomock.Any(), uint(7), uint(8), 4).Return([]data.Message{{ID: 7}}, nil)
	h, err = e.svc.History(ctx, alice, 7, 8, 50)
	if err != nil || h.HasMore || len(h.Messages) != 1 {
		t.Fatalf("last page: %+v %v", h, err)
	}
}

func TestReadReceiptsAndTyping(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{TypingInterval: time.Hour})
	alice := actorWith(1, PermMessagesUse)
	e.expectConversation(7, 1, 2)
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	_, aliceCh := e.bus.SubscribeUser(1)
	_, bobCh := e.bus.SubscribeUser(2)
	_, carolCh := e.bus.SubscribeUser(3)

	e.repo.EXPECT().MarkRead(gomock.Any(), uint(7), uint(1), uint(40)).Return(true, nil)
	e.repo.EXPECT().MarkRead(gomock.Any(), uint(7), uint(1), uint(30)).Return(false, nil)
	if err := e.svc.MarkRead(ctx, 1, 7, 40); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if err := e.svc.MarkRead(ctx, 1, 7, 30); err != nil {
		t.Fatalf("mark read backwards: %v", err)
	}
	for name, ch := range map[string]<-chan SceneEvent{"alice": aliceCh, "bob": bobCh} {
		if evs := drainEvents(ch); len(evs) != 1 || evs[0].Type != EventChatRead {
			t.Fatalf("%s got %+v", name, evs)
		}
	}

	// typing: relayed once per interval, to the other participant only
	for i := 0; i < 3; i++ {
		if err := e.svc.Typing(ctx, alice, 7); err != nil {
			t.Fatalf("typing: %v", err)
		}
	}
	if evs := drainEvents(bobCh); len(evs) != 1 || evs[0].Type != EventChatTyping {
		t.Fatalf("bob got %+v", evs)
	}
	if evs := drainEvents(aliceCh); len(evs) != 0 {
		t.Fatalf("the typer got %+v", evs)
	}
	if evs := drainEvents(carolCh); len(evs) != 0 {
		t.Fatalf("a non participant got %+v", evs)
	}
}

func TestShareToSeveralUsers(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{MaxShareItems: 2})
	alice := actorWith(1, PermMessagesUse, PermScenesView)
	e.expectUsers(2, 3)
	e.social.EXPECT().Relations(gomock.Any(), uint(1), []uint{2, 3}).Return([]data.UserRelation{{UserID: 2, Friend: true}, {UserID: 3}}, nil)
	e.social.EXPECT().Relations(gomock.Any(), uint(2), gomock.Any()).Return(nil, nil).AnyTimes()
	e.repo.EXPECT().SceneCards(gomock.Any(), gomock.Any()).Return([]data.ShareSceneRow{{ID: 10, Title: "Ten"}, {ID: 11, Title: "Eleven"}}, nil).AnyTimes()
	e.repo.EXPECT().FindOrCreateDirect(gomock.Any(), uint(1), uint(2)).Return(&data.Conversation{ID: 7}, false, nil)
	var saved []data.Message
	e.expectInserts(&saved)

	items := []AttachmentInput{{Kind: data.AttachmentScene, SceneID: 10}, {Kind: data.AttachmentScene, SceneID: 11}}
	if _, err := e.svc.Share(ctx, alice, ShareInput{UserIDs: []uint{2}, Items: append(items, items[0])}); !apperrors.IsValidation(err) {
		t.Fatalf("too many items: %v", err)
	}
	results, err := e.svc.Share(ctx, alice, ShareInput{UserIDs: []uint{2, 3, 2, 0}, Body: "look", Items: items})
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if len(results) != 2 || !results[0].OK || results[0].ConversationID != 7 || results[1].OK || results[1].Code != apperrors.CodeMessagesClosed {
		t.Fatalf("results = %+v", results)
	}
	if len(saved) != 2 || saved[0].Body != "look" || saved[1].Body != "" || *saved[0].AttachmentSceneID != 10 || *saved[1].AttachmentSceneID != 11 {
		t.Fatalf("saved = %+v", saved)
	}
}

func TestJoinLobbyFromAnInvite(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	bob := MessageActor{ID: 2, Username: "bob", Access: NewUserAccess(2, "user", false, PermTogetherUse)}
	e.expectConversation(7, 1, 2)
	lobby := uuid.New()
	e.lobbies.members[lobby] = map[uint]bool{1: true}
	alice := uint(1)
	kind := data.AttachmentLobby
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(40)).Return(&data.Message{ID: 40, ConversationID: 7, SenderID: &alice, AttachmentKind: &kind, AttachmentLobbyUUID: &lobby}, nil).AnyTimes()
	e.repo.EXPECT().GetMessage(gomock.Any(), uint(41)).Return(&data.Message{ID: 41, ConversationID: 7, SenderID: &alice, Body: "text"}, nil)

	id, err := e.svc.JoinLobby(ctx, bob, 7, 40)
	if err != nil || id != lobby.String() || len(e.lobbies.accepted) != 1 || e.lobbies.accepted[0] != 2 {
		t.Fatalf("join: %q %v %v", id, err, e.lobbies.accepted)
	}
	_, err = e.svc.JoinLobby(ctx, bob, 7, 41)
	assertMessageCode(t, err, apperrors.CodeShareNotAvailable, http.StatusBadRequest)
	// the inviter left a private lobby: the invite no longer works
	e.lobbies.members[lobby] = map[uint]bool{3: true}
	if _, err := e.svc.JoinLobby(ctx, bob, 7, 40); err == nil {
		t.Fatal("joined through a stale invite")
	}
	// without goon2gether:use
	if _, err := e.svc.JoinLobby(ctx, actorWith(2), 7, 40); !apperrors.IsForbidden(err) {
		t.Fatalf("join without goon2gether:use: %v", err)
	}
}

func TestMessageSettings(t *testing.T) {
	ctx := context.Background()
	e := newMessageTestEnv(t, config.MessagesConfig{})
	bad := "strangers"
	if _, err := e.svc.UpdateSettings(ctx, 1, &bad, nil); !apperrors.IsValidation(err) {
		t.Fatalf("unknown policy: %v", err)
	}
	policy := data.MessagePolicyMutuals
	e.repo.EXPECT().SaveSettings(gomock.Any(), uint(1), &policy, nil).Return(nil)
	e.repo.EXPECT().GetSettings(gomock.Any(), uint(1)).Return(data.MessageSettings{Policy: policy, Toasts: true}, nil)
	v, err := e.svc.UpdateSettings(ctx, 1, &policy, nil)
	if err != nil || v.Policy != policy || !v.Toasts || len(v.Policies) != 4 || v.MaxLength != defaultMessageMaxLength {
		t.Fatalf("settings = %+v %v", v, err)
	}
}
