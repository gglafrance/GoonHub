package core

import (
	"time"

	"go.uber.org/zap"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

// Marker playlists (docs/MARKER_PLAYLISTS.md). A playlist of kind "markers" holds markers
// instead of scenes; the rest (name, visibility, tags, likes, progress) is shared with the scene
// playlists. Access rules:
//   - whoever can view the playlist (owner, anyone when public) sees all its markers, even those
//     of users who do not share theirs: sharing a playlist shares the moments it lists;
//   - only the owner changes it, and adds only markers they see themselves (their own and the
//     shared ones, data.VisibleMarkersSQL), of scenes neither trashed nor deleted;
//   - a marker deleted by its owner leaves the playlists (ON DELETE CASCADE).

// PlaylistMarkerInfo is the marker of a marker playlist entry.
type PlaylistMarkerInfo struct {
	ID           uint   `json:"id"`
	UserID       uint   `json:"user_id"`
	SceneID      uint   `json:"scene_id"`
	Timestamp    int    `json:"timestamp"`
	EndTimestamp *int   `json:"end_timestamp"`
	Label        string `json:"label"`
	Color        string `json:"color"`
	HasThumbnail bool   `json:"has_thumbnail"`
	// ThumbnailVersion versions the thumbnail URL (data.MarkerThumbnailVersion)
	ThumbnailVersion int64 `json:"thumbnail_version"`
}

// PlaylistMarkerScene is the scene of a marker playlist entry (no file information).
type PlaylistMarkerScene struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
}

// PlaylistMarkerEntry is a marker within a marker playlist, with the range it plays
// (data.MarkerPlayRange): Start to End seconds of its scene.
type PlaylistMarkerEntry struct {
	Position int                 `json:"position"`
	AddedAt  time.Time           `json:"added_at"`
	Marker   PlaylistMarkerInfo  `json:"marker"`
	Scene    PlaylistMarkerScene `json:"scene"`
	Start    int                 `json:"start"`
	End      int                 `json:"end"`
}

// PlaylistMarkerFilter selects markers like the marker pages do: one label (a label page), or
// every label with the listing options of GET /markers/all.
type PlaylistMarkerFilter struct {
	Label   string
	Options data.MarkerListOptions
}

// AddPlaylistMarkersResult tells how many markers were added, and how many were skipped
// because they already were in the playlist.
type AddPlaylistMarkersResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
}

// visibleMarkersToAdd dedupes markerIDs and checks that the user sees each of them (and that
// its scene is playable). An unknown or hidden marker fails the whole request with the same
// not found error.
func (s *PlaylistService) visibleMarkersToAdd(userID uint, markerIDs []uint) ([]uint, error) {
	ids := uniqueMarkerIDs(markerIDs)
	if len(ids) == 0 {
		return nil, apperrors.ErrPlaylistMarkersRequired
	}
	visible, err := s.repo.FilterVisibleMarkerIDs(userID, ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check markers", err)
	}
	if len(visible) != len(ids) {
		return nil, apperrors.ErrPlaylistMarkerNotFound
	}
	return visible, nil
}

