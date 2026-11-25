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

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type socialTestEnv struct {
	svc     *SocialService
	repo    *mocks.MockSocialRepository
	privacy *mocks.MockPrivacyRepository
	bus     *EventBus
}

func newTestSocialService(t *testing.T, cfg config.SocialConfig) *socialTestEnv {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSocialRepository(ctrl)
	privacyRepo := mocks.NewMockPrivacyRepository(ctrl)
	bus := NewEventBus(zap.NewNop())
	privacy := NewPrivacyService(privacyRepo, repo, zap.NewNop())
	return &socialTestEnv{svc: NewSocialService(repo, privacy, bus, cfg, zap.NewNop()), repo: repo, privacy: privacyRepo, bus: bus}
}

// expectUsers makes GetUser answer for the given ids.
func (e *socialTestEnv) expectUsers(ids ...uint) {
	for _, id := range ids {
		e.repo.EXPECT().GetUser(gomock.Any(), id).Return(&data.SocialUser{ID: id, Username: "u" + string(rune('0'+id))}, nil).AnyTimes()
	}
}

// expectRelation makes Relations(viewer, [target]) answer the row (any number of times).
func (e *socialTestEnv) expectRelation(viewer, target uint, row data.UserRelation) {
	row.UserID = target
	if row.RequestPolicy == "" {
		row.RequestPolicy = data.FriendRequestPolicyEveryone
	}
	e.repo.EXPECT().Relations(gomock.Any(), viewer, []uint{target}).Return([]data.UserRelation{row}, nil).AnyTimes()
}

// receive returns the next event of the stream, or false after a short wait.
func receive(ch <-chan SceneEvent) (SceneEvent, bool) {
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(200 * time.Millisecond):
		return SceneEvent{}, false
	}
}

func assertSocialCode(t *testing.T, err error, code string, status int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", code)
	}
	if apperrors.GetCode(err) != code || apperrors.GetHTTPStatus(err) != status {
		t.Fatalf("err = %v (%s, %d), want %s %d", err, apperrors.GetCode(err), apperrors.GetHTTPStatus(err), code, status)
	}
}

func TestSendFriendRequestRefusals(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		row    data.UserRelation
		code   string
		status int
	}{
		{"already friends", data.UserRelation{Friend: true}, apperrors.CodeAlreadyFriends, http.StatusConflict},
		{"already sent", data.UserRelation{RequestSent: true}, apperrors.CodeFriendRequestPending, http.StatusConflict},
		{"policy nobody", data.UserRelation{RequestPolicy: data.FriendRequestPolicyNobody}, apperrors.CodeFriendRequestsClosed, http.StatusForbidden},
		{"policy mutuals, not mutual", data.UserRelation{RequestPolicy: data.FriendRequestPolicyMutuals, Following: true}, apperrors.CodeFriendRequestsClosed, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestSocialService(t, config.SocialConfig{})
			e.expectUsers(2)
			e.expectRelation(1, 2, tc.row)
			_, _, err := e.svc.SendFriendRequest(ctx, 1, 2)
			assertSocialCode(t, err, tc.code, tc.status)
		})
	}

	t.Run("self", func(t *testing.T) {
		e := newTestSocialService(t, config.SocialConfig{})
		_, _, err := e.svc.SendFriendRequest(ctx, 1, 1)
		assertSocialCode(t, err, apperrors.CodeSelfRelation, http.StatusBadRequest)
	})
	t.Run("unknown user", func(t *testing.T) {
		e := newTestSocialService(t, config.SocialConfig{})
		e.repo.EXPECT().GetUser(gomock.Any(), uint(9)).Return(nil, gorm.ErrRecordNotFound)
		_, _, err := e.svc.SendFriendRequest(ctx, 1, 9)
		if !apperrors.IsNotFound(err) {
			t.Fatalf("err = %v, want not found", err)
		}
	})
	t.Run("too many pending", func(t *testing.T) {
		e := newTestSocialService(t, config.SocialConfig{MaxPendingRequests: 2})
		e.expectUsers(2)
		e.expectRelation(1, 2, data.UserRelation{})
		e.repo.EXPECT().CountOutgoingRequests(gomock.Any(), uint(1)).Return(int64(2), nil)
		_, _, err := e.svc.SendFriendRequest(ctx, 1, 2)
		assertSocialCode(t, err, apperrors.CodeTooManyFriendRequests, http.StatusTooManyRequests)
	})
}

