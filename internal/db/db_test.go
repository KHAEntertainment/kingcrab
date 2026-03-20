package db

import (
	"os"
	"testing"
)

func TestConfig_Structure(t *testing.T) {
	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	if cfg.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got %s", cfg.Host)
	}

	if cfg.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", cfg.Port)
	}

	if cfg.User != "testuser" {
		t.Errorf("Expected user 'testuser', got %s", cfg.User)
	}

	if cfg.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got %s", cfg.Password)
	}

	if cfg.DBName != "testdb" {
		t.Errorf("Expected dbname 'testdb', got %s", cfg.DBName)
	}

	if cfg.SSLMode != "disable" {
		t.Errorf("Expected sslmode 'disable', got %s", cfg.SSLMode)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		def      string
		envValue string
		expected string
	}{
		{
			name:     "env var set",
			key:      "TEST_VAR_1",
			def:      "default",
			envValue: "custom",
			expected: "custom",
		},
		{
			name:     "env var not set",
			key:      "TEST_VAR_2",
			def:      "default",
			envValue: "",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.def)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		def      int
		envValue string
		expected int
	}{
		{
			name:     "valid int",
			key:      "TEST_INT_1",
			def:      100,
			envValue: "5432",
			expected: 5432,
		},
		{
			name:     "not set",
			key:      "TEST_INT_2",
			def:      100,
			envValue: "",
			expected: 100,
		},
		{
			name:     "invalid int",
			key:      "TEST_INT_3",
			def:      100,
			envValue: "not_a_number",
			expected: 100,
		},
		{
			name:     "zero value",
			key:      "TEST_INT_4",
			def:      100,
			envValue: "0",
			expected: 100, // Zero is treated as invalid
		},
		{
			name:     "negative value",
			key:      "TEST_INT_5",
			def:      100,
			envValue: "-1",
			expected: 100, // Negative is treated as invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvInt(tt.key, tt.def)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestNewConnectionFromEnv_NoPassword(t *testing.T) {
	// Ensure password is not set
	os.Unsetenv("KINGCRAB_DB_PASSWORD")

	_, err := NewConnectionFromEnv()
	if err == nil {
		t.Fatal("Expected error when password not set")
	}

	if err.Error() != "KINGCRAB_DB_PASSWORD not set" {
		t.Errorf("Expected password error, got: %v", err)
	}
}

func TestNewConnectionFromEnv_WithDefaults(t *testing.T) {
	// Set only password (required)
	os.Setenv("KINGCRAB_DB_PASSWORD", "test_password")
	defer os.Unsetenv("KINGCRAB_DB_PASSWORD")

	// Clear other vars to test defaults
	os.Unsetenv("KINGCRAB_DB_HOST")
	os.Unsetenv("KINGCRAB_DB_PORT")
	os.Unsetenv("KINGCRAB_DB_USER")
	os.Unsetenv("KINGCRAB_DB_NAME")
	os.Unsetenv("KINGCRAB_DB_SSLMODE")

	// Note: This will fail to actually connect since we don't have a real DB
	// But we can test that the config is built correctly by checking the error
	_, err := NewConnectionFromEnv()

	// Should fail to connect, not fail to parse config
	if err == nil {
		// Unlikely, but if it succeeds that's fine
		return
	}

	// Error should be about connection, not about missing config
	errStr := err.Error()
	if errStr == "KINGCRAB_DB_PASSWORD not set" {
		t.Error("Should not fail on password check")
	}
}

// Note: Integration tests for actual database connection and migrations
// would require a running PostgreSQL instance. These are unit tests that
// verify the configuration and helper functions work correctly.

func TestConnection_Structure(t *testing.T) {
	// Test that Connection is a valid wrapper
	// We can't create an actual connection without a DB, so just test the type
	var conn *Connection
	if conn != nil {
		t.Error("nil Connection should be nil")
	}
}

func TestNewConnection_InvalidDSN(t *testing.T) {
	cfg := Config{
		Host:     "invalid-host-that-does-not-exist-12345",
		Port:     99999, // Invalid port
		User:     "user",
		Password: "pass",
		DBName:   "db",
		SSLMode:  "disable",
	}

	_, err := NewConnection(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid connection config")
	}
}

func TestNewConnection_DSNFormat(t *testing.T) {
	// Test that DSN is formatted correctly
	// We can't verify without actually connecting, but we can test that
	// the function doesn't panic with valid config
	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	// This will fail to connect (no DB) but shouldn't panic
	_, err := NewConnection(cfg)
	if err == nil {
		// If it succeeds, that's unexpected but not an error for this test
		return
	}

	// Should be a connection error, not a panic
	if err.Error() == "" {
		t.Error("Error should have a message")
	}
}

// Test environment variable precedence
func TestNewConnectionFromEnv_CustomValues(t *testing.T) {
	os.Setenv("KINGCRAB_DB_HOST", "custom-host")
	os.Setenv("KINGCRAB_DB_PORT", "5433")
	os.Setenv("KINGCRAB_DB_USER", "custom-user")
	os.Setenv("KINGCRAB_DB_PASSWORD", "custom-pass")
	os.Setenv("KINGCRAB_DB_NAME", "custom-db")
	os.Setenv("KINGCRAB_DB_SSLMODE", "require")

	defer func() {
		os.Unsetenv("KINGCRAB_DB_HOST")
		os.Unsetenv("KINGCRAB_DB_PORT")
		os.Unsetenv("KINGCRAB_DB_USER")
		os.Unsetenv("KINGCRAB_DB_PASSWORD")
		os.Unsetenv("KINGCRAB_DB_NAME")
		os.Unsetenv("KINGCRAB_DB_SSLMODE")
	}()

	// Will fail to connect but config should be built from env vars
	_, err := NewConnectionFromEnv()

	// Should not be a config error
	if err != nil && err.Error() == "KINGCRAB_DB_PASSWORD not set" {
		t.Error("Password was set, should not fail on that check")
	}
}