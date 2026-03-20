package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/api"
	"github.com/KHAEntertainment/kingcrab/internal/config"
	"github.com/KHAEntertainment/kingcrab/internal/db"
	"github.com/KHAEntertainment/kingcrab/internal/executor"
	"github.com/KHAEntertainment/kingcrab/internal/logger"
	"github.com/KHAEntertainment/kingcrab/internal/notifications"
	"github.com/KHAEntertainment/kingcrab/internal/pam"
)

// ServerV2 is the database-backed KingCrab server
type ServerV2 struct {
	config         *config.Config
	db             *db.Connection
	store          pam.RequestStore
	pam            *pam.PAM
	executor       *executor.Executor
	notifier       *notifications.OpenClawNotifier
	apiHandler     *api.V1Handler
	pamHandler     *pam.Handler
	httpServer     *http.Server
	allowedOrigins []string
	cleanupCancel  context.CancelFunc
}

// NewServerV2 creates and wires a ServerV2 with SQLite-backed storage, PAM, executor,
// notifier, and API handler based on the provided configuration.
//
// The function uses a SQLite-backed in-memory request store for the daemon's request queue,
// initializes PAM from cfg.PAM, creates a command executor using cfg.AllowedCommands,
// and configures an OpenClaw notifier when enabled.
// It returns an initialized *ServerV2 on success. Errors are returned if the function
// fails to initialize PAM or the request store.
func NewServerV2(cfg *config.Config) (*ServerV2, error) {
	// Use in-memory request store (SQLite-compatible approach)
	// The daemon uses a local SQLite/in-memory store for its request queue
	store := pam.NewInMemoryRequestStore()

	// Note: External database connection is intentionally not created here
	// The daemon's request queue uses SQLite/in-memory storage only
	var database *db.Connection = nil

	// Create PAM module
	p, err := pam.NewPAM(cfg.PAM)
	if err != nil {
		// No database to close since we use in-memory storage
		return nil, fmt.Errorf("initialize PAM: %w", err)
	}

	// Create executor
	exec := executor.NewExecutor(cfg.AllowedCommands, 5*time.Minute)

	// Create notifier
	var notifier *notifications.OpenClawNotifier
	if cfg.OpenClaw != nil && cfg.OpenClaw.Enabled {
		notifier = notifications.NewOpenClawNotifier(cfg.OpenClaw.WebhookURL)
		logger.Info("OpenClaw notifications enabled", nil)
	} else {
		notifier = notifications.NewOpenClawNotifier("")
	}

	// Create API handler
	apiHandler := api.NewV1Handler(store, exec, notifier, cfg.AllowedCommands, cfg.RequireReason)

	// Configure allowed origins (default to localhost for now)
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:8080"}

	return &ServerV2{
		config:         cfg,
		db:             database,
		store:          store,
		pam:            p,
		executor:       exec,
		notifier:       notifier,
		apiHandler:     apiHandler,
		allowedOrigins: allowedOrigins,
	}, nil
}

