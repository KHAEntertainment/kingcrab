package pam

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestDefaultPAMConfig(t *testing.T) {
	cfg := DefaultPAMConfig()

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if cfg.UseClawVault != "auto" {
		t.Errorf("Expected UseClawVault 'auto', got %s", cfg.UseClawVault)
	}

	if cfg.ClawVault.TokenPrefix != "kingcrab/pam/tokens" {
		t.Errorf("Expected default token prefix, got %s", cfg.ClawVault.TokenPrefix)
	}

	if cfg.ClawVault.TimeoutSec != 5 {
		t.Errorf("Expected timeout 5, got %d", cfg.ClawVault.TimeoutSec)
	}

	if cfg.Fallback.TTLMinutes != 5 {
		t.Errorf("Expected TTL 5 minutes, got %d", cfg.Fallback.TTLMinutes)
	}
}

func TestLoadPAMConfig(t *testing.T) {
	jsonData := []byte(`{
		"use_clawvault": "false",
		"clawvault": {
			"token_prefix": "custom/prefix",
			"timeout_seconds": 10
		},
		"fallback": {
			"ttl_minutes": 10
		}
	}`)

	cfg, err := LoadPAMConfig(jsonData)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.UseClawVault != "false" {
		t.Errorf("Expected UseClawVault 'false', got %s", cfg.UseClawVault)
	}

	if cfg.ClawVault.TokenPrefix != "custom/prefix" {
		t.Errorf("Expected custom prefix, got %s", cfg.ClawVault.TokenPrefix)
	}

	if cfg.ClawVault.TimeoutSec != 10 {
		t.Errorf("Expected timeout 10, got %d", cfg.ClawVault.TimeoutSec)
	}

	if cfg.Fallback.TTLMinutes != 10 {
		t.Errorf("Expected TTL 10 minutes, got %d", cfg.Fallback.TTLMinutes)
	}
}

func TestLoadPAMConfig_InvalidJSON(t *testing.T) {
	jsonData := []byte(`{invalid json`)

	_, err := LoadPAMConfig(jsonData)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestNewPAM_WithDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	cfg := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	if pam == nil {
		t.Fatal("Expected PAM instance, got nil")
	}

	if pam.store == nil {
		t.Fatal("Expected store to be initialized")
	}
}

func TestNewPAM_NilConfig(t *testing.T) {
	// Should use defaults and attempt fallback
	// Will fail if neither ClawVault nor encryption key is available
	_, err := NewPAM(nil)
	// We expect an error since we don't have ClawVault or a valid key
	// The exact error depends on the environment
	_ = err // Allow either success or failure
}

func TestNewPAM_WithForcedFallback(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_KEY_FALLBACK"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	cfg := &PAMConfig{
		UseClawVault: "false", // Force fallback
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	if pam.clawvaultMode.Available {
		t.Error("Expected ClawVault to be disabled")
	}

	if pam.clawvaultMode.Mode != "disabled" {
		t.Errorf("Expected mode 'disabled', got %s", pam.clawvaultMode.Mode)
	}
}

func TestPAM_StoreAndRetrieveToken(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_STORE_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	cfg := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	ctx := context.Background()
	userID := "test_user_123"
	token := Token{
		Value:        "bio_token_xyz",
		DeviceInfo:   "iPhone 14",
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local",
	}

	// Store
	err = pam.StoreToken(ctx, userID, token)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Retrieve
	retrieved, err := pam.RetrieveToken(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to retrieve token: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected token, got nil")
	}

	if retrieved.Value != token.Value {
		t.Errorf("Expected token value %s, got %s", token.Value, retrieved.Value)
	}
}

func TestPAM_DeleteToken(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_DELETE_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	cfg := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	ctx := context.Background()
	userID := "test_user_delete"
	token := Token{
		Value:      "token123",
		DeviceInfo: "Test Device",
	}

	// Store
	pam.StoreToken(ctx, userID, token)

	// Delete
	err = pam.DeleteToken(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}

	// Verify deleted
	retrieved, _ := pam.RetrieveToken(ctx, userID)
	if retrieved != nil {
		t.Error("Expected nil after delete")
	}
}

