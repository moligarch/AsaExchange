package eventbus

import (
	"AsaExchange/internal/core/ports"
	"context"
	"sync"

	"github.com/rs/zerolog"
)

// inMemoryEventBus implements the ports.EventBus interface
type inMemoryEventBus struct {
	log         zerolog.Logger
	subscribers map[string][]ports.EventHandler
	mu          sync.RWMutex
}

// NewInMemoryEventBus creates a new, empty event bus
func NewInMemoryEventBus(baseLogger *zerolog.Logger) ports.EventBus {
	return &inMemoryEventBus{
		log:         baseLogger.With().Str("component", "in_memory_bus").Logger(),
		subscribers: make(map[string][]ports.EventHandler),
	}
}

// Publish sends an event to all subscribers of a topic
func (b *inMemoryEventBus) Publish(ctx context.Context, topic string, data interface{}) error {
	b.mu.RLock() // Lock for reading the map
	defer b.mu.RUnlock()

	handlers, ok := b.subscribers[topic]
	if !ok {
		// No subscribers for this topic, which is fine
		b.log.Warn().Str("topic", topic).Msg("Published event with no subscribers")
		return nil
	}

	event := ports.Event{
		Topic: topic,
		Data:  data,
	}

	// We launch each handler in its own goroutine
	// so that one slow handler doesn't block all the others.
	for _, handler := range handlers {
		go func(h ports.EventHandler) {
			// We pass a new background context so the handler
			// isn't cancelled if the *publisher's* context is.
			if err := h(context.Background(), event); err != nil {
				b.log.Error().Err(err).Str("topic", topic).Msg("Event handler failed")
			}
		}(handler)
	}

	b.log.Info().Str("topic", topic).Int("handlers", len(handlers)).Msg("Event published")
	return nil
}

// Subscribe registers a handler for a specific topic
func (b *inMemoryEventBus) Subscribe(topic string, handler ports.EventHandler) {
	b.mu.Lock() // Lock for writing to the map
	defer b.mu.Unlock()

	b.subscribers[topic] = append(b.subscribers[topic], handler)
	b.log.Info().Str("topic", topic).Msg("New handler subscribed to topic")
}
