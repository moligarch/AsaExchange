package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserVerificationStatus is a custom type for our ENUM
type UserVerificationStatus string

const (
	VerificationPending  UserVerificationStatus = "pending"
	VerificationLevel1   UserVerificationStatus = "level_1"
	VerificationRejected UserVerificationStatus = "rejected"
)

// User represents a user in the system.
type User struct {
	ID                 uuid.UUID
	TelegramID         int64
	FirstName          string
	LastName           *string // Use pointer for nullable fields
	PhoneNumber        *string // Encrypted
	GovernmentID       *string // Encrypted
	LocationCountry    *string // Nullable
	VerificationStatus UserVerificationStatus
	IsModerator        bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
