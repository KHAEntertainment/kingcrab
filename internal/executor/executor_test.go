package executor

import (
	"slices"
	"testing"
)

// tokenizeCommand tests

func TestTokenizeCommand_Simple(t *testing.T) {
	tokens, err := tokenizeCommand("mkdir /tmp/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"mkdir", "/tmp/foo"}
	if !slicesEqual(tokens, want) {
		t.Errorf("got %v, want %v", tokens, want)
	}
}

func TestTokenizeCommand_DoubleQuotedSpacePath(t *testing.T) {
	// mkdir "/tmp/a b" must produce exactly two argv entries, not three.
	tokens, err := tokenizeCommand(`mkdir "/tmp/a b"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"mkdir", "/tmp/a b"}
	if !slicesEqual(tokens, want) {
		t.Errorf("got %v, want %v", tokens, want)
	}
}

func TestTokenizeCommand_SingleQuotedSpacePath(t *testing.T) {
	tokens, err := tokenizeCommand("mkdir '/tmp/a b'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"mkdir", "/tmp/a b"}
	if !slicesEqual(tokens, want) {
		t.Errorf("got %v, want %v", tokens, want)
	}
}

func TestTokenizeCommand_OppositeQuoteInsideQuotedSpan(t *testing.T) {
	// Single quote inside a double-quoted span is a literal character.
	tokens, err := tokenizeCommand(`echo "it's fine"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"echo", "it's fine"}
	if !slicesEqual(tokens, want) {
		t.Errorf("got %v, want %v", tokens, want)
	}
}

func TestTokenizeCommand_UnclosedDoubleQuote(t *testing.T) {
	_, err := tokenizeCommand(`mkdir "/tmp/a b`)
	if err == nil {
		t.Fatal("expected error for unclosed double quote, got nil")
	}
}

func TestTokenizeCommand_UnclosedSingleQuote(t *testing.T) {
	_, err := tokenizeCommand("mkdir '/tmp/a b")
	if err == nil {
		t.Fatal("expected error for unclosed single quote, got nil")
	}
}

func TestTokenizeCommand_EmptyString(t *testing.T) {
	tokens, err := tokenizeCommand("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected empty slice, got %v", tokens)
	}
}

func TestTokenizeCommand_MultipleSpaces(t *testing.T) {
	tokens, err := tokenizeCommand("ls   -la   /tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"ls", "-la", "/tmp"}
	if !slicesEqual(tokens, want) {
		t.Errorf("got %v, want %v", tokens, want)
	}
}

// TokenizeAndMatch tests

func TestTokenizeAndMatch_QuotedCommandMatchesPattern(t *testing.T) {
	// The unquoted token content should match the pattern token.
	if !TokenizeAndMatch("mkdir *", `mkdir "/tmp/a b"`) {
		t.Error("expected match for quoted path against wildcard pattern")
	}
}

func TestTokenizeAndMatch_UnclosedQuoteRejected(t *testing.T) {
	if TokenizeAndMatch("mkdir *", `mkdir "/tmp/a b`) {
		t.Error("expected no match for command with unclosed quote")
	}
}

func TestTokenizeAndMatch_UnclosedQuoteInPatternRejected(t *testing.T) {
	if TokenizeAndMatch(`mkdir "`, "mkdir foo") {
		t.Error("expected no match for pattern with unclosed quote")
	}
}

func TestTokenizeAndMatch_ExactMatch(t *testing.T) {
	if !TokenizeAndMatch("ls -la", "ls -la") {
		t.Error("expected exact match")
	}
}

func TestTokenizeAndMatch_WildcardMatch(t *testing.T) {
	if !TokenizeAndMatch("ls *", "ls /tmp") {
		t.Error("expected wildcard match")
	}
}

func TestTokenizeAndMatch_NoMatch(t *testing.T) {
	if TokenizeAndMatch("ls", "rm /tmp/foo") {
		t.Error("expected no match")
	}
}

// slicesEqual is a helper for comparing string slices.
func slicesEqual(a, b []string) bool {
	return slices.Equal(a, b)
}
