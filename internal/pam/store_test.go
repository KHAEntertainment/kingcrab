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

func TestEncryptDecryptAESGCM(t *testing.T) {
	// Generate a 32-byte key
	key := make([]byte, 32)
	rand.Read(key)

	plaintext := []byte("test secret data")

	t.Run("successful encryption and decryption", func(t *testing.T) {
		encrypted, err := EncryptAESGCM(key, plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		if encrypted == "" {
			t.Fatal("encrypted data is empty")
		}

		decrypted, err := DecryptAESGCM(key, encrypted)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if decrypted != string(plaintext) {
			t.Errorf("decrypted data doesn't match: expected %s, got %s", plaintext, decrypted)
		}
	})

	t.Run("invalid key length for encryption", func(t *testing.T) {
		shortKey := make([]byte, 16)
		_, err := EncryptAESGCM(shortKey, plaintext)
		if err == nil {
			t.Fatal("expected error for invalid key length")
		}
	})

	t.Run("invalid key length for decryption", func(t *testing.T) {
		encrypted, _ := EncryptAESGCM(key, plaintext)
		shortKey := make([]byte, 16)
		_, err := DecryptAESGCM(shortKey, encrypted)
		if err == nil {
			t.Fatal("expected error for invalid key length")
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		encrypted, _ := EncryptAESGCM(key, plaintext)
		tampered := encrypted[:len(encrypted)-10] + "tampered"
		_, err := DecryptAESGCM(key, tampered)
		if err == nil {
			t.Fatal("expected error for tampered ciphertext")
		}
	})

	t.Run("different nonces produce different ciphertexts", func(t *testing.T) {
		encrypted1, _ := EncryptAESGCM(key, plaintext)
		encrypted2, _ := EncryptAESGCM(key, plaintext)

		if encrypted1 == encrypted2 {
			t.Error("expected different ciphertexts due to different nonces")
		}
	})
}

func TestSanitizeUserID(t *testing.T) {
	t.Run("consistent output for same input", func(t *testing.T) {
		userID := "test-user"
		hash1 := sanitizeUserID(userID)
		hash2 := sanitizeUserID(userID)

		if hash1 != hash2 {
			t.Error("expected consistent hash for same input")
		}
	})

	t.Run("different output for different input", func(t *testing.T) {
		hash1 := sanitizeUserID("user1")
		hash2 := sanitizeUserID("user2")

		if hash1 == hash2 {
			t.Error("expected different hashes for different inputs")
		}
	})

	t.Run("prevents path traversal", func(t *testing.T) {
		maliciousID := "../../../etc/passwd"
		hash := sanitizeUserID(maliciousID)

		// Hash should not contain path separators
		if filepath.IsAbs(hash) || filepath.Dir(hash) != "." {
			t.Error("sanitized ID should not be a path")
		}
	})

	t.Run("output is hex string", func(t *testing.T) {
		userID := "test-user"
		hash := sanitizeUserID(userID)

		// Should be valid hex
		_, err := hex.DecodeString(hash)
		if err != nil {
			t.Errorf("expected valid hex string, got error: %v", err)
		}
	})
}

func TestLocalEncryptedTokenStore(t *testing.T) {
	// Create temporary directory for tests
	tmpDir := t.TempDir()
	keyEnv := "TEST_ENCRYPTION_KEY"

	// Generate and set encryption key
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	t.Run("store and retrieve token", func(t *testing.T) {
		store, err := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		ctx := context.Background()
		userID := "test-user"
		token := Token{
			Value:        "biometric-token",
			DeviceInfo:   "iPhone 13",
			EnrolledAt:   time.Now(),
			LastUsedAt:   time.Now(),
			TokenStorage: "local",
		}

		// Store token
		err = store.Store(ctx, userID, token)
		if err != nil {
			t.Fatalf("failed to store token: %v", err)
		}

		// Retrieve token
		retrieved, err := store.Retrieve(ctx, userID)
		if err != nil {
			t.Fatalf("failed to retrieve token: %v", err)
		}

		if retrieved == nil {
			t.Fatal("expected token, got nil")
		}

		if retrieved.Value != token.Value {
			t.Errorf("expected token value %s, got %s", token.Value, retrieved.Value)
		}

		if retrieved.DeviceInfo != token.DeviceInfo {
			t.Errorf("expected device info %s, got %s", token.DeviceInfo, retrieved.DeviceInfo)
		}
	})

	t.Run("retrieve non-existent token", func(t *testing.T) {
		store, _ := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
		ctx := context.Background()

		retrieved, err := store.Retrieve(ctx, "non-existent-user")
		if err != nil {
			t.Fatalf("expected no error for non-existent token, got: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil for non-existent token")
		}
	})

	t.Run("delete token", func(t *testing.T) {
		store, _ := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
		ctx := context.Background()
		userID := "delete-test-user"

		token := Token{Value: "test-token"}
		store.Store(ctx, userID, token)

		// Delete
		err := store.Delete(ctx, userID)
		if err != nil {
			t.Fatalf("failed to delete token: %v", err)
		}

		// Verify deleted
		retrieved, _ := store.Retrieve(ctx, userID)
		if retrieved != nil {
			t.Error("expected token to be deleted")
		}
	})

	t.Run("delete non-existent token", func(t *testing.T) {
		store, _ := NewLocalEncryptedTokenStore(tmpDir, keyEnv)
		ctx := context.Background()

		err := store.Delete(ctx, "non-existent")
		if err != nil {
			t.Errorf("expected no error deleting non-existent token, got: %v", err)
		}
	})

	t.Run("missing encryption key", func(t *testing.T) {
		badKeyEnv := "NONEXISTENT_KEY_ENV"
		store, _ := NewLocalEncryptedTokenStore(tmpDir, badKeyEnv)
		ctx := context.Background()

		err := store.Store(ctx, "test", Token{Value: "test"})
		if err == nil {
			t.Fatal("expected error for missing encryption key")
		}
	})

	t.Run("invalid encryption key", func(t *testing.T) {
		badKeyEnv := "BAD_KEY_ENV"
		os.Setenv(badKeyEnv, "not-valid-hex")
		defer os.Unsetenv(badKeyEnv)

		store, _ := NewLocalEncryptedTokenStore(tmpDir, badKeyEnv)
		ctx := context.Background()

		err := store.Store(ctx, "test", Token{Value: "test"})
		if err == nil {
			t.Fatal("expected error for invalid encryption key")
		}
	})
}

func TestInitLocalTokenStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	t.Run("successful initialization", func(t *testing.T) {
		keyEnv := "TEST_INIT_KEY"
		key := make([]byte, 32)
		rand.Read(key)
		os.Setenv(keyEnv, hex.EncodeToString(key))
		defer os.Unsetenv(keyEnv)

		store, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if store == nil {
			t.Fatal("expected store, got nil")
		}
	})

	t.Run("missing key environment variable", func(t *testing.T) {
		keyEnv := "MISSING_KEY_ENV"
		_, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)
		if err == nil {
			t.Fatal("expected error for missing key env var")
		}
	})

	t.Run("invalid key format", func(t *testing.T) {
		keyEnv := "INVALID_KEY_ENV"
		os.Setenv(keyEnv, "not-hex")
		defer os.Unsetenv(keyEnv)

		_, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)
		if err == nil {
			t.Fatal("expected error for invalid key format")
		}
	})

	t.Run("key too short", func(t *testing.T) {
		keyEnv := "SHORT_KEY_ENV"
		shortKey := make([]byte, 16) // Only 16 bytes instead of 32
		rand.Read(shortKey)
		os.Setenv(keyEnv, hex.EncodeToString(shortKey))
		defer os.Unsetenv(keyEnv)

		_, err := InitLocalTokenStore(ctx, tmpDir, keyEnv)
		if err == nil {
			t.Fatal("expected error for short key")
		}
	})
}

