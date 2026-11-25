package core

import (
	"testing"

	"go.uber.org/zap"
)

func TestEventBusTargetedDelivery(t *testing.T) {
	bus := NewEventBus(zap.NewNop())
	idA1, a1 := bus.SubscribeUser(1)
	_, a2 := bus.SubscribeUser(1) // a second tab of user 1
	_, b := bus.SubscribeUser(2)
	_, anon := bus.Subscribe()

	bus.PublishToUser(1, SceneEvent{Type: "social:followed"})
	for name, ch := range map[string]<-chan SceneEvent{"a1": a1, "a2": a2} {
		ev, ok := receive(ch)
		if !ok || ev.Type != "social:followed" || ev.UserID != 1 {
			t.Fatalf("%s got %+v, %v", name, ev, ok)
		}
	}
	for name, ch := range map[string]<-chan SceneEvent{"b": b, "anon": anon} {
		if ev, ok := receive(ch); ok {
			t.Fatalf("%s got the targeted event %+v", name, ev)
		}
	}

	// broadcasts still reach everyone
	bus.Publish(SceneEvent{Type: "scene:completed", SceneID: 3})
	for name, ch := range map[string]<-chan SceneEvent{"a1": a1, "a2": a2, "b": b, "anon": anon} {
		if ev, ok := receive(ch); !ok || ev.Type != "scene:completed" {
			t.Fatalf("%s got %+v, %v", name, ev, ok)
		}
	}

	// a Publish with a UserID is targeted too; user 0 sends nothing
	bus.Publish(SceneEvent{Type: "social:x", UserID: 2})
	bus.PublishToUser(0, SceneEvent{Type: "social:y"})
	if ev, ok := receive(b); !ok || ev.Type != "social:x" {
		t.Fatalf("b got %+v, %v", ev, ok)
	}
	if ev, ok := receive(anon); ok {
		t.Fatalf("anon got %+v", ev)
	}

	bus.PublishToUsers([]uint{1, 2}, SceneEvent{Type: "social:z"})
	if _, ok := receive(a1); !ok {
		t.Fatal("a1 missed PublishToUsers")
	}
	if _, ok := receive(b); !ok {
		t.Fatal("b missed PublishToUsers")
	}
	bus.Unsubscribe(idA1)
}
