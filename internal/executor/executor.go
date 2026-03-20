package executor

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/logger"
)

// Errors
var (
	ErrCommandNotAllowed = errors.New("command not in allowlist")
)

// Executor runs approved commands
type Executor struct {
	allowedCommands []string
	maxDuration     time.Duration
}

// NewExecutor creates a new executor with allowlist and max duration
func NewExecutor(allowedCommands []string, maxDuration time.Duration) *Executor {
	return &Executor{
		allowedCommands: allowedCommands,
		maxDuration:     maxDuration,
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

	duration := e.maxDuration
	if duration == 0 {
		duration = 5 * time.Minute
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	output, err := cmd.CombinedOutput()
	durationMs := time.Since(start).Milliseconds()

	result := &ExecuteResult{
		Duration: durationMs,
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
			"duration_ms": durationMs,
		})
	}

	result.Stdout = string(output)

	return result, nil
}
