package ports

import (
	"AsaExchange/internal/core/domain"
	"context"

	"github.com/google/uuid"
)

// UserRepository defines the persistence operations for Users.
type UserRepository interface {
	// Create saves a new user to the database.
	Create(ctx context.Context, user *domain.User) error

	// GetByTelegramID finds a user by their unique Telegram ID.
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)

	// GetByID finds a user by their internal UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)

	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID) error

	// GetNextPendingUser finds the oldest user in 'pending' status.
	GetNextPendingUser(ctx context.Context) (*domain.User, error)
}
