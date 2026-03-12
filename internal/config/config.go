package config

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/KHAEntertainment/kingcrab/internal/pam"
)

// Config holds KingCrab configuration
type Config struct {
	Version            string          `json:"version"`
	Listen            ListenConfig    `json:"listen"`
	AllowedCommands   []string        `json:"allowedCommands"`
	RequireReason     bool            `json:"requireReason"`
	AutoApproveTimeout int            `json:"autoApproveTimeout"`
	LogDir            string          `json:"logDir"`
	DataDir           string          `json:"dataDir"`
	Port              int             `json:"port"`
	SocketPath        string          `json:"socketPath"`
	LogLevel          string          `json:"logLevel"`

	// Telegram bot for approvals
	Telegram          *TelegramConfig `json:"telegram"`

	// PAM (Privileged Access Management) config
	PAM               *pam.PAMConfig  `json:"pam"`
}

type ListenConfig struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Port int    `json:"port"`
}

// TelegramConfig for the approval bot
type TelegramConfig struct {
	BotToken      string  `json:"botToken"`
	AllowedUsers  []int64 `json:"allowedUsers"`
	WebhookURL    string  `json:"webhookUrl"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Version: "0.1.0",
		Listen: ListenConfig{
			Type: "unix",
			Path: "/var/run/kingcrab.sock",
		},
		AllowedCommands: []string{
			"apt install *",
			"apt update",
			"systemctl restart *",
			"systemctl start *",
			"systemctl stop *",
			"systemctl status *",
		},
		RequireReason:     true,
		AutoApproveTimeout: 0,
		LogDir:            "/var/log/kingcrab",
		DataDir:           "/var/lib/kingcrab",
		Port:              8080,
		SocketPath:        "/var/run/kingcrab.sock",
		LogLevel:          "info",
		Telegram: &TelegramConfig{
			AllowedUsers: []int64{}, // Empty = no restrictions (but require enrollment)
		},
		PAM: pam.DefaultPAMConfig(),
	}
}

// Load loads config from file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Config file doesn't exist, return defaults
		return cfg, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse JSON (allow unknown fields)
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply env overrides
	if port := os.Getenv("KINGCRAB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 && p < 65536 {
			cfg.Port = p
		}
	}

	// Ensure socket path is set
	if cfg.SocketPath == "" && cfg.Listen.Path != "" {
		cfg.SocketPath = cfg.Listen.Path
	}

	return cfg, nil
}
