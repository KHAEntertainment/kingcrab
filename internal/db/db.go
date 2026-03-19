package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// Config holds database connection configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Connection holds the database connection pool
type Connection struct {
	*sql.DB
}

// NewConnection creates a new database connection
func NewConnection(cfg Config) (*Connection, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Connection{db}, nil
}

// NewConnectionFromEnv creates connection from environment variables
func NewConnectionFromEnv() (*Connection, error) {
	cfg := Config{
		Host:     getEnv("KINGCRAB_DB_HOST", "localhost"),
		Port:     getEnvInt("KINGCRAB_DB_PORT", 5432),
		User:     getEnv("KINGCRAB_DB_USER", "kingcrab"),
		Password: os.Getenv("KINGCRAB_DB_PASSWORD"),
		DBName:   getEnv("KINGCRAB_DB_NAME", "kingcrab"),
		SSLMode:  getEnv("KINGCRAB_DB_SSLMODE", "disable"),
	}

	if cfg.Password == "" {
		return nil, fmt.Errorf("KINGCRAB_DB_PASSWORD not set")
	}

	return NewConnection(cfg)
}

// getEnv returns environment variable or default
func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

// getEnvInt returns environment variable as int or default
func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		var i int
		fmt.Sscanf(val, "%d", &i)
		if i > 0 {
			return i
		}
	}
	return def
}

// Close closes the database connection
func (c *Connection) Close() error {
	return c.DB.Close()
}

// RunMigrations runs the database migrations
func (c *Connection) RunMigrations(ctx context.Context) error {
	migrations := []string{
		// Enable UUID extension
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

		// authorized_users
		`CREATE TABLE IF NOT EXISTS authorized_users (
			id SERIAL PRIMARY KEY,
			telegram_id BIGINT UNIQUE,
			clawvault_id VARCHAR(255) UNIQUE,
			username VARCHAR(255),
			display_name VARCHAR(255),
			auth_mode VARCHAR(20) DEFAULT 'biometric',
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			CONSTRAINT one_identity CHECK (
				(telegram_id IS NOT NULL) OR (clawvault_id IS NOT NULL)
			)
		)`,

		// enrolled_devices
		`CREATE TABLE IF NOT EXISTS enrolled_devices (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES authorized_users(id) ON DELETE CASCADE,
			token_ref VARCHAR(512),
			token_storage VARCHAR(20) DEFAULT 'local',
			device_info VARCHAR(255),
			device_hash VARCHAR(64),
			is_active BOOLEAN DEFAULT TRUE,
			enrolled_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			last_used_at TIMESTAMP WITH TIME ZONE,
			expires_at TIMESTAMP WITH TIME ZONE
		)`,

		// elevation_requests
		`CREATE TABLE IF NOT EXISTS elevation_requests (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			requester VARCHAR(255) NOT NULL,
			target_system VARCHAR(255) NOT NULL,
			command TEXT NOT NULL,
			reason TEXT,
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			approved_at TIMESTAMP WITH TIME ZONE,
			executed_at TIMESTAMP WITH TIME ZONE,
			approved_by TEXT,
			ip_address VARCHAR(45),
			user_agent VARCHAR(512),
			output TEXT,
			exit_code INTEGER
		)`,

		// approval_audit
		`CREATE TABLE IF NOT EXISTS approval_audit (
			id SERIAL PRIMARY KEY,
			request_id UUID REFERENCES elevation_requests(id) ON DELETE SET NULL,
			device_id INTEGER REFERENCES enrolled_devices(id) ON DELETE SET NULL,
			user_id INTEGER REFERENCES authorized_users(id) ON DELETE SET NULL,
			action VARCHAR(30) NOT NULL,
			ip_address VARCHAR(45),
			user_agent VARCHAR(512),
			details JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_users_telegram ON authorized_users(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_clawvault ON authorized_users(clawvault_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_active ON authorized_users(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_user ON enrolled_devices(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_active ON enrolled_devices(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_status ON elevation_requests(status)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_created ON elevation_requests(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_expires ON elevation_requests(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_request ON approval_audit(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_created ON approval_audit(created_at)`,
	}

	for _, m := range migrations {
		if _, err := c.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// Ping tests the database connection
func (c *Connection) Ping(ctx context.Context) error {
	return c.PingContext(ctx)
}