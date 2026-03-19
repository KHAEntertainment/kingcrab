package pam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TokenStore defines the interface for storing biometric tokens
type TokenStore interface {
	Store(ctx context.Context, userID string, token Token) error
	Retrieve(ctx context.Context, userID string) (*Token, error)
	Delete(ctx context.Context, userID string) error
}

// Token represents an enrolled biometric token
type Token struct {
	Value           string    `json:"token"`
	DeviceInfo      string    `json:"device_info"`
	EnrolledAt      time.Time `json:"enrolled_at"`
	LastUsedAt      time.Time `json:"last_used_at"`
	TokenStorage    string    `json:"token_storage"` // "local" or "clawvault"
}

// PAMConfig holds PAM-specific configuration
type PAMConfig struct {
	// UseClawVault: auto | true | false
	UseClawVault string `json:"use_clawvault"`

	// ClawVault-specific settings
	ClawVault ClawVaultConfig `json:"clawvault"`

	// Fallback settings (when ClawVault unavailable)
	Fallback FallbackConfig `json:"fallback"`
}

// ClawVaultConfig for ClawVault integration
type ClawVaultConfig struct {
	Socket       string `json:"socket"`        // Socket path
	Host         string `json:"host"`          // Network endpoint
	TokenPrefix  string `json:"token_prefix"` // Key prefix for tokens
	TimeoutSec   int    `json:"timeout_seconds"`
}

// FallbackConfig for local encrypted storage
type FallbackConfig struct {
	EncryptionKeyEnv string `json:"encryption_key_env"` // Env var for key
	StoragePath      string `json:"storage_path"`        // Where to store encrypted tokens
	TTLMinutes       int    `json:"ttl_minutes"`
	AuthorizedUsers  []User `json:"authorized_users"`
}

// User represents an authorized user
type User struct {
	TelegramID int64  `json:"telegram_id"`
	Name       string `json:"name"`
}

// PAM represents the PAM subsystem
type PAM struct {
	store         TokenStore
	config        *PAMConfig
	clawvaultMode ClawVaultStatus
	mu            sync.RWMutex
}

// ClawVaultStatus indicates ClawVault availability
type ClawVaultStatus struct {
	Available     bool
	Mode          string // "socket", "network", "tailscale"
	Endpoint      string
	HasPAMSupport bool
	Error         error
}

// NewPAM creates a new PAM instance
func NewPAM(config *PAMConfig) (*PAM, error) {
	if config == nil {
		config = DefaultPAMConfig()
	}

	pam := &PAM{
		config: config,
	}

	// Detect and initialize token store
	store, mode, err := pam.detectAndInitStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token store: %w", err)
	}

	pam.store = store
	pam.clawvaultMode = mode

	return pam, nil
}

// detectAndInitStore determines which store to use
func (p *PAM) detectAndInitStore() (TokenStore, ClawVaultStatus, error) {
	mode := p.detectClawVault()

	// Check config override
	if p.config.UseClawVault == "false" {
		store, err := p.newFallbackStore()
		if err != nil {
			return nil, ClawVaultStatus{Available: false, Mode: "disabled"}, err
		}
		return store, ClawVaultStatus{Available: false, Mode: "disabled"}, nil
	}

	if p.config.UseClawVault == "true" && !mode.Available {
		return nil, mode, errors.New("ClawVault required but not available")
	}

	if mode.Available {
		// Use ClawVault store
		store := NewClawVaultTokenStore(p.config.ClawVault.TokenPrefix, p.config.ClawVault.TimeoutSec)
		
		// Verify availability
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := store.CheckAvailability(ctx); err != nil {
			// If explicitly required, fail; otherwise fallback
			if p.config.UseClawVault == "true" {
				mode.Error = err
				return nil, mode, fmt.Errorf("clawvault required but availability check failed: %w", err)
			}
			store, fallbackErr := p.newFallbackStore()
			if fallbackErr != nil {
				return nil, ClawVaultStatus{Available: false, Mode: "fallback", Error: err}, fallbackErr
			}
			return store, ClawVaultStatus{Available: false, Mode: "fallback", Error: err}, nil
		}
		
		return store, mode, nil
	}

	// Fallback to local
	store, err := p.newFallbackStore()
	if err != nil {
		return nil, ClawVaultStatus{Available: false, Mode: "fallback"}, err
	}
	return store, ClawVaultStatus{Available: false, Mode: "fallback"}, nil
}

// newFallbackStore creates local encrypted storage
func (p *PAM) newFallbackStore() (TokenStore, error) {
	storagePath := p.config.Fallback.StoragePath
	if storagePath == "" {
		storagePath = filepath.Join(os.Getenv("HOME"), ".config", "kingcrab", "tokens")
	}
	return NewLocalEncryptedTokenStore(storagePath, p.config.Fallback.EncryptionKeyEnv)
}

