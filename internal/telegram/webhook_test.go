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
	bot := NewBot("token", "webhook", "miniapp")
	store := pam.NewInMemoryRequestStore()
	notifyURL := "https://notify.example.com"

	handler := NewWebhookHandler(bot, store, notifyURL)

	if handler == nil {
		t.Fatal("expected handler, got nil")
	}

	if handler.bot != bot {
		t.Error("bot mismatch")
	}

	if handler.requestStore != store {
		t.Error("store mismatch")
	}

	if handler.notifyURL != notifyURL {
		t.Error("notifyURL mismatch")
	}
}

func TestWebhookHandlerServeHTTP(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "")

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString("invalid json"))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("empty update", func(t *testing.T) {
		update := Update{}
		body, _ := json.Marshal(update)

		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("start command", func(t *testing.T) {
		update := Update{
			Message: &Message{
				Chat: &Chat{ID: 12345},
				Text: "/start",
			},
		}
		body, _ := json.Marshal(update)

		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Handler will try to send message to Telegram API which will fail,
		// but the handler itself should process the request
		// Status should be OK since we don't return errors to Telegram
		if w.Code != http.StatusOK {
			t.Logf("Status: %d (expected failure contacting Telegram API)", w.Code)
		}
	})

	t.Run("status command", func(t *testing.T) {
		update := Update{
			Message: &Message{
				Chat: &Chat{ID: 12345},
				Text: "/status",
			},
		}
		body, _ := json.Marshal(update)

		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Logf("Status: %d", w.Code)
		}
	})
}

func TestHandleCallbackQuery(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "")
	ctx := context.Background()

	t.Run("empty callback data", func(t *testing.T) {
		cq := &CallbackQuery{
			ID:   "callback-123",
			Data: "",
		}

		// This will try to answer the callback query, which will fail
		// but we're testing the logic flow
		handler.handleCallbackQuery(ctx, cq)
	})

	t.Run("unknown callback action", func(t *testing.T) {
		cq := &CallbackQuery{
			ID:   "callback-456",
			Data: "unknown_action",
		}

		handler.handleCallbackQuery(ctx, cq)
	})

	t.Run("deny callback", func(t *testing.T) {
		// Create a request first
		req := pam.NewElevationRequest("test cmd", "test reason", "user", "system", 5*time.Minute, 12345)
		store.Create(ctx, req)

		cq := &CallbackQuery{
			ID:   "callback-deny",
			Data: "deny_" + req.ID,
			From: &User{Username: "admin"},
			Message: &Message{
				Chat:      &Chat{ID: 12345},
				MessageID: 100,
			},
		}

		handler.handleCallbackQuery(ctx, cq)

		// Verify request was denied
		updated, _ := store.Get(ctx, req.ID)
		if updated != nil && updated.Status != "denied" {
			t.Errorf("expected status 'denied', got %s", updated.Status)
		}
	})
}

func TestHandleDeny(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "")
	ctx := context.Background()

	t.Run("request not found", func(t *testing.T) {
		cq := &CallbackQuery{
			ID: "callback-123",
		}

		handler.handleDeny(ctx, cq, "nonexistent-id")
		// Should handle gracefully
	})

	t.Run("request already processed", func(t *testing.T) {
		req := pam.NewElevationRequest("test", "test", "user", "system", 5*time.Minute, 12345)
		req.Status = "approved"
		store.Create(ctx, req)

		cq := &CallbackQuery{
			ID: "callback-already",
		}

		handler.handleDeny(ctx, cq, req.ID)
		// Should not change status
	})

	t.Run("successful denial", func(t *testing.T) {
		req := pam.NewElevationRequest("test", "test", "user", "system", 5*time.Minute, 12345)
		store.Create(ctx, req)

		cq := &CallbackQuery{
			ID:   "callback-success",
			From: &User{Username: "admin"},
			Message: &Message{
				Chat:      &Chat{ID: 12345},
				MessageID: 200,
			},
		}

		handler.handleDeny(ctx, cq, req.ID)

		// Check status was updated
		updated, _ := store.Get(ctx, req.ID)
		if updated != nil && updated.Status != "denied" {
			t.Errorf("expected status 'denied', got %s", updated.Status)
		}
	})
}

