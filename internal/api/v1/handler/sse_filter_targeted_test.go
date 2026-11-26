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
	broadcastPresence := core.SceneEvent{Type: core.EventPresenceUpdate, Data: map[string]any{"user": map[string]any{"id": 2}}}
	chatToAlice := core.SceneEvent{Type: core.EventChatMessage, UserID: 1, Data: map[string]any{"conversation_id": 4, "message": map[string]any{"id": 9}}}
	broadcastChat := core.SceneEvent{Type: core.EventChatTyping, Data: map[string]any{"conversation_id": 4, "user_id": 2}}
	reactionToAlice := core.SceneEvent{Type: core.EventChatReaction, UserID: 1, Data: map[string]any{"conversation_id": 4, "message_id": 9, "reactions": []any{}}}
	broadcastReaction := core.SceneEvent{Type: core.EventChatReaction, Data: map[string]any{"conversation_id": 4, "message_id": 9, "reactions": []any{}}}

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
		{"broadcast presence events are dropped", broadcastPresence, alice, false},
		{"broadcast presence events are dropped for superusers", broadcastPresence, admin, false},
		{"a message event goes to its participant", chatToAlice, alice, true},
		{"a message event never reaches another user", chatToAlice, bob, false},
		{"a message event never reaches a superuser", chatToAlice, admin, false},
		{"broadcast chat events are dropped", broadcastChat, alice, false},
		{"broadcast chat events are dropped for superusers", broadcastChat, admin, false},
		{"a reaction event goes to its participant", reactionToAlice, alice, true},
		{"a reaction event never reaches another user", reactionToAlice, bob, false},
		{"a reaction event never reaches a superuser", reactionToAlice, admin, false},
		{"broadcast reaction events are dropped", broadcastReaction, alice, false},
		{"broadcast reaction events are dropped for superusers", broadcastReaction, admin, false},
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
