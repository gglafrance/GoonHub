package apperrors

import "net/http"

// ErrDuplicateGroupNotFound creates a NotFoundError for a duplicate group.
func ErrDuplicateGroupNotFound(id uint) *NotFoundError {
	return NewNotFoundError("duplicate_group", id)
}

// ErrRescanAlreadyRunning is returned when a rescan is already in progress.
var ErrRescanAlreadyRunning = &ConflictError{
	baseError: baseError{
		message:    "a library rescan is already running",
		code:       "RESCAN_ALREADY_RUNNING",
		httpStatus: http.StatusConflict,
	},
	Resource: "rescan",
}
