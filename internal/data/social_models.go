package data

import "time"

// UserFollow is a one-way follow: FollowerID follows FollowedID (docs/SOCIAL.md).
type UserFollow struct {
	FollowerID uint      `gorm:"primaryKey;autoIncrement:false"`
	FollowedID uint      `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
}

func (UserFollow) TableName() string { return "user_follows" }

// UserFriendship is a friendship, stored once per pair with the smaller id first.
type UserFriendship struct {
	UserLowID  uint      `gorm:"primaryKey;autoIncrement:false"`
	UserHighID uint      `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
}

func (UserFriendship) TableName() string { return "user_friendships" }

// FriendRequest is a pending friend request (accepted, declined and cancelled ones are
// deleted).
type FriendRequest struct {
	ID         uint      `gorm:"primarykey"`
	SenderID   uint      `gorm:"not null"`
	ReceiverID uint      `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
}

func (FriendRequest) TableName() string { return "friend_requests" }

// UserPrivacySetting is a user's explicit choice of audience for a privacy item. No row means
// the catalog default.
type UserPrivacySetting struct {
	UserID    uint      `gorm:"primaryKey;autoIncrement:false"`
	Item      string    `gorm:"primaryKey;size:64"`
	Audience  string    `gorm:"not null;size:16"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (UserPrivacySetting) TableName() string { return "user_privacy_settings" }

// Friend request policies: who may send a user a friend request.
const (
	FriendRequestPolicyEveryone = "everyone"
	FriendRequestPolicyMutuals  = "mutuals"
	FriendRequestPolicyNobody   = "nobody"
)

// UserSocialSettings holds the social preferences of a user. No row means the defaults.
type UserSocialSettings struct {
	UserID              uint      `gorm:"primaryKey;autoIncrement:false"`
	FriendRequestPolicy string    `gorm:"not null;size:16;default:'everyone'"`
	UpdatedAt           time.Time `gorm:"not null;default:now()"`
}

func (UserSocialSettings) TableName() string { return "user_social_settings" }

// SocialUser is the public card of a user in the social lists: no role, no login dates.
// Since is the date of the listed relation (follow, friendship, request) when there is one.
// DisplayName and AvatarVersion come from the profile (user_profiles, docs/PROFILES.md;
// empty and 0 without one).
type SocialUser struct {
	ID            uint      `gorm:"column:id"`
	Username      string    `gorm:"column:username"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	Since         time.Time `gorm:"column:since"`
	DisplayName   string    `gorm:"column:display_name"`
	AvatarVersion int64     `gorm:"column:avatar_version"`
}

// socialUserColumns are the SocialUser columns of users aliased u, joined to their profile
// with socialUserProfileJoin.
const (
	socialUserColumns     = `u.id, u.username, u.created_at, COALESCE(up.display_name, '') AS display_name, COALESCE(up.avatar_version, 0) AS avatar_version`
	socialUserProfileJoin = ` LEFT JOIN user_profiles up ON up.user_id = u.id`
)

// UserRelation is the raw relation of a viewer toward another user, read in one query.
type UserRelation struct {
	UserID          uint   `gorm:"column:user_id"`
	Following       bool   `gorm:"column:following"`        // the viewer follows the user
	FollowedBy      bool   `gorm:"column:followed_by"`      // the user follows the viewer
	Friend          bool   `gorm:"column:friend"`           // they are friends
	RequestSent     bool   `gorm:"column:request_sent"`     // the viewer sent the user a pending request
	RequestReceived bool   `gorm:"column:request_received"` // the user sent the viewer a pending request
	RequestPolicy   string `gorm:"column:request_policy"`   // the user's friend request policy
}

// SocialCounts are the relation counts of a user.
type SocialCounts struct {
	Friends   int64 `gorm:"column:friends"`
	Followers int64 `gorm:"column:followers"`
	Following int64 `gorm:"column:following"`
}

// FriendRequestOutcome is the result of SendFriendRequest.
type FriendRequestOutcome string

const (
	// FriendRequestSent: a new pending request was created.
	FriendRequestSent FriendRequestOutcome = "sent"
	// FriendRequestAccepted: the receiver had already requested the sender, so they are now
	// friends (the pending request was accepted).
	FriendRequestAccepted FriendRequestOutcome = "accepted"
	// FriendRequestAlreadySent: a request from the sender is already pending.
	FriendRequestAlreadySent FriendRequestOutcome = "already_sent"
	// FriendRequestAlreadyFriends: they are already friends.
	FriendRequestAlreadyFriends FriendRequestOutcome = "already_friends"
)
