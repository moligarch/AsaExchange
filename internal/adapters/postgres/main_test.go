package postgres

import (
	"AsaExchange/internal/adapters/security"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"encoding/hex"
	"log"
	"os"
	"testing"

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
	testDB, err = NewDB(context.Background(), cfg.DatabaseURL, &nopLogger)
	if err != nil {
		log.Fatalf("TestMain: Failed to connect to test database: %v", err)
	}

	// 5. Run tests
	code := m.Run()

	// 6. Teardown
	testDB.Close()
	os.Exit(code)
}
