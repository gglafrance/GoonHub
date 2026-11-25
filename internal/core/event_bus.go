package core

import (
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SceneEvent struct {
	Type    string `json:"type"`
	SceneID uint   `json:"scene_id"`
	Data    any    `json:"data,omitempty"`
	// UserID targets the event at one user (PublishToUser): only the streams of that user
	// receive it, with its data as is. 0 = every subscriber (then filtered by the SSE rules).
	// Never sent to the client.
	UserID uint `json:"-"`
}

// eventSubscriber is a stream of events; userID is the user it belongs to (0 = no user: it
// never receives targeted events).
type eventSubscriber struct {
	ch     chan SceneEvent
	userID uint
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]eventSubscriber
	logger      *zap.Logger
}

func NewEventBus(logger *zap.Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[string]eventSubscriber),
		logger:      logger.With(zap.String("component", "event_bus")),
	}
}

// Subscribe opens a stream that belongs to no user: it receives the broadcast events only.
func (eb *EventBus) Subscribe() (string, <-chan SceneEvent) {
	return eb.SubscribeUser(0)
}

// SubscribeUser opens a stream of a user: it receives the broadcast events and the events
// targeted at that user (PublishToUser).
func (eb *EventBus) SubscribeUser(userID uint) (string, <-chan SceneEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := uuid.New().String()
	ch := make(chan SceneEvent, 50)
	eb.subscribers[id] = eventSubscriber{ch: ch, userID: userID}

	eb.logger.Debug("New subscriber", zap.String("subscriber_id", id), zap.Uint("user_id", userID))
	return id, ch
}

func (eb *EventBus) Unsubscribe(id string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if sub, ok := eb.subscribers[id]; ok {
		close(sub.ch)
		delete(eb.subscribers, id)
		eb.logger.Debug("Subscriber removed", zap.String("subscriber_id", id))
	}
}

// Publish sends the event to every subscriber. An event with a UserID is only sent to that
// user's streams (as PublishToUser).
func (eb *EventBus) Publish(event SceneEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	eb.logger.Debug("Publishing event",
		zap.String("type", event.Type),
		zap.Uint("scene_id", event.SceneID),
		zap.Uint("target_user_id", event.UserID),
		zap.Int("subscriber_count", len(eb.subscribers)),
	)

	for id, sub := range eb.subscribers {
		if event.UserID != 0 && sub.userID != event.UserID {
			continue
		}
		select {
		case sub.ch <- event:
		default:
			eb.logger.Warn("Subscriber channel full, dropping event",
				zap.String("subscriber_id", id),
				zap.String("event_type", event.Type),
			)
		}
	}
}

// PublishToUser sends the event to the streams of one user only (every tab or device of that
// user), never to other users. userID 0 sends nothing.
func (eb *EventBus) PublishToUser(userID uint, event SceneEvent) {
	if userID == 0 {
		return
	}
	event.UserID = userID
	eb.Publish(event)
}

// PublishToUsers sends a copy of the event to the streams of each user.
func (eb *EventBus) PublishToUsers(userIDs []uint, event SceneEvent) {
	for _, id := range userIDs {
		eb.PublishToUser(id, event)
	}
}
