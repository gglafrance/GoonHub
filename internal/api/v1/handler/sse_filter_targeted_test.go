package handler

import (
	"testing"

	"goonhub/internal/core"
)

func TestSSETargetedEvents(t *testing.T) {
	alice := core.NewUserAccess(1, "user", false, core.PermScenesView)
	bob := core.NewUserAccess(2, "user", false)
	admin := core.NewUserAccess(3, "admin", true)

	toAlice := core.SceneEvent{Type: core.EventSocialFriendRequest, UserID: 1, Data: map[string]any{"user": map[string]any{"id": 2}, "incoming_requests": 1}}
	broadcastSocial := core.SceneEvent{Type: core.EventSocialFollowed, Data: map[string]any{"user": map[string]any{"id": 2}}}
	targetedOther := core.SceneEvent{Type: "presence:update", UserID: 1, Data: map[string]any{"x": 1}}

	tests := []struct {
		name   string
		event  core.SceneEvent
		access *core.UserAccess
		send   bool
	}{
		{"target gets it", toAlice, alice, true},
		{"another user does not", toAlice, bob, false},
		{"a superuser does not either", toAlice, admin, false},
		{"broadcast social events are dropped", broadcastSocial, alice, false},
		{"broadcast social events are dropped for superusers", broadcastSocial, admin, false},
		{"any targeted type goes to its target only", targetedOther, alice, true},
		{"any targeted type, other user", targetedOther, bob, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, send := eventForUser(tc.event, tc.access)
			if send != tc.send {
				t.Fatalf("send = %v, want %v", send, tc.send)
			}
			if send {
				data, ok := ev.Data.(map[string]any)
				if !ok || len(data) != len(tc.event.Data.(map[string]any)) {
					t.Fatalf("targeted data must be sent as is, got %v", ev.Data)
				}
			}
		})
	}
}
