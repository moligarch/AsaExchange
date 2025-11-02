package ports

// SecurityPort defines the interface for encrypting and decrypting sensitive data.
// This allows us to swap the implementation (e.g., from AES to something else)
// without changing any business logic that uses it.
type SecurityPort interface {
	// Encrypt takes a plaintext and returns a secure, encrypted ciphertext.
	Encrypt(plaintext []byte) (ciphertext []byte, err error)

	// Decrypt takes a ciphertext and returns the original plaintext.
	Decrypt(ciphertext []byte) (plaintext []byte, err error)
}
