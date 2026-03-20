package executor

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/logger"
)

// TokenizeAndMatch performs tokenized pattern matching against a command
func TokenizeAndMatch(pattern, command string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == command {
		return true
	}

	// Tokenize pattern and command by whitespace
	patternTokens := tokenizeCommand(pattern)
	commandTokens := tokenizeCommand(command)

	if len(patternTokens) == 0 || len(commandTokens) == 0 {
		return false
	}

	// Check if last pattern token is a wildcard
	hasWildcard := false
	matchTokens := patternTokens
	if patternTokens[len(patternTokens)-1] == "*" {
		hasWildcard = true
		matchTokens = patternTokens[:len(patternTokens)-1]
	}

	// Command must have at least as many tokens as non-wildcard pattern tokens
	if len(commandTokens) < len(matchTokens) {
		return false
	}

	// If no wildcard, command must match pattern exactly in token count
	if !hasWildcard && len(commandTokens) != len(matchTokens) {
		return false
	}

	// Match all non-wildcard pattern tokens
	for i, patternToken := range matchTokens {
		if commandTokens[i] != patternToken {
			return false
		}
	}

	// Reject commands containing shell metacharacters or operators
	for _, token := range commandTokens {
		if containsShellMetacharacters(token) {
			return false
		}
	}

	return true
}

// tokenizeCommand splits a command into whitespace-separated tokens
func tokenizeCommand(s string) []string {
	var tokens []string
	var current []rune

	for _, ch := range s {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		} else {
			current = append(current, ch)
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	return tokens
}

// containsShellMetacharacters checks if a token contains dangerous shell characters
func containsShellMetacharacters(token string) bool {
	// Reject tokens containing shell operators and metacharacters
	dangerousChars := ";|&<>`$(){}[]!*?~"
	for _, ch := range token {
		for _, danger := range dangerousChars {
			if ch == danger {
				return true
			}
		}
	}
	return false
}

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

// IsAllowed checks if a command matches the allowlist using tokenized matching
func (e *Executor) IsAllowed(command string) bool {
	for _, pattern := range e.allowedCommands {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		if TokenizeAndMatch(pattern, command) {
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

	// Tokenize command and execute directly without shell
	argv := tokenizeCommand(command)
	if len(argv) == 0 {
		return nil, errors.New("empty command")
	}

	var cmd *exec.Cmd
	if len(argv) == 1 {
		cmd = exec.CommandContext(ctx, argv[0])
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
	}

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
		result.Stdout = string(output)
		logger.Error("Command execution failed", map[string]interface{}{
			"command": command,
			"error":   err.Error(),
		})
		return result, err
	}

	result.ExitCode = 0
	result.Stdout = string(output)
	logger.Info("Command executed", map[string]interface{}{
		"command":     command,
		"duration_ms": durationMs,
	})

	return result, nil
}