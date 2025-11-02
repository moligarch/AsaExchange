package security

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/rs/zerolog"
)

// helper function to generate a valid key
func generateKey(length int) []byte {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}

func TestAESService_EncryptDecrypt_Roundtrip(t *testing.T) {
	// Create a "No-Op" logger that discards all logs
	nopLogger := zerolog.Nop()

	// Test cases for both AES-128 and AES-256
	testCases := []struct {
		name    string
		key     []byte
		payload []byte
	}{
		{
			name:    "AES-128 (16-byte key)",
			key:     generateKey(16),
			payload: []byte("this is a secret message"),
		},
		{
			name:    "AES-256 (32-byte key)",
			key:     generateKey(32),
			payload: []byte("this is a much more secret message 12345"),
		},
		{
			name:    "Empty Payload",
			key:     generateKey(32),
			payload: []byte(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewAESService(tc.key, &nopLogger)
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			// 1. Encrypt
			ciphertext, err := service.Encrypt(tc.payload)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			if bytes.Equal(ciphertext, tc.payload) {
				t.Fatal("Encryption did not change the data")
			}

			// 2. Decrypt
			plaintext, err := service.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			// 3. Verify
			if !bytes.Equal(plaintext, tc.payload) {
				t.Fatalf("Decrypted data does not match original. \nGot: %s\nWant: %s",
					string(plaintext), string(tc.payload))
			}
		})
	}
}

func TestAESService_Decrypt_Tampered(t *testing.T) {
	nopLogger := zerolog.Nop()
	key := generateKey(32)
	payload := []byte("do not tamper with this")

	service, err := NewAESService(key, &nopLogger)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ciphertext, err := service.Encrypt(payload)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Tamper with the ciphertext (flip a bit)
	ciphertext[len(ciphertext)-1] = ^ciphertext[len(ciphertext)-1]

	_, err = service.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("Decryption succeeded on tampered data, but it should have failed.")
	}
	t.Logf("Got expected decryption error: %v", err)
}

func TestNewAESService_InvalidKey(t *testing.T) {
	nopLogger := zerolog.Nop()
	_, err := NewAESService([]byte("badkey"), &nopLogger)
	if err == nil {
		t.Fatal("Service creation should fail with invalid key length")
	}
	t.Logf("Got expected creation error: %v", err)
}
