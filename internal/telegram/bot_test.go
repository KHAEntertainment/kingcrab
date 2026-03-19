package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewBot(t *testing.T) {
	token := "test_token_123"
	webhookURL := "https://example.com/webhook"
	miniAppURL := "https://example.com/app"

	bot := NewBot(token, webhookURL, miniAppURL)

	if bot == nil {
		t.Fatal("Expected bot, got nil")
	}

	if bot.token != token {
		t.Errorf("Expected token %s, got %s", token, bot.token)
	}

	if bot.webhookURL != webhookURL {
		t.Errorf("Expected webhook URL %s, got %s", webhookURL, bot.webhookURL)
	}

	if bot.miniAppURL != miniAppURL {
		t.Errorf("Expected mini app URL %s, got %s", miniAppURL, bot.miniAppURL)
	}

	if bot.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if bot.httpClient.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", bot.httpClient.Timeout)
	}
}

func TestBuildApprovalKeyboard(t *testing.T) {
	bot := NewBot("token", "webhook", "https://example.com/app")
	requestID := "test-request-123"

	keyboard := bot.BuildApprovalKeyboard(requestID)

	if keyboard == nil {
		t.Fatal("Expected keyboard, got nil")
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(keyboard.InlineKeyboard))
	}

	// First row: Approve button
	approveRow := keyboard.InlineKeyboard[0]
	if len(approveRow) != 1 {
		t.Fatalf("Expected 1 button in first row, got %d", len(approveRow))
	}

	approveBtn := approveRow[0]
	if approveBtn.Text != "🔐 Authenticate & Approve" {
		t.Errorf("Expected approve text, got %s", approveBtn.Text)
	}

	expectedURL := "https://example.com/app/pam.html?request_id=test-request-123"
	if approveBtn.URL != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, approveBtn.URL)
	}

	// Second row: Deny button
	denyRow := keyboard.InlineKeyboard[1]
	if len(denyRow) != 1 {
		t.Fatalf("Expected 1 button in second row, got %d", len(denyRow))
	}

	denyBtn := denyRow[0]
	if denyBtn.Text != "❌ Deny" {
		t.Errorf("Expected deny text, got %s", denyBtn.Text)
	}

	expectedCallbackData := "deny_test-request-123"
	if denyBtn.CallbackData != expectedCallbackData {
		t.Errorf("Expected callback data %s, got %s", expectedCallbackData, denyBtn.CallbackData)
	}
}

func TestBuildApprovalKeyboard_TrailingSlash(t *testing.T) {
	bot := NewBot("token", "webhook", "https://example.com/app/")
	requestID := "test-123"

	keyboard := bot.BuildApprovalKeyboard(requestID)
	approveBtn := keyboard.InlineKeyboard[0][0]

	expectedURL := "https://example.com/app/pam.html?request_id=test-123"
	if approveBtn.URL != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, approveBtn.URL)
	}
}

func TestBuildApprovalMessage(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	expiresAt := time.Now().Add(5 * time.Minute)

	req := &ElevationRequest{
		ID:           "test-123",
		TargetSystem: "web-server-01",
		Requester:    "admin@example.com",
		Command:      "systemctl restart nginx",
		Reason:       "Deploy new config",
		ExpiresAt:    expiresAt,
	}

	message := bot.BuildApprovalMessage(req)

	if !strings.Contains(message, "🔐 Elevation Request") {
		t.Error("Expected title in message")
	}

	if !strings.Contains(message, "web-server-01") {
		t.Error("Expected target system in message")
	}

	if !strings.Contains(message, "admin@example.com") {
		t.Error("Expected requester in message")
	}

	if !strings.Contains(message, "systemctl restart nginx") {
		t.Error("Expected command in message")
	}

	if !strings.Contains(message, "Deploy new config") {
		t.Error("Expected reason in message")
	}

	if !strings.Contains(message, "Expires:") {
		t.Error("Expected expiry time in message")
	}
}

func TestBuildApprovalResultMessage_Approved(t *testing.T) {
	bot := NewBot("token", "webhook", "app")

	req := &ElevationRequest{
		ID:           "test-123",
		TargetSystem: "web-server-01",
		Command:      "ls -la",
		Reason:       "Check files",
		ExpiresAt:    time.Now(),
	}

	message := bot.BuildApprovalResultMessage(req, true, "admin")

	if !strings.Contains(message, "✅") {
		t.Error("Expected approve emoji")
	}

	if !strings.Contains(message, "Approved") {
		t.Error("Expected 'Approved' text")
	}

	if !strings.Contains(message, "admin") {
		t.Error("Expected approver name")
	}

	if !strings.Contains(message, "web-server-01") {
		t.Error("Expected target system")
	}

	if !strings.Contains(message, "ls -la") {
		t.Error("Expected command")
	}
}

