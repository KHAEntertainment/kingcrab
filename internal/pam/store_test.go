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
	// Generate a valid 32-byte key
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plaintext := []byte("sensitive token data")

	t.Run("successful encryption and decryption", func(t *testing.T) {
		encrypted, err := EncryptAESGCM(key, plaintext)
		if err != nil {
			t.Fatalf("EncryptAESGCM() error = %v", err)
		}

		if encrypted == "" {
			t.Error("EncryptAESGCM() returned empty string")
		}

		decrypted, err := DecryptAESGCM(key, encrypted)
		if err != nil {
			t.Fatalf("DecryptAESGCM() error = %v", err)
		}

		if decrypted != string(plaintext) {
			t.Errorf("DecryptAESGCM() = %q, want %q", decrypted, string(plaintext))
		}
	})

	t.Run("invalid key length for encryption", func(t *testing.T) {
		invalidKey := make([]byte, 16) // Wrong size
		_, err := EncryptAESGCM(invalidKey, plaintext)
		if err == nil {
			t.Error("EncryptAESGCM() expected error for invalid key length, got nil")
		}
	})

	t.Run("invalid key length for decryption", func(t *testing.T) {
		invalidKey := make([]byte, 16)
		_, err := DecryptAESGCM(invalidKey, "dummy")
		if err == nil {
			t.Error("DecryptAESGCM() expected error for invalid key length, got nil")
		}
	})

	t.Run("decryption with wrong key fails", func(t *testing.T) {
		encrypted, _ := EncryptAESGCM(key, plaintext)

		wrongKey := make([]byte, 32)
		rand.Read(wrongKey)

		_, err := DecryptAESGCM(wrongKey, encrypted)
		if err == nil {
			t.Error("DecryptAESGCM() expected error with wrong key, got nil")
		}
	})

	t.Run("decryption of invalid ciphertext", func(t *testing.T) {
		_, err := DecryptAESGCM(key, "invalid json")
		if err == nil {
			t.Error("DecryptAESGCM() expected error for invalid ciphertext, got nil")
		}
	})

	t.Run("each encryption produces different ciphertext", func(t *testing.T) {
		encrypted1, _ := EncryptAESGCM(key, plaintext)
		encrypted2, _ := EncryptAESGCM(key, plaintext)

		if encrypted1 == encrypted2 {
			t.Error("EncryptAESGCM() should produce different ciphertexts due to random nonce")
		}
	})
}

func TestSanitizeUserID(t *testing.T) {
	tests := []struct {
		name   string
		userID string
	}{
		{
			name:   "normal user ID",
			userID: "tg:123456",
		},
		{
			name:   "user ID with special characters",
			userID: "../../../etc/passwd",
		},
		{
			name:   "user ID with slashes",
			userID: "../../secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeUserID(tt.userID)

			// Should be a hex string
			if _, err := hex.DecodeString(sanitized); err != nil {
				t.Errorf("sanitizeUserID() result is not valid hex: %v", err)
			}

			// Should be 64 characters (SHA256)
			if len(sanitized) != 64 {
				t.Errorf("sanitizeUserID() length = %d, want 64", len(sanitized))
			}

			// Should not contain path traversal characters
			if containsPathTraversal(sanitized) {
				t.Error("sanitizeUserID() result contains path traversal characters")
			}
		})
	}

	// Different inputs should produce different outputs
	t.Run("different inputs produce different outputs", func(t *testing.T) {
		id1 := sanitizeUserID("user1")
		id2 := sanitizeUserID("user2")

		if id1 == id2 {
			t.Error("sanitizeUserID() should produce different outputs for different inputs")
		}
	})
}

func containsPathTraversal(s string) bool {
	return filepath.Clean(s) != s || s != filepath.Base(s)
}

func TestLocalEncryptedTokenStore(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Generate encryption key
	key := make([]byte, 32)
	rand.Read(key)
	keyHex := hex.EncodeToString(key)

	keyEnv := "TEST_ENCRYPTION_KEY"
	os.Setenv(keyEnv, keyHex)
	defer os.Unsetenv(keyEnv)

	store, err := NewLocalEncryptedTokenStore(tempDir, keyEnv)
	if err != nil {
		t.Fatalf("NewLocalEncryptedTokenStore() error = %v", err)
	}

	ctx := context.Background()
	userID := "tg:123456"
	token := Token{
		Value:        "biometric-token-123",
		DeviceInfo:   "Test Device",
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local",
	}

	t.Run("store and retrieve token", func(t *testing.T) {
		err := store.Store(ctx, userID, token)
		if err != nil {
			t.Fatalf("Store() error = %v", err)
		}

		retrieved, err := store.Retrieve(ctx, userID)
		if err != nil {
			t.Fatalf("Retrieve() error = %v", err)
		}

		if retrieved == nil {
			t.Fatal("Retrieve() returned nil token")
		}

		if retrieved.Value != token.Value {
			t.Errorf("Token value = %v, want %v", retrieved.Value, token.Value)
		}
		if retrieved.DeviceInfo != token.DeviceInfo {
			t.Errorf("DeviceInfo = %v, want %v", retrieved.DeviceInfo, token.DeviceInfo)
		}
	})

	t.Run("retrieve non-existent token", func(t *testing.T) {
		retrieved, err := store.Retrieve(ctx, "nonexistent")
		if err != nil {
			t.Errorf("Retrieve() error = %v, want nil", err)
		}
		if retrieved != nil {
			t.Error("Retrieve() should return nil for non-existent token")
		}
	})

	t.Run("delete token", func(t *testing.T) {
		userID2 := "tg:789"
		store.Store(ctx, userID2, token)

		err := store.Delete(ctx, userID2)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		retrieved, _ := store.Retrieve(ctx, userID2)
		if retrieved != nil {
			t.Error("Token should be deleted")
		}
	})

	t.Run("delete non-existent token", func(t *testing.T) {
		err := store.Delete(ctx, "nonexistent")
		if err != nil {
			t.Errorf("Delete() error = %v, want nil for non-existent token", err)
		}
	})

	t.Run("missing encryption key", func(t *testing.T) {
		os.Unsetenv(keyEnv)
		defer os.Setenv(keyEnv, keyHex)

		err := store.Store(ctx, "test", token)
		if err == nil {
			t.Error("Store() expected error when encryption key missing, got nil")
		}
	})

	t.Run("invalid encryption key", func(t *testing.T) {
		os.Setenv(keyEnv, "invalid-hex")
		defer os.Setenv(keyEnv, keyHex)

		err := store.Store(ctx, "test", token)
		if err == nil {
			t.Error("Store() expected error for invalid encryption key, got nil")
		}
	})
}