func TestSendFriendRequestNotifiesTheReceiverOnly(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{})
	e.expectUsers(1, 2)
	e.expectRelation(1, 2, data.UserRelation{})
	e.repo.EXPECT().SendFriendRequest(gomock.Any(), uint(1), uint(2)).Return(data.FriendRequestSent, nil)
	e.repo.EXPECT().CountIncomingRequests(gomock.Any(), uint(2)).Return(int64(3), nil)

	_, receiver := e.bus.SubscribeUser(2)
	_, sender := e.bus.SubscribeUser(1)
	_, anonymous := e.bus.Subscribe()

	accepted, _, err := e.svc.SendFriendRequest(ctx, 1, 2)
	if err != nil || accepted {
		t.Fatalf("accepted = %v, err = %v", accepted, err)
	}
	ev, ok := receive(receiver)
	if !ok || ev.Type != EventSocialFriendRequest || ev.UserID != 2 {
		t.Fatalf("receiver got %+v, %v", ev, ok)
	}
	payload := ev.Data.(map[string]any)
	if payload["incoming_requests"] != int64(3) || payload["user"].(map[string]any)["id"] != uint(1) {
		t.Fatalf("payload = %v", payload)
	}
	if ev, ok := receive(sender); ok {
		t.Fatalf("the sender got %+v", ev)
	}
	if ev, ok := receive(anonymous); ok {
		t.Fatalf("a stream without user got %+v", ev)
	}
}

func TestSendFriendRequestToSomeoneWhoAskedAccepts(t *testing.T) {
	ctx := context.Background()
	// the receiver refuses requests, but asked first: sending accepts their request
	e := newTestSocialService(t, config.SocialConfig{FriendRequestsPerHour: 1, MaxPendingRequests: 1})
	e.expectUsers(1, 2)
	e.expectRelation(1, 2, data.UserRelation{RequestReceived: true, RequestPolicy: data.FriendRequestPolicyNobody})
	e.repo.EXPECT().SendFriendRequest(gomock.Any(), uint(1), uint(2)).Return(data.FriendRequestAccepted, nil)
	e.repo.EXPECT().CountIncomingRequests(gomock.Any(), uint(1)).Return(int64(0), nil)

	_, other := e.bus.SubscribeUser(2)
	_, me := e.bus.SubscribeUser(1)
	accepted, _, err := e.svc.SendFriendRequest(ctx, 1, 2)
	if err != nil || !accepted {
		t.Fatalf("accepted = %v, err = %v", accepted, err)
	}
	if ev, ok := receive(other); !ok || ev.Type != EventSocialFriendRequestAccepted {
		t.Fatalf("other got %+v, %v", ev, ok)
	}
	if ev, ok := receive(me); !ok || ev.Type != EventSocialRequestsChanged {
		t.Fatalf("me got %+v, %v", ev, ok)
	}
}

func TestSendFriendRequestRateLimit(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{FriendRequestsPerHour: 3})
	e.expectUsers(1, 2, 3, 4)
	e.expectRelation(1, 2, data.UserRelation{})
	e.repo.EXPECT().SendFriendRequest(gomock.Any(), uint(1), uint(2)).Return(data.FriendRequestSent, nil)
	e.repo.EXPECT().CountIncomingRequests(gomock.Any(), uint(2)).Return(int64(1), nil)
	e.expectRelation(1, 4, data.UserRelation{})
	// burst = 3/3 = 1 request at once, then one every 20 minutes
	if _, _, err := e.svc.SendFriendRequest(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	_, _, err := e.svc.SendFriendRequest(ctx, 1, 4)
	assertSocialCode(t, err, apperrors.CodeSocialRateLimited, http.StatusTooManyRequests)
	// another user has their own budget
	e.expectRelation(3, 2, data.UserRelation{})
	e.repo.EXPECT().SendFriendRequest(gomock.Any(), uint(3), uint(2)).Return(data.FriendRequestSent, nil)
	e.repo.EXPECT().CountIncomingRequests(gomock.Any(), uint(2)).Return(int64(1), nil)
	if _, _, err := e.svc.SendFriendRequest(ctx, 3, 2); err != nil {
		t.Fatal(err)
	}
}

func TestFollowNotifiesOnceAndUnfollowIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{})
	e.expectUsers(1, 2)
	gomock.InOrder(
		e.repo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2}}, nil),
		e.repo.EXPECT().Follow(gomock.Any(), uint(1), uint(2)).Return(true, nil),
		e.repo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, Following: true}}, nil),
		// following again: no write, no event
		e.repo.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, Following: true}}, nil),
	)
	_, followed := e.bus.SubscribeUser(2)
	rel, err := e.svc.Follow(ctx, 1, 2)
	if err != nil || !rel.Following {
		t.Fatalf("rel = %+v, err = %v", rel, err)
	}
	if ev, ok := receive(followed); !ok || ev.Type != EventSocialFollowed {
		t.Fatalf("followed got %+v, %v", ev, ok)
	}
	if _, err := e.svc.Follow(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if ev, ok := receive(followed); ok {
		t.Fatalf("second follow notified %+v", ev)
	}

	if _, err := e.svc.Follow(ctx, 1, 1); apperrors.GetCode(err) != apperrors.CodeSelfRelation {
		t.Fatalf("self follow: %v", err)
	}
}

