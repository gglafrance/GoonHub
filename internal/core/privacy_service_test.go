package core

import (
	"context"
	"reflect"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestPrivacyService(t *testing.T) (*PrivacyService, *mocks.MockPrivacyRepository, *mocks.MockSocialRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockPrivacyRepository(ctrl)
	social := mocks.NewMockSocialRepository(ctrl)
	return NewPrivacyService(repo, social, zap.NewNop()), repo, social
}

// The exact semantics of the audiences (docs/SOCIAL.md).
func TestRelationshipPassesAudiences(t *testing.T) {
	none := Relationship{}
	follower := Relationship{Following: true}
	followedOnly := Relationship{FollowedBy: true}
	mutual := Relationship{Following: true, FollowedBy: true, Mutual: true}
	friend := Relationship{Friend: true}
	friendNoFollow := Relationship{Friend: true, RequestSent: true}

	tests := []struct {
		name string
		rel  Relationship
		want map[Audience]bool
	}{
		{"stranger", none, map[Audience]bool{AudiencePublic: true}},
		{"followed by the owner only", followedOnly, map[Audience]bool{AudiencePublic: true}},
		{"follower", follower, map[Audience]bool{AudiencePublic: true, AudienceFollowers: true}},
		{"mutual", mutual, map[Audience]bool{AudiencePublic: true, AudienceFollowers: true, AudienceMutuals: true}},
		{"friend", friend, map[Audience]bool{AudiencePublic: true, AudienceFollowers: true, AudienceMutuals: true, AudienceFriends: true}},
		{"friend without follows", friendNoFollow, map[Audience]bool{AudiencePublic: true, AudienceFollowers: true, AudienceMutuals: true, AudienceFriends: true}},
		{"self", selfRelationship, map[Audience]bool{AudiencePublic: true, AudienceFollowers: true, AudienceMutuals: true, AudienceFriends: true, AudiencePrivate: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, a := range Audiences {
				if got := tc.rel.Passes(a.Value); got != tc.want[a.Value] {
					t.Fatalf("Passes(%s) = %v, want %v", a.Value, got, tc.want[a.Value])
				}
			}
		})
	}
}

func TestRelationshipFromRequestPolicy(t *testing.T) {
	tests := []struct {
		name string
		row  data.UserRelation
		want bool
	}{
		{"everyone", data.UserRelation{RequestPolicy: "everyone"}, true},
		{"mutuals, not mutual", data.UserRelation{RequestPolicy: "mutuals", Following: true}, false},
		{"mutuals, mutual", data.UserRelation{RequestPolicy: "mutuals", Following: true, FollowedBy: true}, true},
		{"nobody", data.UserRelation{RequestPolicy: "nobody"}, false},
		{"nobody but they asked me", data.UserRelation{RequestPolicy: "nobody", RequestReceived: true}, true},
		{"already friends", data.UserRelation{RequestPolicy: "everyone", Friend: true}, false},
		{"already sent", data.UserRelation{RequestPolicy: "everyone", RequestSent: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := relationshipFrom(tc.row).CanSendFriendRequest; got != tc.want {
				t.Fatalf("CanSendFriendRequest = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrivacyCatalogIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range PrivacyCatalog {
		if item.Key == "" || item.Label == "" || item.Group == "" {
			t.Fatalf("incomplete item %+v", item)
		}
		if seen[item.Key] {
			t.Fatalf("duplicate item %s", item.Key)
		}
		seen[item.Key] = true
		if !item.Default.Valid() {
			t.Fatalf("item %s has an invalid default %q", item.Key, item.Default)
		}
		if len(item.Key) > 64 {
			t.Fatalf("item key %s longer than the column", item.Key)
		}
	}
}

func TestVisibleToSelfAlwaysAndNoQueryForPublic(t *testing.T) {
	svc, repo, _ := newTestPrivacyService(t)
	ctx := context.Background()
	// owner 2 is public by default for the profile; the viewer (1) is in the list too
	repo.EXPECT().AudiencesFor(gomock.Any(), []uint{2}, PrivacyProfile).Return(map[uint]string{}, nil)

	got, err := svc.VisibleTo(ctx, 1, []uint{1, 2}, PrivacyProfile)
	if err != nil {
		t.Fatal(err)
	}
	if !got[1] || !got[2] {
		t.Fatalf("got %v, want both visible", got)
	}
}

func TestVisibleToBatch(t *testing.T) {
	svc, repo, social := newTestPrivacyService(t)
	ctx := context.Background()
	// activity defaults to friends; 3 made it public, 4 private, 5 followers
	repo.EXPECT().AudiencesFor(gomock.Any(), []uint{2, 3, 4, 5, 6}, PrivacyActivity).
		Return(map[uint]string{3: "public", 4: "private", 5: "followers"}, nil)
	// only the restricted owners (not public, not private) are looked up
	social.EXPECT().Relations(gomock.Any(), uint(1), []uint{2, 5, 6}).Return([]data.UserRelation{
		{UserID: 2, Friend: true},
		{UserID: 5, Following: true},
		{UserID: 6, Following: true, FollowedBy: true},
	}, nil)

	got, err := svc.VisibleTo(ctx, 1, []uint{2, 3, 4, 5, 6}, PrivacyActivity)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint]bool{2: true, 3: true, 4: false, 5: true, 6: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPrivateHidesFromSuperusersToo(t *testing.T) {
	// the service has no notion of permissions: an administrator is a viewer like any other
	svc, repo, _ := newTestPrivacyService(t)
	repo.EXPECT().AudiencesFor(gomock.Any(), []uint{2}, PrivacyStats).Return(map[uint]string{2: "private"}, nil)
	ok, err := svc.CanView(context.Background(), 1, 2, PrivacyStats)
	if err != nil || ok {
		t.Fatalf("CanView = %v, %v; want false", ok, err)
	}
}

func TestVisibleToUnknownItem(t *testing.T) {
	svc, _, _ := newTestPrivacyService(t)
	if _, err := svc.VisibleTo(context.Background(), 1, []uint{2}, "nope"); err == nil {
		t.Fatal("expected an error for an unknown item")
	}
}

func TestVisibilityOneOwnerManyItems(t *testing.T) {
	svc, repo, social := newTestPrivacyService(t)
	repo.EXPECT().ListSettings(gomock.Any(), uint(2)).Return([]data.UserPrivacySetting{
		{UserID: 2, Item: PrivacyFriendsList, Audience: "mutuals"},
		{UserID: 2, Item: PrivacyFollowersList, Audience: "followers"},
	}, nil)
	social.EXPECT().Relations(gomock.Any(), uint(1), []uint{2}).Return([]data.UserRelation{{UserID: 2, Following: true}}, nil).Times(1)

	got, err := svc.Visibility(context.Background(), 1, 2, []string{PrivacyProfile, PrivacyFriendsList, PrivacyFollowersList})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{PrivacyProfile: true, PrivacyFriendsList: false, PrivacyFollowersList: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAudienceMembers(t *testing.T) {
	ctx := context.Background()
	t.Run("public is everyone", func(t *testing.T) {
		svc, repo, _ := newTestPrivacyService(t)
		repo.EXPECT().AudiencesFor(gomock.Any(), []uint{1}, PrivacyProfile).Return(map[uint]string{}, nil)
		m, err := svc.AudienceMembers(ctx, 1, PrivacyProfile)
		if err != nil || !m.Everyone || !m.Contains(42) {
			t.Fatalf("got %+v, %v", m, err)
		}
	})
	t.Run("followers include friends", func(t *testing.T) {
		svc, repo, social := newTestPrivacyService(t)
		repo.EXPECT().AudiencesFor(gomock.Any(), []uint{1}, PrivacyOnlineStatus).Return(map[uint]string{1: "followers"}, nil)
		social.EXPECT().FollowerIDs(gomock.Any(), uint(1)).Return([]uint{5, 3}, nil)
		social.EXPECT().FriendIDs(gomock.Any(), uint(1)).Return([]uint{3, 9}, nil)
		m, err := svc.AudienceMembers(ctx, 1, PrivacyOnlineStatus)
		if err != nil {
			t.Fatal(err)
		}
		if m.Everyone || !reflect.DeepEqual(m.UserIDs, []uint{3, 5, 9}) || !m.Contains(5) || m.Contains(4) {
			t.Fatalf("got %+v", m)
		}
	})
	t.Run("friends by default", func(t *testing.T) {
		svc, repo, social := newTestPrivacyService(t)
		repo.EXPECT().AudiencesFor(gomock.Any(), []uint{1}, PrivacyNowWatching).Return(map[uint]string{}, nil)
		social.EXPECT().FriendIDs(gomock.Any(), uint(1)).Return([]uint{7}, nil)
		m, err := svc.AudienceMembers(ctx, 1, PrivacyNowWatching)
		if err != nil || !reflect.DeepEqual(m.UserIDs, []uint{7}) {
			t.Fatalf("got %+v, %v", m, err)
		}
	})
	t.Run("private is nobody", func(t *testing.T) {
		svc, repo, _ := newTestPrivacyService(t)
		repo.EXPECT().AudiencesFor(gomock.Any(), []uint{1}, PrivacyActivity).Return(map[uint]string{1: "private"}, nil)
		m, err := svc.AudienceMembers(ctx, 1, PrivacyActivity)
		if err != nil || m.Everyone || len(m.UserIDs) != 0 {
			t.Fatalf("got %+v, %v", m, err)
		}
	})
}

func TestGetSettingsDefaults(t *testing.T) {
	svc, repo, social := newTestPrivacyService(t)
	repo.EXPECT().ListSettings(gomock.Any(), uint(1)).Return([]data.UserPrivacySetting{
		{UserID: 1, Item: PrivacyStats, Audience: "public"},
		{UserID: 1, Item: "removed_item", Audience: "public"},
	}, nil)
	social.EXPECT().GetFriendRequestPolicy(gomock.Any(), uint(1)).Return("mutuals", nil)

	s, err := svc.GetSettings(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items) != len(PrivacyCatalog) || s.FriendRequestPolicy != "mutuals" {
		t.Fatalf("got %+v", s)
	}
	for _, it := range s.Items {
		switch it.Item.Key {
		case PrivacyStats:
			if it.Audience != AudiencePublic || it.IsDefault {
				t.Fatalf("stats = %+v", it)
			}
		default:
			if it.Audience != it.Item.Default || !it.IsDefault {
				t.Fatalf("%s = %+v, want the default", it.Item.Key, it)
			}
		}
	}
}

func TestUpdateSettings(t *testing.T) {
	ctx := context.Background()
	t.Run("sets, resets and the policy", func(t *testing.T) {
		svc, repo, social := newTestPrivacyService(t)
		policy := "nobody"
		repo.EXPECT().SaveSettings(gomock.Any(), uint(1), map[string]string{PrivacyStats: "private"}, []string{PrivacyActivity}).Return(nil)
		social.EXPECT().SetFriendRequestPolicy(gomock.Any(), uint(1), "nobody").Return(nil)
		repo.EXPECT().ListSettings(gomock.Any(), uint(1)).Return(nil, nil)
		social.EXPECT().GetFriendRequestPolicy(gomock.Any(), uint(1)).Return("nobody", nil)
		if _, err := svc.UpdateSettings(ctx, 1, UpdatePrivacyInput{
			Items:               map[string]string{PrivacyStats: "private", PrivacyActivity: "default"},
			FriendRequestPolicy: &policy,
		}); err != nil {
			t.Fatal(err)
		}
	})
	invalid := []struct {
		name string
		in   UpdatePrivacyInput
	}{
		{"unknown item", UpdatePrivacyInput{Items: map[string]string{"nope": "public"}}},
		{"unknown audience", UpdatePrivacyInput{Items: map[string]string{PrivacyStats: "admins"}}},
		{"unknown policy", UpdatePrivacyInput{FriendRequestPolicy: ptrString("friends")}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newTestPrivacyService(t)
			if _, err := svc.UpdateSettings(ctx, 1, tc.in); !apperrors.IsValidation(err) {
				t.Fatalf("err = %v, want a validation error", err)
			}
		})
	}
}

func ptrString(s string) *string { return &s }
