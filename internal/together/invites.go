package together

import (
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/google/uuid"
)

// Lobby invites sent as messages (docs/MESSAGES.md, docs/GOON2GETHER.md "Service hooks"):
// the Hub implements core.LobbyInvites.

var _ core.LobbyInvites = (*Hub)(nil)

// CanInvite returns nil when the user is a member of the lobby (only members invite).
func (h *Hub) CanInvite(id uuid.UUID, inviterID uint) error {
	return h.withLobby(id, func(l *lobby) error {
		if l.members[inviterID] == nil {
			return apperrors.NewForbiddenError("only members of the lobby can invite to it")
		}
		return nil
	})
}

// InviteCard describes a lobby to the user invited by inviterID: nil when the lobby no longer
// exists. Joinable: the lobby is public, or the inviter is still a member; never when the
// viewer is banned. The title of the item playing needs scenes:view.
func (h *Hub) InviteCard(id uuid.UUID, inviterID uint, viewer *core.UserAccess) (*core.LobbyInviteCard, error) {
	var card *core.LobbyInviteCard
	err := h.withLobby(id, func(l *lobby) error {
		var viewerID uint
		if viewer != nil {
			viewerID = viewer.UserID
		}
		c := &core.LobbyInviteCard{
			ID:          l.row.UUID.String(),
			Name:        l.row.Name,
			Visibility:  l.row.Visibility,
			OnlineCount: len(l.onlineUsers()),
			IsMember:    l.members[viewerID] != nil,
		}
		if l.row.HostUserID != nil {
			if m := l.members[*l.row.HostUserID]; m != nil && m.anon == 0 {
				c.HostUsername = m.username
			}
		}
		if cur := l.current(); cur != nil {
			c.NowPlaying = &core.LobbyInviteScene{SceneID: cur.sceneID}
			if viewer.Has(core.PermScenesView) {
				c.NowPlaying.Title = cur.title
			}
		}
		c.Joinable = !l.banned[viewerID] && (l.row.Visibility == data.LobbyVisibilityPublic || l.members[inviterID] != nil)
		card = c
		return nil
	})
	if apperrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return card, nil
}

// AcceptInvite makes the user a member of the lobby: already a member, a public lobby, or a
// private one whose inviter is still a member (as if they had opened the invite link; the
// lobby's bans apply).
func (h *Hub) AcceptInvite(id uuid.UUID, inviterID, userID uint, username string) error {
	return h.withLobby(id, func(l *lobby) error {
		if l.members[userID] != nil {
			return nil
		}
		if l.row.Visibility != data.LobbyVisibilityPublic && (inviterID == 0 || l.members[inviterID] == nil) {
			return apperrors.ErrShareNotAvailable("this invite no longer works: the member who sent it left the lobby")
		}
		return l.join(userID, username, false)
	})
}
