package apperrors

import (
	"fmt"
	"net/http"
)

// Video-specific error types and sentinel errors.

// ErrInvalidFileExtension is returned when a file has an unsupported extension.
var ErrInvalidFileExtension = &ValidationError{
	baseError: baseError{
		message:    "invalid file extension",
		code:       "INVALID_FILE_EXTENSION",
		httpStatus: http.StatusBadRequest,
	},
	Field: "file",
}

// ErrInvalidImageExtension is returned when an image file has an unsupported extension.
var ErrInvalidImageExtension = &ValidationError{
	baseError: baseError{
		message:    "invalid image extension, allowed: .jpg, .jpeg, .png, .webp",
		code:       "INVALID_IMAGE_EXTENSION",
		httpStatus: http.StatusBadRequest,
	},
	Field: "thumbnail",
}

// ErrVideoDimensionsNotAvailable is returned when video dimensions are needed but not extracted yet.
var ErrVideoDimensionsNotAvailable = &ValidationError{
	baseError: baseError{
		message:    "video dimensions not available, metadata must be extracted first",
		code:       "VIDEO_DIMENSIONS_NOT_AVAILABLE",
		httpStatus: http.StatusBadRequest,
	},
}

// ErrVideoNotFound creates a NotFoundError for a video.
func ErrVideoNotFound(id uint) *NotFoundError {
	return NewNotFoundError("video", id)
}

// ErrTagNotFound creates a NotFoundError for a tag.
func ErrTagNotFound(id uint) *NotFoundError {
	return NewNotFoundError("tag", id)
}

// ErrTagNotFoundByName creates a NotFoundError for a tag by name.
func ErrTagNotFoundByName(name string) *NotFoundError {
	return NewNotFoundError("tag", name)
}

// ErrActorNotFound creates a NotFoundError for an actor.
func ErrActorNotFound(id uint) *NotFoundError {
	return NewNotFoundError("actor", id)
}

// ErrActorNotFoundByName creates a NotFoundError for an actor by name.
func ErrActorNotFoundByName(name string) *NotFoundError {
	return NewNotFoundError("actor", name)
}

// ErrTagAlreadyExists is returned when trying to create a duplicate tag.
func ErrTagAlreadyExists(name string) *ConflictError {
	return NewConflictError("tag", fmt.Sprintf("tag '%s' already exists", name))
}

// ErrActorAlreadyExists is returned when trying to create a duplicate actor.
func ErrActorAlreadyExists(name string) *ConflictError {
	return NewConflictError("actor", fmt.Sprintf("actor '%s' already exists", name))
}

// ErrStudioNotFound creates a NotFoundError for a studio.
func ErrStudioNotFound(id uint) *NotFoundError {
	return NewNotFoundError("studio", id)
}

// ErrStudioNotFoundByName creates a NotFoundError for a studio by name.
func ErrStudioNotFoundByName(name string) *NotFoundError {
	return NewNotFoundError("studio", name)
}

// ErrStudioAlreadyExists is returned when trying to create a duplicate studio.
func ErrStudioAlreadyExists(name string) *ConflictError {
	return NewConflictError("studio", fmt.Sprintf("studio '%s' already exists", name))
}

// ErrVideoProcessingFailed creates an internal error for processing failures.
func ErrVideoProcessingFailed(videoID uint, cause error) *InternalError {
	return NewInternalError(fmt.Sprintf("failed to process video %d", videoID), cause)
}

// ErrVideoFileNotFound is returned when the video file doesn't exist on disk.
func ErrVideoFileNotFound(path string) *NotFoundError {
	return &NotFoundError{
		baseError: baseError{
			message:    "video file not found",
			code:       "VIDEO_FILE_NOT_FOUND",
			httpStatus: http.StatusNotFound,
		},
		Resource: "video_file",
		ID:       path,
	}
}
