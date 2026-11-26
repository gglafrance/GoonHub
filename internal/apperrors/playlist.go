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

// ErrPlaylistInvalidKind is returned when the playlist kind is not "scenes" or "markers".
var ErrPlaylistInvalidKind = &ValidationError{
	baseError: baseError{
		message:    "kind must be 'scenes' or 'markers'",
		code:       "PLAYLIST_INVALID_KIND",
		httpStatus: http.StatusBadRequest,
	},
	Field: "kind",
}

// ErrPlaylistKindMismatch is returned when scenes are sent to a marker playlist, or markers to
// a scene playlist.
var ErrPlaylistKindMismatch = &ValidationError{
	baseError: baseError{
		message:    "this playlist does not hold this kind of item",
		code:       "PLAYLIST_KIND_MISMATCH",
		httpStatus: http.StatusBadRequest,
	},
	Field: "kind",
}

// ErrPlaylistFull is returned when adding markers would exceed the playlist size limit.
var ErrPlaylistFull = &ValidationError{
	baseError: baseError{
		message:    "the playlist would exceed its maximum number of markers",
		code:       "PLAYLIST_FULL",
		httpStatus: http.StatusBadRequest,
	},
	Field: "marker_ids",
}

// ErrPlaylistMarkersRequired is returned when no marker is given to add, remove or reorder.
var ErrPlaylistMarkersRequired = &ValidationError{
	baseError: baseError{
		message:    "marker_ids must not be empty",
		code:       "PLAYLIST_MARKERS_REQUIRED",
		httpStatus: http.StatusBadRequest,
	},
	Field: "marker_ids",
}

// ErrPlaylistMarkerNotFound is returned when a marker to add does not exist or is not visible
// to the user (both answer the same, so as not to reveal private markers).
var ErrPlaylistMarkerNotFound = &NotFoundError{
	baseError: baseError{
		message:    "marker not found",
		code:       "PLAYLIST_MARKER_NOT_FOUND",
		httpStatus: http.StatusNotFound,
	},
}

// ErrPlaylistMarkerNotInPlaylist is returned when a marker to remove is not in the playlist.
var ErrPlaylistMarkerNotInPlaylist = &ValidationError{
	baseError: baseError{
		message:    "marker is not in this playlist",
		code:       "PLAYLIST_MARKER_NOT_IN_PLAYLIST",
		httpStatus: http.StatusBadRequest,
	},
	Field: "marker_id",
}