func TestNewLocalEncryptedTokenStore_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "nested", "path", "tokens")

	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "TEST_KEY_2"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	_, err := NewLocalEncryptedTokenStore(storagePath, keyEnv)
	if err != nil {
		t.Fatalf("NewLocalEncryptedTokenStore() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		t.Error("NewLocalEncryptedTokenStore() did not create storage directory")
	}
}

func TestInitLocalTokenStore(t *testing.T) {
	tempDir := t.TempDir()
	keyEnv := "TEST_INIT_KEY"

	t.Run("successful initialization", func(t *testing.T) {
		key := make([]byte, 32)
		rand.Read(key)
		os.Setenv(keyEnv, hex.EncodeToString(key))
		defer os.Unsetenv(keyEnv)

		store, err := InitLocalTokenStore(context.Background(), tempDir, keyEnv)
		if err != nil {
			t.Fatalf("InitLocalTokenStore() error = %v", err)
		}

		if store == nil {
			t.Error("InitLocalTokenStore() returned nil store")
		}
	})

	t.Run("missing environment variable", func(t *testing.T) {
		os.Unsetenv(keyEnv)

		_, err := InitLocalTokenStore(context.Background(), tempDir, keyEnv)
		if err == nil {
			t.Error("InitLocalTokenStore() expected error when env var missing, got nil")
		}
	})

	t.Run("invalid key format", func(t *testing.T) {
		os.Setenv(keyEnv, "not-valid-hex")
		defer os.Unsetenv(keyEnv)

		_, err := InitLocalTokenStore(context.Background(), tempDir, keyEnv)
		if err == nil {
			t.Error("InitLocalTokenStore() expected error for invalid key, got nil")
		}
	})

	t.Run("key wrong length", func(t *testing.T) {
		shortKey := make([]byte, 16)
		rand.Read(shortKey)
		os.Setenv(keyEnv, hex.EncodeToString(shortKey))
		defer os.Unsetenv(keyEnv)

		_, err := InitLocalTokenStore(context.Background(), tempDir, keyEnv)
		if err == nil {
			t.Error("InitLocalTokenStore() expected error for wrong key length, got nil")
		}
	})
}

func TestTokenExpirationChecker(t *testing.T) {
	tests := []struct {
		name       string
		token      *Token
		ttlMinutes int
		want       bool
	}{
		{
			name: "token still valid",
			token: &Token{
				LastUsedAt: time.Now().Add(-2 * time.Minute),
			},
			ttlMinutes: 5,
			want:       true,
		},
		{
			name: "token expired",
			token: &Token{
				LastUsedAt: time.Now().Add(-10 * time.Minute),
			},
			ttlMinutes: 5,
			want:       false,
		},
		{
			name: "token just at boundary",
			token: &Token{
				LastUsedAt: time.Now().Add(-5 * time.Minute),
			},
			ttlMinutes: 5,
			want:       false,
		},
		{
			name:       "nil token",
			token:      nil,
			ttlMinutes: 5,
			want:       false,
		},
		{
			name: "zero TTL",
			token: &Token{
				LastUsedAt: time.Now(),
			},
			ttlMinutes: 0,
			want:       false,
		},
		{
			name: "negative TTL",
			token: &Token{
				LastUsedAt: time.Now(),
			},
			ttlMinutes: -1,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenExpirationChecker(tt.token, tt.ttlMinutes)
			if got != tt.want {
				t.Errorf("TokenExpirationChecker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHexDecode(t *testing.T) {
	t.Run("valid hex string", func(t *testing.T) {
		input := "48656c6c6f" // "Hello"
		result, err := hexDecode(input)
		if err != nil {
			t.Fatalf("hexDecode() error = %v", err)
		}

		expected := []byte("Hello")
		if string(result) != string(expected) {
			t.Errorf("hexDecode() = %v, want %v", result, expected)
		}
	})

	t.Run("invalid hex string", func(t *testing.T) {
		input := "not-hex"
		_, err := hexDecode(input)
		if err == nil {
			t.Error("hexDecode() expected error for invalid hex, got nil")
		}
	})

	t.Run("odd length hex string", func(t *testing.T) {
		input := "abc" // Odd length
		_, err := hexDecode(input)
		if err == nil {
			t.Error("hexDecode() expected error for odd length, got nil")
		}
	})
}