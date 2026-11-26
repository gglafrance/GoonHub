package core

import (
	"context"
	"fmt"

	"goonhub/internal/data"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Share cards (docs/MESSAGES.md, "Shares"): a message stores references (ids), resolved for
// the viewer every time it is read, with the viewer's permissions. A card never carries file
// information (size, path, codecs): only what showing and opening it needs.

func sceneThumbnailURL(id uint, path string, version int64) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("/thumbnails/%d?v=%d", id, version)
}

func markerThumbnailURL(id uint, path string, version int64) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("/marker-thumbnails/%d?v=%d", id, version)
}

func unavailable(kind, reason string) *AttachmentView {
	return &AttachmentView{Kind: kind, Reason: reason}
}

// lobbyRef is a lobby invite to resolve: the lobby and who sent it.
type lobbyRef struct {
	id      uuid.UUID
	inviter uint
}

// resolveAttachments resolves the attachments of the messages for the viewer (map by message
// id; messages without attachment, and deleted ones, are absent). Batched: one query per kind
// of item, one hub call per distinct lobby invite.
func (s *MessageService) resolveAttachments(ctx context.Context, msgs []data.Message, viewer *UserAccess) (map[uint]*AttachmentView, error) {
	out := make(map[uint]*AttachmentView)
	var sceneIDs, markerIDs, playlistIDs []uint
	lobbies := make(map[lobbyRef]*LobbyInviteCard)
	for _, m := range msgs {
		if m.DeletedAt != nil || m.AttachmentKind == nil {
			continue
		}
		switch *m.AttachmentKind {
		case data.AttachmentScene:
			if m.AttachmentSceneID != nil {
				sceneIDs = append(sceneIDs, *m.AttachmentSceneID)
			}
		case data.AttachmentMarker:
			if m.AttachmentMarkerID != nil {
				markerIDs = append(markerIDs, *m.AttachmentMarkerID)
			}
		case data.AttachmentPlaylist:
			if m.AttachmentPlaylistID != nil {
				playlistIDs = append(playlistIDs, *m.AttachmentPlaylistID)
			}
		case data.AttachmentLobby:
			if m.AttachmentLobbyUUID != nil && m.SenderID != nil {
				lobbies[lobbyRef{id: *m.AttachmentLobbyUUID, inviter: *m.SenderID}] = nil
			}
		}
	}

	canScenes := viewer.Has(PermScenesView)
	scenes := make(map[uint]data.ShareSceneRow)
	markers := make(map[uint]data.ShareMarkerRow)
	playlists := make(map[uint]data.SharePlaylistRow)
	if canScenes && len(sceneIDs) > 0 {
		rows, err := s.repo.SceneCards(ctx, sceneIDs)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			scenes[r.ID] = r
		}
	}
	if canScenes && len(markerIDs) > 0 {
		rows, err := s.repo.MarkerCards(ctx, markerIDs)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			markers[r.ID] = r
		}
	}
	if canScenes && len(playlistIDs) > 0 {
		rows, err := s.repo.PlaylistCards(ctx, playlistIDs)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			playlists[r.ID] = r
		}
	}
	if s.lobbies != nil && viewer.Has(PermTogetherUse) {
		for ref := range lobbies {
			card, err := s.lobbies.InviteCard(ref.id, ref.inviter, viewer)
			if err != nil {
				s.logger.Warn("Failed to resolve a lobby invite", zap.String("lobby", ref.id.String()), zap.Error(err))
				continue
			}
			lobbies[ref] = card
		}
	}

	for _, m := range msgs {
		if m.DeletedAt != nil || m.AttachmentKind == nil {
			continue
		}
		kind := *m.AttachmentKind
		switch kind {
		case data.AttachmentScene:
			out[m.ID] = s.sceneView(m, scenes, canScenes)
		case data.AttachmentMarker:
			out[m.ID] = s.markerView(m, markers, canScenes)
		case data.AttachmentPlaylist:
			out[m.ID] = s.playlistView(m, playlists, canScenes, viewer)
		case data.AttachmentLobby:
			out[m.ID] = s.lobbyView(m, lobbies, viewer)
		default:
			out[m.ID] = unavailable(kind, ShareReasonNotFound)
		}
	}
	return out, nil
}

