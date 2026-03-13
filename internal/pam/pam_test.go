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

func TestNewPAM(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	t.Run("with default config", func(t *testing.T) {
		config := &PAMConfig{
			UseClawVault: "false", // Disable ClawVault for testing
			Fallback: FallbackConfig{
				StoragePath:      tmpDir,
				EncryptionKeyEnv: keyEnv,
			},
		}

		pam, err := NewPAM(config)
		if err != nil {
			t.Fatalf("failed to create PAM: %v", err)
		}

		if pam == nil {
			t.Fatal("expected PAM instance, got nil")
		}

		if pam.store == nil {
			t.Error("expected store to be initialized")
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		// Should use defaults, but may fail without proper key
		_, err := NewPAM(nil)
		// Error is expected since no encryption key is set in default config
		if err == nil {
			t.Log("NewPAM with nil config succeeded (ClawVault may be available)")
		}
	})
}

func TestDefaultPAMConfig(t *testing.T) {
	config := DefaultPAMConfig()

	if config == nil {
		t.Fatal("expected config, got nil")
	}

	if config.UseClawVault != "auto" {
		t.Errorf("expected UseClawVault 'auto', got %s", config.UseClawVault)
	}

	if config.ClawVault.TokenPrefix != "kingcrab/pam/tokens" {
		t.Errorf("expected default token prefix, got %s", config.ClawVault.TokenPrefix)
	}

	if config.ClawVault.TimeoutSec != 5 {
		t.Errorf("expected timeout 5, got %d", config.ClawVault.TimeoutSec)
	}

	if config.Fallback.TTLMinutes != 5 {
		t.Errorf("expected TTL 5, got %d", config.Fallback.TTLMinutes)
	}
}

func TestLoadPAMConfig(t *testing.T) {
	t.Run("valid JSON config", func(t *testing.T) {
		jsonData := []byte(`{
			"use_clawvault": "true",
			"clawvault": {
				"token_prefix": "test/tokens",
				"timeout_seconds": 10
			},
			"fallback": {
				"ttl_minutes": 15
			}
		}`)

		config, err := LoadPAMConfig(jsonData)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if config.UseClawVault != "true" {
			t.Errorf("expected UseClawVault 'true', got %s", config.UseClawVault)
		}

		if config.ClawVault.TokenPrefix != "test/tokens" {
			t.Errorf("expected token prefix 'test/tokens', got %s", config.ClawVault.TokenPrefix)
		}

		if config.ClawVault.TimeoutSec != 10 {
			t.Errorf("expected timeout 10, got %d", config.ClawVault.TimeoutSec)
		}

		if config.Fallback.TTLMinutes != 15 {
			t.Errorf("expected TTL 15, got %d", config.Fallback.TTLMinutes)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		jsonData := []byte(`{invalid json}`)

		_, err := LoadPAMConfig(jsonData)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("partial config uses defaults", func(t *testing.T) {
		jsonData := []byte(`{"use_clawvault": "false"}`)

		config, err := LoadPAMConfig(jsonData)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		// Should merge with defaults
		if config.Fallback.TTLMinutes != 5 {
			t.Error("expected default TTL to be preserved")
		}
	})
}

func TestPAMStoreToken(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_STORE_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			StoragePath:      tmpDir,
			EncryptionKeyEnv: keyEnv,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("failed to create PAM: %v", err)
	}

	ctx := context.Background()
	userID := "test-user"
	token := Token{
		Value:        "biometric-token-123",
		DeviceInfo:   "iPhone 14",
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local",
	}

	t.Run("store and retrieve token", func(t *testing.T) {
		err := pam.StoreToken(ctx, userID, token)
		if err != nil {
			t.Fatalf("failed to store token: %v", err)
		}

		retrieved, err := pam.RetrieveToken(ctx, userID)
		if err != nil {
			t.Fatalf("failed to retrieve token: %v", err)
		}

		if retrieved == nil {
			t.Fatal("expected token, got nil")
		}

		if retrieved.Value != token.Value {
			t.Errorf("expected value %s, got %s", token.Value, retrieved.Value)
		}
	})

	t.Run("delete token", func(t *testing.T) {
		// Store first
		pam.StoreToken(ctx, userID, token)

		// Delete
		err := pam.DeleteToken(ctx, userID)
		if err != nil {
			t.Fatalf("failed to delete token: %v", err)
		}

		// Verify deleted
		retrieved, _ := pam.RetrieveToken(ctx, userID)
		if retrieved != nil {
			t.Error("expected token to be deleted")
		}
	})
}

func TestPAMGetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_PAM_STATUS_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			StoragePath:      tmpDir,
			EncryptionKeyEnv: keyEnv,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("failed to create PAM: %v", err)
	}

	status := pam.GetStatus()

	if status == nil {
		t.Fatal("expected status, got nil")
	}

	mode, ok := status["mode"]
	if !ok {
		t.Error("expected 'mode' in status")
	}

	if mode != "disabled" && mode != "fallback" {
		t.Logf("mode is %v (may be using ClawVault)", mode)
	}

	// Should have these keys
	if _, ok := status["clawvault_found"]; !ok {
		t.Error("expected 'clawvault_found' in status")
	}

	if _, ok := status["endpoint"]; !ok {
		t.Error("expected 'endpoint' in status")
	}
}

func TestCheckPort(t *testing.T) {
	t.Run("invalid host", func(t *testing.T) {
		// Should return false for unreachable host
		result := checkPort("192.0.2.1", "9999")
		if result {
			t.Error("expected false for unreachable host")
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		result := checkPort("localhost", "99999")
		if result {
			t.Error("expected false for invalid port")
		}
	})
}

func TestUser(t *testing.T) {
	user := User{
		TelegramID: 12345,
		Name:       "Test User",
	}

	// Test JSON marshaling
	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal user: %v", err)
	}

	var unmarshaled User
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal user: %v", err)
	}

	if unmarshaled.TelegramID != user.TelegramID {
		t.Error("TelegramID mismatch after JSON round-trip")
	}

	if unmarshaled.Name != user.Name {
		t.Error("Name mismatch after JSON round-trip")
	}
}

func TestToken(t *testing.T) {
	now := time.Now()
	token := Token{
		Value:        "test-token",
		DeviceInfo:   "Test Device",
		EnrolledAt:   now,
		LastUsedAt:   now,
		TokenStorage: "local",
	}

	// Test JSON marshaling
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("failed to marshal token: %v", err)
	}

	var unmarshaled Token
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal token: %v", err)
	}

	if unmarshaled.Value != token.Value {
		t.Error("Value mismatch after JSON round-trip")
	}

	if unmarshaled.DeviceInfo != token.DeviceInfo {
		t.Error("DeviceInfo mismatch after JSON round-trip")
	}

	if unmarshaled.TokenStorage != token.TokenStorage {
		t.Error("TokenStorage mismatch after JSON round-trip")
	}
}

func TestClawVaultStatus(t *testing.T) {
	status := ClawVaultStatus{
		Available:     true,
		Mode:          "socket",
		Endpoint:      "/tmp/test.sock",
		HasPAMSupport: true,
	}

	if !status.Available {
		t.Error("expected Available to be true")
	}

	if status.Mode != "socket" {
		t.Errorf("expected mode 'socket', got %s", status.Mode)
	}

	if status.Endpoint != "/tmp/test.sock" {
		t.Errorf("expected endpoint '/tmp/test.sock', got %s", status.Endpoint)
	}
}