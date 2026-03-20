package main

import (
	"fmt"
	"os"

	"github.com/KHAEntertainment/kingcrab/internal/config"
	"github.com/KHAEntertainment/kingcrab/internal/daemon"
	"github.com/KHAEntertainment/kingcrab/internal/logger"
)

const version = "1.0.0"

func main() {
	logger.Info("KingCrab starting", map[string]interface{}{
		"version": version,
	})

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("Failed to load config", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Create server (v2 with database support)
	server, err := daemon.NewServerV2(cfg)
	if err != nil {
		logger.Error("Failed to create server", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Handle signals
	server.HandleSignals()

	// Start server
	if err := server.Start(); err != nil {
		logger.Error("Server error", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	configPath := os.Getenv("KINGCRAB_CONFIG")
	if configPath == "" {
		configPath = "/etc/kingcrab/config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}
