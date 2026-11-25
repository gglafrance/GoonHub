package apperrors

import (
	"errors"
	"net/http"
)

// Social error codes (docs/SOCIAL.md).
const (
	CodeSelfRelation          = "SELF_RELATION"
	CodeAlreadyFriends        = "ALREADY_FRIENDS"
	CodeFriendRequestPending  = "FRIEND_REQUEST_PENDING"
	CodeFriendRequestsClosed  = "FRIEND_REQUESTS_CLOSED"
	CodeFriendRequestNotFound = "FRIEND_REQUEST_NOT_FOUND"
	CodeTooManyFriendRequests = "TOO_MANY_PENDING_REQUESTS"
	CodeSocialRateLimited     = "SOCIAL_RATE_LIMITED"
	CodePrivacyRestricted     = "PRIVACY_RESTRICTED"
)

// SocialError is an error of the social features, with its own code and HTTP status.
type SocialError struct {
	baseError
}

func newSocialError(code string, status int, message string) *SocialError {
	return &SocialError{baseError: baseError{message: message, code: code, httpStatus: status}}
}

// ErrSelfRelation: following or friending oneself.
func ErrSelfRelation() *SocialError {
	return newSocialError(CodeSelfRelation, http.StatusBadRequest, "you cannot do this with yourself")
}

// ErrAlreadyFriends: a friend request to a friend.
func ErrAlreadyFriends() *SocialError {
	return newSocialError(CodeAlreadyFriends, http.StatusConflict, "you are already friends")
}

// ErrFriendRequestPending: a second request to the same user.
func ErrFriendRequestPending() *SocialError {
	return newSocialError(CodeFriendRequestPending, http.StatusConflict, "a friend request is already pending")
}

// ErrFriendRequestsClosed: the receiver's policy refuses the request.
func ErrFriendRequestsClosed() *SocialError {
	return newSocialError(CodeFriendRequestsClosed, http.StatusForbidden, "this user does not accept friend requests from you")
}

// ErrFriendRequestNotFound: no such pending request.
func ErrFriendRequestNotFound() *SocialError {
	return newSocialError(CodeFriendRequestNotFound, http.StatusNotFound, "friend request not found")
}

// ErrTooManyPendingRequests: the cap of pending outgoing requests is reached.
func ErrTooManyPendingRequests() *SocialError {
	return newSocialError(CodeTooManyFriendRequests, http.StatusTooManyRequests, "too many pending friend requests, wait for some answers or cancel some")
}

// ErrSocialRateLimited: too many social actions in a short time.
func ErrSocialRateLimited() *SocialError {
	return newSocialError(CodeSocialRateLimited, http.StatusTooManyRequests, "too many requests, try again later")
}

// ErrPrivacyRestricted: the owner's privacy settings hide the information from the viewer.
func ErrPrivacyRestricted() *SocialError {
	return newSocialError(CodePrivacyRestricted, http.StatusForbidden, "this information is private")
}

// IsPrivacyRestricted checks if an error is ErrPrivacyRestricted.
func IsPrivacyRestricted(err error) bool {
	var s *SocialError
	return errors.As(err, &s) && s.code == CodePrivacyRestricted
}
