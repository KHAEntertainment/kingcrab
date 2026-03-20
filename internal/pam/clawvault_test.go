package pam

import (
	"context"
	"testing"
	"time"
)

func TestNewClawVaultClient(t *testing.T) {
	client := NewClawVaultClient(10)
	if client == nil {
		t.Fatal("Expected client, got nil")
	}

	if client.timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", client.timeout)
	}
}

func TestNewClawVaultClient_ZeroTimeout(t *testing.T) {
	client := NewClawVaultClient(0)
	if client == nil {
		t.Fatal("Expected client, got nil")
	}

	// Should default to 5 seconds
	if client.timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", client.timeout)
	}
}

func TestNewClawVaultClient_NegativeTimeout(t *testing.T) {
	client := NewClawVaultClient(-1)
	if client == nil {
		t.Fatal("Expected client, got nil")
	}

	// Should default to 5 seconds
	if client.timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", client.timeout)
	}
}

func TestNewClawVaultTokenStore(t *testing.T) {
	store := NewClawVaultTokenStore("test/prefix", 10)
	if store == nil {
		t.Fatal("Expected store, got nil")
	}

	if store.prefix != "test/prefix" {
		t.Errorf("Expected prefix 'test/prefix', got %s", store.prefix)
	}

	if store.timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", store.timeout)
	}

	if store.client == nil {
		t.Error("Expected client to be initialized")
	}
}

// Test that ClawVaultTokenStore implements TokenStore interface
func TestClawVaultTokenStore_ImplementsInterface(t *testing.T) {
	var _ TokenStore = (*ClawVaultTokenStore)(nil)
}

// Test CheckAvailability (will fail if tools not installed)
func TestClawVaultClient_CheckAvailability(t *testing.T) {
	client := NewClawVaultClient(2)
	ctx := context.Background()

	err := client.CheckAvailability(ctx)
	// Will likely fail unless clawvault and secret-tool are installed
	// We just verify it doesn't panic
	_ = err
}

// Test ClawVault operations (will fail without actual tools)
func TestClawVaultClient_Operations_NoTools(t *testing.T) {
	client := NewClawVaultClient(1)
	ctx := context.Background()

	// These will fail if tools aren't installed, but verify they don't panic

	_, err := client.ListSecrets(ctx)
	_ = err // Expected to fail

	_, err = client.GetSecret(ctx, "test-key")
	_ = err // Expected to fail

	err = client.SetSecret(ctx, "test-key", "test-value")
	_ = err // Expected to fail

	err = client.DeleteSecret(ctx, "test-key")
	_ = err // Expected to fail

	_, err = client.SearchSecret(ctx, "test-key")
	_ = err // Expected to fail
}

func TestClawVaultTokenStore_Operations_NoTools(t *testing.T) {
	store := NewClawVaultTokenStore("test/prefix", 1)
	ctx := context.Background()

	token := Token{
		Value:      "test-token",
		DeviceInfo: "test-device",
	}

	// These will fail if tools aren't installed
	err := store.Store(ctx, "user123", token)
	_ = err // Expected to fail

	_, err = store.Retrieve(ctx, "user123")
	_ = err // Expected to fail

	err = store.Delete(ctx, "user123")
	_ = err // Expected to fail

	err = store.CheckAvailability(ctx)
	_ = err // Expected to fail
}

// Test ClawVaultSecret structure
func TestClawVaultSecret_Structure(t *testing.T) {
	secret := ClawVaultSecret{
		Name:        "my-secret",
		Description: "Test secret",
		Provider:    "test-provider",
	}

	if secret.Name != "my-secret" {
		t.Errorf("Expected name 'my-secret', got %s", secret.Name)
	}

	if secret.Description != "Test secret" {
		t.Errorf("Expected description 'Test secret', got %s", secret.Description)
	}

	if secret.Provider != "test-provider" {
		t.Errorf("Expected provider 'test-provider', got %s", secret.Provider)
	}
}

// Test token key formatting
func TestClawVaultTokenStore_KeyFormat(t *testing.T) {
	store := NewClawVaultTokenStore("my/prefix", 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to avoid actual execution

	// Attempt to store (will fail fast due to cancelled context)
	token := Token{Value: "test"}
	_ = store.Store(ctx, "user123", token)

	// The key would be "my/prefix/user123" but we can't verify without mocking
}

// Test timeout contexts
func TestClawVaultClient_ContextTimeout(t *testing.T) {
	client := NewClawVaultClient(1)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should fail fast
	_, err := client.ListSecrets(ctx)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

// Verify the token store can be created with zero timeout
func TestNewClawVaultTokenStore_ZeroTimeout(t *testing.T) {
	store := NewClawVaultTokenStore("prefix", 0)
	if store == nil {
		t.Fatal("Expected store, got nil")
	}

	// Client should have default timeout
	if store.client.timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", store.client.timeout)
	}
}