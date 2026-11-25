package core

// Privacy framework (docs/SOCIAL.md): each user chooses, per information item, who can see it.
// The items are declared here, like the permissions in rbac_catalog.go; the database only
// stores the explicit choices (user_privacy_settings, no row = the item's default).

// Audience is who can see an item. Ordered from the widest to the narrowest: a viewer who
// passes an audience passes every wider one.
type Audience string

const (
	// AudiencePublic: every signed-in user of the instance.
	AudiencePublic Audience = "public"
	// AudienceFollowers: the users who follow the owner (follows need no approval), mutuals
	// and friends.
	AudienceFollowers Audience = "followers"
	// AudienceMutuals: the users the owner follows and who follow the owner back, and friends.
	AudienceMutuals Audience = "mutuals"
	// AudienceFriends: the owner's friends only.
	AudienceFriends Audience = "friends"
	// AudiencePrivate: the owner only.
	AudiencePrivate Audience = "private"
)

// AudienceInfo describes an audience for the settings UI.
type AudienceInfo struct {
	Value       Audience
	Label       string
	Description string
}

// Audiences lists the audiences, from the widest to the narrowest.
var Audiences = []AudienceInfo{
	{AudiencePublic, "Everyone", "Every signed-in user"},
	{AudienceFollowers, "Followers", "Users who follow you, mutuals and friends"},
	{AudienceMutuals, "Mutuals", "Users you follow who follow you back, and friends"},
	{AudienceFriends, "Friends", "Your friends only"},
	{AudiencePrivate, "Only me", "Nobody else, administrators included"},
}

// rank is the minimum viewer tier an audience requires.
func (a Audience) rank() int {
	switch a {
	case AudiencePublic:
		return tierNone
	case AudienceFollowers:
		return tierFollower
	case AudienceMutuals:
		return tierMutual
	case AudienceFriends:
		return tierFriend
	default:
		return tierSelf
	}
}

// Valid reports whether the audience is known.
func (a Audience) Valid() bool {
	for _, info := range Audiences {
		if info.Value == a {
			return true
		}
	}
	return false
}

// Viewer tiers: the closest relation of a viewer toward an owner. A higher tier passes every
// audience a lower tier passes.
const (
	tierNone     = 0 // any signed-in user
	tierFollower = 1 // follows the owner
	tierMutual   = 2 // follows the owner and is followed back
	tierFriend   = 3 // friend of the owner
	tierSelf     = 4 // the owner
)

// Privacy item keys. Wave 2 features add theirs here (and to PrivacyCatalog).
const (
	PrivacyProfile       = "profile"
	PrivacyFavorites     = "favorites"
	PrivacyStats         = "stats"
	PrivacyActivity      = "activity"
	PrivacyFriendsList   = "friends_list"
	PrivacyFollowersList = "followers_list"
	PrivacyOnlineStatus  = "online_status"
	PrivacyNowWatching   = "now_watching"
)

// Privacy item groups (shown by the UI, in this order).
const (
	PrivacyGroupProfile     = "Profile"
	PrivacyGroupConnections = "Connections"
	PrivacyGroupPresence    = "Presence"
)

// PrivacyItem is an information item a user can restrict.
type PrivacyItem struct {
	Key         string
	Label       string
	Description string
	Group       string
	// Default is the audience of users who did not choose.
	Default Audience
}

// PrivacyCatalog lists the privacy items, in display order. Changing a default applies to
// every user who did not choose for that item.
var PrivacyCatalog = []PrivacyItem{
	{Key: PrivacyProfile, Label: "Profile page", Description: "Seeing your profile page at all. What your profile shows (lists, favorites, stats, activity) also needs its own item.", Group: PrivacyGroupProfile, Default: AudiencePublic},
	{Key: PrivacyFavorites, Label: "Favorites", Description: "Your favorite videos and markers on your profile", Group: PrivacyGroupProfile, Default: AudienceFriends},
	{Key: PrivacyStats, Label: "Stats", Description: "Your watch statistics", Group: PrivacyGroupProfile, Default: AudienceFriends},
	{Key: PrivacyActivity, Label: "Activity", Description: "What you watched recently and your most watched videos", Group: PrivacyGroupProfile, Default: AudienceFriends},
	{Key: PrivacyFriendsList, Label: "Friends list", Description: "Who your friends are", Group: PrivacyGroupConnections, Default: AudiencePublic},
	{Key: PrivacyFollowersList, Label: "Followers and following", Description: "Who follows you and who you follow", Group: PrivacyGroupConnections, Default: AudiencePublic},
	{Key: PrivacyOnlineStatus, Label: "Online status", Description: "Whether you are online", Group: PrivacyGroupPresence, Default: AudienceFriends},
	{Key: PrivacyNowWatching, Label: "Now watching", Description: "What you are watching right now", Group: PrivacyGroupPresence, Default: AudienceFriends},
}

// privacyItemByKey returns the catalog item of a key.
func privacyItemByKey(key string) (PrivacyItem, bool) {
	for _, item := range PrivacyCatalog {
		if item.Key == key {
			return item, true
		}
	}
	return PrivacyItem{}, false
}

// FriendRequestPolicyInfo describes a friend request policy for the settings UI.
type FriendRequestPolicyInfo struct {
	Value       string
	Label       string
	Description string
}

// FriendRequestPolicies lists who may send a user friend requests (data.FriendRequestPolicy*).
var FriendRequestPolicies = []FriendRequestPolicyInfo{
	{"everyone", "Everyone", "Any signed-in user"},
	{"mutuals", "Mutuals", "Users you follow who follow you back"},
	{"nobody", "Nobody", "Nobody can send you a request (you can still send some)"},
}

func validFriendRequestPolicy(p string) bool {
	for _, info := range FriendRequestPolicies {
		if info.Value == p {
			return true
		}
	}
	return false
}
