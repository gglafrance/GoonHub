package apperrors

import "net/http"

// ErrShareLinkNotFound creates a NotFoundError for a share link.
func ErrShareLinkNotFound(token string) *NotFoundError {
	return NewNotFoundError("share_link", token)
}

// ErrShareLinkExpired is returned when a share link has expired.
var ErrShareLinkExpired = &ValidationError{
	baseError: baseError{
		message:    "share link has expired",
		code:       "SHARE_LINK_EXPIRED",
		httpStatus: http.StatusGone,
	},
}

// ErrShareLinkAuthRequired is returned when accessing an auth-required share link without authentication.
var ErrShareLinkAuthRequired = &UnauthorizedError{
	baseError: baseError{
		message:    "authentication required to access this shared scene",
		code:       "AUTH_REQUIRED",
		httpStatus: http.StatusUnauthorized,
	},
}
