package telegram

import (
	"strings"
	"testing"
	"time"
)

func TestNewBot(t *testing.T) {
	token := "test-token"
	webhookURL := "https://example.com/webhook"
	miniAppURL := "https://example.com/app"

	bot := NewBot(token, webhookURL, miniAppURL)

	if bot == nil {
		t.Fatal("expected bot, got nil")
	}

	if bot.token != token {
		t.Errorf("expected token %s, got %s", token, bot.token)
	}

	if bot.webhookURL != webhookURL {
		t.Errorf("expected webhookURL %s, got %s", webhookURL, bot.webhookURL)
	}

	if bot.miniAppURL != miniAppURL {
		t.Errorf("expected miniAppURL %s, got %s", miniAppURL, bot.miniAppURL)
	}

	if bot.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}

	if bot.httpClient.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", bot.httpClient.Timeout)
	}
}

func TestBuildApprovalKeyboard(t *testing.T) {
	bot := NewBot("token", "webhook", "https://example.com")
	requestID := "test-request-123"

	keyboard := bot.BuildApprovalKeyboard(requestID)

	if keyboard == nil {
		t.Fatal("expected keyboard, got nil")
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Errorf("expected 2 rows, got %d", len(keyboard.InlineKeyboard))
	}

	// Check first row (approval button)
	if len(keyboard.InlineKeyboard[0]) != 1 {
		t.Error("expected 1 button in first row")
	}

	approveBtn := keyboard.InlineKeyboard[0][0]
	if !strings.Contains(approveBtn.Text, "Authenticate") {
		t.Errorf("expected 'Authenticate' in button text, got %s", approveBtn.Text)
	}

	if !strings.Contains(approveBtn.URL, requestID) {
		t.Errorf("expected request ID in URL, got %s", approveBtn.URL)
	}

	if !strings.Contains(approveBtn.URL, "pam.html") {
		t.Error("expected pam.html in URL")
	}

	// Check second row (deny button)
	if len(keyboard.InlineKeyboard[1]) != 1 {
		t.Error("expected 1 button in second row")
	}

	denyBtn := keyboard.InlineKeyboard[1][0]
	if !strings.Contains(denyBtn.Text, "Deny") {
		t.Errorf("expected 'Deny' in button text, got %s", denyBtn.Text)
	}

	expectedCallback := "deny_" + requestID
	if denyBtn.CallbackData != expectedCallback {
		t.Errorf("expected callback data %s, got %s", expectedCallback, denyBtn.CallbackData)
	}
}

func TestBuildApprovalMessage(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")
	expiresAt := time.Now().Add(5 * time.Minute)

	req := &ElevationRequest{
		ID:           "test-id",
		TargetSystem: "web-server",
		Requester:    "admin@example.com",
		Command:      "systemctl restart nginx",
		Reason:       "deploying updates",
		ExpiresAt:    expiresAt,
	}

	message := bot.BuildApprovalMessage(req)

	// Check that message contains key information
	if !strings.Contains(message, "Elevation Request") {
		t.Error("expected 'Elevation Request' in message")
	}

	if !strings.Contains(message, req.TargetSystem) {
		t.Error("expected target system in message")
	}

	if !strings.Contains(message, req.Requester) {
		t.Error("expected requester in message")
	}

	if !strings.Contains(message, "systemctl restart nginx") {
		t.Error("expected command in message")
	}

	if !strings.Contains(message, req.Reason) {
		t.Error("expected reason in message")
	}

	if !strings.Contains(message, "Expires") {
		t.Error("expected expiration info in message")
	}
}

