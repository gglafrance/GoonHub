package apperrors

import (
	"fmt"
	"net/http"
)

// Scene-specific error types and sentinel errors.

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

// ErrSceneDimensionsNotAvailable is returned when scene dimensions are needed but not extracted yet.
var ErrSceneDimensionsNotAvailable = &ValidationError{
	baseError: baseError{
		message:    "scene dimensions not available, metadata must be extracted first",
		code:       "SCENE_DIMENSIONS_NOT_AVAILABLE",
		httpStatus: http.StatusBadRequest,
	},
}

// ErrSceneNotFound creates a NotFoundError for a scene.
func ErrSceneNotFound(id uint) *NotFoundError {
	return NewNotFoundError("scene", id)
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

// ErrSceneProcessingFailed creates an internal error for processing failures.
func ErrSceneProcessingFailed(sceneID uint, cause error) *InternalError {
	return NewInternalError(fmt.Sprintf("failed to process scene %d", sceneID), cause)
}

// ErrSceneFileNotFound is returned when the scene file doesn't exist on disk.
func ErrSceneFileNotFound(path string) *NotFoundError {
	return &NotFoundError{
		baseError: baseError{
			message:    "scene file not found",
			code:       "SCENE_FILE_NOT_FOUND",
			httpStatus: http.StatusNotFound,
		},
		Resource: "scene_file",
		ID:       path,
	}
}
