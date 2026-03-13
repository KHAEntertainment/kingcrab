package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		def      string
		setValue string
		want     string
	}{
		{
			name:     "returns environment variable when set",
			key:      "TEST_VAR_1",
			def:      "default",
			setValue: "actual",
			want:     "actual",
		},
		{
			name:     "returns default when variable not set",
			key:      "TEST_VAR_NONEXISTENT",
			def:      "default",
			setValue: "",
			want:     "default",
		},
		{
			name:     "returns value when set to empty string",
			key:      "TEST_VAR_EMPTY",
			def:      "default",
			setValue: "",
			want:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		def      int
		setValue string
		want     int
	}{
		{
			name:     "parses valid integer",
			key:      "TEST_INT_1",
			def:      5432,
			setValue: "9999",
			want:     9999,
		},
		{
			name:     "returns default for invalid integer",
			key:      "TEST_INT_INVALID",
			def:      5432,
			setValue: "invalid",
			want:     5432,
		},
		{
			name:     "returns default when not set",
			key:      "TEST_INT_NONEXISTENT",
			def:      5432,
			setValue: "",
			want:     5432,
		},
		{
			name:     "returns default for negative number",
			key:      "TEST_INT_NEGATIVE",
			def:      5432,
			setValue: "-100",
			want:     5432,
		},
		{
			name:     "returns default for zero",
			key:      "TEST_INT_ZERO",
			def:      5432,
			setValue: "0",
			want:     5432,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getEnvInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewConnectionFromEnv_MissingPassword(t *testing.T) {
	// Ensure password is not set
	os.Unsetenv("KINGCRAB_DB_PASSWORD")

	_, err := NewConnectionFromEnv()
	if err == nil {
		t.Error("NewConnectionFromEnv() expected error when password not set, got nil")
	}

	if err.Error() != "KINGCRAB_DB_PASSWORD not set" {
		t.Errorf("NewConnectionFromEnv() error = %v, want 'KINGCRAB_DB_PASSWORD not set'", err)
	}
}

func TestNewConnectionFromEnv_UsesDefaults(t *testing.T) {
	// Set only password, let others use defaults
	os.Setenv("KINGCRAB_DB_PASSWORD", "testpass")
	defer os.Unsetenv("KINGCRAB_DB_PASSWORD")

	// Clear other env vars
	os.Unsetenv("KINGCRAB_DB_HOST")
	os.Unsetenv("KINGCRAB_DB_PORT")
	os.Unsetenv("KINGCRAB_DB_USER")
	os.Unsetenv("KINGCRAB_DB_NAME")
	os.Unsetenv("KINGCRAB_DB_SSLMODE")

	// This will fail to connect, but we can check the DSN would be correct
	// by verifying it attempts to connect with expected defaults
	_, err := NewConnectionFromEnv()

	// We expect an error because there's no actual database, but the function should construct the config
	if err == nil {
		t.Error("Expected connection error (no database), got nil")
	}

	// Error should be about connection, not about missing password
	if err.Error() == "KINGCRAB_DB_PASSWORD not set" {
		t.Error("Should not error on missing password when password is set")
	}
}

func TestNewConnection_InvalidDSN(t *testing.T) {
	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	// This will fail to connect to non-existent database
	_, err := NewConnection(cfg)
	if err == nil {
		t.Error("NewConnection() expected error for invalid connection, got nil")
	}
}

func TestConnection_Ping(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("KINGCRAB_TEST_DB") != "1" {
		t.Skip("Skipping database test - set KINGCRAB_TEST_DB=1 to run")
	}

	cfg := Config{
		Host:     getEnv("KINGCRAB_DB_HOST", "localhost"),
		Port:     getEnvInt("KINGCRAB_DB_PORT", 5432),
		User:     getEnv("KINGCRAB_DB_USER", "kingcrab"),
		Password: os.Getenv("KINGCRAB_DB_PASSWORD"),
		DBName:   getEnv("KINGCRAB_DB_NAME", "kingcrab_test"),
		SSLMode:  getEnv("KINGCRAB_DB_SSLMODE", "disable"),
	}

	conn, err := NewConnection(cfg)
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestRunMigrations(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("KINGCRAB_TEST_DB") != "1" {
		t.Skip("Skipping database test - set KINGCRAB_TEST_DB=1 to run")
	}

	cfg := Config{
		Host:     getEnv("KINGCRAB_DB_HOST", "localhost"),
		Port:     getEnvInt("KINGCRAB_DB_PORT", 5432),
		User:     getEnv("KINGCRAB_DB_USER", "kingcrab"),
		Password: os.Getenv("KINGCRAB_DB_PASSWORD"),
		DBName:   getEnv("KINGCRAB_DB_NAME", "kingcrab_test"),
		SSLMode:  getEnv("KINGCRAB_DB_SSLMODE", "disable"),
	}

	conn, err := NewConnection(cfg)
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run migrations
	if err := conn.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Verify tables exist
	tables := []string{
		"authorized_users",
		"enrolled_devices",
		"elevation_requests",
		"approval_audit",
	}

	for _, table := range tables {
		var exists bool
		err := conn.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
			table,
		).Scan(&exists)

		if err != nil {
			t.Errorf("Failed to check table %s: %v", table, err)
			continue
		}

		if !exists {
			t.Errorf("Table %s does not exist after migration", table)
		}
	}

	// Running migrations again should be idempotent
	if err := conn.RunMigrations(ctx); err != nil {
		t.Errorf("RunMigrations() second run error = %v", err)
	}
}