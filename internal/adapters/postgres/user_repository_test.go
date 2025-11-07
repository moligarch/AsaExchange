package postgres

import (
	"AsaExchange/internal/core/domain"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestUserRepository_Create_GetByTelegramID_Roundtrip(t *testing.T) {
	// 1. Setup
	nopLogger := zerolog.Nop()
	repo := NewUserRepository(testDB, testSecSvc, &nopLogger)
	ctx := context.Background()

	phone := "123456789"
	govID := "ABC-123"
	firstName := "Test"
	strategy := "manual"
	photoID := "file_id_abc"

	user := &domain.User{
		ID:                   uuid.New(),
		TelegramID:           time.Now().UnixNano(),
		FirstName:            &firstName,
		LastName:             func(s string) *string { return &s }("User"),
		PhoneNumber:          &phone,
		GovernmentID:         &govID,
		LocationCountry:      func(s string) *string { return &s }("USA"),
		VerificationStatus:   domain.VerificationPending,
		State:                domain.StateAwaitingLastName,
		IsModerator:          false,
		VerificationStrategy: &strategy,
		GovernmentIDPhotoID:  &photoID,
	}

	// 2. Run Create
	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
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
	if *foundUser.FirstName != *user.FirstName {
		t.Errorf("FirstName mismatch: got %s, want %s", *foundUser.FirstName, *user.FirstName)
	}
	if *foundUser.PhoneNumber != *user.PhoneNumber {
		t.Errorf("PhoneNumber mismatch (decryption failed?): got %s, want %s", *foundUser.PhoneNumber, *user.PhoneNumber)
	}
	if *foundUser.GovernmentID != *user.GovernmentID {
		t.Errorf("GovernmentID mismatch (decryption failed?): got %s, want %s", *foundUser.GovernmentID, *user.GovernmentID)
	}
	if foundUser.State != user.State {
		t.Errorf("State mismatch: got %s, want %s", foundUser.State, user.State)
	}
	if *foundUser.VerificationStrategy != *user.VerificationStrategy {
		t.Errorf("VerificationStrategy mismatch: got %s, want %s", *foundUser.VerificationStrategy, *user.VerificationStrategy)
	}
	if *foundUser.GovernmentIDPhotoID != *user.GovernmentIDPhotoID {
		t.Errorf("GovernmentIDPhotoID mismatch: got %s, want %s", *foundUser.GovernmentIDPhotoID, *user.GovernmentIDPhotoID)
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

func TestUserRepository_Update(t *testing.T) {
	// 1. Setup
	nopLogger := zerolog.Nop()
	repo := NewUserRepository(testDB, testSecSvc, &nopLogger)
	ctx := t.Context()

	user, cleanup := createTestUser(t, repo)
	defer cleanup()

	// 2. Modify the user struct
	newFirstName := "Moein"
	newLastName := "Verkiani"
	newState := domain.StateAwaitingLastName
	newPhotoID := "file_id_xyz"

	user.FirstName = &newFirstName
	user.LastName = &newLastName
	user.State = newState
	user.GovernmentIDPhotoID = &newPhotoID

	// 3. Run Update
	err := repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 4. Verify
	updatedUser, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if *updatedUser.FirstName != newFirstName {
		t.Errorf("FirstName was not updated: got %s, want %s", *updatedUser.FirstName, newFirstName)
	}
	if *updatedUser.LastName != newLastName {
		t.Errorf("LastName was not updated: got %s, want %s", *updatedUser.LastName, newLastName)
	}
	if *updatedUser.GovernmentIDPhotoID != newPhotoID {
		t.Errorf("GovernmentIDPhotoID was not updated: got %s, want %s", *updatedUser.GovernmentIDPhotoID, newPhotoID)
	}
	if updatedUser.State != newState {
		t.Errorf("State was not updated: got %s, want %s", updatedUser.State, newState)
	}
	t.Logf("Successfully updated user")
}

func TestUserRepository_Delete(t *testing.T) {
	// 1. Setup
	nopLogger := zerolog.Nop()
	repo := NewUserRepository(testDB, testSecSvc, &nopLogger)
	ctx := t.Context()

	user, _ := createTestUser(t, repo)

	// 2. Run Delete
	err := repo.Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 3. Verify
	deletedUser, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed after delete: %v", err)
	}
	if deletedUser != nil {
		t.Fatal("User was found after delete, but should be nil")
	}
	t.Logf("Successfully deleted user")
}
