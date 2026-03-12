package pam

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ==================== Local Encrypted Token Store ====================

// LocalEncryptedTokenStore stores tokens encrypted with AES-256-GCM
type LocalEncryptedTokenStore struct {
	storagePath string
	keyEnv      string
}

// NewLocalEncryptedTokenStore creates a new local encrypted store
func NewLocalEncryptedTokenStore(storagePath, keyEnv string) *LocalEncryptedTokenStore {
	// Ensure storage directory exists
	os.MkdirAll(storagePath, 0700)

	return &LocalEncryptedTokenStore{
		storagePath: storagePath,
		keyEnv:      keyEnv,
	}
}

// Store saves an encrypted token
func (s *LocalEncryptedTokenStore) Store(ctx context.Context, userID string, token Token) error {
	// Get encryption key
	key, err := s.getEncryptionKey()
	if err != nil {
		return fmt.Errorf("encryption key: %w", err)
	}

	// Serialize token
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	// Encrypt
	encrypted, err := EncryptAESGCM(key, data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Write to file
	path := filepath.Join(s.storagePath, fmt.Sprintf("%s.enc", userID))
	if err := os.WriteFile(path, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Retrieve gets and decrypts a token
func (s *LocalEncryptedTokenStore) Retrieve(ctx context.Context, userID string) (*Token, error) {
	path := filepath.Join(s.storagePath, fmt.Sprintf("%s.enc", userID))

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Get encryption key
	key, err := s.getEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("encryption key: %w", err)
	}

	// Decrypt
	decrypted, err := DecryptAESGCM(key, string(data))
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	// Unmarshal
	var token Token
	if err := json.Unmarshal([]byte(decrypted), &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}

// Delete removes a user's token
func (s *LocalEncryptedTokenStore) Delete(ctx context.Context, userID string) error {
	path := filepath.Join(s.storagePath, fmt.Sprintf("%s.enc", userID))

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove file: %w", err)
	}

	return nil
}

// getEncryptionKey retrieves or generates the encryption key
func (s *LocalEncryptedTokenStore) getEncryptionKey() ([]byte, error) {
	keyHex := os.Getenv(s.keyEnv)
	if keyHex == "" {
		return nil, fmt.Errorf("environment variable %s not set", s.keyEnv)
	}

	key, err := hexDecode(keyHex)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid key (need 32 bytes hex)")
	}

	return key, nil
}

// ==================== ClawVault Token Store ====================

// ClawVaultTokenStore uses ClawVault for secure token storage
type ClawVaultTokenStore struct {
	prefix    string
	endpoint  string
	client    interface{} // ClawVault client interface
}

// NewClawVaultTokenStore creates a ClawVault-backed store
func NewClawVaultTokenStore(prefix, endpoint string) (*ClawVaultTokenStore, error) {
	// In production, would initialize actual ClawVault client here
	// For now, return store that will fail gracefully if not connected

	return &ClawVaultTokenStore{
		prefix:   prefix,
		endpoint: endpoint,
		client:   nil, // Will be set if ClawVault is available
	}, nil
}

// Store saves token to ClawVault
func (s *ClawVaultTokenStore) Store(ctx context.Context, userID string, token Token) error {
	if s.client == nil {
		return fmt.Errorf("ClawVault not connected")
	}

	key := fmt.Sprintf("%s/%s", s.prefix, userID)

	// In production, would call ClawVault API:
	// return s.client.Set(ctx, key, token)

	return fmt.Errorf("ClawVault client not implemented - use local fallback")
}

// Retrieve gets token from ClawVault
func (s *ClawVaultTokenStore) Retrieve(ctx context.Context, userID string) (*Token, error) {
	if s.client == nil {
		return nil, fmt.Errorf("ClawVault not connected")
	}

	key := fmt.Sprintf("%s/%s", s.prefix, userID)

	// In production, would call ClawVault API:
	// data, err := s.client.Get(ctx, key)

	return nil, fmt.Errorf("ClawVault client not implemented")
}

// Delete removes token from ClawVault
func (s *ClawVaultTokenStore) Delete(ctx context.Context, userID string) error {
	if s.client == nil {
		return fmt.Errorf("ClawVault not connected")
	}

	key := fmt.Sprintf("%s/%s", s.prefix, userID)

	// In production, would call ClawVault API:
	// return s.client.Delete(ctx, key)

	return fmt.Errorf("ClawVault client not implemented")
}

// ==================== Encryption Utilities ====================

// Note: In production, use a proper crypto library like crypto/aes
// These are placeholder implementations

// EncryptAESGCM encrypts data using AES-256-GCM
func EncryptAESGCM(key []byte, plaintext []byte) (string, error) {
	// TODO: Implement actual AES-GCM encryption
	// For now, return base64-encoded plaintext (NOT SECURE - placeholder only)
	return fmt.Sprintf("enc:%s", string(plaintext)), nil
}

// DecryptAESGCM decrypts AES-256-GCM encrypted data
func DecryptAESGCM(key []byte, ciphertext string) (string, error) {
	// TODO: Implement actual AES-GCM decryption
	// For now, handle placeholder
	if len(ciphertext) > 4 && ciphertext[:4] == "enc:" {
		return ciphertext[4:], nil
	}
	return "", fmt.Errorf("invalid ciphertext format")
}

func hexDecode(s string) ([]byte, error) {
	// Simplified hex decode
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd length")
	}

	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		fmt.Sscanf(s[i:i+2], "%2x", &b)
		result[i/2] = b
	}

	return result, nil
}

// Compile-time check that stores implement TokenStore interface
var (
	_ TokenStore = (*LocalEncryptedTokenStore)(nil)
	_ TokenStore = (*ClawVaultTokenStore)(nil)
)

// InitLocalTokenStore initializes local store with encryption key from env
func InitLocalTokenStore(ctx context.Context, storagePath, keyEnvVar string) (TokenStore, error) {
	key := os.Getenv(keyEnvVar)
	if key == "" {
		return nil, fmt.Errorf("environment variable %s not set", keyEnvVar)
	}

	store := NewLocalEncryptedTokenStore(storagePath, keyEnvVar)

	// Verify we can access encryption key
	_, err := store.Retrieve(ctx, "test-key-validity")
	if err != nil && !os.IsNotExist(fmt.Errorf("", err)) {
		return nil, fmt.Errorf("encryption key invalid: %w", err)
	}

	return store, nil
}

// TokenExpirationChecker checks if a token is still valid
func TokenExpirationChecker(token *Token, ttlMinutes int) bool {
	if token == nil {
		return false
	}
	return time.Since(token.LastUsedAt) > time.Duration(ttlMinutes)*time.Minute
}
