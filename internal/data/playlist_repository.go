package data

type PlaylistRepository interface {
	// CRUD
	Create(playlist *Playlist) error
	GetByUUID(uuid string) (*Playlist, error)
	GetByID(id uint) (*Playlist, error)
	Update(playlist *Playlist) error
	Delete(id uint) error

	// Listing
	List(params PlaylistListParams) ([]Playlist, int64, error)

	// Scenes
	AddScenes(playlistID uint, sceneIDs []uint) error
	RemoveScene(playlistID uint, sceneID uint) error
	RemoveScenes(playlistID uint, sceneIDs []uint) error
	ReorderScenes(playlistID uint, sceneIDs []uint) error
	GetPlaylistScenes(playlistID uint) ([]PlaylistScene, error)
	GetMaxPosition(playlistID uint) (int, error)

	// Tags
	GetPlaylistTags(playlistID uint) ([]Tag, error)
	SetPlaylistTags(playlistID uint, tagIDs []uint) error

	// Likes
	ToggleLike(userID uint, playlistID uint) (bool, error)
	GetLikeStatus(userID uint, playlistID uint) (bool, error)
	GetLikeCount(playlistID uint) (int64, error)

	// Progress
	GetProgress(userID uint, playlistID uint) (*PlaylistProgress, error)
	// UpsertProgress records the last item played: a scene, or a marker (markerID, in a
	// marker playlist) with its scene.
	UpsertProgress(userID uint, playlistID uint, sceneID uint, markerID *uint, positionS float64) error

	// Stats
	GetSceneCount(playlistID uint) (int64, error)
	GetTotalDuration(playlistID uint) (int64, error)
	GetThumbnailScenes(playlistID uint, limit int) ([]Scene, error)

	// Markers (marker playlists, see playlist_marker_repository.go)
	// AddMarkers appends the markers not already in the playlist, in the given order, and
	// returns how many were added. It fails with an error for which IsPlaylistFull is true
	// when the playlist would hold more than maxItems markers (0 = no limit); nothing is
	// added then.
	AddMarkers(playlistID uint, markerIDs []uint, maxItems int) (int, error)
	// RemoveMarkers removes markers from the playlist and returns how many were removed.
	RemoveMarkers(playlistID uint, markerIDs []uint) (int64, error)
	// ReorderMarkers gives the listed markers the positions 0..n-1, in the given order.
	ReorderMarkers(playlistID uint, markerIDs []uint) error
	// GetPlaylistMarkers returns the playable markers (scene neither trashed nor deleted), by
	// position.
	GetPlaylistMarkers(playlistID uint) ([]PlaylistMarkerRow, error)
	// HasMarker reports whether the marker is in the playlist and returns its scene.
	HasMarker(playlistID uint, markerID uint) (bool, uint, error)
	// GetMarkerStats counts the playable markers, their scenes, and sums their play length
	// (defaultDuration seconds for a marker without an end, see docs/MARKER_PLAYLISTS.md).
	GetMarkerStats(playlistID uint, defaultDuration int) (PlaylistMarkerStats, error)
	// GetThumbnailMarkers returns the first playable markers having a thumbnail, by position.
	GetThumbnailMarkers(playlistID uint, limit int) ([]MarkerThumbnailRef, error)
	// FilterVisibleMarkerIDs keeps, in order and without duplicates, the markers the user sees
	// (VisibleMarkersSQL) whose scene is neither trashed nor deleted.
	FilterVisibleMarkerIDs(userID uint, markerIDs []uint) ([]uint, error)
	// ResolveVisibleMarkerIDs returns the markers the user sees matching a marker listing
	// (label = "" for every label; opts as GET /markers/all) whose scene is neither trashed
	// nor deleted, in the listing order, at most limit of them.
	ResolveVisibleMarkerIDs(userID uint, label string, opts MarkerListOptions, limit int) ([]uint, error)
}
