package response

import (
	"time"

	"goonhub/internal/core"
	"goonhub/internal/data"
)

// RelationshipResponse is the relation of the signed-in user toward another user.
type RelationshipResponse struct {
	Self                 bool `json:"self"`
	Following            bool `json:"following"`
	FollowedBy           bool `json:"followed_by"`
	Mutual               bool `json:"mutual"`
	Friend               bool `json:"friend"`
	RequestSent          bool `json:"request_sent"`
	RequestReceived      bool `json:"request_received"`
	CanSendFriendRequest bool `json:"can_send_friend_request"`
	CanMessage           bool `json:"can_message"`
}

func NewRelationshipResponse(r core.Relationship) RelationshipResponse {
	return RelationshipResponse{
		Self:                 r.Self,
		Following:            r.Following,
		FollowedBy:           r.FollowedBy,
		Mutual:               r.Mutual,
		Friend:               r.Friend,
		RequestSent:          r.RequestSent,
		RequestReceived:      r.RequestReceived,
		CanSendFriendRequest: r.CanSendFriendRequest,
		CanMessage:           r.CanMessage,
	}
}

// SocialUserResponse is the public card of a user: no role, no login dates. DisplayName and
// AvatarVersion come from the profile (docs/PROFILES.md): the avatar is at
// /api/v1/profiles/avatar/<id>/<size>?v=<avatar_version>, none when the version is 0.
type SocialUserResponse struct {
	ID            uint      `json:"id"`
	Username      string    `json:"username"`
	CreatedAt     time.Time `json:"created_at"`
	DisplayName   string    `json:"display_name"`
	AvatarVersion int64     `json:"avatar_version"`
}

func NewSocialUserResponse(u data.SocialUser) SocialUserResponse {
	return SocialUserResponse{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt, DisplayName: u.DisplayName, AvatarVersion: u.AvatarVersion}
}

// SocialUserEntryResponse is a user in a social list. Since is the date of the listed relation
// (follow, friendship, request; the account creation in the directory).
type SocialUserEntryResponse struct {
	SocialUserResponse
	Since        time.Time            `json:"since"`
	Relationship RelationshipResponse `json:"relationship"`
}

func NewSocialUserPage(page *core.SocialUserPage, pageNum, limit int) PaginatedResponse[SocialUserEntryResponse] {
	items := make([]SocialUserEntryResponse, len(page.Entries))
	for i, e := range page.Entries {
		items[i] = SocialUserEntryResponse{
			SocialUserResponse: NewSocialUserResponse(e.User),
			Since:              e.User.Since,
			Relationship:       NewRelationshipResponse(e.Relationship),
		}
	}
	return NewPaginatedResponse(items, pageNum, limit, page.Total)
}

// SocialProfileResponse is GET /social/users/:username. Counts are absent when hidden.
type SocialProfileResponse struct {
	User         SocialUserResponse   `json:"user"`
	Relationship RelationshipResponse `json:"relationship"`
	Visibility   struct {
		Profile   bool `json:"profile"`
		Friends   bool `json:"friends_list"`
		Followers bool `json:"followers_list"`
	} `json:"visibility"`
	Counts struct {
		Friends   *int64 `json:"friends,omitempty"`
		Followers *int64 `json:"followers,omitempty"`
		Following *int64 `json:"following,omitempty"`
	} `json:"counts"`
}

func NewSocialProfileResponse(p *core.SocialProfile) SocialProfileResponse {
	var out SocialProfileResponse
	out.User = NewSocialUserResponse(p.User)
	out.Relationship = NewRelationshipResponse(p.Relationship)
	out.Visibility.Profile = p.ProfileVisible
	out.Visibility.Friends = p.FriendsVisible
	out.Visibility.Followers = p.FollowersVisible
	out.Counts.Friends = p.Friends
	out.Counts.Followers = p.Followers
	out.Counts.Following = p.Following
	return out
}

// SocialSummaryResponse is GET /social/me. User is the card of the signed-in user (avatar,
// display name for the header).
type SocialSummaryResponse struct {
	User             *SocialUserResponse `json:"user,omitempty"`
	IncomingRequests int64               `json:"incoming_requests"`
	OutgoingRequests int64               `json:"outgoing_requests"`
	Friends          int64               `json:"friends"`
	Followers        int64               `json:"followers"`
	Following        int64               `json:"following"`
}

func NewSocialSummaryResponse(s *core.SocialSummary) SocialSummaryResponse {
	var user *SocialUserResponse
	if s.User != nil {
		u := NewSocialUserResponse(*s.User)
		user = &u
	}
	return SocialSummaryResponse{
		User:             user,
		IncomingRequests: s.IncomingRequests,
		OutgoingRequests: s.OutgoingRequests,
		Friends:          s.Friends,
		Followers:        s.Followers,
		Following:        s.Following,
	}
}

// RelationshipChangeResponse answers the social actions: the new relation.
type RelationshipChangeResponse struct {
	UserID       uint                 `json:"user_id"`
	Relationship RelationshipResponse `json:"relationship"`
}

// FriendRequestSentResponse answers POST /social/friend-requests: status is "sent" or
// "accepted" (the other user had already sent a request).
type FriendRequestSentResponse struct {
	Status string `json:"status"`
	RelationshipChangeResponse
}

// PrivacyItemResponse is a privacy item of the catalog with the user's audience.
type PrivacyItemResponse struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Default     string `json:"default"`
	Audience    string `json:"audience"`
	IsDefault   bool   `json:"is_default"`
}

// PrivacyOptionResponse is an audience or a friend request policy.
type PrivacyOptionResponse struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// PrivacySettingsResponse is GET/PUT /social/privacy: the catalog with the user's choices and
// the options for the UI.
type PrivacySettingsResponse struct {
	Items                 []PrivacyItemResponse   `json:"items"`
	Audiences             []PrivacyOptionResponse `json:"audiences"`
	FriendRequestPolicy   string                  `json:"friend_request_policy"`
	FriendRequestPolicies []PrivacyOptionResponse `json:"friend_request_policies"`
}

func NewPrivacySettingsResponse(s *core.PrivacySettings) PrivacySettingsResponse {
	out := PrivacySettingsResponse{
		Items:                 make([]PrivacyItemResponse, len(s.Items)),
		Audiences:             make([]PrivacyOptionResponse, len(core.Audiences)),
		FriendRequestPolicy:   s.FriendRequestPolicy,
		FriendRequestPolicies: make([]PrivacyOptionResponse, len(core.FriendRequestPolicies)),
	}
	for i, it := range s.Items {
		out.Items[i] = PrivacyItemResponse{
			Key:         it.Item.Key,
			Label:       it.Item.Label,
			Description: it.Item.Description,
			Group:       it.Item.Group,
			Default:     string(it.Item.Default),
			Audience:    string(it.Audience),
			IsDefault:   it.IsDefault,
		}
	}
	for i, a := range core.Audiences {
		out.Audiences[i] = PrivacyOptionResponse{Value: string(a.Value), Label: a.Label, Description: a.Description}
	}
	for i, p := range core.FriendRequestPolicies {
		out.FriendRequestPolicies[i] = PrivacyOptionResponse{Value: p.Value, Label: p.Label, Description: p.Description}
	}
	return out
}
