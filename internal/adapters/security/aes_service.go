package security

import (
	"AsaExchange/internal/core/ports" // Check path
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog" // Import logger
)

// aesService implements the SecurityPort interface using AES-GCM.
type aesService struct {
	gcm cipher.AEAD
	log zerolog.Logger // Store the contextual logger
}

// NewAESService creates a new security service.
// It now accepts a baseLogger and adds its own context.
func NewAESService(encryptionKey []byte, baseLogger *zerolog.Logger) (ports.SecurityPort, error) {
	if len(encryptionKey) != 16 && len(encryptionKey) != 32 {
		return nil, errors.New("encryptionKey must be 16 or 32 bytes")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("could not create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("could not create GCM: %w", err)
	}

	// YOUR PATTERN: Constructor creates its own contextual logger
	log := baseLogger.With().Str("component", "security_service").Logger()
	log.Info().Msg("Security service initialized") // Log from the service itself

	return &aesService{gcm: gcm, log: log}, nil
}

// Encrypt encrypts data using AES-GCM.
func (s *aesService) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		s.log.Error().Err(err).Msg("Failed to generate nonce")
		return nil, fmt.Errorf("could not generate nonce: %w", err)
	}

	ciphertext := s.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM.
func (s *aesService) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := s.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext is too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := s.gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		// Log a warning: this can happen if data is tampered with
		s.log.Warn().Err(err).Msg("Failed to decrypt ciphertext (tampered or corrupt?)")
		return nil, fmt.Errorf("could not decrypt: %w", err)
	}

	return plaintext, nil
}
