package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserBankAccount holds the payout info for a user.
type UserBankAccount struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	AccountName    string
	Currency       string
	BankName       string
	AccountDetails string // Encrypted
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
