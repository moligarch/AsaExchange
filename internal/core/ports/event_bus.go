package ports

import "context"

// Event is a generic wrapper for any event payload
type Event struct {
	Topic string
	Data  interface{}
}

// EventHandler is a function that can handle a specific event
type EventHandler func(ctx context.Context, event Event) error

// EventBus defines the interface for our in-process pub/sub system
type EventBus interface {
	// Publish sends an event to all subscribers of a topic
	Publish(ctx context.Context, topic string, data interface{}) error

	// Subscribe registers a handler for a specific topic
	Subscribe(topic string, handler EventHandler)
}