// detectClawVault checks for ClawVault availability
func (p *PAM) detectClawVault() ClawVaultStatus {
	// Primary check: secret-tool (used by ClawVault on Linux)
	// This is the underlying mechanism ClawVault uses

	// Check if secret-tool is available (works on Linux with GNOME Keyring)
	client := NewClawVaultClient(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.CheckAvailability(ctx); err == nil {
		return ClawVaultStatus{
			Available: true,
			Mode:      "secret-tool",
			Endpoint:  "gnome-keyring",
		}
	}

	// Check configured socket first (if provided)
	if p.config.ClawVault.Socket != "" {
		if _, err := os.Stat(p.config.ClawVault.Socket); err == nil {
			return ClawVaultStatus{
				Available: true,
				Mode:      "socket",
				Endpoint:  p.config.ClawVault.Socket,
			}
		}
	}

	// Check configured host first (if provided)
	if p.config.ClawVault.Host != "" {
		host := p.config.ClawVault.Host
		// Simple TCP check - try to reach the endpoint
		if strings.HasPrefix(host, "http://") {
			host = strings.TrimPrefix(host, "http://")
		}
		if strings.HasPrefix(host, "https://") {
			host = strings.TrimPrefix(host, "https://")
		}
		if strings.Contains(host, ":") {
			port := strings.Split(host, ":")[1]
			if checkPort(strings.Split(host, ":")[0], port) {
				return ClawVaultStatus{
					Available: true,
					Mode:      "network",
					Endpoint:  p.config.ClawVault.Host,
				}
			}
		}
	}

	// Fallback: Check for socket (legacy/local ClawVault)
	socketPaths := []string{
		"/var/run/clawvault.sock",
		filepath.Join(os.Getenv("HOME"), ".clawvault", "clawvault.sock"),
	}

	for _, path := range socketPaths {
		if _, err := os.Stat(path); err == nil {
			return ClawVaultStatus{
				Available: true,
				Mode:     "socket",
				Endpoint: path,
			}
		}
	}

	// Fallback: Check network endpoints
	hosts := []string{
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3001",
	}

	for _, host := range hosts {
		// Simple TCP check - try to reach the endpoint
		if strings.HasPrefix(host, "http://") {
			host = strings.TrimPrefix(host, "http://")
		}
		if strings.Contains(host, ":") {
			port := strings.Split(host, ":")[1]
			if checkPort(strings.Split(host, ":")[0], port) {
				return ClawVaultStatus{
					Available: true,
					Mode:      "network",
					Endpoint:  "http://" + host,
				}
			}
		}
	}

	// Check Tailscale
	if tsIP := os.Getenv("CLAWVAULT_TAILSCALE_IP"); tsIP != "" {
		if checkPort(tsIP, "3000") {
			return ClawVaultStatus{
				Available: true,
				Mode:      "tailscale",
				Endpoint:  "http://" + tsIP + ":3000",
			}
		}
	}

	return ClawVaultStatus{Available: false}
}

// checkPort attempts a TCP connection to check if a port is open
func checkPort(host, port string) bool {
	address := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// StoreToken stores a biometric token for a user
func (p *PAM) StoreToken(ctx context.Context, userID string, token Token) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.Store(ctx, userID, token)
}

// RetrieveToken retrieves a user's biometric token
func (p *PAM) RetrieveToken(ctx context.Context, userID string) (*Token, error) {
	p.mu.RLock()
	token, err := p.store.Retrieve(ctx, userID)
	p.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	if token == nil {
		return nil, nil
	}

	// Check if token has expired using configured TTL
	ttl := p.config.Fallback.TTLMinutes
	if ttl > 0 && !TokenExpirationChecker(token, ttl) {
		// Token has expired, delete it
		p.mu.Lock()
		deleteErr := p.store.Delete(ctx, userID)
		p.mu.Unlock()

		if deleteErr != nil {
			return nil, fmt.Errorf("token expired and failed to delete: %w", deleteErr)
		}

		return nil, fmt.Errorf("token expired (TTL: %d minutes)", ttl)
	}

	return token, nil
}

// DeleteToken removes a user's biometric token
func (p *PAM) DeleteToken(ctx context.Context, userID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.Delete(ctx, userID)
}

// GetStatus returns PAM status info
func (p *PAM) GetStatus() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"mode":            p.clawvaultMode.Mode,
		"clawvault_found": p.clawvaultMode.Available,
		"endpoint":        p.clawvaultMode.Endpoint,
	}
}

// DefaultPAMConfig returns sensible defaults
func DefaultPAMConfig() *PAMConfig {
	return &PAMConfig{
		UseClawVault: "auto",
		ClawVault: ClawVaultConfig{
			TokenPrefix: "kingcrab/pam/tokens",
			TimeoutSec:  5,
		},
		Fallback: FallbackConfig{
			EncryptionKeyEnv: "PAM_FALLBACK_ENCRYPTION_KEY",
			TTLMinutes:       5,
		},
	}
}

// LoadPAMConfig loads PAM config from JSON
func LoadPAMConfig(data []byte) (*PAMConfig, error) {
	cfg := DefaultPAMConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}