func TestBuildApprovalResultMessage_Denied(t *testing.T) {
	bot := NewBot("token", "webhook", "app")

	req := &ElevationRequest{
		ID:           "test-123",
		TargetSystem: "db-server-01",
		Command:      "rm -rf data",
		Reason:       "Cleanup",
		ExpiresAt:    time.Now(),
	}

	message := bot.BuildApprovalResultMessage(req, false, "")

	if !strings.Contains(message, "🚫") {
		t.Error("Expected deny emoji")
	}

	if !strings.Contains(message, "Denied") {
		t.Error("Expected 'Denied' text")
	}

	if !strings.Contains(message, "db-server-01") {
		t.Error("Expected target system")
	}

	if !strings.Contains(message, "rm -rf data") {
		t.Error("Expected command")
	}

	// Should not contain approver for denied
	if strings.Contains(message, "By:") {
		t.Error("Should not contain 'By:' for denied request")
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<script>alert('xss')</script>", "&lt;script&gt;alert('xss')&lt;/script&gt;"},
		{"Hello & goodbye", "Hello &amp; goodbye"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"normal text", "normal text"},
		{"<>&\"", "&lt;&gt;&amp;&quot;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("escapeHTML(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewBotFromEnv_NoToken(t *testing.T) {
	// Ensure token env var is not set
	t.Setenv("KINGCRAB_TELEGRAM_BOT_TOKEN", "")

	bot := NewBotFromEnv()
	if bot != nil {
		t.Error("Expected nil when token not set")
	}
}

func TestNewBotFromEnv_WithToken(t *testing.T) {
	t.Setenv("KINGCRAB_TELEGRAM_BOT_TOKEN", "test_token")
	t.Setenv("KINGCRAB_TELEGRAM_WEBHOOK_URL", "https://webhook.com")
	t.Setenv("KINGCRAB_TELEGRAM_MINIAPP_URL", "https://app.com")

	bot := NewBotFromEnv()
	if bot == nil {
		t.Fatal("Expected bot, got nil")
	}

	if bot.token != "test_token" {
		t.Errorf("Expected token 'test_token', got %s", bot.token)
	}

	if bot.webhookURL != "https://webhook.com" {
		t.Errorf("Expected webhook URL, got %s", bot.webhookURL)
	}

	if bot.miniAppURL != "https://app.com" {
		t.Errorf("Expected mini app URL, got %s", bot.miniAppURL)
	}
}

func TestInlineKeyboardMarkup_Structure(t *testing.T) {
	keyboard := InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "Button 1", URL: "https://example.com"},
			},
			{
				{Text: "Button 2", CallbackData: "callback_data"},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(keyboard.InlineKeyboard))
	}

	if keyboard.InlineKeyboard[0][0].Text != "Button 1" {
		t.Error("First button text mismatch")
	}

	if keyboard.InlineKeyboard[1][0].CallbackData != "callback_data" {
		t.Error("Second button callback data mismatch")
	}
}

func TestElevationRequest_Structure(t *testing.T) {
	expiresAt := time.Now().Add(10 * time.Minute)
	req := ElevationRequest{
		ID:           "test-id",
		TargetSystem: "server-01",
		Command:      "reboot",
		Reason:       "maintenance",
		Requester:    "ops",
		ExpiresAt:    expiresAt,
	}

	if req.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", req.ID)
	}

	if req.Command != "reboot" {
		t.Errorf("Expected command 'reboot', got %s", req.Command)
	}

	if !req.ExpiresAt.Equal(expiresAt) {
		t.Error("ExpiresAt mismatch")
	}
}

func TestAPIResponse_Structure(t *testing.T) {
	resp := APIResponse{
		Ok:          true,
		ErrorCode:   0,
		Description: "Success",
	}

	if !resp.Ok {
		t.Error("Expected Ok true")
	}

	if resp.Description != "Success" {
		t.Errorf("Expected description 'Success', got %s", resp.Description)
	}
}

// Test that SendMessage would construct proper payload (without actually sending)
func TestBot_SendMessage_Payload(t *testing.T) {
	bot := NewBot("token", "webhook", "app")

	// We can't easily test the actual HTTP call without a test server,
	// but we can verify the bot is properly initialized
	if bot.token != "token" {
		t.Error("Token not set correctly")
	}

	// Verify context cancellation doesn't panic
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This will fail to send but shouldn't panic
	_ = bot.SendMessage(ctx, 123, "test", nil)
}

// Test HTML escaping with command examples
func TestEscapeHTML_CommandExamples(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "script tag",
			command:  "echo '<script>alert(1)</script>'",
			expected: "echo '&lt;script&gt;alert(1)&lt;/script&gt;'",
		},
		{
			name:     "ampersand",
			command:  "curl https://api.com?foo=1&bar=2",
			expected: "curl https://api.com?foo=1&amp;bar=2",
		},
		{
			name:     "quotes",
			command:  `echo "Hello World"`,
			expected: `echo &quot;Hello World&quot;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeHTML(tt.command)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}