func TestBuildApprovalResultMessage(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")

	req := &ElevationRequest{
		ID:           "test-id",
		TargetSystem: "db-server",
		Command:      "pg_dump database",
		Requester:    "dba",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}

	t.Run("approved message", func(t *testing.T) {
		approver := "admin"
		message := bot.BuildApprovalResultMessage(req, true, approver)

		if !strings.Contains(message, "Approved") {
			t.Error("expected 'Approved' in message")
		}

		if !strings.Contains(message, approver) {
			t.Error("expected approver in message")
		}

		if !strings.Contains(message, req.TargetSystem) {
			t.Error("expected target system in message")
		}

		if !strings.Contains(message, "pg_dump database") {
			t.Error("expected command in message")
		}

		// Should show who approved
		if !strings.Contains(message, "By:") {
			t.Error("expected 'By:' in approved message")
		}
	})

	t.Run("denied message", func(t *testing.T) {
		message := bot.BuildApprovalResultMessage(req, false, "")

		if !strings.Contains(message, "Denied") {
			t.Error("expected 'Denied' in message")
		}

		if !strings.Contains(message, req.TargetSystem) {
			t.Error("expected target system in message")
		}

		if !strings.Contains(message, "pg_dump database") {
			t.Error("expected command in message")
		}

		// Should not show approver for denied
		if strings.Contains(message, "By:") {
			t.Error("should not show 'By:' in denied message")
		}
	})
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plain text", "plain text"},
		{"<script>alert('xss')</script>", "&lt;script&gt;alert('xss')&lt;/script&gt;"},
		{"A & B", "A &amp; B"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"<tag>content</tag>", "&lt;tag&gt;content&lt;/tag&gt;"},
		{"&<>\"", "&amp;&lt;&gt;&quot;"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestNewBotFromEnv(t *testing.T) {
	t.Run("with token set", func(t *testing.T) {
		t.Setenv("KINGCRAB_TELEGRAM_BOT_TOKEN", "test-token")
		t.Setenv("KINGCRAB_TELEGRAM_WEBHOOK_URL", "https://webhook.url")
		t.Setenv("KINGCRAB_TELEGRAM_MINIAPP_URL", "https://miniapp.url")

		bot := NewBotFromEnv()
		if bot == nil {
			t.Fatal("expected bot, got nil")
		}

		if bot.token != "test-token" {
			t.Error("expected token from env")
		}
	})

	t.Run("without token", func(t *testing.T) {
		// Clear env vars
		t.Setenv("KINGCRAB_TELEGRAM_BOT_TOKEN", "")

		bot := NewBotFromEnv()
		if bot != nil {
			t.Error("expected nil bot when token not set")
		}
	})
}

func TestInlineKeyboardButton(t *testing.T) {
	t.Run("URL button", func(t *testing.T) {
		btn := InlineKeyboardButton{
			Text: "Click me",
			URL:  "https://example.com",
			Type: "url",
		}

		if btn.Text != "Click me" {
			t.Error("text mismatch")
		}
		if btn.URL != "https://example.com" {
			t.Error("URL mismatch")
		}
		if btn.CallbackData != "" {
			t.Error("URL button should not have callback data")
		}
	})

	t.Run("callback button", func(t *testing.T) {
		btn := InlineKeyboardButton{
			Text:         "Action",
			CallbackData: "action_123",
			Type:         "callback",
		}

		if btn.CallbackData != "action_123" {
			t.Error("callback data mismatch")
		}
		if btn.URL != "" {
			t.Error("callback button should not have URL")
		}
	})
}

func TestAPIResponse(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		resp := APIResponse{
			Ok:     true,
			Result: []byte(`{"message_id": 123}`),
		}

		if !resp.Ok {
			t.Error("expected Ok to be true")
		}
	})

	t.Run("error response", func(t *testing.T) {
		resp := APIResponse{
			Ok:          false,
			ErrorCode:   400,
			Description: "Bad Request",
		}

		if resp.Ok {
			t.Error("expected Ok to be false")
		}

		if resp.ErrorCode != 400 {
			t.Errorf("expected error code 400, got %d", resp.ErrorCode)
		}

		if resp.Description != "Bad Request" {
			t.Errorf("expected description 'Bad Request', got %s", resp.Description)
		}
	})
}

func TestElevationRequestType(t *testing.T) {
	expiresAt := time.Now().Add(10 * time.Minute)

	req := &ElevationRequest{
		ID:           "req-123",
		TargetSystem: "api-server",
		Command:      "docker restart api",
		Reason:       "memory leak",
		Requester:    "ops-team",
		ExpiresAt:    expiresAt,
	}

	if req.ID != "req-123" {
		t.Error("ID mismatch")
	}
	if req.TargetSystem != "api-server" {
		t.Error("TargetSystem mismatch")
	}
	if req.Command != "docker restart api" {
		t.Error("Command mismatch")
	}
	if req.Reason != "memory leak" {
		t.Error("Reason mismatch")
	}
	if req.Requester != "ops-team" {
		t.Error("Requester mismatch")
	}
	if !req.ExpiresAt.Equal(expiresAt) {
		t.Error("ExpiresAt mismatch")
	}
}