func (s *MessageService) sceneView(m data.Message, rows map[uint]data.ShareSceneRow, canScenes bool) *AttachmentView {
	if !canScenes {
		return unavailable(data.AttachmentScene, ShareReasonNoPermission)
	}
	if m.AttachmentSceneID == nil {
		return unavailable(data.AttachmentScene, ShareReasonNotFound)
	}
	r, ok := rows[*m.AttachmentSceneID]
	switch {
	case !ok || r.DeletedAt != nil:
		return unavailable(data.AttachmentScene, ShareReasonNotFound)
	case r.TrashedAt != nil:
		return unavailable(data.AttachmentScene, ShareReasonTrashed)
	}
	return &AttachmentView{
		Kind:      data.AttachmentScene,
		Available: true,
		Time:      m.AttachmentTimeS,
		Scene: &ShareSceneCard{
			ID:           r.ID,
			Title:        r.Title,
			Duration:     r.Duration,
			ThumbnailURL: sceneThumbnailURL(r.ID, r.ThumbnailPath, r.UpdatedAt.UnixMilli()),
		},
	}
}

func (s *MessageService) markerView(m data.Message, rows map[uint]data.ShareMarkerRow, canScenes bool) *AttachmentView {
	if !canScenes {
		return unavailable(data.AttachmentMarker, ShareReasonNoPermission)
	}
	if m.AttachmentMarkerID == nil {
		return unavailable(data.AttachmentMarker, ShareReasonNotFound)
	}
	r, ok := rows[*m.AttachmentMarkerID]
	switch {
	case !ok || r.SceneDeletedAt != nil:
		return unavailable(data.AttachmentMarker, ShareReasonNotFound)
	case r.SceneTrashedAt != nil:
		return unavailable(data.AttachmentMarker, ShareReasonTrashed)
	}
	return &AttachmentView{
		Kind:      data.AttachmentMarker,
		Available: true,
		Marker: &ShareMarkerCard{
			ID:            r.ID,
			SceneID:       r.SceneID,
			SceneTitle:    r.SceneTitle,
			SceneDuration: r.SceneDuration,
			Start:         r.Timestamp,
			End:           r.EndTimestamp,
			Label:         r.Label,
			Color:         r.Color,
			OwnerUsername: r.OwnerUsername,
			ThumbnailURL:  markerThumbnailURL(r.ID, r.ThumbnailPath, data.MarkerThumbnailVersion(r.UpdatedAt)),
		},
	}
}

func (s *MessageService) playlistView(m data.Message, rows map[uint]data.SharePlaylistRow, canScenes bool, viewer *UserAccess) *AttachmentView {
	if !canScenes {
		return unavailable(data.AttachmentPlaylist, ShareReasonNoPermission)
	}
	if m.AttachmentPlaylistID == nil {
		return unavailable(data.AttachmentPlaylist, ShareReasonNotFound)
	}
	r, ok := rows[*m.AttachmentPlaylistID]
	if !ok {
		return unavailable(data.AttachmentPlaylist, ShareReasonNotFound)
	}
	if r.UserID != viewer.UserID {
		if r.Visibility != "public" {
			return unavailable(data.AttachmentPlaylist, ShareReasonPrivate)
		}
		if !viewer.Has(PermPlaylistsPublic) {
			return unavailable(data.AttachmentPlaylist, ShareReasonNoPermission)
		}
	}
	card := &SharePlaylistCard{
		UUID:          r.UUID.String(),
		Name:          r.Name,
		Kind:          r.Kind,
		Visibility:    r.Visibility,
		OwnerUsername: r.OwnerUsername,
		ItemCount:     r.ItemCount,
	}
	switch {
	case r.ThumbnailSceneID != nil && r.ThumbnailSceneAt != nil:
		card.ThumbnailURL = fmt.Sprintf("/thumbnails/%d?v=%d", *r.ThumbnailSceneID, r.ThumbnailSceneAt.UnixMilli())
	case r.ThumbnailMarkerID != nil && r.ThumbnailMarkerAt != nil:
		card.ThumbnailURL = fmt.Sprintf("/marker-thumbnails/%d?v=%d", *r.ThumbnailMarkerID, data.MarkerThumbnailVersion(*r.ThumbnailMarkerAt))
	}
	return &AttachmentView{Kind: data.AttachmentPlaylist, Available: true, Playlist: card}
}

func (s *MessageService) lobbyView(m data.Message, cards map[lobbyRef]*LobbyInviteCard, viewer *UserAccess) *AttachmentView {
	if s.lobbies == nil || m.AttachmentLobbyUUID == nil || m.SenderID == nil {
		return unavailable(data.AttachmentLobby, ShareReasonNotFound)
	}
	if !viewer.Has(PermTogetherUse) {
		return unavailable(data.AttachmentLobby, ShareReasonNoPermission)
	}
	card := cards[lobbyRef{id: *m.AttachmentLobbyUUID, inviter: *m.SenderID}]
	if card == nil {
		return unavailable(data.AttachmentLobby, ShareReasonNotFound)
	}
	if !card.IsMember && !card.Joinable {
		return unavailable(data.AttachmentLobby, ShareReasonNotJoinable)
	}
	return &AttachmentView{Kind: data.AttachmentLobby, Available: true, Lobby: card}
}
