package apperrors

import (
	"net/http"
)

// Saved search error types and sentinel errors.

// ErrSavedSearchNotFound creates a NotFoundError for a saved search.
func ErrSavedSearchNotFound(id any) *NotFoundError {
	return NewNotFoundError("saved_search", id)
}

// ErrSavedSearchNameRequired is returned when saved search name is empty.
var ErrSavedSearchNameRequired = &ValidationError{
	baseError: baseError{
		message:    "saved search name is required",
		code:       "SAVED_SEARCH_NAME_REQUIRED",
		httpStatus: http.StatusBadRequest,
	},
	Field: "name",
}

// ErrSavedSearchNameTooLong is returned when saved search name exceeds max length.
var ErrSavedSearchNameTooLong = &ValidationError{
	baseError: baseError{
		message:    "saved search name must not exceed 255 characters",
		code:       "SAVED_SEARCH_NAME_TOO_LONG",
		httpStatus: http.StatusBadRequest,
	},
	Field: "name",
}

// ErrSavedSearchForbidden is returned when user tries to access another user's saved search.
var ErrSavedSearchForbidden = &ForbiddenError{
	baseError: baseError{
		message:    "you do not have permission to access this saved search",
		code:       "SAVED_SEARCH_FORBIDDEN",
		httpStatus: http.StatusForbidden,
	},
}