func uniqueMarkerIDs(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// playlistMarkerEntries returns the playable markers of a playlist with their play range.
func (s *PlaylistService) playlistMarkerEntries(playlistID uint) ([]PlaylistMarkerEntry, error) {
	rows, err := s.repo.GetPlaylistMarkers(playlistID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get playlist markers", err)
	}
	entries := make([]PlaylistMarkerEntry, len(rows))
	for i, row := range rows {
		start, end := data.MarkerPlayRange(row.Timestamp, row.EndTimestamp, row.SceneDuration, s.markerDefaultDuration)
		entries[i] = PlaylistMarkerEntry{
			Position: row.Position,
			AddedAt:  row.AddedAt,
			Marker: PlaylistMarkerInfo{
				ID:               row.MarkerID,
				UserID:           row.MarkerUserID,
				SceneID:          row.SceneID,
				Timestamp:        row.Timestamp,
				EndTimestamp:     row.EndTimestamp,
				Label:            row.Label,
				Color:            row.Color,
				HasThumbnail:     row.ThumbnailPath != "",
				ThumbnailVersion: data.MarkerThumbnailVersion(row.UpdatedAt),
			},
			Scene: PlaylistMarkerScene{
				ID:       row.SceneID,
				Title:    row.SceneTitle,
				Duration: row.SceneDuration,
			},
			Start: start,
			End:   end,
		}
	}
	return entries, nil
}

// addVisibleMarkers adds already checked markers and maps the repository errors.
func (s *PlaylistService) addVisibleMarkers(playlist *data.Playlist, ids []uint) (*AddPlaylistMarkersResult, error) {
	added, err := s.repo.AddMarkers(playlist.ID, ids, s.maxMarkers)
	if err != nil {
		if data.IsPlaylistFull(err) {
			return nil, apperrors.ErrPlaylistFull
		}
		return nil, apperrors.NewInternalError("failed to add markers to playlist", err)
	}
	return &AddPlaylistMarkersResult{Added: added, Skipped: len(ids) - added}, nil
}

// AddMarkers adds markers to a marker playlist of the user. Markers already in it are skipped.
func (s *PlaylistService) AddMarkers(userID uint, uuid string, markerIDs []uint) (*AddPlaylistMarkersResult, error) {
	playlist, err := s.ownedPlaylistOfKind(userID, uuid, data.PlaylistKindMarkers)
	if err != nil {
		return nil, err
	}
	ids, err := s.visibleMarkersToAdd(userID, markerIDs)
	if err != nil {
		return nil, err
	}
	result, err := s.addVisibleMarkers(playlist, ids)
	if err != nil {
		return nil, err
	}
	s.logger.Info("Markers added to playlist",
		zap.Uint("user_id", userID),
		zap.String("uuid", uuid),
		zap.Int("added", result.Added),
	)
	return result, nil
}

// AddMarkersFromFilter adds every marker of a marker listing (a label, or the filtered list of
// all markers) that the user sees, in the listing order. A listing larger than the playlist
// limit is refused as a whole.
func (s *PlaylistService) AddMarkersFromFilter(userID uint, uuid string, filter PlaylistMarkerFilter) (*AddPlaylistMarkersResult, error) {
	playlist, err := s.ownedPlaylistOfKind(userID, uuid, data.PlaylistKindMarkers)
	if err != nil {
		return nil, err
	}
	limit := maxPlaylistMarkerResolve
	if s.maxMarkers > 0 {
		// One more than the limit tells a listing that cannot fit
		limit = s.maxMarkers + 1
	}
	ids, err := s.repo.ResolveVisibleMarkerIDs(userID, filter.Label, filter.Options, limit)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list markers", err)
	}
	if len(ids) == 0 {
		return &AddPlaylistMarkersResult{}, nil
	}
	if s.maxMarkers > 0 && len(ids) > s.maxMarkers {
		return nil, apperrors.ErrPlaylistFull
	}
	result, err := s.addVisibleMarkers(playlist, ids)
	if err != nil {
		return nil, err
	}
	s.logger.Info("Markers added to playlist from a listing",
		zap.Uint("user_id", userID),
		zap.String("uuid", uuid),
		zap.Int("added", result.Added),
	)
	return result, nil
}

// RemoveMarker removes one marker from a marker playlist of the user.
func (s *PlaylistService) RemoveMarker(userID uint, uuid string, markerID uint) error {
	return s.RemoveMarkers(userID, uuid, []uint{markerID})
}

// RemoveMarkers removes markers from a marker playlist of the user.
func (s *PlaylistService) RemoveMarkers(userID uint, uuid string, markerIDs []uint) error {
	playlist, err := s.ownedPlaylistOfKind(userID, uuid, data.PlaylistKindMarkers)
	if err != nil {
		return err
	}
	ids := uniqueMarkerIDs(markerIDs)
	if len(ids) == 0 {
		return apperrors.ErrPlaylistMarkersRequired
	}
	removed, err := s.repo.RemoveMarkers(playlist.ID, ids)
	if err != nil {
		return apperrors.NewInternalError("failed to remove markers from playlist", err)
	}
	if removed == 0 {
		return apperrors.ErrPlaylistMarkerNotInPlaylist
	}
	return nil
}

// ReorderMarkers sets the order of the markers of a marker playlist of the user.
func (s *PlaylistService) ReorderMarkers(userID uint, uuid string, markerIDs []uint) error {
	playlist, err := s.ownedPlaylistOfKind(userID, uuid, data.PlaylistKindMarkers)
	if err != nil {
		return err
	}
	ids := uniqueMarkerIDs(markerIDs)
	if len(ids) == 0 {
		return apperrors.ErrPlaylistMarkersRequired
	}
	if err := s.repo.ReorderMarkers(playlist.ID, ids); err != nil {
		return apperrors.NewInternalError("failed to reorder markers", err)
	}
	return nil
}
