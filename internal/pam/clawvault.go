package pam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ==================== ClawVault Client ====================

// ClawVaultClient wraps clawvault CLI commands
type ClawVaultClient struct {
	timeout time.Duration
}

// NewClawVaultClient creates a new ClawVault client
func NewClawVaultClient(timeoutSec int) *ClawVaultClient {
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	return &ClawVaultClient{
		timeout: time.Duration(timeoutSec) * time.Second,
	}
}

// Secret represents a ClawVault secret (metadata)
type ClawVaultSecret struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
}

// ListSecrets lists all stored secrets (metadata only)
func (c *ClawVaultClient) ListSecrets(ctx context.Context) ([]ClawVaultSecret, error) {
	cmd := exec.CommandContext(ctx, "clawvault", "list", "--json")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("clawvault list failed: %w - %s", err, stderr.String())
	}

	// Parse JSON output
	var secrets []ClawVaultSecret
	if err := json.Unmarshal(stdout.Bytes(), &secrets); err != nil {
		return nil, fmt.Errorf("parse secrets: %w", err)
	}

	return secrets, nil
}

// GetSecret retrieves a secret value by name
// Note: clawvault doesn't have a non-interactive get command, so we use a workaround
// This requires clawvault to be configured with the exec-provider pattern
func (c *ClawVaultClient) GetSecret(ctx context.Context, key string) (string, error) {
	// Try using clawvault resolve (exec-provider pattern)
	cmd := exec.CommandContext(ctx, "clawvault", "resolve", key)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("clawvault resolve failed: %w - %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// SetSecret stores a secret (interactive, requires stdin)
// For non-interactive use, we store directly to keyring via secret-tool
func (c *ClawVaultClient) SetSecret(ctx context.Context, key, value string) error {
	// Use secret-tool directly (what clawvault uses under the hood)
	// This avoids the interactive prompt
	cmd := exec.CommandContext(ctx, "secret-tool", "store",
		"--label", fmt.Sprintf("KingCrab: %s", key),
		"kingcrab", key)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	go func() {
		defer stdin.Close()
		stdin.Write([]byte(value))
	}()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("secret-tool store failed: %w - %s", err, stderr.String())
	}

	return nil
}

// DeleteSecret removes a secret
func (c *ClawVaultClient) DeleteSecret(ctx context.Context, key string) error {
	cmd := exec.CommandContext(ctx, "secret-tool", "clear", "kingcrab", key)
	

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("secret-tool delete failed: %w - %s", err, stderr.String())
	}

	return nil
}

// SearchSecret checks if a secret exists
func (c *ClawVaultClient) SearchSecret(ctx context.Context, key string) (bool, error) {
	cmd := exec.CommandContext(ctx, "secret-tool", "search", "kingcrab", key)
	

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Exit code 0 = found, non-zero = not found or error
		if strings.Contains(stderr.String(), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("secret-tool search failed: %w - %s", err, stderr.String())
	}

	return true, nil
}

// CheckAvailability tests if clawvault/secret-tool is available
func (c *ClawVaultClient) CheckAvailability(ctx context.Context) error {
	// Check for secret-tool (works on Linux with GNOME Keyring)
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return fmt.Errorf("secret-tool not found: %w", err)
	}

	// Also check for clawvault binary (needed for GetSecret/resolve)
	if _, err := exec.LookPath("clawvault"); err != nil {
		return fmt.Errorf("clawvault not found: %w", err)
	}

	return nil
}

// ==================== ClawVault Token Store ====================

// ClawVaultTokenStore uses ClawVault for secure token storage
type ClawVaultTokenStore struct {
	prefix   string
	client   *ClawVaultClient
	timeout  time.Duration
}

// NewClawVaultTokenStore creates a ClawVault-backed store
func NewClawVaultTokenStore(prefix string, timeoutSec int) *ClawVaultTokenStore {
	return &ClawVaultTokenStore{
		prefix:  prefix,
		client:  NewClawVaultClient(timeoutSec),
		timeout: time.Duration(timeoutSec) * time.Second,
	}
}

// Store saves token to ClawVault
func (s *ClawVaultTokenStore) Store(ctx context.Context, userID string, token Token) error {
	key := fmt.Sprintf("%s/%s", s.prefix, userID)

	// Serialize token to JSON
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	// Store via secret-tool
	if err := s.client.SetSecret(ctx, key, string(data)); err != nil {
		return fmt.Errorf("set secret: %w", err)
	}

	return nil
}

// Retrieve gets token from ClawVault
func (s *ClawVaultTokenStore) Retrieve(ctx context.Context, userID string) (*Token, error) {
	key := fmt.Sprintf("%s/%s", s.prefix, userID)

	// Try secret-tool first (direct keyring access)
	data, err := s.client.GetSecret(ctx, key)
	if err != nil {
		// Fall back to clawvault resolve
		data, err = s.client.GetSecret(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get secret: %w", err)
		}
	}

	if data == "" {
		return nil, nil
	}

	// Unmarshal token
	var token Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}

// Delete removes token from ClawVault
func (s *ClawVaultTokenStore) Delete(ctx context.Context, userID string) error {
	key := fmt.Sprintf("%s/%s", s.prefix, userID)

	if err := s.client.DeleteSecret(ctx, key); err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}

	return nil
}

// CheckAvailability tests if ClawVault is available
func (s *ClawVaultTokenStore) CheckAvailability(ctx context.Context) error {
	return s.client.CheckAvailability(ctx)
}
