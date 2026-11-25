package data

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// Integration test of the social graph SQL against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN,
// a disposable database, see importTestDB).
func TestSocialRepositoryIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	mk := func(name string) User {
		u := User{Username: "soc-" + name + "-" + suffix, Password: "x", Role: "user"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u
	}
	a, b, c, d := mk("a"), mk("b"), mk("c"), mk("d")
	t.Cleanup(func() {
		if err := db.Where("username LIKE ?", "%-"+suffix).Delete(&User{}).Error; err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	repo := NewSocialRepository(db)
	privacy := NewPrivacyRepository(db)

	// follows: a -> b, b -> a (mutuals), c -> a
	for _, f := range [][2]uint{{a.ID, b.ID}, {b.ID, a.ID}, {c.ID, a.ID}} {
		created, err := repo.Follow(ctx, f[0], f[1])
		if err != nil || !created {
			t.Fatalf("follow %v: %v %v", f, created, err)
		}
	}
	if created, err := repo.Follow(ctx, a.ID, b.ID); err != nil || created {
		t.Fatalf("second follow: %v %v", created, err)
	}
	if _, err := repo.Follow(ctx, a.ID, a.ID); err == nil {
		t.Fatal("self follow accepted by the schema")
	}

	// friend requests: a -> c sent, then c -> a accepts it
	if out, err := repo.SendFriendRequest(ctx, a.ID, c.ID); err != nil || out != FriendRequestSent {
		t.Fatalf("send: %v %v", out, err)
	}
	if out, err := repo.SendFriendRequest(ctx, a.ID, c.ID); err != nil || out != FriendRequestAlreadySent {
		t.Fatalf("send again: %v %v", out, err)
	}
	rels, err := repo.Relations(ctx, c.ID, []uint{a.ID, b.ID, a.ID})
	if err != nil || len(rels) != 2 {
		t.Fatalf("relations: %v %v", rels, err)
	}
	byID := map[uint]UserRelation{}
	for _, r := range rels {
		byID[r.UserID] = r
	}
	if r := byID[a.ID]; !r.Following || r.FollowedBy || !r.RequestReceived || r.RequestSent || r.Friend || r.RequestPolicy != "everyone" {
		t.Fatalf("c toward a = %+v", r)
	}
	if out, err := repo.SendFriendRequest(ctx, c.ID, a.ID); err != nil || out != FriendRequestAccepted {
		t.Fatalf("crossed request: %v %v", out, err)
	}
	if out, err := repo.SendFriendRequest(ctx, a.ID, c.ID); err != nil || out != FriendRequestAlreadyFriends {
		t.Fatalf("request to a friend: %v %v", out, err)
	}
	if n, _ := repo.CountIncomingRequests(ctx, c.ID); n != 0 {
		t.Fatalf("pending after accept: %d", n)
	}

	// b -> d, accepted by d; d -> a declined
	if _, err := repo.SendFriendRequest(ctx, b.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.AcceptFriendRequest(ctx, d.ID, b.ID); err != nil || !ok {
		t.Fatalf("accept: %v %v", ok, err)
	}
	if ok, err := repo.AcceptFriendRequest(ctx, d.ID, b.ID); err != nil || ok {
		t.Fatalf("accept twice: %v %v", ok, err)
	}
	if _, err := repo.SendFriendRequest(ctx, d.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	in, total, err := repo.ListIncomingRequests(ctx, a.ID, 1, 10)
	if err != nil || total != 1 || len(in) != 1 || in[0].ID != d.ID || in[0].Since.IsZero() {
		t.Fatalf("incoming: %v %d %v", in, total, err)
	}
	out, total, err := repo.ListOutgoingRequests(ctx, d.ID, 1, 10)
	if err != nil || total != 1 || out[0].ID != a.ID {
		t.Fatalf("outgoing: %v %d %v", out, total, err)
	}
	if ok, err := repo.DeleteFriendRequest(ctx, d.ID, a.ID); err != nil || !ok {
		t.Fatalf("decline: %v %v", ok, err)
	}

	// lists and counts
	friends, total, err := repo.ListFriends(ctx, a.ID, 1, 10)
	if err != nil || total != 1 || friends[0].ID != c.ID {
		t.Fatalf("friends of a: %v %d %v", friends, total, err)
	}
	followers, total, err := repo.ListFollowers(ctx, a.ID, 1, 1)
	if err != nil || total != 2 || len(followers) != 1 || followers[0].ID != c.ID {
		t.Fatalf("followers of a (page 1, most recent first): %v %d %v", followers, total, err)
	}
	following, _, err := repo.ListFollowing(ctx, a.ID, 1, 10)
	if err != nil || len(following) != 1 || following[0].ID != b.ID {
		t.Fatalf("following of a: %v %v", following, err)
	}
	counts, err := repo.Counts(ctx, a.ID)
	if err != nil || counts != (SocialCounts{Friends: 1, Followers: 2, Following: 1}) {
		t.Fatalf("counts: %+v %v", counts, err)
	}
	if ids, _ := repo.MutualIDs(ctx, a.ID); len(ids) != 1 || ids[0] != b.ID {
		t.Fatalf("mutuals of a: %v", ids)
	}
	if ids, _ := repo.FriendIDs(ctx, c.ID); len(ids) != 1 || ids[0] != a.ID {
		t.Fatalf("friends of c: %v", ids)
	}
	if ids, _ := repo.FollowerIDs(ctx, a.ID); len(ids) != 2 {
		t.Fatalf("followers ids of a: %v", ids)
	}

	// search: prefix first, viewer excluded, wildcards escaped
	found, total, err := repo.SearchUsers(ctx, a.ID, "soc-", 1, 10)
	if err != nil || total < 3 {
		t.Fatalf("search: %v %d %v", found, total, err)
	}
	for _, u := range found {
		if u.ID == a.ID {
			t.Fatal("search lists the viewer")
		}
	}
	if _, total, _ := repo.SearchUsers(ctx, a.ID, "%_"+suffix, 1, 10); total != 0 {
		t.Fatalf("wildcards not escaped: %d", total)
	}

	// friend request policy
	if p, _ := repo.GetFriendRequestPolicy(ctx, b.ID); p != FriendRequestPolicyEveryone {
		t.Fatalf("default policy %q", p)
	}
	if err := repo.SetFriendRequestPolicy(ctx, b.ID, FriendRequestPolicyNobody); err != nil {
		t.Fatal(err)
	}
	if rels, _ := repo.Relations(ctx, a.ID, []uint{b.ID}); rels[0].RequestPolicy != FriendRequestPolicyNobody || !rels[0].Following || !rels[0].FollowedBy {
		t.Fatalf("a toward b = %+v", rels[0])
	}

	// privacy settings
	if err := privacy.SaveSettings(ctx, a.ID, map[string]string{"stats": "private", "activity": "public"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := privacy.SaveSettings(ctx, a.ID, map[string]string{"stats": "friends"}, []string{"activity"}); err != nil {
		t.Fatal(err)
	}
	rows, err := privacy.ListSettings(ctx, a.ID)
	if err != nil || len(rows) != 1 || rows[0].Item != "stats" || rows[0].Audience != "friends" {
		t.Fatalf("settings: %v %v", rows, err)
	}
	aud, err := privacy.AudiencesFor(ctx, []uint{a.ID, b.ID}, "stats")
	if err != nil || len(aud) != 1 || aud[a.ID] != "friends" {
		t.Fatalf("audiences: %v %v", aud, err)
	}
	if err := privacy.SaveSettings(ctx, a.ID, map[string]string{"stats": "admins"}, nil); err == nil {
		t.Fatal("invalid audience accepted by the schema")
	}

	// crossed concurrent requests give one friendship, no pending request
	var wg sync.WaitGroup
	results := make([]FriendRequestOutcome, 2)
	for i, pair := range [][2]uint{{b.ID, c.ID}, {c.ID, b.ID}} {
		wg.Add(1)
		go func(i int, from, to uint) {
			defer wg.Done()
			res, err := repo.SendFriendRequest(ctx, from, to)
			if err != nil {
				t.Errorf("concurrent send: %v", err)
			}
			results[i] = res
		}(i, pair[0], pair[1])
	}
	wg.Wait()
	if !((results[0] == FriendRequestSent && results[1] == FriendRequestAccepted) || (results[0] == FriendRequestAccepted && results[1] == FriendRequestSent)) {
		t.Fatalf("concurrent outcomes %v", results)
	}
	if rels, _ := repo.Relations(ctx, b.ID, []uint{c.ID}); !rels[0].Friend || rels[0].RequestSent || rels[0].RequestReceived {
		t.Fatalf("b toward c after crossed requests = %+v", rels[0])
	}

	// removing a friend, then deleting a user cascades everything
	if ok, err := repo.RemoveFriend(ctx, c.ID, a.ID); err != nil || !ok {
		t.Fatalf("remove friend: %v %v", ok, err)
	}
	if err := db.Delete(&User{}, b.ID).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var left int64
	db.Raw(`SELECT (SELECT COUNT(*) FROM user_follows WHERE follower_id = ? OR followed_id = ?)
		+ (SELECT COUNT(*) FROM user_friends WHERE user_id = ?)
		+ (SELECT COUNT(*) FROM friend_requests WHERE sender_id = ? OR receiver_id = ?)
		+ (SELECT COUNT(*) FROM user_social_settings WHERE user_id = ?)`, b.ID, b.ID, b.ID, b.ID, b.ID, b.ID).Scan(&left)
	if left != 0 {
		t.Fatalf("%d rows left after deleting the user", left)
	}
}
