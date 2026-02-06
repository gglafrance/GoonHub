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
	UpsertProgress(userID uint, playlistID uint, sceneID uint, positionS float64) error

	// Stats
	GetSceneCount(playlistID uint) (int64, error)
	GetTotalDuration(playlistID uint) (int64, error)
	GetThumbnailScenes(playlistID uint, limit int) ([]Scene, error)
}
