package apperrors

import (
	"fmt"
	"net/http"
	"time"
)

// Auth-specific error types.

// ErrInvalidCredentials is returned when login credentials are incorrect.
var ErrInvalidCredentials = &UnauthorizedError{
	baseError: baseError{
		message:    "invalid credentials",
		code:       "INVALID_CREDENTIALS",
		httpStatus: http.StatusUnauthorized,
	},
}

// ErrTokenExpired is returned when a token has expired.
var ErrTokenExpired = &UnauthorizedError{
	baseError: baseError{
		message:    "token expired",
		code:       "TOKEN_EXPIRED",
		httpStatus: http.StatusUnauthorized,
	},
}

// ErrTokenInvalid is returned when a token is malformed or invalid.
var ErrTokenInvalid = &UnauthorizedError{
	baseError: baseError{
		message:    "invalid token",
		code:       "TOKEN_INVALID",
		httpStatus: http.StatusUnauthorized,
	},
}

// ErrTokenRevoked is returned when a token has been revoked.
var ErrTokenRevoked = &UnauthorizedError{
	baseError: baseError{
		message:    "token has been revoked",
		code:       "TOKEN_REVOKED",
		httpStatus: http.StatusUnauthorized,
	},
}

// AccountLockedError represents an account lockout due to too many failed attempts.
type AccountLockedError struct {
	baseError
	UnlockAt time.Time
}

// NewAccountLockedError creates an AccountLockedError.
func NewAccountLockedError(unlockAt time.Time) *AccountLockedError {
	remaining := time.Until(unlockAt).Round(time.Second)
	return &AccountLockedError{
		baseError: baseError{
			message:    fmt.Sprintf("account locked, try again in %v", remaining),
			code:       "ACCOUNT_LOCKED",
			httpStatus: http.StatusTooManyRequests,
		},
		UnlockAt: unlockAt,
	}
}

// IsAccountLocked checks if an error is an AccountLockedError.
func IsAccountLocked(err error) bool {
	var locked *AccountLockedError
	return As(err, &locked)
}

// ErrUserNotFound creates a NotFoundError for a user.
func ErrUserNotFound(id uint) *NotFoundError {
	return NewNotFoundError("user", id)
}

// ErrUserNotFoundByUsername creates a NotFoundError for a user by username.
func ErrUserNotFoundByUsername(username string) *NotFoundError {
	return NewNotFoundError("user", username)
}

// ErrUsernameAlreadyExists is returned when trying to create a user with an existing username.
func ErrUsernameAlreadyExists(username string) *ConflictError {
	return NewConflictError("user", fmt.Sprintf("username '%s' already exists", username))
}

// ErrRoleNotFound creates a NotFoundError for a role.
func ErrRoleNotFound(role string) *NotFoundError {
	return NewNotFoundError("role", role)
}

// ErrPermissionDenied is returned when a user lacks required permissions.
func ErrPermissionDenied(action string) *ForbiddenError {
	return NewForbiddenError(fmt.Sprintf("permission denied: %s", action))
}

// As is a convenience wrapper around errors.As for use in this package.
func As(err error, target any) bool {
	return asError(err, target)
}

// asError performs the type assertion. This avoids import cycle with errors package.
func asError(err error, target any) bool {
	if err == nil {
		return false
	}
	// Use type switch for common cases
	switch t := target.(type) {
	case **AccountLockedError:
		if e, ok := err.(*AccountLockedError); ok {
			*t = e
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			return asError(u.Unwrap(), target)
		}
	}
	return false
}
