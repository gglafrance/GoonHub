package core

import (
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type VideoEvent struct {
	Type    string `json:"type"`
	VideoID uint   `json:"video_id"`
	Data    any    `json:"data,omitempty"`
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan VideoEvent
	logger      *zap.Logger
}

func NewEventBus(logger *zap.Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[string]chan VideoEvent),
		logger:      logger.With(zap.String("component", "event_bus")),
	}
}

func (eb *EventBus) Subscribe() (string, <-chan VideoEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := uuid.New().String()
	ch := make(chan VideoEvent, 50)
	eb.subscribers[id] = ch

	eb.logger.Debug("New subscriber", zap.String("subscriber_id", id))
	return id, ch
}

func (eb *EventBus) Unsubscribe(id string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if ch, ok := eb.subscribers[id]; ok {
		close(ch)
		delete(eb.subscribers, id)
		eb.logger.Debug("Subscriber removed", zap.String("subscriber_id", id))
	}
}

func (eb *EventBus) Publish(event VideoEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	eb.logger.Debug("Publishing event",
		zap.String("type", event.Type),
		zap.Uint("video_id", event.VideoID),
		zap.Int("subscriber_count", len(eb.subscribers)),
	)

	for id, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			eb.logger.Warn("Subscriber channel full, dropping event",
				zap.String("subscriber_id", id),
				zap.String("event_type", event.Type),
			)
		}
	}
}
