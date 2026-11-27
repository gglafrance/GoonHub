package together

import (
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/google/uuid"
)

// Lobby invites sent as messages (core.LobbyInvites).
func TestLobbyInvites(t *testing.T) {
	e := newEnv(t)
	id := e.lobby(CreateInput{Name: "Private"}, 1)
	access := func(u uint) *core.UserAccess { a, _ := e.access.ResolveUser(u); return a }

	// Only members invite
	if err := e.hub.CanInvite(id, alice); err != nil {
		t.Fatalf("the host cannot invite: %v", err)
	}
	if err := e.hub.CanInvite(id, bob); !apperrors.IsForbidden(err) {
		t.Fatalf("a non member invited: %v", err)
	}

	// The card shown to bob, invited by alice
	card, err := e.hub.InviteCard(id, alice, access(bob))
	if err != nil || card == nil {
		t.Fatalf("card: %+v %v", card, err)
	}
	if card.Name != "Private" || card.Visibility != data.LobbyVisibilityPrivate || card.IsMember || !card.Joinable ||
		card.NowPlaying == nil || card.NowPlaying.SceneID != 1 || card.NowPlaying.Title != "One" || card.HostUsername != "alice" {
		t.Fatalf("card = %+v", card)
	}
	// no title without scenes:view
	e.access.mu.Lock()
	e.access.perms[carol] = []string{core.PermTogetherUse}
	e.access.mu.Unlock()
	if c, _ := e.hub.InviteCard(id, alice, access(carol)); c == nil || c.NowPlaying == nil || c.NowPlaying.Title != "" {
		t.Fatalf("card without scenes:view = %+v", c)
	}
	// an invite from a non member of a private lobby cannot be used
	if c, _ := e.hub.InviteCard(id, carol, access(bob)); c == nil || c.Joinable {
		t.Fatalf("card of a stale invite = %+v", c)
	}
	if err := e.hub.AcceptInvite(id, carol, bob, "bob"); err == nil {
		t.Fatal("joined a private lobby through a non member")
	}

	// Accepting makes bob a member
	if err := e.hub.AcceptInvite(id, alice, bob, "bob"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if s, err := e.hub.GetLobby(e.actor(bob), id); err != nil || !s.IsMember {
		t.Fatalf("bob after accepting: %+v %v", s, err)
	}
	if c, _ := e.hub.InviteCard(id, alice, access(bob)); c == nil || !c.IsMember {
		t.Fatalf("card of a member = %+v", c)
	}
	// accepting again is fine
	if err := e.hub.AcceptInvite(id, 0, bob, "bob"); err != nil {
		t.Fatalf("accept again: %v", err)
	}

	// A closed lobby: no card
	if err := e.hub.CloseLobby(e.actor(alice), id); err != nil {
		t.Fatalf("close: %v", err)
	}
	if c, err := e.hub.InviteCard(id, alice, access(bob)); err != nil || c != nil {
		t.Fatalf("card of a closed lobby: %+v %v", c, err)
	}
	if c, err := e.hub.InviteCard(uuid.New(), alice, access(bob)); err != nil || c != nil {
		t.Fatalf("card of an unknown lobby: %+v %v", c, err)
	}
}
