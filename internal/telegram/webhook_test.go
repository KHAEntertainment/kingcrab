package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/pam"
)

func TestNewWebhookHandler(t *testing.T) {
	bot := NewBot("token", "", "")
	store := pam.NewInMemoryRequestStore()
	notifyURL := "https://example.com/notify"

	handler := NewWebhookHandler(bot, store, notifyURL)

	if handler.bot != bot {
		t.Error("Handler bot should be set")
	}

	if handler.requestStore != store {
		t.Error("Handler requestStore should be set")
	}

	if handler.notifyURL != notifyURL {
		t.Errorf("notifyURL = %s, want %s", handler.notifyURL, notifyURL)
	}
}

func TestWebhookHandler_ServeHTTP_InvalidJSON(t *testing.T) {
	handler := NewWebhookHandler(nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_HandleStart(t *testing.T) {
	// Create mock HTTP server for bot API
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
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	update := Update{
		Message: &Message{
			Chat: &Chat{ID: 123456},
			Text: "/start",
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if lastPayload["chat_id"].(float64) != 123456 {
		t.Error("SendMessage should be called with correct chat ID")
	}

	if lastPayload["text"] == nil || lastPayload["text"] == "" {
		t.Error("SendMessage should be called with welcome text")
	}
}

func TestWebhookHandler_HandleStatus(t *testing.T) {
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
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	// Create pending request
	req := pam.NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(context.Background(), req)

	update := Update{
		Message: &Message{
			Chat: &Chat{ID: 123456},
			Text: "/status",
		},
	}

	body, _ := json.Marshal(update)
	httpReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if lastPayload["chat_id"].(float64) != 123456 {
		t.Error("SendMessage should be called")
	}

	// Verify message mentions pending request
	text := lastPayload["text"].(string)
	if !strings.Contains(text, "1 pending") {
		t.Errorf("Status message should mention pending request, got: %s", text)
	}
}

func TestWebhookHandler_HandleCallbackQuery_InvalidData(t *testing.T) {
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
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	update := Update{
		CallbackQuery: &CallbackQuery{
			ID:   "callback-id",
			Data: "",
			From: &User{Username: "testuser"},
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if lastPayload["callback_query_id"] != "callback-id" {
		t.Error("AnswerCallbackQuery should be called")
	}
}

func TestWebhookHandler_HandleDeny(t *testing.T) {
	var lastPayloadMap = make(map[string]map[string]interface{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		// Store by method path
		lastPayloadMap[r.URL.Path] = payload
		json.NewEncoder(w).Encode(APIResponse{Ok: true})
	}))
	defer server.Close()

	bot := NewBot("test-token", "", "")
	bot.httpClient = &http.Client{
		Transport: &mockTransport{server: server},
	}
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	// Create pending request
	ctx := context.Background()
	req := pam.NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	update := Update{
		CallbackQuery: &CallbackQuery{
			ID:   "callback-id",
			Data: "deny_" + req.ID,
			From: &User{Username: "testuser"},
			Message: &Message{
				MessageID: 789,
				Chat:      &Chat{ID: 123456},
			},
		},
	}

	body, _ := json.Marshal(update)
	httpReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	// Verify request was denied
	updated, _ := store.Get(ctx, req.ID)
	if updated.Status != "denied" {
		t.Errorf("Request status = %s, want 'denied'", updated.Status)
	}

	// Check that API methods were called (check payload map has entries)
	if len(lastPayloadMap) == 0 {
		t.Error("No API calls were made")
	}
}

func TestWebhookHandler_HandleDeny_RequestNotFound(t *testing.T) {
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
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	update := Update{
		CallbackQuery: &CallbackQuery{
			ID:   "callback-id",
			Data: "deny_nonexistent-id",
			From: &User{Username: "testuser"},
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if lastPayload["callback_query_id"] != "callback-id" {
		t.Error("AnswerCallbackQuery should be called")
	}

	if lastPayload["show_alert"].(bool) != true {
		t.Error("Should show alert for not found request")
	}
}

func TestWebhookHandler_HandleDeny_AlreadyProcessed(t *testing.T) {
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
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	// Create already approved request
	ctx := context.Background()
	req := pam.NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	req.Status = "approved"
	store.Create(ctx, req)

	update := Update{
		CallbackQuery: &CallbackQuery{
			ID:   "callback-id",
			Data: "deny_" + req.ID,
			From: &User{Username: "testuser"},
		},
	}

	body, _ := json.Marshal(update)
	httpReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	// Status should remain approved
	updated, _ := store.Get(ctx, req.ID)
	if updated.Status != "approved" {
		t.Errorf("Request status = %s, want 'approved'", updated.Status)
	}
}

func TestWebhookHandler_HandleCallbackQuery_UnknownAction(t *testing.T) {
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
	store := pam.NewInMemoryRequestStore()

	handler := NewWebhookHandler(bot, store, "")

	update := Update{
		CallbackQuery: &CallbackQuery{
			ID:   "callback-id",
			Data: "unknown_action",
			From: &User{Username: "testuser"},
		},
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if lastPayload["callback_query_id"] != "callback-id" {
		t.Error("AnswerCallbackQuery should be called for unknown action")
	}
}

func TestWebhookHandler_UnhandledUpdate(t *testing.T) {
	handler := NewWebhookHandler(nil, nil, "")

	update := Update{
		UpdateID: 123,
		// No message or callback query
	}

	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d (unhandled updates should return OK)", w.Code, http.StatusOK)
	}
}

func TestToElevationRequest(t *testing.T) {
	now := time.Now()
	pamReq := &pam.ElevationRequest{
		ID:           "test-id",
		TargetSystem: "web-01",
		Command:      "systemctl restart nginx",
		Reason:       "Deploy",
		Requester:    "admin",
		ExpiresAt:    now,
	}

	telegramReq := toElevationRequest(pamReq)

	if telegramReq.ID != pamReq.ID {
		t.Errorf("ID = %s, want %s", telegramReq.ID, pamReq.ID)
	}

	if telegramReq.TargetSystem != pamReq.TargetSystem {
		t.Errorf("TargetSystem = %s, want %s", telegramReq.TargetSystem, pamReq.TargetSystem)
	}

	if telegramReq.Command != pamReq.Command {
		t.Errorf("Command = %s, want %s", telegramReq.Command, pamReq.Command)
	}

	if telegramReq.Reason != pamReq.Reason {
		t.Errorf("Reason = %s, want %s", telegramReq.Reason, pamReq.Reason)
	}

	if telegramReq.Requester != pamReq.Requester {
		t.Errorf("Requester = %s, want %s", telegramReq.Requester, pamReq.Requester)
	}

	if !telegramReq.ExpiresAt.Equal(pamReq.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", telegramReq.ExpiresAt, pamReq.ExpiresAt)
	}
}