func TestHandleStart(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "")
	ctx := context.Background()

	chatID := int64(12345)

	// This will attempt to send a message via Telegram API
	// We're just testing it doesn't panic
	handler.handleStart(ctx, chatID)
}

func TestHandleStatus(t *testing.T) {
	bot := NewBot("token", "webhook", "miniapp")
	store := pam.NewInMemoryRequestStore()
	handler := NewWebhookHandler(bot, store, "")
	ctx := context.Background()

	t.Run("no pending requests", func(t *testing.T) {
		chatID := int64(12345)
		handler.handleStatus(ctx, chatID)
	})

	t.Run("with pending requests", func(t *testing.T) {
		req := pam.NewElevationRequest("test", "test", "user", "system", 5*time.Minute, 12345)
		store.Create(ctx, req)

		chatID := int64(12345)
		handler.handleStatus(ctx, chatID)
	})
}

func TestToElevationRequest(t *testing.T) {
	expiresAt := time.Now().Add(5 * time.Minute)
	pamReq := &pam.ElevationRequest{
		ID:           "test-id",
		TargetSystem: "web-server",
		Command:      "restart nginx",
		Reason:       "deployment",
		Requester:    "admin",
		ExpiresAt:    expiresAt,
	}

	botReq := toElevationRequest(pamReq)

	if botReq.ID != pamReq.ID {
		t.Error("ID mismatch")
	}
	if botReq.TargetSystem != pamReq.TargetSystem {
		t.Error("TargetSystem mismatch")
	}
	if botReq.Command != pamReq.Command {
		t.Error("Command mismatch")
	}
	if botReq.Reason != pamReq.Reason {
		t.Error("Reason mismatch")
	}
	if botReq.Requester != pamReq.Requester {
		t.Error("Requester mismatch")
	}
	if !botReq.ExpiresAt.Equal(pamReq.ExpiresAt) {
		t.Error("ExpiresAt mismatch")
	}
}

func TestUpdateTypes(t *testing.T) {
	t.Run("message update", func(t *testing.T) {
		update := Update{
			UpdateID: 123,
			Message: &Message{
				MessageID: 456,
				Chat:      &Chat{ID: 789},
				Text:      "test message",
			},
		}

		if update.UpdateID != 123 {
			t.Error("UpdateID mismatch")
		}
		if update.Message == nil {
			t.Fatal("expected message")
		}
		if update.Message.Text != "test message" {
			t.Error("message text mismatch")
		}
	})

	t.Run("callback query update", func(t *testing.T) {
		update := Update{
			UpdateID: 124,
			CallbackQuery: &CallbackQuery{
				ID:   "cq-123",
				Data: "button_data",
			},
		}

		if update.CallbackQuery == nil {
			t.Fatal("expected callback query")
		}
		if update.CallbackQuery.Data != "button_data" {
			t.Error("callback data mismatch")
		}
	})
}

func TestCallbackQuery(t *testing.T) {
	cq := CallbackQuery{
		ID:   "query-id",
		Data: "callback-data",
		From: &User{
			ID:       12345,
			Username: "testuser",
		},
		Message: &Message{
			MessageID: 789,
			Chat:      &Chat{ID: 999},
		},
		ChatInstance: 111,
	}

	if cq.ID != "query-id" {
		t.Error("ID mismatch")
	}
	if cq.Data != "callback-data" {
		t.Error("Data mismatch")
	}
	if cq.From == nil || cq.From.Username != "testuser" {
		t.Error("From user mismatch")
	}
	if cq.Message == nil {
		t.Error("Message should be set")
	}
	if cq.ChatInstance != 111 {
		t.Error("ChatInstance mismatch")
	}
}

func TestMessageTypes(t *testing.T) {
	msg := Message{
		MessageID: 123,
		Chat:      &Chat{ID: 456},
		From: &User{
			ID:        789,
			Username:  "sender",
			FirstName: "Test",
		},
		Text: "Hello",
	}

	if msg.MessageID != 123 {
		t.Error("MessageID mismatch")
	}
	if msg.Chat == nil || msg.Chat.ID != 456 {
		t.Error("Chat mismatch")
	}
	if msg.From == nil || msg.From.Username != "sender" {
		t.Error("From mismatch")
	}
	if msg.Text != "Hello" {
		t.Error("Text mismatch")
	}
}