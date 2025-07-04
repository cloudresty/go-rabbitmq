// Package encryption provides message encryption/decryption capabilities for the go-rabbitmq library.
//
// This package implements industry-standard encryption algorithms for securing message data
// in transit and at rest. It supports AES-GCM encryption and provides a pluggable
// interface for custom encryption implementations.
//
// Features:
//   - AES-256-GCM encryption with authenticated encryption
//   - Automatic nonce generation and validation
//   - Pluggable encryption interface for custom algorithms
//   - Zero-configuration setup with secure defaults
//   - Performance-optimized implementations
//
// Example usage:
//
//	// Create AES-GCM encryptor
//	key := make([]byte, 32) // 256-bit key
//	rand.Read(key)
//	encryptor := encryption.NewAESGCM(key)
//
//	// Use with publisher
//	publisher, err := rabbitmq.NewPublisher(
//		rabbitmq.WithEncryption(encryptor),
//	)
//
//	// Messages are automatically encrypted before publishing
//	err = publisher.Publish(ctx, "secure.exchange", "key", message)
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/cloudresty/go-rabbitmq"
)

// AESGCM implements the MessageEncryptor interface using AES-GCM encryption.
// AES-GCM provides both confidentiality and authenticity, making it ideal for
// message encryption in distributed systems.
type AESGCM struct {
	gcm cipher.AEAD
	key []byte
}

// NewAESGCM creates a new AES-GCM encryptor with the provided key.
// The key must be exactly 32 bytes (256 bits) for AES-256-GCM.
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-GCM requires 32-byte key, got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &AESGCM{
		gcm: gcm,
		key: key,
	}, nil
}

// Encrypt encrypts the provided data using AES-GCM.
// Returns the encrypted data with the nonce prepended.
func (a *AESGCM) Encrypt(data []byte) ([]byte, error) {
	// Generate a random nonce
	nonce := make([]byte, a.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := a.gcm.Seal(nil, nonce, data, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// Decrypt decrypts the provided data using AES-GCM.
// Expects the nonce to be prepended to the ciphertext.
func (a *AESGCM) Decrypt(data []byte) ([]byte, error) {
	nonceSize := a.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: expected at least %d bytes, got %d", nonceSize, len(data))
	}

	// Extract nonce and ciphertext
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	// Decrypt the data
	plaintext, err := a.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// Algorithm returns the encryption algorithm identifier.
func (a *AESGCM) Algorithm() string {
	return "AES-256-GCM"
}

// NewNoEncryption creates a no-op encryptor for testing and development.
// This encryptor passes data through unchanged.
func NewNoEncryption() rabbitmq.MessageEncryptor {
	return &noEncryption{}
}

// noEncryption is a no-op implementation for testing
type noEncryption struct{}

func (n *noEncryption) Encrypt(data []byte) ([]byte, error) { return data, nil }
func (n *noEncryption) Decrypt(data []byte) ([]byte, error) { return data, nil }
func (n *noEncryption) Algorithm() string                   { return "none" }

// Ensure AESGCM implements the interface at compile time
var _ rabbitmq.MessageEncryptor = (*AESGCM)(nil)
var _ rabbitmq.MessageEncryptor = (*noEncryption)(nil)
