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

	if cfg.UseClawVault != "auto" {
		t.Errorf("UseClawVault = %s, want 'auto'", cfg.UseClawVault)
	}

	if cfg.ClawVault.TokenPrefix != "kingcrab/pam/tokens" {
		t.Errorf("TokenPrefix = %s, want 'kingcrab/pam/tokens'", cfg.ClawVault.TokenPrefix)
	}

	if cfg.ClawVault.TimeoutSec != 5 {
		t.Errorf("TimeoutSec = %d, want 5", cfg.ClawVault.TimeoutSec)
	}

	if cfg.Fallback.TTLMinutes != 5 {
		t.Errorf("TTLMinutes = %d, want 5", cfg.Fallback.TTLMinutes)
	}
}

func TestLoadPAMConfig(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		jsonData := `{
			"use_clawvault": "true",
			"clawvault": {
				"token_prefix": "custom/prefix",
				"timeout_seconds": 10
			},
			"fallback": {
				"ttl_minutes": 15
			}
		}`

		cfg, err := LoadPAMConfig([]byte(jsonData))
		if err != nil {
			t.Fatalf("LoadPAMConfig() error = %v", err)
		}

		if cfg.UseClawVault != "true" {
			t.Errorf("UseClawVault = %s, want 'true'", cfg.UseClawVault)
		}

		if cfg.ClawVault.TokenPrefix != "custom/prefix" {
			t.Errorf("TokenPrefix = %s, want 'custom/prefix'", cfg.ClawVault.TokenPrefix)
		}

		if cfg.ClawVault.TimeoutSec != 10 {
			t.Errorf("TimeoutSec = %d, want 10", cfg.ClawVault.TimeoutSec)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := LoadPAMConfig([]byte("not json"))
		if err == nil {
			t.Error("LoadPAMConfig() expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty JSON uses defaults", func(t *testing.T) {
		cfg, err := LoadPAMConfig([]byte("{}"))
		if err != nil {
			t.Fatalf("LoadPAMConfig() error = %v", err)
		}

		// Should have default values
		if cfg.UseClawVault != "auto" {
			t.Errorf("UseClawVault = %s, want 'auto'", cfg.UseClawVault)
		}
	})
}

func TestNewPAM_FallbackMode(t *testing.T) {
	// Setup encryption key
	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "TEST_PAM_FALLBACK_KEY"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	tempDir := t.TempDir()

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tempDir,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("NewPAM() error = %v", err)
	}

	if pam.clawvaultMode.Available {
		t.Error("ClawVault should not be available when UseClawVault=false")
	}

	status := pam.GetStatus()
	if status["mode"] != "disabled" {
		t.Errorf("Mode = %v, want 'disabled'", status["mode"])
	}
}

func TestNewPAM_NilConfig(t *testing.T) {
	// Setup key for fallback
	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "KINGCRAB_ENCRYPTION_KEY"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	// Should use defaults
	_, err := NewPAM(nil)
	// May error if HOME not set or other issues, but shouldn't panic
	if err != nil {
		t.Logf("NewPAM(nil) error (expected in test env): %v", err)
	}
}

func TestPAM_StoreRetrieveDelete(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "TEST_PAM_STORE_KEY"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	tempDir := t.TempDir()

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tempDir,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("NewPAM() error = %v", err)
	}

	ctx := context.Background()
	userID := "tg:123456"
	token := Token{
		Value:        "test-biometric-token",
		DeviceInfo:   "Test Device",
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local",
	}

	t.Run("store token", func(t *testing.T) {
		err := pam.StoreToken(ctx, userID, token)
		if err != nil {
			t.Fatalf("StoreToken() error = %v", err)
		}
	})

	t.Run("retrieve token", func(t *testing.T) {
		retrieved, err := pam.RetrieveToken(ctx, userID)
		if err != nil {
			t.Fatalf("RetrieveToken() error = %v", err)
		}

		if retrieved == nil {
			t.Fatal("RetrieveToken() returned nil")
		}

		if retrieved.Value != token.Value {
			t.Errorf("Token value = %s, want %s", retrieved.Value, token.Value)
		}
	})

	t.Run("delete token", func(t *testing.T) {
		err := pam.DeleteToken(ctx, userID)
		if err != nil {
			t.Fatalf("DeleteToken() error = %v", err)
		}

		retrieved, _ := pam.RetrieveToken(ctx, userID)
		if retrieved != nil {
			t.Error("Token should be deleted")
		}
	})

	t.Run("retrieve non-existent token", func(t *testing.T) {
		retrieved, err := pam.RetrieveToken(ctx, "nonexistent")
		if err != nil {
			t.Errorf("RetrieveToken() error = %v, want nil", err)
		}
		if retrieved != nil {
			t.Error("RetrieveToken() should return nil for non-existent token")
		}
	})
}

