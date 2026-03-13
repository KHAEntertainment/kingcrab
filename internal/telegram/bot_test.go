package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewBot(t *testing.T) {
	token := "test-token"
	webhookURL := "https://example.com/webhook"
	miniAppURL := "https://example.com/app"

	bot := NewBot(token, webhookURL, miniAppURL)

	if bot.token != token {
		t.Errorf("token = %s, want %s", bot.token, token)
	}

	if bot.webhookURL != webhookURL {
		t.Errorf("webhookURL = %s, want %s", bot.webhookURL, webhookURL)
	}

	if bot.miniAppURL != miniAppURL {
		t.Errorf("miniAppURL = %s, want %s", bot.miniAppURL, miniAppURL)
	}

	if bot.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewBotFromEnv(t *testing.T) {
	t.Run("creates bot when token set", func(t *testing.T) {
		os.Setenv("KINGCRAB_TELEGRAM_BOT_TOKEN", "test-token")
		os.Setenv("KINGCRAB_TELEGRAM_WEBHOOK_URL", "webhook")
		os.Setenv("KINGCRAB_TELEGRAM_MINIAPP_URL", "app")
		defer os.Unsetenv("KINGCRAB_TELEGRAM_BOT_TOKEN")
		defer os.Unsetenv("KINGCRAB_TELEGRAM_WEBHOOK_URL")
		defer os.Unsetenv("KINGCRAB_TELEGRAM_MINIAPP_URL")

		bot := NewBotFromEnv()
		if bot == nil {
			t.Error("NewBotFromEnv() should return bot when token set")
		}
	})

	t.Run("returns nil when token not set", func(t *testing.T) {
		os.Unsetenv("KINGCRAB_TELEGRAM_BOT_TOKEN")

		bot := NewBotFromEnv()
		if bot != nil {
			t.Error("NewBotFromEnv() should return nil when token not set")
		}
	})
}

func TestBot_BuildApprovalKeyboard(t *testing.T) {
	bot := NewBot("token", "", "https://example.com/app")

	requestID := "test-request-123"
	keyboard := bot.BuildApprovalKeyboard(requestID)

	if keyboard == nil {
		t.Fatal("BuildApprovalKeyboard() returned nil")
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Errorf("Keyboard rows = %d, want 2", len(keyboard.InlineKeyboard))
	}

	// Check approve button
	if len(keyboard.InlineKeyboard[0]) != 1 {
		t.Errorf("First row buttons = %d, want 1", len(keyboard.InlineKeyboard[0]))
	} else {
		approveBtn := keyboard.InlineKeyboard[0][0]
		if approveBtn.Type != "url" {
			t.Errorf("Approve button type = %s, want 'url'", approveBtn.Type)
		}
		if !strings.Contains(approveBtn.URL, requestID) {
			t.Errorf("Approve URL should contain request ID %s, got %s", requestID, approveBtn.URL)
		}
	}

	// Check deny button
	if len(keyboard.InlineKeyboard[1]) != 1 {
		t.Errorf("Second row buttons = %d, want 1", len(keyboard.InlineKeyboard[1]))
	} else {
		denyBtn := keyboard.InlineKeyboard[1][0]
		if denyBtn.Type != "callback" {
			t.Errorf("Deny button type = %s, want 'callback'", denyBtn.Type)
		}
		expectedCallback := "deny_" + requestID
		if denyBtn.CallbackData != expectedCallback {
			t.Errorf("Deny callback = %s, want %s", denyBtn.CallbackData, expectedCallback)
		}
	}
}

func TestBot_BuildApprovalMessage(t *testing.T) {
	bot := NewBot("token", "", "")

	req := &ElevationRequest{
		TargetSystem: "web-01",
		Requester:    "admin",
		Command:      "systemctl restart nginx",
		Reason:       "Deploy new version",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}

	message := bot.BuildApprovalMessage(req)

	if !strings.Contains(message, req.TargetSystem) {
		t.Error("Message should contain target system")
	}

	if !strings.Contains(message, req.Requester) {
		t.Error("Message should contain requester")
	}

	if !strings.Contains(message, req.Command) {
		t.Error("Message should contain command")
	}

	if !strings.Contains(message, req.Reason) {
		t.Error("Message should contain reason")
	}

	if !strings.Contains(message, "<b>") {
		t.Error("Message should contain HTML formatting")
	}
}

func TestBot_BuildApprovalResultMessage(t *testing.T) {
	bot := NewBot("token", "", "")

	req := &ElevationRequest{
		TargetSystem: "web-01",
		Command:      "systemctl restart nginx",
	}

	t.Run("approved message", func(t *testing.T) {
		message := bot.BuildApprovalResultMessage(req, true, "admin")

		if !strings.Contains(message, "Approved") {
			t.Error("Message should contain 'Approved'")
		}

		if !strings.Contains(message, "admin") {
			t.Error("Message should contain approver name")
		}

		if !strings.Contains(message, "✅") {
			t.Error("Message should contain approval emoji")
		}
	})

	t.Run("denied message", func(t *testing.T) {
		message := bot.BuildApprovalResultMessage(req, false, "")

		if !strings.Contains(message, "Denied") {
			t.Error("Message should contain 'Denied'")
		}

		if !strings.Contains(message, "🚫") {
			t.Error("Message should contain denial emoji")
		}
	})
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "normal text",
			want:  "normal text",
		},
		{
			input: "<script>alert('xss')</script>",
			want:  "&lt;script&gt;alert('xss')&lt;/script&gt;",
		},
		{
			input: "rm -rf & dangerous",
			want:  "rm -rf &amp; dangerous",
		},
		{
			input: `"quoted"`,
			want:  "&quot;quoted&quot;",
		},
		{
			input: "& < > \"",
			want:  "&amp; &lt; &gt; &quot;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeHTML(tt.input)
			if got != tt.want {
				t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBot_SendMessage(t *testing.T) {
	// Create mock server
	var lastRequest *http.Request
	var lastPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastRequest = r
		json.NewDecoder(r.Body).Decode(&lastPayload)

		// Return success response
		resp := APIResponse{
			Ok: true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create bot with mock server
	bot := NewBot("test-token", "", "")
	// Replace API URL by setting a custom client that redirects to our server
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}

	ctx := context.Background()
	chatID := int64(123456)
	text := "Test message"
	keyboard := &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{{Text: "Button", URL: "https://example.com"}},
		},
	}

	err := bot.SendMessage(ctx, chatID, text, keyboard)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if lastRequest.Method != "POST" {
		t.Errorf("Method = %s, want POST", lastRequest.Method)
	}

	if lastPayload["chat_id"].(float64) != float64(chatID) {
		t.Errorf("chat_id = %v, want %v", lastPayload["chat_id"], chatID)
	}

	if lastPayload["text"] != text {
		t.Errorf("text = %v, want %v", lastPayload["text"], text)
	}

	if lastPayload["parse_mode"] != "HTML" {
		t.Errorf("parse_mode = %v, want HTML", lastPayload["parse_mode"])
	}
}

func TestBot_SendMessage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Ok:          false,
			ErrorCode:   400,
			Description: "Bad Request",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	bot := NewBot("test-token", "", "")
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}

	err := bot.SendMessage(context.Background(), 123, "test", nil)
	if err == nil {
		t.Error("SendMessage() expected error for API error, got nil")
	}

	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("Error message should contain 'API error', got: %v", err)
	}
}

