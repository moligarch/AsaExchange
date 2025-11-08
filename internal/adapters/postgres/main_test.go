package postgres

import (
	"AsaExchange/internal/adapters/security"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"encoding/hex"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var (
	testDB     *DB
	testSecSvc ports.SecurityPort
)

// TestMain sets up a connection to the test database.
func TestMain(m *testing.M) {
	// 1. Load config to get DB URL and Encryption Key
	// We MUST load the .env file from the project root.
	// This assumes tests are run from the package directory.
	// We need to go up 3 levels: /postgres -> /adapters -> /internal -> ROOT
	os.Chdir("../../../") // Go to root to find .env

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("TestMain: Failed to load config: %v", err)
	}

	// 2. Set up logger
	nopLogger := zerolog.Nop()

	// 3. Set up Security Service
	keyBytes, _ := hex.DecodeString(cfg.EncryptionKey)
	testSecSvc, err = security.NewAESService(keyBytes, &nopLogger)
	if err != nil {
		log.Fatalf("TestMain: Failed to create security service: %v", err)
	}

	// 4. Set up DB Connection
	testDB, err = NewDB(context.Background(), cfg.Postgres.URL, &nopLogger)
	if err != nil {
		log.Fatalf("TestMain: Failed to connect to test database: %v", err)
	}

	// 5. Run tests
	code := m.Run()

	// 6. Teardown
	testDB.Close()
	os.Exit(code)
}

// Helper to create a user for testing
func createTestUser(t *testing.T, repo ports.UserRepository) (*domain.User, func()) {
	strategy := "manual"
	user := &domain.User{
		ID:                   uuid.New(),
		TelegramID:           time.Now().UnixNano(),
		FirstName:            func(s string) *string { return &s }("Test"),
		LastName:             func(s string) *string { return &s }("User"),
		State:                domain.StateAwaitingFirstName,
		VerificationStatus:   domain.VerificationPending,
		VerificationStrategy: &strategy,
	}
	ctx := t.Context()
	err := repo.Create(ctx, user)
	if err != nil {
		log.Fatalf("createTestUser failed: %v", err)
	}

	cleanup := func() {
		err := repo.Delete(ctx, user.ID)
		if err != nil {
			log.Printf("Warning: failed to cleanup test user %s: %v", user.ID, err)
		}
	}

	return user, cleanup
}

// Helper to clean up the DB after tests
func cleanupTestUser(t *testing.T, id uuid.UUID) {
	_, err := testDB.pool.Exec(t.Context(), "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		t.Logf("Warning: Failed to cleanup user %s: %v", id, err)
	}
}

// Helper to clean up the bank account
func cleanupTestUserBankAccount(t *testing.T, id uuid.UUID) {
	_, err := testDB.pool.Exec(t.Context(), "DELETE FROM user_bank_accounts WHERE id = $1", id)
	if err != nil {
		t.Logf("Warning: Failed to cleanup bank account %s: %v", id, err)
	}
}
