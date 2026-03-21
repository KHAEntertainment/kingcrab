package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/logger"
)

// TokenizeAndMatch determines whether a command string matches a space-separated
// pattern that may end with a trailing '*' wildcard.
// It returns true if the pattern is "*" or exactly equals the command, or if
// every non-wildcard pattern token equals the corresponding command token while
// satisfying token-count rules (command must have at least as many tokens as
// the non-wildcard pattern tokens; if the pattern has no trailing '*', token
// counts must be equal). Commands or patterns that are empty do not match, and
// any command containing shell metacharacters/operators is rejected.
// Commands or patterns with mismatched or unclosed quotes are rejected.
func TokenizeAndMatch(pattern, command string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == command {
		return true
	}

	// Tokenize pattern and command, rejecting on quote errors
	patternTokens, err := tokenizeCommand(pattern)
	if err != nil {
		return false
	}
	commandTokens, err := tokenizeCommand(command)
	if err != nil {
		return false
	}

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

// tokenizeCommand splits the input string s into tokens, honouring single and
// double quotes so that whitespace inside a quoted span is preserved as part of
// a single token (quotes themselves are stripped from the result). It returns
// an error if a quoted section is never closed. It treats space, tab, newline,
// and carriage return as delimiters outside of quoted spans and omits empty
// tokens produced by consecutive delimiters. The returned slice contains tokens
// in their original order.
func tokenizeCommand(s string) ([]string, error) {
	var tokens []string
	var current []rune
	var inQuote rune // 0 = not in quote, '\'' or '"' = in quote

	for _, ch := range s {
		switch {
		case (ch == '\'' || ch == '"') && inQuote == 0:
			// Opening quote — enter quoted span without adding the quote char.
			inQuote = ch
		case ch == inQuote:
			// Closing quote — leave quoted span without adding the quote char.
			inQuote = 0
		case inQuote == 0 && (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'):
			// Unquoted whitespace — flush the current token if non-empty.
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		default:
			// Regular character, or whitespace/opposite-quote inside a quoted span.
			current = append(current, ch)
		}
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote %c in command", inQuote)
	}

	// Flush the final token.
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	return tokens, nil
}

// containsShellMetacharacters reports whether the token contains any shell
// metacharacter that is considered dangerous for command execution (one of
// `;|&<>`$(){}[]!*?~`).
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

// NewExecutor returns a new Executor configured with the provided allowlist of command patterns and maximum execution duration.
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
	argv, err := tokenizeCommand(command)
	if err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}
	if len(argv) == 0 {
		return nil, errors.New("empty command")
	}

	var cmd *exec.Cmd
	if len(argv) == 1 {
		cmd = exec.CommandContext(ctx, argv[0])
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
	}

	// Create buffers to capture stdout and stderr separately
	var stdoutBuilder, stderrBuilder strings.Builder
	cmd.Stdout = &stdoutBuilder
	cmd.Stderr = &stderrBuilder

	err = cmd.Run()
	durationMs := time.Since(start).Milliseconds()

	result := &ExecuteResult{
		Stdout:   stdoutBuilder.String(),
		Stderr:   stderrBuilder.String(),
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
		return result, err
	}

	result.ExitCode = 0
	logger.Info("Command executed", map[string]interface{}{
		"command":     command,
		"duration_ms": durationMs,
	})

	return result, nil
}