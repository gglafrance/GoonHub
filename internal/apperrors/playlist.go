package apperrors

import (
	"net/http"
)

// Playlist error types and sentinel errors.

// ErrPlaylistNotFound creates a NotFoundError for a playlist.
func ErrPlaylistNotFound(id any) *NotFoundError {
	return NewNotFoundError("playlist", id)
}

// ErrPlaylistNameRequired is returned when playlist name is empty.
var ErrPlaylistNameRequired = &ValidationError{
	baseError: baseError{
		message:    "playlist name is required",
		code:       "PLAYLIST_NAME_REQUIRED",
		httpStatus: http.StatusBadRequest,
	},
	Field: "name",
}

// ErrPlaylistNameTooLong is returned when playlist name exceeds max length.
var ErrPlaylistNameTooLong = &ValidationError{
	baseError: baseError{
		message:    "playlist name must not exceed 255 characters",
		code:       "PLAYLIST_NAME_TOO_LONG",
		httpStatus: http.StatusBadRequest,
	},
	Field: "name",
}

// ErrPlaylistForbidden is returned when user tries to access another user's private playlist.
var ErrPlaylistForbidden = &ForbiddenError{
	baseError: baseError{
		message:    "you do not have permission to access this playlist",
		code:       "PLAYLIST_FORBIDDEN",
		httpStatus: http.StatusForbidden,
	},
}

// ErrPlaylistInvalidVisibility is returned when visibility value is invalid.
var ErrPlaylistInvalidVisibility = &ValidationError{
	baseError: baseError{
		message:    "visibility must be 'private' or 'public'",
		code:       "PLAYLIST_INVALID_VISIBILITY",
		httpStatus: http.StatusBadRequest,
	},
	Field: "visibility",
}

// ErrPlaylistSceneAlreadyAdded is returned when a scene is already in the playlist.
var ErrPlaylistSceneAlreadyAdded = &ConflictError{
	baseError: baseError{
		message:    "scene is already in this playlist",
		code:       "PLAYLIST_SCENE_ALREADY_ADDED",
		httpStatus: http.StatusConflict,
	},
}

// ErrPlaylistSceneNotInPlaylist is returned when trying to remove a scene not in the playlist.
var ErrPlaylistSceneNotInPlaylist = &ValidationError{
	baseError: baseError{
		message:    "scene is not in this playlist",
		code:       "PLAYLIST_SCENE_NOT_IN_PLAYLIST",
		httpStatus: http.StatusBadRequest,
	},
	Field: "scene_id",
}
