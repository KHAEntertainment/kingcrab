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

	// Additional configuration sections
	Auth              *AuthConfig      `json:"auth,omitempty"`
	Telegram          *TelegramConfig  `json:"telegram,omitempty"`
	Audit             *AuditConfig     `json:"audit,omitempty"`
	RateLimit         *RateLimitConfig `json:"rateLimit,omitempty"`
	Webhook           *WebhookConfig   `json:"webhook,omitempty"`
	Retention         *RetentionConfig `json:"retention,omitempty"`
	Security          *SecurityConfig  `json:"security,omitempty"`

	// Database configuration
	Database          *DatabaseConfig  `json:"database,omitempty"`

	// PAM (Privileged Access Management) config
	PAM               *pam.PAMConfig   `json:"pam,omitempty"`

	// OpenClaw integration
	OpenClaw          *OpenClawConfig  `json:"openclaw,omitempty"`
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

// AuthConfig for authentication settings
type AuthConfig struct {
	PluginToken string `json:"pluginToken,omitempty"`
}

// AuditConfig for audit logging settings
type AuditConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	LogPath string `json:"logPath,omitempty"`
}

// RateLimitConfig for rate limiting settings
type RateLimitConfig struct {
	Enabled        bool `json:"enabled,omitempty"`
	RequestsPerMin int  `json:"requestsPerMin,omitempty"`
}

// WebhookConfig for webhook notification settings
type WebhookConfig struct {
	URL    string `json:"url,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// RetentionConfig for data retention settings
type RetentionConfig struct {
	Days int `json:"days,omitempty"`
}

// SecurityConfig for security settings
type SecurityConfig struct {
	AllowedCommands []string `json:"allowedCommands,omitempty"`
	RequireReason   bool     `json:"requireReason,omitempty"`
}

// DatabaseConfig for database connection settings
type DatabaseConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	PasswordEnv string `json:"passwordEnv"`
	DBName      string `json:"dbname"`
	SSLMode     string `json:"sslmode"`
}

// OpenClawConfig for OpenClaw integration
type OpenClawConfig struct {
	WebhookURL    string  `json:"webhookUrl"`    // Webhook URL for notifications
	Enabled       bool    `json:"enabled"`       // Enable OpenClaw integration
}

// DefaultConfig returns a *Config populated with sensible defaults for KingCrab.
// The defaults include Version "0.1.0", a unix listen socket at /var/run/kingcrab.sock (also used as SocketPath),
// a curated list of AllowedCommands, RequireReason true, AutoApproveTimeout 0, LogDir "/var/log/kingcrab",
// DataDir "/var/lib/kingcrab", Port 8080, and LogLevel "info". Telegram is initialized with an empty AllowedUsers slice,
// PAM is set via pam.DefaultPAMConfig(), and OpenClaw.WebhookURL is populated from the KINGCRAB_OPENCLAW_WEBHOOK
// environment variable with OpenClaw.Enabled set when that variable is non-empty.
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
		OpenClaw: &OpenClawConfig{
			WebhookURL: os.Getenv("KINGCRAB_OPENCLAW_WEBHOOK"),
			Enabled:    os.Getenv("KINGCRAB_OPENCLAW_WEBHOOK") != "",
		},
	}
}

// Load loads configuration from the JSON file at the given path.
// If the file does not exist, it returns the defaults produced by DefaultConfig.
// If the file exists, it parses JSON into the default config and returns an error
// if reading or unmarshalling fails.
// Environment variable KINGCRAB_PORT, when set to an integer 1–65535, overrides cfg.Port.
// KINGCRAB_DB_* variables override the database section (see applyDatabaseEnvOverrides).
// If cfg.SocketPath is empty after loading and cfg.Listen.Path is set, cfg.SocketPath
// is set to cfg.Listen.Path.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Config file doesn't exist, return defaults
		applyDatabaseEnvOverrides(cfg)
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
			cfg.Listen.Port = p
		}
	}

	// Apply KINGCRAB_DB_* env overrides so the installer's systemd environment
	// variables are reflected in cfg.Database at runtime.
	applyDatabaseEnvOverrides(cfg)

	// Ensure socket path is set
	if cfg.SocketPath == "" && cfg.Listen.Path != "" {
		cfg.SocketPath = cfg.Listen.Path
	}

	return cfg, nil
}

// applyDatabaseEnvOverrides populates cfg.Database from KINGCRAB_DB_* environment
// variables. When any of those variables are set, cfg.Database is created if nil.
// Existing non-empty JSON values are not overwritten, so the config file takes
// precedence over environment variables for any field that is already set.
// The one exception is PasswordEnv: if KINGCRAB_DB_PASSWORD is present in the
// environment and PasswordEnv is still empty, it is defaulted to "KINGCRAB_DB_PASSWORD".
func applyDatabaseEnvOverrides(cfg *Config) {
	host := os.Getenv("KINGCRAB_DB_HOST")
	portStr := os.Getenv("KINGCRAB_DB_PORT")
	user := os.Getenv("KINGCRAB_DB_USER")
	password := os.Getenv("KINGCRAB_DB_PASSWORD")
	dbname := os.Getenv("KINGCRAB_DB_NAME")
	sslmode := os.Getenv("KINGCRAB_DB_SSLMODE")

	hasAny := host != "" || portStr != "" || user != "" || password != "" || dbname != "" || sslmode != ""
	if !hasAny {
		return
	}

	if cfg.Database == nil {
		cfg.Database = &DatabaseConfig{}
	}
	if host != "" && cfg.Database.Host == "" {
		cfg.Database.Host = host
	}
	if portStr != "" && cfg.Database.Port == 0 {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p < 65536 {
			cfg.Database.Port = p
		}
	}
	if user != "" && cfg.Database.User == "" {
		cfg.Database.User = user
	}
	if password != "" && cfg.Database.PasswordEnv == "" {
		cfg.Database.PasswordEnv = "KINGCRAB_DB_PASSWORD"
	}
	if dbname != "" && cfg.Database.DBName == "" {
		cfg.Database.DBName = dbname
	}
	if sslmode != "" && cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = sslmode
	}
}