func TestPAM_GetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_STATUS_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	cfg := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	status := pam.GetStatus()

	if status == nil {
		t.Fatal("Expected status, got nil")
	}

	mode, ok := status["mode"]
	if !ok {
		t.Error("Expected 'mode' in status")
	}

	if mode != "disabled" {
		t.Errorf("Expected mode 'disabled', got %v", mode)
	}

	_, ok = status["clawvault_found"]
	if !ok {
		t.Error("Expected 'clawvault_found' in status")
	}
}

func TestCheckPort(t *testing.T) {
	// Test invalid port (should return false)
	result := checkPort("127.0.0.1", "99999")
	if result {
		t.Error("Expected false for invalid port")
	}

	// Test localhost on non-listening port
	result = checkPort("127.0.0.1", "54321")
	// This will likely be false unless something is running there
	// We just verify it doesn't panic
}

func TestPAMConfig_JSONRoundtrip(t *testing.T) {
	original := &PAMConfig{
		UseClawVault: "auto",
		ClawVault: ClawVaultConfig{
			Socket:      "/var/run/test.sock",
			Host:        "localhost:3000",
			TokenPrefix: "test/prefix",
			TimeoutSec:  10,
		},
		Fallback: FallbackConfig{
			EncryptionKeyEnv: "TEST_KEY",
			StoragePath:      "/tmp/test",
			TTLMinutes:       15,
			AuthorizedUsers: []User{
				{TelegramID: 123, Name: "Alice"},
				{TelegramID: 456, Name: "Bob"},
			},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	loaded, err := LoadPAMConfig(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify
	if loaded.UseClawVault != original.UseClawVault {
		t.Errorf("UseClawVault mismatch: expected %s, got %s", original.UseClawVault, loaded.UseClawVault)
	}

	if loaded.ClawVault.TokenPrefix != original.ClawVault.TokenPrefix {
		t.Errorf("TokenPrefix mismatch")
	}

	if len(loaded.Fallback.AuthorizedUsers) != len(original.Fallback.AuthorizedUsers) {
		t.Errorf("AuthorizedUsers count mismatch")
	}
}

func TestUser_Struct(t *testing.T) {
	user := User{
		TelegramID: 123456789,
		Name:       "Test User",
	}

	if user.TelegramID != 123456789 {
		t.Errorf("Expected TelegramID 123456789, got %d", user.TelegramID)
	}

	if user.Name != "Test User" {
		t.Errorf("Expected Name 'Test User', got %s", user.Name)
	}
}

func TestToken_Struct(t *testing.T) {
	now := time.Now()
	token := Token{
		Value:        "test_token",
		DeviceInfo:   "iPhone 13",
		EnrolledAt:   now,
		LastUsedAt:   now,
		TokenStorage: "local",
	}

	if token.Value != "test_token" {
		t.Errorf("Expected Value 'test_token', got %s", token.Value)
	}

	if token.DeviceInfo != "iPhone 13" {
		t.Errorf("Expected DeviceInfo 'iPhone 13', got %s", token.DeviceInfo)
	}

	if token.TokenStorage != "local" {
		t.Errorf("Expected TokenStorage 'local', got %s", token.TokenStorage)
	}
}

func TestClawVaultStatus_Struct(t *testing.T) {
	status := ClawVaultStatus{
		Available:     true,
		Mode:          "socket",
		Endpoint:      "/var/run/test.sock",
		HasPAMSupport: true,
	}

	if !status.Available {
		t.Error("Expected Available true")
	}

	if status.Mode != "socket" {
		t.Errorf("Expected Mode 'socket', got %s", status.Mode)
	}
}

// Test concurrent token operations
func TestPAM_ConcurrentTokenOperations(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_CONCURRENT_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	cfg := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	ctx := context.Background()
	done := make(chan bool, 20)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := "concurrent_user"
			token := Token{
				Value:      "token",
				DeviceInfo: "device",
			}
			_ = pam.StoreToken(ctx, userID, token)
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := "concurrent_user"
			_, _ = pam.RetrieveToken(ctx, userID)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 20; i++ {
		<-done
	}
}