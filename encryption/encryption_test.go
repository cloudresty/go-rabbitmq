package encryption

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestAESGCM_NewAESGCM(t *testing.T) {
	tests := []struct {
		name      string
		keyLength int
		wantErr   bool
	}{
		{
			name:      "valid 32-byte key",
			keyLength: 32,
			wantErr:   false,
		},
		{
			name:      "invalid 16-byte key",
			keyLength: 16,
			wantErr:   true,
		},
		{
			name:      "invalid 24-byte key",
			keyLength: 24,
			wantErr:   true,
		},
		{
			name:      "invalid empty key",
			keyLength: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLength)
			if _, err := rand.Read(key); err != nil {
				t.Fatalf("Failed to generate random key: %v", err)
			}

			encryptor, err := NewAESGCM(key)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewAESGCM() expected error, got nil")
				}
				if encryptor != nil {
					t.Errorf("NewAESGCM() expected nil encryptor on error, got %v", encryptor)
				}
			} else {
				if err != nil {
					t.Errorf("NewAESGCM() unexpected error: %v", err)
				}
				if encryptor == nil {
					t.Errorf("NewAESGCM() expected encryptor, got nil")
				}
			}
		})
	}
}

func TestAESGCM_EncryptDecrypt(t *testing.T) {
	// Create encryptor with random key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	encryptor, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "small data",
			data: []byte("hello world"),
		},
		{
			name: "medium data",
			data: []byte("This is a longer message that should test the encryption properly with some reasonable length."),
		},
		{
			name: "large data",
			data: bytes.Repeat([]byte("A"), 10000),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := encryptor.Encrypt(tt.data)
			if err != nil {
				t.Fatalf("Encrypt() failed: %v", err)
			}

			// Verify encrypted data is different (unless empty)
			if len(tt.data) > 0 && bytes.Equal(tt.data, encrypted) {
				t.Error("Encrypted data should not equal original data")
			}

			// Verify encrypted data is longer (nonce + ciphertext + auth tag)
			if len(encrypted) <= len(tt.data) {
				t.Errorf("Encrypted data length %d should be greater than original %d", len(encrypted), len(tt.data))
			}

			// Decrypt
			decrypted, err := encryptor.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() failed: %v", err)
			}

			// Verify decrypted data matches original
			if !bytes.Equal(tt.data, decrypted) {
				t.Errorf("Decrypted data does not match original.\nOriginal:  %q\nDecrypted: %q", tt.data, decrypted)
			}
		})
	}
}

func TestAESGCM_DecryptInvalidData(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	encryptor, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "too short data",
			data: []byte{0x01, 0x02},
		},
		{
			name: "corrupted data",
			data: make([]byte, 32), // Random bytes that won't decrypt properly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tt.data)
			if err == nil {
				t.Error("Decrypt() should have failed with invalid data")
			}
		})
	}
}

func TestAESGCM_Algorithm(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}

	encryptor, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	algorithm := encryptor.Algorithm()
	expected := "AES-256-GCM"

	if algorithm != expected {
		t.Errorf("Algorithm() = %q, expected %q", algorithm, expected)
	}
}

func TestAESGCM_DifferentKeysProduceDifferentResults(t *testing.T) {
	data := []byte("test message")

	// Create two encryptors with different keys
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("Failed to generate key1: %v", err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("Failed to generate key2: %v", err)
	}

	encryptor1, err := NewAESGCM(key1)
	if err != nil {
		t.Fatalf("Failed to create encryptor1: %v", err)
	}

	encryptor2, err := NewAESGCM(key2)
	if err != nil {
		t.Fatalf("Failed to create encryptor2: %v", err)
	}

	// Encrypt same data with both encryptors
	encrypted1, err := encryptor1.Encrypt(data)
	if err != nil {
		t.Fatalf("Encrypt1() failed: %v", err)
	}

	encrypted2, err := encryptor2.Encrypt(data)
	if err != nil {
		t.Fatalf("Encrypt2() failed: %v", err)
	}

	// Results should be different
	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("Different keys should produce different encrypted results")
	}

	// Decryption with wrong key should fail
	_, err = encryptor1.Decrypt(encrypted2)
	if err == nil {
		t.Error("Decryption with wrong key should fail")
	}
}

func TestNoEncryption(t *testing.T) {
	encryptor := NewNoEncryption()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "text data",
			data: []byte("hello world"),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt (should return same data)
			encrypted, err := encryptor.Encrypt(tt.data)
			if err != nil {
				t.Fatalf("Encrypt() failed: %v", err)
			}

			if !bytes.Equal(tt.data, encrypted) {
				t.Errorf("NoEncryption Encrypt() should return same data.\nOriginal:  %q\nEncrypted: %q", tt.data, encrypted)
			}

			// Decrypt (should return same data)
			decrypted, err := encryptor.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() failed: %v", err)
			}

			if !bytes.Equal(tt.data, decrypted) {
				t.Errorf("NoEncryption Decrypt() should return same data.\nOriginal:  %q\nDecrypted: %q", tt.data, decrypted)
			}
		})
	}

	// Check algorithm
	if encryptor.Algorithm() != "none" {
		t.Errorf("NoEncryption Algorithm() = %q, expected %q", encryptor.Algorithm(), "none")
	}
}

func BenchmarkAESGCM_Encrypt(b *testing.B) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		b.Fatalf("Failed to generate key: %v", err)
	}

	encryptor, err := NewAESGCM(key)
	if err != nil {
		b.Fatalf("Failed to create encryptor: %v", err)
	}

	data := make([]byte, 1024) // 1KB of data
	if _, err := rand.Read(data); err != nil {
		b.Fatalf("Failed to generate data: %v", err)
	}

	for b.Loop() {
		_, err := encryptor.Encrypt(data)
		if err != nil {
			b.Fatalf("Encrypt failed: %v", err)
		}
	}
}

func BenchmarkAESGCM_Decrypt(b *testing.B) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		b.Fatalf("Failed to generate key: %v", err)
	}

	encryptor, err := NewAESGCM(key)
	if err != nil {
		b.Fatalf("Failed to create encryptor: %v", err)
	}

	data := make([]byte, 1024) // 1KB of data
	if _, err := rand.Read(data); err != nil {
		b.Fatalf("Failed to generate data: %v", err)
	}

	encrypted, err := encryptor.Encrypt(data)
	if err != nil {
		b.Fatalf("Failed to encrypt test data: %v", err)
	}

	for b.Loop() {
		_, err := encryptor.Decrypt(encrypted)
		if err != nil {
			b.Fatalf("Decrypt failed: %v", err)
		}
	}
}
