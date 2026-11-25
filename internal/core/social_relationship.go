package core

import "goonhub/internal/data"

// Relationship is the relation of a viewer toward another user (docs/SOCIAL.md).
type Relationship struct {
	// Self: the viewer is the user.
	Self bool
	// Following: the viewer follows the user.
	Following bool
	// FollowedBy: the user follows the viewer.
	FollowedBy bool
	// Mutual: both follow each other.
	Mutual bool
	// Friend: they are friends.
	Friend bool
	// RequestSent: the viewer sent the user a friend request, still pending.
	RequestSent bool
	// RequestReceived: the user sent the viewer a friend request, still pending.
	RequestReceived bool
	// CanSendFriendRequest: a friend request from the viewer would be accepted by the
	// user's policy (or accepts the request the user sent). It does not look at the viewer's
	// social:use permission.
	CanSendFriendRequest bool
}

// tier is the closest relation of the viewer toward the user, for the audiences.
func (r Relationship) tier() int {
	switch {
	case r.Self:
		return tierSelf
	case r.Friend:
		return tierFriend
	case r.Mutual:
		return tierMutual
	case r.Following:
		return tierFollower
	default:
		return tierNone
	}
}

// Passes reports whether the viewer is in the audience. The owner passes every audience
// (private included); a friend passes every audience but private; a mutual passes followers
// and mutuals; a follower passes followers; any signed-in user passes public.
func (r Relationship) Passes(a Audience) bool {
	return r.tier() >= a.rank()
}

// selfRelationship is the relation of a user toward themselves.
var selfRelationship = Relationship{Self: true}

// relationshipFrom builds a Relationship from the raw repository row.
func relationshipFrom(row data.UserRelation) Relationship {
	r := Relationship{
		Following:       row.Following,
		FollowedBy:      row.FollowedBy,
		Mutual:          row.Following && row.FollowedBy,
		Friend:          row.Friend,
		RequestSent:     row.RequestSent,
		RequestReceived: row.RequestReceived,
	}
	if !r.Friend && !r.RequestSent {
		switch {
		case r.RequestReceived:
			r.CanSendFriendRequest = true
		case row.RequestPolicy == data.FriendRequestPolicyMutuals:
			r.CanSendFriendRequest = r.Mutual
		case row.RequestPolicy == data.FriendRequestPolicyNobody:
			r.CanSendFriendRequest = false
		default:
			r.CanSendFriendRequest = true
		}
	}
	return r
}