func TestTokenExpirationChecker(t *testing.T) {
	t.Run("token is valid", func(t *testing.T) {
		token := &Token{
			LastUsedAt: time.Now().Add(-2 * time.Minute),
		}

		if !TokenExpirationChecker(token, 5) {
			t.Error("expected token to be valid")
		}
	})

	t.Run("token is expired", func(t *testing.T) {
		token := &Token{
			LastUsedAt: time.Now().Add(-10 * time.Minute),
		}

		if TokenExpirationChecker(token, 5) {
			t.Error("expected token to be expired")
		}
	})

	t.Run("nil token", func(t *testing.T) {
		if TokenExpirationChecker(nil, 5) {
			t.Error("expected nil token to be invalid")
		}
	})

	t.Run("zero TTL", func(t *testing.T) {
		token := &Token{
			LastUsedAt: time.Now(),
		}

		if TokenExpirationChecker(token, 0) {
			t.Error("expected false for zero TTL")
		}
	})

	t.Run("negative TTL", func(t *testing.T) {
		token := &Token{
			LastUsedAt: time.Now(),
		}

		if TokenExpirationChecker(token, -1) {
			t.Error("expected false for negative TTL")
		}
	})

	t.Run("token used just now", func(t *testing.T) {
		token := &Token{
			LastUsedAt: time.Now(),
		}

		if !TokenExpirationChecker(token, 5) {
			t.Error("expected token used just now to be valid")
		}
	})
}

func TestHexDecode(t *testing.T) {
	t.Run("valid hex string", func(t *testing.T) {
		hex := "48656c6c6f" // "Hello"
		decoded, err := hexDecode(hex)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if string(decoded) != "Hello" {
			t.Errorf("expected 'Hello', got %s", string(decoded))
		}
	})

	t.Run("invalid hex string", func(t *testing.T) {
		_, err := hexDecode("not-hex")
		if err == nil {
			t.Fatal("expected error for invalid hex")
		}
	})

	t.Run("odd length hex string", func(t *testing.T) {
		_, err := hexDecode("abc")
		if err == nil {
			t.Fatal("expected error for odd length hex")
		}
	})
}