package request

// SendFriendRequestRequest is the body of POST /social/friend-requests.
type SendFriendRequestRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

// UpdatePrivacyRequest is the body of PUT /social/privacy. Items maps a privacy item key to an
// audience (public, followers, mutuals, friends, private) or "default"; absent items are
// unchanged. FriendRequestPolicy (everyone, mutuals, nobody) is unchanged when absent.
type UpdatePrivacyRequest struct {
	Items               map[string]string `json:"items"`
	FriendRequestPolicy *string           `json:"friend_request_policy"`
}
