package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/pam"
)

func TestNewWebhookHandler(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	notifyURL := "https://notify.example.com"
	secretToken := "test-secret"

	handler := NewWebhookHandler(bot, store, notifyURL, secretToken)

	if handler == nil {
		t.Fatal("Expected handler, got nil")
	}

	if handler.bot != bot {
		t.Error("Bot not set correctly")
	}

	if handler.requestStore != store {
		t.Error("Request store not set correctly")
	}

	if handler.notifyURL != notifyURL {
		t.Errorf("Expected notify URL %s, got %s", notifyURL, handler.notifyURL)
	}
}

func TestWebhookHandler_InvalidJSON(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_StartCommand(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	update := Update{
		UpdateID: 123,
		Message: &Message{
			MessageID: 456,
			Chat:      &Chat{ID: 789},
			Text:      "/start",
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_StatusCommand(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	update := Update{
		UpdateID: 123,
		Message: &Message{
			MessageID: 456,
			Chat:      &Chat{ID: 789},
			Text:      "/status",
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_CallbackQuery_EmptyData(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	update := Update{
		UpdateID: 123,
		CallbackQuery: &CallbackQuery{
			ID:   "callback-123",
			From: &User{ID: 456, Username: "testuser"},
			Data: "", // Empty
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_CallbackQuery_UnknownAction(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	update := Update{
		UpdateID: 123,
		CallbackQuery: &CallbackQuery{
			ID:   "callback-123",
			From: &User{ID: 456, Username: "testuser"},
			Data: "unknown_action",
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_DenyRequest_NotFound(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	update := Update{
		UpdateID: 123,
		CallbackQuery: &CallbackQuery{
			ID:   "callback-123",
			From: &User{ID: 456, Username: "testuser"},
			Data: "deny_nonexistent-request-id",
			Message: &Message{
				MessageID: 789,
				Chat:      &Chat{ID: 123},
			},
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_DenyRequest_AlreadyProcessed(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	// Create a request that's already approved
	ctx := context.Background()
	req := pam.NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	req.Status = "approved"
	store.Create(ctx, req)

	update := Update{
		UpdateID: 123,
		CallbackQuery: &CallbackQuery{
			ID:   "callback-123",
			From: &User{ID: 456, Username: "testuser"},
			Data: "deny_" + req.ID,
			Message: &Message{
				MessageID: 789,
				Chat:      &Chat{ID: 123},
			},
		},
	}

	body, _ := json.Marshal(update)
	httpReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestToElevationRequest(t *testing.T) {
	now := time.Now()
	pamReq := &pam.ElevationRequest{
		ID:           "test-123",
		TargetSystem: "server-01",
		Command:      "reboot",
		Reason:       "maintenance",
		Requester:    "admin",
		ExpiresAt:    now,
	}

	telegramReq := toElevationRequest(pamReq)

	if telegramReq.ID != pamReq.ID {
		t.Errorf("Expected ID %s, got %s", pamReq.ID, telegramReq.ID)
	}

	if telegramReq.TargetSystem != pamReq.TargetSystem {
		t.Errorf("Expected target system %s, got %s", pamReq.TargetSystem, telegramReq.TargetSystem)
	}

	if telegramReq.Command != pamReq.Command {
		t.Errorf("Expected command %s, got %s", pamReq.Command, telegramReq.Command)
	}

	if telegramReq.Reason != pamReq.Reason {
		t.Errorf("Expected reason %s, got %s", pamReq.Reason, telegramReq.Reason)
	}

	if telegramReq.Requester != pamReq.Requester {
		t.Errorf("Expected requester %s, got %s", pamReq.Requester, telegramReq.Requester)
	}

	if !telegramReq.ExpiresAt.Equal(pamReq.ExpiresAt) {
		t.Error("ExpiresAt mismatch")
	}
}

func TestUpdate_Structure(t *testing.T) {
	update := Update{
		UpdateID: 123,
		Message: &Message{
			MessageID: 456,
			Chat:      &Chat{ID: 789},
			Text:      "/start",
		},
	}

	if update.UpdateID != 123 {
		t.Errorf("Expected update ID 123, got %d", update.UpdateID)
	}

	if update.Message == nil {
		t.Fatal("Expected message, got nil")
	}

	if update.Message.Text != "/start" {
		t.Errorf("Expected text '/start', got %s", update.Message.Text)
	}
}

func TestMessage_Structure(t *testing.T) {
	msg := Message{
		MessageID: 123,
		Chat:      &Chat{ID: 456},
		From:      &User{ID: 789, Username: "test"},
		Text:      "Hello",
	}

	if msg.MessageID != 123 {
		t.Errorf("Expected message ID 123, got %d", msg.MessageID)
	}

	if msg.Chat.ID != 456 {
		t.Errorf("Expected chat ID 456, got %d", msg.Chat.ID)
	}

	if msg.From.Username != "test" {
		t.Errorf("Expected username 'test', got %s", msg.From.Username)
	}

	if msg.Text != "Hello" {
		t.Errorf("Expected text 'Hello', got %s", msg.Text)
	}
}

func TestCallbackQuery_Structure(t *testing.T) {
	cq := CallbackQuery{
		ID:   "callback-123",
		From: &User{ID: 456, Username: "user"},
		Message: &Message{
			MessageID: 789,
			Chat:      &Chat{ID: 111},
		},
		Data:         "button_data",
		ChatInstance: "999",
	}

	if cq.ID != "callback-123" {
		t.Errorf("Expected ID 'callback-123', got %s", cq.ID)
	}

	if cq.From.ID != 456 {
		t.Errorf("Expected user ID 456, got %d", cq.From.ID)
	}

	if cq.Data != "button_data" {
		t.Errorf("Expected data 'button_data', got %s", cq.Data)
	}

	if cq.ChatInstance != "999" {
		t.Errorf("Expected chat instance '999', got %s", cq.ChatInstance)
	}
}

func TestUser_Structure(t *testing.T) {
	user := User{
		ID:        12345,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	}

	if user.ID != 12345 {
		t.Errorf("Expected ID 12345, got %d", user.ID)
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", user.Username)
	}

	if user.FirstName != "Test" {
		t.Errorf("Expected first name 'Test', got %s", user.FirstName)
	}

	if user.LastName != "User" {
		t.Errorf("Expected last name 'User', got %s", user.LastName)
	}
}

func TestWebhookHandler_EmptyUpdate(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	// Empty update (no message, no callback)
	update := Update{
		UpdateID: 123,
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_StatusCommand_WithPendingRequests(t *testing.T) {
	bot := NewBot("token", "webhook", "app")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "", "")

	// Add pending requests
	ctx := context.Background()
	req1 := pam.NewElevationRequest("cmd1", "reason1", "user", "system", 5*time.Minute, 123)
	req2 := pam.NewElevationRequest("cmd2", "reason2", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req1)
	store.Create(ctx, req2)

	update := Update{
		UpdateID: 123,
		Message: &Message{
			MessageID: 456,
			Chat:      &Chat{ID: 789},
			Text:      "/status",
		},
	}

	body, _ := json.Marshal(update)
	httpReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}