// Start starts the HTTP server
func (s *ServerV2) Start() error {
	// Create socket directory if using Unix socket
	if s.config.Listen.Type == "unix" && s.config.Listen.Path != "" {
		socketDir := filepath.Dir(s.config.Listen.Path)
		if err := os.MkdirAll(socketDir, 0755); err != nil {
			return fmt.Errorf("create socket directory: %w", err)
		}
		// Remove old socket if exists
		_ = os.Remove(s.config.Listen.Path)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Register API routes
	s.apiHandler.RegisterRoutes(mux)

	// Register PAM handler for biometric authentication only if Telegram is configured
	// The PAM handler is a http.Handler that routes /api/pam/* requests
	var botToken string
	if s.config.Telegram != nil {
		botToken = s.config.Telegram.BotToken
	}
	if botToken != "" {
		pamHandler := pam.NewHandler(
			s.pam,
			s.store,
			botToken,
			nil, // allowedUsers - queried from DB
			s.config.AllowedCommands,
			5,   // TTL minutes
		)
		s.pamHandler = pamHandler
		mux.Handle("/api/pam/", pamHandler)
		logger.Info("PAM biometric handler registered at /api/pam/", nil)
	} else {
		logger.Info("Telegram botToken not configured, skipping PAM handler registration", nil)
	}

	// Add middleware
	handler := s.loggingMiddleware(mux)
	handler = s.corsMiddleware(handler)

	addr := s.getListenAddress()
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("Starting KingCrab v2 server", map[string]interface{}{
		"address": addr,
		"type":    s.config.Listen.Type,
	})

	// Start cleanup goroutine for expired requests
	cleanupCtx, cancel := context.WithCancel(context.Background())
	s.cleanupCancel = cancel
	go s.cleanupOldRequests(cleanupCtx)

	if s.config.Listen.Type == "unix" {
		listener, err := net.Listen("unix", s.config.Listen.Path)
		if err != nil {
			return fmt.Errorf("listen on unix socket: %w", err)
		}
		// Set socket permissions
		if err := os.Chmod(s.config.Listen.Path, 0770); err != nil {
			logger.Warn("Failed to set socket permissions", map[string]interface{}{
				"error": err.Error(),
			})
		}
		return s.httpServer.Serve(listener)
	}

	return s.httpServer.ListenAndServe()
}

// Stop stops the server gracefully
func (s *ServerV2) Stop(ctx context.Context) error {
	logger.Info("Stopping KingCrab server", nil)

	// Stop cleanup goroutine
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			logger.Error("Failed to shutdown HTTP server", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			logger.Error("Failed to close database", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return nil
}

// getListenAddress returns the listen address
func (s *ServerV2) getListenAddress() string {
	if s.config.Listen.Type == "unix" {
		return s.config.Listen.Path
	}
	if s.config.Listen.Type == "tcp" && s.config.Listen.Port != 0 {
		return fmt.Sprintf("127.0.0.1:%d", s.config.Listen.Port)
	}
	// Default to localhost:8080
	return "127.0.0.1:8080"
}

// loggingMiddleware adds request logging
func (s *ServerV2) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		logger.Info("Request received", map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
			"remote": r.RemoteAddr,
		})

		// Wrap response writer to capture status code
		wrapped := &responseWrapper{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		logger.Info("Request completed", map[string]interface{}{
			"method":   r.Method,
			"path":     r.URL.Path,
			"status":   wrapped.status,
			"duration": time.Since(start).Milliseconds(),
		})
	})
}

// corsMiddleware adds CORS headers
func (s *ServerV2) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is in whitelist
		allowed := false
		for _, allowedOrigin := range s.allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// Set CORS headers if origin is allowed
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWrapper) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// cleanupOldRequests runs periodically to clean up old requests
func (s *ServerV2) cleanupOldRequests(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup(ctx)
		}
	}
}

// performCleanup executes the actual cleanup logic
func (s *ServerV2) performCleanup(ctx context.Context) {
	logger.Info("Running cleanup of old requests", nil)

	// Expire old pending requests that have passed their expiration time
	expiredCount, err := s.store.ExpireOldRequests(ctx)
	if err != nil {
		logger.Error("Failed to expire old requests", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if expiredCount > 0 {
		logger.Info("Expired old requests", map[string]interface{}{
			"count": expiredCount,
		})
	} else {
		logger.Info("No expired requests found", nil)
	}
}

// Handle signals for graceful shutdown
func (s *ServerV2) HandleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		for sig := range sigCh {
			logger.Info("Received signal", map[string]interface{}{
				"signal": sig.String(),
			})

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = s.Stop(ctx)
				os.Exit(0)
			case syscall.SIGUSR1:
				// Reload configuration
				logger.Info("Reloading configuration", nil)
				// TODO: Implement config reload
			}
		}
	}()
}