func TestAcceptDeclineCancelNotFound(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{})
	e.expectUsers(2)
	e.repo.EXPECT().AcceptFriendRequest(gomock.Any(), uint(1), uint(2)).Return(false, nil)
	e.repo.EXPECT().DeleteFriendRequest(gomock.Any(), uint(2), uint(1)).Return(false, nil)
	e.repo.EXPECT().DeleteFriendRequest(gomock.Any(), uint(1), uint(2)).Return(false, nil)
	e.repo.EXPECT().RemoveFriend(gomock.Any(), uint(1), uint(2)).Return(false, nil)

	if _, err := e.svc.AcceptFriendRequest(ctx, 1, 2); apperrors.GetCode(err) != apperrors.CodeFriendRequestNotFound {
		t.Fatalf("accept: %v", err)
	}
	if _, err := e.svc.DeclineFriendRequest(ctx, 1, 2); apperrors.GetCode(err) != apperrors.CodeFriendRequestNotFound {
		t.Fatalf("decline: %v", err)
	}
	if _, err := e.svc.CancelFriendRequest(ctx, 1, 2); apperrors.GetCode(err) != apperrors.CodeFriendRequestNotFound {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := e.svc.RemoveFriend(ctx, 1, 2); !apperrors.IsNotFound(err) {
		t.Fatalf("remove: %v", err)
	}
}

func TestAcceptFriendRequestNotifiesTheSender(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{})
	e.expectUsers(1, 2)
	e.expectRelation(1, 2, data.UserRelation{Friend: true})
	e.repo.EXPECT().AcceptFriendRequest(gomock.Any(), uint(1), uint(2)).Return(true, nil)
	e.repo.EXPECT().CountIncomingRequests(gomock.Any(), uint(1)).Return(int64(0), nil)
	_, sender := e.bus.SubscribeUser(2)
	rel, err := e.svc.AcceptFriendRequest(ctx, 1, 2)
	if err != nil || !rel.Friend {
		t.Fatalf("rel = %+v, err = %v", rel, err)
	}
	ev, ok := receive(sender)
	if !ok || ev.Type != EventSocialFriendRequestAccepted || ev.Data.(map[string]any)["user"].(map[string]any)["id"] != uint(1) {
		t.Fatalf("sender got %+v, %v", ev, ok)
	}
}

func TestListRelationsPrivacy(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{})
	owner := &data.SocialUser{ID: 2, Username: "owner"}
	e.repo.EXPECT().GetUserByUsername(gomock.Any(), "owner").Return(owner, nil).AnyTimes()
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(2)).Return([]data.UserPrivacySetting{{UserID: 2, Item: PrivacyFriendsList, Audience: "friends"}}, nil).AnyTimes()
	e.expectRelation(1, 2, data.UserRelation{Following: true})

	if _, err := e.svc.ListRelations(ctx, 1, "owner", SocialListFriends, 1, 10); !apperrors.IsPrivacyRestricted(err) {
		t.Fatalf("friends list: %v, want privacy restricted", err)
	}

	// followers list is public by default: listed with the viewer's relations
	e.repo.EXPECT().ListFollowers(gomock.Any(), uint(2), 1, 10).Return([]data.SocialUser{{ID: 1, Username: "me"}, {ID: 3, Username: "c"}}, int64(2), nil)
	e.repo.EXPECT().Relations(gomock.Any(), uint(1), []uint{3}).Return([]data.UserRelation{{UserID: 3, FollowedBy: true}}, nil)
	page, err := e.svc.ListRelations(ctx, 1, "owner", SocialListFollowers, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || !page.Entries[0].Relationship.Self || !page.Entries[1].Relationship.FollowedBy {
		t.Fatalf("page = %+v", page)
	}

	// the owner sees their own private list
	e.repo.EXPECT().ListFriends(gomock.Any(), uint(2), 1, 10).Return(nil, int64(0), nil)
	if _, err := e.svc.ListRelations(ctx, 2, "owner", SocialListFriends, 1, 10); err != nil {
		t.Fatalf("own list: %v", err)
	}
}

func TestProfileCountsFollowPrivacy(t *testing.T) {
	ctx := context.Background()
	e := newTestSocialService(t, config.SocialConfig{})
	e.repo.EXPECT().GetUserByUsername(gomock.Any(), "owner").Return(&data.SocialUser{ID: 2, Username: "owner"}, nil)
	e.expectRelation(1, 2, data.UserRelation{})
	e.privacy.EXPECT().ListSettings(gomock.Any(), uint(2)).Return([]data.UserPrivacySetting{{UserID: 2, Item: PrivacyFollowersList, Audience: "private"}}, nil)
	e.repo.EXPECT().Counts(gomock.Any(), uint(2)).Return(data.SocialCounts{Friends: 4, Followers: 5, Following: 6}, nil)

	p, err := e.svc.Profile(ctx, 1, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if !p.ProfileVisible || !p.FriendsVisible || p.FollowersVisible {
		t.Fatalf("visibility = %+v", p)
	}
	if p.Friends == nil || *p.Friends != 4 || p.Followers != nil || p.Following != nil {
		t.Fatalf("counts = %v %v %v", p.Friends, p.Followers, p.Following)
	}
}
