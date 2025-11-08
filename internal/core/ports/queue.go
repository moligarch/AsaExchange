package ports

import (
	"context"

	"github.com/google/uuid"
)

// NewVerificationEvent holds the data for a new user pending review.
// This is the "payload" our queue will transmit.
type NewVerificationEvent struct {
	UserID  uuid.UUID
	FileID  string // The Telegram FileID of the photo
	Caption string // The formatted text (Name, GovID, etc.)
}

// VerificationQueue is the abstract interface for our "notifier."
type VerificationQueue interface {
	// Publish is called by the Customer Bot (registration handler)
	// It returns the unique "storage reference" (which is the message_id in our MVP)
	Publish(ctx context.Context, event NewVerificationEvent) (storageRef string, err error)

	// Subscribe is called by the Moderator Bot on startup.
	// It runs in a goroutine, listening for new events from the queue
	// and passing them to the handler function.
	Subscribe(ctx context.Context, handler func(event NewVerificationEvent) error)
}
