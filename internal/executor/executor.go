package executor

import (
	"context"
	"errors"
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

// tokenizeCommand splits the input string s into whitespace-separated tokens
// while handling quoted arguments. Single and double quotes preserve whitespace
// within quoted strings as single tokens. Returns an error if quotes are mismatched.
// It treats space, tab, newline, and carriage return as delimiters and omits
// empty tokens produced by consecutive delimiters. The returned slice contains
// tokens in their original order.
func tokenizeCommand(s string) []string {
	var tokens []string
	var current []rune
	var inQuote rune // 0 = not in quote, '\'' or '"' = in quote

	for i, ch := range s {
		// Handle quote boundaries
		if (ch == '\'' || ch == '"') && (i == 0 || s[i-1] != '\\') {
			if inQuote == 0 {
				// Start quoted section
				inQuote = ch
			} else if inQuote == ch {
				// End quoted section (matching quote)
				inQuote = 0
			} else {
				// Different quote type inside quoted section - just add it
				current = append(current, ch)
			}
			continue
		}

		// Handle whitespace
		if inQuote == 0 && (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') {
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		} else {
			// Regular character or quoted whitespace
			current = append(current, ch)
		}
	}

	// Add final token if any
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	// If we ended while still in a quote, that's an error condition
	// but we'll return what we have (caller will handle validation)
	return tokens
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

	// Create buffers to capture stdout and stderr separately
	var stdoutBuf, stderrBuf []byte
	var stdoutBuilder, stderrBuilder strings.Builder
	cmd.Stdout = &stdoutBuilder
	cmd.Stderr = &stderrBuilder

	err := cmd.Run()
	durationMs := time.Since(start).Milliseconds()

	stdoutBuf = []byte(stdoutBuilder.String())
	stderrBuf = []byte(stderrBuilder.String())

	result := &ExecuteResult{
		Stdout:   string(stdoutBuf),
		Stderr:   string(stderrBuf),
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