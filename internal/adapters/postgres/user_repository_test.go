package postgres

import (
	"AsaExchange/internal/core/domain"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Helper to clean up the DB after tests
func cleanupTestUser(t *testing.T, id uuid.UUID) {
	_, err := testDB.pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		t.Logf("Warning: Failed to cleanup user %s: %v", id, err)
	}
}

func TestUserRepository_Create_GetByTelegramID_Roundtrip(t *testing.T) {
	// 1. Setup
	nopLogger := zerolog.Nop()
	repo := NewUserRepository(testDB, testSecSvc, &nopLogger)

	phone := "123456789"
	govID := "ABC-123"

	user := &domain.User{
		ID:                 uuid.New(),
		TelegramID:         time.Now().UnixNano(), // Unique ID for testing
		FirstName:          "Test",
		LastName:           func(s string) *string { return &s }("User"),
		PhoneNumber:        &phone,
		GovernmentID:       &govID,
		LocationCountry:    func(s string) *string { return &s }("USA"),
		VerificationStatus: domain.VerificationPending,
		IsModerator:        false,
	}

	// 2. Run Create
	ctx := context.Background()
	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	// Defer cleanup
	defer cleanupTestUser(t, user.ID)

	// 3. Run GetByTelegramID
	foundUser, err := repo.GetByTelegramID(ctx, user.TelegramID)
	if err != nil {
		t.Fatalf("Failed to get user by telegram ID: %v", err)
	}
	if foundUser == nil {
		t.Fatalf("GetByTelegramID: user not found, but should exist")
	}

	// 4. Verify
	if foundUser.ID != user.ID {
		t.Errorf("ID mismatch: got %v, want %v", foundUser.ID, user.ID)
	}
	if foundUser.FirstName != user.FirstName {
		t.Errorf("FirstName mismatch: got %s, want %s", foundUser.FirstName, user.FirstName)
	}
	if *foundUser.PhoneNumber != *user.PhoneNumber {
		t.Errorf("PhoneNumber mismatch (decryption failed?): got %s, want %s", *foundUser.PhoneNumber, *user.PhoneNumber)
	}
	if *foundUser.GovernmentID != *user.GovernmentID {
		t.Errorf("GovernmentID mismatch (decryption failed?): got %s, want %s", *foundUser.GovernmentID, *user.GovernmentID)
	}
	t.Logf("Successfully created and retrieved user %s", user.ID)
}

func TestUserRepository_GetByTelegramID_NotFound(t *testing.T) {
	nopLogger := zerolog.Nop()
	repo := NewUserRepository(testDB, testSecSvc, &nopLogger)

	// Use a telegram ID that cannot exist
	nonExistentID := int64(-12345)

	foundUser, err := repo.GetByTelegramID(context.Background(), nonExistentID)
	if err != nil {
		t.Fatalf("GetByTelegramID for non-existent user returned an error: %v", err)
	}
	if foundUser != nil {
		t.Fatalf("GetByTelegramID found a user, but it should not exist")
	}
}
