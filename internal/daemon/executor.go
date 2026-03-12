package daemon

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/logger"
)

// Executor runs approved commands
type Executor struct {
	allowedCommands []string
}

// NewExecutor creates a new executor with allowlist
func NewExecutor(allowedCommands []string) *Executor {
	return &Executor{
		allowedCommands: allowedCommands,
	}
}

// ExecuteResult holds command execution results
type ExecuteResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration int64 // milliseconds
}

// IsAllowed checks if a command matches the allowlist
func (e *Executor) IsAllowed(command string) bool {
	// Simple wildcard matching for MVP
	for _, pattern := range e.allowedCommands {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		
		// Handle wildcard patterns
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(command, prefix) {
				return true
			}
		}
		
		// Exact match
		if command == pattern {
			return true
		}
	}
	return false
}

// Execute runs a command if allowed
func (e *Executor) Execute(command string) (*ExecuteResult, error) {
	if !e.IsAllowed(command) {
		return nil, ErrCommandNotAllowed
	}

	start := time.Now()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	
	output, err := cmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()

	result := &ExecuteResult{
		Duration: duration,
	}

	if err != nil {
		result.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		logger.Error("Command execution failed", map[string]interface{}{
			"command": command,
			"error":   err.Error(),
		})
	} else {
		result.ExitCode = 0
		logger.Info("Command executed", map[string]interface{}{
			"command":     command,
			"duration_ms": duration,
		})
	}

	result.Stdout = string(output)

	return result, nil
}
