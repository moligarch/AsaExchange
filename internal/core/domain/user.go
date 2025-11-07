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

// UserState is a custom type for our state machine ENUM
type UserState string

const (
	StateNone                   UserState = "none"
	StateAwaitingFirstName      UserState = "awaiting_first_name"
	StateAwaitingLastName       UserState = "awaiting_last_name"
	StateAwaitingPhoneNumber    UserState = "awaiting_phone_number"
	StateAwaitingGovID          UserState = "awaiting_gov_id"
	StateAwaitingLocation       UserState = "awaiting_location"
	StateAwaitingIdentityDoc    UserState = "awaiting_identity_doc"
	StateAwaitingPolicyApproval UserState = "awaiting_policy_approval"
)

// User represents a user in the system.
type User struct {
	ID                   uuid.UUID
	TelegramID           int64
	FirstName            *string // Nullable
	LastName             *string // Nullable
	PhoneNumber          *string // Encrypted
	GovernmentID         *string // Encrypted
	LocationCountry      *string // Nullable
	VerificationStatus   UserVerificationStatus
	State                UserState
	VerificationStrategy *string // Nullable
	GovernmentIDPhotoID  *string // Nullable, Telegram FileID
	IsModerator          bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
