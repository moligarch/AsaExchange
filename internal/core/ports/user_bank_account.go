package ports

import (
	"AsaExchange/internal/core/domain"
	"context"

	"github.com/google/uuid"
)

// UserBankAccountRepository defines persistence for user bank accounts.
type UserBankAccountRepository interface {
	// Create saves a new bank account for a user.
	Create(ctx context.Context, acct *domain.UserBankAccount) error

	// GetByUserID finds all bank accounts for a given user.
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.UserBankAccount, error)
}
