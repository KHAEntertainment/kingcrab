package pam

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestKey creates a test encryption key
func generateTestKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}

func TestEncryptAESGCM_Success(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	plaintext := []byte("test secret data")

	encrypted, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if encrypted == "" {
		t.Fatal("Encrypted data is empty")
	}

	// Verify it's different from plaintext
	if encrypted == string(plaintext) {
		t.Error("Encrypted data should differ from plaintext")
	}
}

func TestEncryptAESGCM_InvalidKeySize(t *testing.T) {
	key := make([]byte, 16) // Wrong size
	plaintext := []byte("test")

	_, err := EncryptAESGCM(key, plaintext)
	if err == nil {
		t.Fatal("Expected error for invalid key size")
	}

	if err.Error() != "key must be 32 bytes" {
		t.Errorf("Expected 'key must be 32 bytes', got: %v", err)
	}
}

func TestDecryptAESGCM_Success(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	original := []byte("test secret message")

	// Encrypt
	encrypted, err := EncryptAESGCM(key, original)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Decrypt
	decrypted, err := DecryptAESGCM(key, encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != string(original) {
		t.Errorf("Decrypted data doesn't match original.\nExpected: %s\nGot: %s", original, decrypted)
	}
}

func TestDecryptAESGCM_InvalidKeySize(t *testing.T) {
	key := make([]byte, 16) // Wrong size
	ciphertext := "dummy"

	_, err := DecryptAESGCM(key, ciphertext)
	if err == nil {
		t.Fatal("Expected error for invalid key size")
	}
}

func TestDecryptAESGCM_InvalidData(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	_, err := DecryptAESGCM(key, "invalid json data")
	if err == nil {
		t.Fatal("Expected error for invalid encrypted data")
	}
}

func TestDecryptAESGCM_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	plaintext := []byte("secret")

	encrypted, err := EncryptAESGCM(key1, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with wrong key
	_, err = DecryptAESGCM(key2, encrypted)
	if err == nil {
		t.Fatal("Expected error when decrypting with wrong key")
	}
}

func TestSanitizeUserID(t *testing.T) {
	tests := []struct {
		name   string
		userID string
	}{
		{"normal user", "user123"},
		{"with slash", "../../../etc/passwd"},
		{"with dots", "..."},
		{"special chars", "user@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeUserID(tt.userID)

			// Should be hex-encoded SHA256 (64 chars)
			if len(result) != 64 {
				t.Errorf("Expected 64 char hash, got %d", len(result))
			}

			// Should not contain path separators
			if filepath.IsAbs(result) {
				t.Error("Sanitized ID should not be absolute path")
			}

			// Should be consistent
			result2 := sanitizeUserID(tt.userID)
			if result != result2 {
				t.Error("Sanitization not consistent")
			}
		})
	}
}

func TestLocalEncryptedTokenStore_StoreAndRetrieve(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_ENCRYPTION_KEY"
	key := generateTestKey()
	os.Setenv(keyEnv, key)
	defer os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	userID := "test_user_123"
	token := Token{
		Value:        "biometric_token_xyz",
		DeviceInfo:   "iPhone 13",
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local",
	}

	// Store
	err = store.Store(ctx, userID, token)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Retrieve
	retrieved, err := store.Retrieve(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to retrieve token: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved token is nil")
	}

	if retrieved.Value != token.Value {
		t.Errorf("Expected token value %s, got %s", token.Value, retrieved.Value)
	}

	if retrieved.DeviceInfo != token.DeviceInfo {
		t.Errorf("Expected device info %s, got %s", token.DeviceInfo, retrieved.DeviceInfo)
	}
}

func TestLocalEncryptedTokenStore_RetrieveNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_ENCRYPTION_KEY"
	key := generateTestKey()
	os.Setenv(keyEnv, key)
	defer os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	retrieved, err := store.Retrieve(ctx, "nonexistent_user")

	if err != nil {
		t.Fatalf("Expected no error for nonexistent user, got: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil for nonexistent user")
	}
}

func TestLocalEncryptedTokenStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_ENCRYPTION_KEY"
	key := generateTestKey()
	os.Setenv(keyEnv, key)
	defer os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	userID := "test_user_delete"
	token := Token{
		Value:      "token123",
		DeviceInfo: "Test Device",
	}

	// Store
	err = store.Store(ctx, userID, token)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Delete
	err = store.Delete(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}

	// Verify deleted
	retrieved, err := store.Retrieve(ctx, userID)
	if err != nil {
		t.Fatalf("Expected no error after delete, got: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil after delete")
	}
}

func TestLocalEncryptedTokenStore_DeleteNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_ENCRYPTION_KEY"
	key := generateTestKey()
	os.Setenv(keyEnv, key)
	defer os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()

	// Should not error when deleting nonexistent
	err = store.Delete(ctx, "nonexistent_user")
	if err != nil {
		t.Errorf("Expected no error when deleting nonexistent, got: %v", err)
	}
}

func TestLocalEncryptedTokenStore_NoEncryptionKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "MISSING_KEY_ENV"
	os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	token := Token{Value: "test"}

	err = store.Store(ctx, "user", token)
	if err == nil {
		t.Fatal("Expected error when encryption key not set")
	}
}

func TestLocalEncryptedTokenStore_InvalidKeyFormat(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "INVALID_KEY_ENV"
	os.Setenv(keyEnv, "not_valid_hex")
	defer os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	token := Token{Value: "test"}

	err = store.Store(ctx, "user", token)
	if err == nil {
		t.Fatal("Expected error for invalid key format")
	}
}

func TestInitLocalTokenStore_Success(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_INIT_KEY"
	key := generateTestKey()
	os.Setenv(keyEnv, key)
	defer os.Unsetenv(keyEnv)

	ctx := context.Background()
	store, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if store == nil {
		t.Fatal("Expected store, got nil")
	}
}

func TestInitLocalTokenStore_NoKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "MISSING_INIT_KEY"
	os.Unsetenv(keyEnv)

	ctx := context.Background()
	_, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)

	if err == nil {
		t.Fatal("Expected error when key not set")
	}
}

func TestInitLocalTokenStore_InvalidKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "INVALID_INIT_KEY"
	os.Setenv(keyEnv, "short") // Too short
	defer os.Unsetenv(keyEnv)

	ctx := context.Background()
	_, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)

	if err == nil {
		t.Fatal("Expected error for invalid key")
	}
}

func TestTokenExpirationChecker(t *testing.T) {
	tests := []struct {
		name       string
		token      *Token
		ttlMinutes int
		expected   bool
	}{
		{
			name: "valid token",
			token: &Token{
				LastUsedAt: time.Now().Add(-2 * time.Minute),
			},
			ttlMinutes: 5,
			expected:   true,
		},
		{
			name: "expired token",
			token: &Token{
				LastUsedAt: time.Now().Add(-10 * time.Minute),
			},
			ttlMinutes: 5,
			expected:   false,
		},
		{
			name:       "nil token",
			token:      nil,
			ttlMinutes: 5,
			expected:   false,
		},
		{
			name: "zero ttl",
			token: &Token{
				LastUsedAt: time.Now(),
			},
			ttlMinutes: 0,
			expected:   false,
		},
		{
			name: "negative ttl",
			token: &Token{
				LastUsedAt: time.Now(),
			},
			ttlMinutes: -1,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenExpirationChecker(tt.token, tt.ttlMinutes)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHexDecode(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{"valid hex", "48656c6c6f", false},
		{"empty", "", false},
		{"invalid hex", "not hex", true},
		{"odd length", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hexDecode(tt.input)
			if tt.shouldError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// Test that different plaintexts produce different ciphertexts
func TestEncryptAESGCM_Uniqueness(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	plaintext1 := []byte("message 1")
	plaintext2 := []byte("message 2")

	encrypted1, _ := EncryptAESGCM(key, plaintext1)
	encrypted2, _ := EncryptAESGCM(key, plaintext2)

	if encrypted1 == encrypted2 {
		t.Error("Different plaintexts should produce different ciphertexts")
	}

	// Even same plaintext should produce different ciphertext due to random nonce
	encrypted3, _ := EncryptAESGCM(key, plaintext1)
	if encrypted1 == encrypted3 {
		t.Error("Same plaintext encrypted twice should produce different ciphertext (random nonce)")
	}
}