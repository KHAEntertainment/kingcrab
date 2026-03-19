package pam

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
func NewLocalEncryptedTokenStore(storagePath, keyEnv string) (*LocalEncryptedTokenStore, error) {
	// Ensure storage directory exists
	if err := os.MkdirAll(storagePath, 0700); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	return &LocalEncryptedTokenStore{
		storagePath: storagePath,
		keyEnv:      keyEnv,
	}, nil
}

// sanitizeUserID prevents path traversal by hashing the userID
func sanitizeUserID(userID string) string {
	h := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(h[:])
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
	path := filepath.Join(s.storagePath, fmt.Sprintf("%s.enc", sanitizeUserID(userID)))
	if err := os.WriteFile(path, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Retrieve gets and decrypts a token
func (s *LocalEncryptedTokenStore) Retrieve(ctx context.Context, userID string) (*Token, error) {
	path := filepath.Join(s.storagePath, fmt.Sprintf("%s.enc", sanitizeUserID(userID)))

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
	path := filepath.Join(s.storagePath, fmt.Sprintf("%s.enc", sanitizeUserID(userID)))

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

// ==================== Encryption Utilities ====================

// EncryptedData represents the format stored on disk
// Format: nonce (12 bytes) + ciphertext + tag (16 bytes), all hex-encoded
type EncryptedData struct {
	Nonce    string `json:"nonce"`
	Cipher   string `json:"cipher"`
	KeyID    string `json:"key_id,omitempty"`
}

// EncryptAESGCM encrypts data using AES-256-GCM
// Returns hex-encoded nonce + ciphertext
func EncryptAESGCM(key []byte, plaintext []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("key must be 32 bytes")
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// Generate random nonce (12 bytes)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Split nonce + ciphertext (nonce is first 12 bytes)
	nonceHex := hex.EncodeToString(nonce)
	cipherHex := hex.EncodeToString(ciphertext[12:])

	// Return as JSON for future extensibility
	encData := EncryptedData{
		Nonce: nonceHex,
		Cipher: cipherHex,
	}
	result, err := json.Marshal(encData)
	if err != nil {
		return "", fmt.Errorf("marshal encrypted data: %w", err)
	}

	return string(result), nil
}

// DecryptAESGCM decrypts AES-256-GCM encrypted data
func DecryptAESGCM(key []byte, ciphertext string) (string, error) {
	if len(key) != 32 {
		return "", errors.New("key must be 32 bytes")
	}

	// Parse encrypted data
	var encData EncryptedData
	if err := json.Unmarshal([]byte(ciphertext), &encData); err != nil {
		return "", fmt.Errorf("parse encrypted data: %w", err)
	}

	// Decode hex
	nonce, err := hex.DecodeString(encData.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	cipherText, err := hex.DecodeString(encData.Cipher)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// Decrypt (appends tag to ciphertext internally)
	plaintext, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// hexDecode decodes a hex string
func hexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// Compile-time check that stores implement TokenStore interface
var (
	_ TokenStore = (*LocalEncryptedTokenStore)(nil)
	_ TokenStore = (*ClawVaultTokenStore)(nil)
)

// InitLocalTokenStore initializes local store with encryption key from env
func InitLocalTokenStore(ctx context.Context, storagePath, keyEnvVar string) (TokenStore, error) {
	// Validate the key exists and is valid before creating store
	keyHex := os.Getenv(keyEnvVar)
	if keyHex == "" {
		return nil, fmt.Errorf("environment variable %s not set", keyEnvVar)
	}

	key, err := hexDecode(keyHex)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key (need 32 bytes hex): %w", err)
	}

	store, err := NewLocalEncryptedTokenStore(storagePath, keyEnvVar)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	return store, nil
}

// TokenExpirationChecker checks if a token is still valid (not expired)
func TokenExpirationChecker(token *Token, ttlMinutes int) bool {
	if token == nil || ttlMinutes <= 0 {
		return false
	}
	return time.Since(token.LastUsedAt) <= time.Duration(ttlMinutes)*time.Minute
}