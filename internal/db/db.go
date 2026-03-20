package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

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
		SSLMode:  getEnv("KINGCRAB_DB_SSLMODE", "prefer"), // Use "require" in production
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

// RunMigrations runs the database migrations from the SQL file
func (c *Connection) RunMigrations(ctx context.Context) error {
	// Read the migration SQL file from embedded filesystem
	sqlBytes, err := migrationsFS.ReadFile("migrations/001_pam_schema.sql")
	if err != nil {
		return fmt.Errorf("read embedded migration: %w", err)
	}

	sqlContent := string(sqlBytes)

	// Split into individual statements (simple approach: split on semicolon + newline)
	// This handles most SQL statements but may need refinement for complex cases
	statements := splitSQLStatements(sqlContent)

	// Execute each statement
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Skip comments
		if strings.HasPrefix(stmt, "--") {
			continue
		}

		if _, err := c.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration statement %d failed: %w\nStatement: %s", i+1, err, stmt[:min(100, len(stmt))])
		}
	}

	return nil
}

// splitSQLStatements splits SQL content into individual statements
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	var inFunction bool

	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		// Track if we're inside a function definition
		if strings.Contains(strings.ToUpper(trimmed), "CREATE OR REPLACE FUNCTION") ||
			strings.Contains(strings.ToUpper(trimmed), "CREATE FUNCTION") {
			inFunction = true
		}

		current.WriteString(line)
		current.WriteString("\n")

		// End of function
		if inFunction && strings.Contains(trimmed, "$$") && strings.Count(current.String(), "$$")%2 == 0 {
			// Check if this is the closing $$
			afterDollar := strings.TrimSpace(strings.Split(trimmed, "$$")[len(strings.Split(trimmed, "$$"))-1])
			if strings.HasSuffix(afterDollar, ";") || afterDollar == "" {
				inFunction = false
				statements = append(statements, current.String())
				current.Reset()
			}
		} else if !inFunction && strings.HasSuffix(trimmed, ";") {
			// Regular statement end
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	// Add any remaining content
	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ping tests the database connection
func (c *Connection) Ping(ctx context.Context) error {
	return c.PingContext(ctx)
}