func TestPAM_GetStatus(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "TEST_PAM_STATUS_KEY"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	tempDir := t.TempDir()

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tempDir,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("NewPAM() error = %v", err)
	}

	status := pam.GetStatus()

	if _, ok := status["mode"]; !ok {
		t.Error("Status should contain 'mode' key")
	}

	if _, ok := status["clawvault_found"]; !ok {
		t.Error("Status should contain 'clawvault_found' key")
	}

	if _, ok := status["endpoint"]; !ok {
		t.Error("Status should contain 'endpoint' key")
	}

	clawvaultFound, ok := status["clawvault_found"].(bool)
	if ok && clawvaultFound {
		t.Log("ClawVault found in test environment (unexpected but ok)")
	}
}

func TestCheckPort(t *testing.T) {
	// Test with known closed port
	t.Run("closed port", func(t *testing.T) {
		result := checkPort("127.0.0.1", "65534")
		if result {
			t.Log("Port 65534 is open (unexpected, may have service running)")
		}
	})

	t.Run("invalid host", func(t *testing.T) {
		result := checkPort("invalid.host.that.does.not.exist", "80")
		if result {
			t.Error("checkPort() should return false for invalid host")
		}
	})
}

func TestPAM_DetectClawVault(t *testing.T) {
	config := &PAMConfig{
		UseClawVault: "auto",
	}

	pam := &PAM{
		config: config,
	}

	status := pam.detectClawVault()

	// Should return a ClawVaultStatus (may or may not be available)
	t.Logf("ClawVault detection: Available=%v, Mode=%s, Endpoint=%s",
		status.Available, status.Mode, status.Endpoint)

	// Test that it doesn't panic
	if status.Available {
		t.Logf("ClawVault is available on this system")
	} else {
		t.Logf("ClawVault is not available on this system (expected in most test envs)")
	}
}

func TestPAM_ConcurrentAccess(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "TEST_PAM_CONCURRENT_KEY"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	tempDir := t.TempDir()

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tempDir,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("NewPAM() error = %v", err)
	}

	ctx := context.Background()

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := "tg:" + string(rune(id))
			token := Token{
				Value:      "token",
				DeviceInfo: "device",
				EnrolledAt: time.Now(),
				LastUsedAt: time.Now(),
			}
			pam.StoreToken(ctx, userID, token)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := "tg:" + string(rune(id))
			pam.RetrieveToken(ctx, userID)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPAM_TokenSerialization(t *testing.T) {
	token := Token{
		Value:        "test-token",
		DeviceInfo:   "iPhone 15",
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local",
	}

	// Test JSON marshaling
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Test JSON unmarshaling
	var decoded Token
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Value != token.Value {
		t.Errorf("Value = %s, want %s", decoded.Value, token.Value)
	}

	if decoded.DeviceInfo != token.DeviceInfo {
		t.Errorf("DeviceInfo = %s, want %s", decoded.DeviceInfo, token.DeviceInfo)
	}
}