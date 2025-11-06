package postgres

import (
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Helper to create a user for testing
func createTestUser(t *testing.T, repo ports.UserRepository) *domain.User {
	user := &domain.User{
		ID:                 uuid.New(),
		TelegramID:         time.Now().UnixNano(),
		FirstName:          func(s string) *string { return &s }("BankAccount"),
		LastName:           func(s string) *string { return &s }("TestUser"),
		VerificationStatus: domain.VerificationPending,
		State:              domain.StateAwaitingFirstName,
	}
	err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create test user for bank acct test: %v", err)
	}
	return user
}

// Helper to clean up the bank account
func cleanupTestUserBankAccount(t *testing.T, id uuid.UUID) {
	_, err := testDB.pool.Exec(context.Background(), "DELETE FROM user_bank_accounts WHERE id = $1", id)
	if err != nil {
		t.Logf("Warning: Failed to cleanup bank account %s: %v", id, err)
	}
}

func TestUserBankAccountRepository_Create_GetByUserID_Roundtrip(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()

	// We need a user repo to create a user
	userRepo := NewUserRepository(testDB, testSecSvc, &nopLogger)
	// This is the repo we are testing
	bankRepo := NewUserBankAccountRepository(testDB, testSecSvc, &nopLogger)

	// Create a user to own the account
	user := createTestUser(t, userRepo)
	defer cleanupTestUser(t, user.ID) // Clean up the user

	// 2. Create Bank Account
	acctDetails := "IBAN: DE89 3704 0044 0532 0130 00"
	acct := &domain.UserBankAccount{
		ID:             uuid.New(),
		UserID:         user.ID,
		AccountName:    "My N26",
		Currency:       "EUR",
		BankName:       "N26",
		AccountDetails: acctDetails,
	}

	err := bankRepo.Create(ctx, acct)
	if err != nil {
		t.Fatalf("Failed to create bank account: %v", err)
	}
	defer cleanupTestUserBankAccount(t, acct.ID) // Clean up the account

	// 3. Run GetByUserID
	foundAccts, err := bankRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get bank accounts: %v", err)
	}
	if len(foundAccts) == 0 {
		t.Fatal("No bank accounts found, expected 1")
	}
	if len(foundAccts) > 1 {
		t.Fatalf("Found %d accounts, expected 1", len(foundAccts))
	}

	foundAcct := foundAccts[0]

	// 4. Verify
	if foundAcct.ID != acct.ID {
		t.Errorf("ID mismatch: got %v, want %v", foundAcct.ID, acct.ID)
	}
	if foundAcct.BankName != acct.BankName {
		t.Errorf("BankName mismatch: got %s, want %s", foundAcct.BankName, acct.BankName)
	}
	if foundAcct.AccountDetails != acctDetails {
		t.Errorf("AccountDetails mismatch (decryption failed?): got %s, want %s",
			foundAcct.AccountDetails, acctDetails)
	}
	t.Logf("Successfully created and retrieved bank account %s", acct.ID)
}

func TestUserBankAccountRepository_GetByUserID_NotFound(t *testing.T) {
	nopLogger := zerolog.Nop()
	bankRepo := NewUserBankAccountRepository(testDB, testSecSvc, &nopLogger)

	// Use a UUID that cannot exist
	nonExistentUserID := uuid.New()

	foundAccts, err := bankRepo.GetByUserID(context.Background(), nonExistentUserID)
	if err != nil {
		t.Fatalf("GetByUserID for non-existent user returned an error: %v", err)
	}
	if len(foundAccts) != 0 {
		t.Fatalf("Found %d accounts, but should be 0", len(foundAccts))
	}
}