func TestBot_EditMessageText(t *testing.T) {
	var lastPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastPayload)
		json.NewEncoder(w).Encode(APIResponse{Ok: true})
	}))
	defer server.Close()

	bot := NewBot("test-token", "", "")
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}

	err := bot.EditMessageText(context.Background(), 123, 456, "new text", nil)
	if err != nil {
		t.Fatalf("EditMessageText() error = %v", err)
	}

	if lastPayload["message_id"].(float64) != 456 {
		t.Errorf("message_id = %v, want 456", lastPayload["message_id"])
	}

	if lastPayload["text"] != "new text" {
		t.Errorf("text = %v, want 'new text'", lastPayload["text"])
	}
}

func TestBot_AnswerCallbackQuery(t *testing.T) {
	var lastPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastPayload)
		json.NewEncoder(w).Encode(APIResponse{Ok: true})
	}))
	defer server.Close()

	bot := NewBot("test-token", "", "")
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}

	err := bot.AnswerCallbackQuery(context.Background(), "callback-id", "Response text", true)
	if err != nil {
		t.Fatalf("AnswerCallbackQuery() error = %v", err)
	}

	if lastPayload["callback_query_id"] != "callback-id" {
		t.Errorf("callback_query_id = %v, want 'callback-id'", lastPayload["callback_query_id"])
	}

	if lastPayload["text"] != "Response text" {
		t.Errorf("text = %v, want 'Response text'", lastPayload["text"])
	}

	if lastPayload["show_alert"].(bool) != true {
		t.Errorf("show_alert = %v, want true", lastPayload["show_alert"])
	}
}

func TestBot_SetWebhook(t *testing.T) {
	var lastPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastPayload)
		json.NewEncoder(w).Encode(APIResponse{Ok: true})
	}))
	defer server.Close()

	bot := NewBot("test-token", "", "")
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}

	webhookURL := "https://example.com/webhook"
	err := bot.SetWebhook(context.Background(), webhookURL)
	if err != nil {
		t.Fatalf("SetWebhook() error = %v", err)
	}

	if lastPayload["url"] != webhookURL {
		t.Errorf("url = %v, want %v", lastPayload["url"], webhookURL)
	}
}

func TestBot_DeleteWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(APIResponse{Ok: true})
	}))
	defer server.Close()

	bot := NewBot("test-token", "", "")
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}

	err := bot.DeleteWebhook(context.Background())
	if err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}
}

// mockTransport redirects all requests to test server
type mockTransport struct {
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(m.server.URL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}