package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/pam"
)

// WebhookHandler handles Telegram webhook callbacks
type WebhookHandler struct {
	bot          *Bot
	requestStore pam.RequestStore
	notifyURL    string // URL to call when request is approved/denied
	secretToken  string // Secret token for webhook verification
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(bot *Bot, requestStore pam.RequestStore, notifyURL string, secretToken string) *WebhookHandler {
	return &WebhookHandler{
		bot:          bot,
		requestStore: requestStore,
		notifyURL:    notifyURL,
		secretToken:  secretToken,
	}
}

// Handle processes incoming webhook requests
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify webhook origin using secret token
	if h.secretToken != "" {
		receivedToken := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if receivedToken != h.secretToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Handle callback query (inline button press)
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(ctx, update.CallbackQuery)
		return
	}

	// Handle /start command
	if update.Message != nil && update.Message.Text == "/start" {
		h.handleStart(ctx, update.Message.Chat.ID)
		return
	}

	// Handle /status command
	if update.Message != nil && update.Message.Text == "/status" {
		h.handleStatus(ctx, update.Message.Chat.ID)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleCallbackQuery handles inline button presses
func (h *WebhookHandler) handleCallbackQuery(ctx context.Context, cq *CallbackQuery) {
	// Parse callback data
	data := cq.Data
	if data == "" {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Invalid request", true)
		return
	}

	// Handle deny buttons
	if len(data) > 5 && data[:5] == "deny_" {
		requestID := data[5:]
		h.handleDeny(ctx, cq, requestID)
		return
	}

	// Unknown callback
	h.bot.AnswerCallbackQuery(ctx, cq.ID, "Unknown action", false)
}

// handleDeny handles denial of a request
func (h *WebhookHandler) handleDeny(ctx context.Context, cq *CallbackQuery, requestID string) {
	// Guard against nil From
	if cq.From == nil {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Invalid callback query", true)
		return
	}

	// Get request
	req, err := h.requestStore.Get(ctx, requestID)
	if err != nil || req == nil {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Request not found", true)
		return
	}

	// Atomically deny the request (only if still pending)
	username := cq.From.Username
	if username == "" {
		username = fmt.Sprintf("tg:%d", cq.From.ID)
	}

	success, err := h.requestStore.UpdateStateIf(ctx, requestID, "pending", "denied", username)
	if err != nil {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Error updating request", true)
		return
	}

	if !success {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Request already processed", false)
		return
	}

	// Send confirmation
	h.bot.AnswerCallbackQuery(ctx, cq.ID, "Request denied", false)

	// Update message if available
	if cq.Message != nil && cq.Message.Chat != nil {
		// Compute approver string with fallback for display
		approver := cq.From.Username
		if approver == "" {
			approver = fmt.Sprintf("tg:%d", cq.From.ID)
		}
		msg := h.bot.BuildApprovalResultMessage(toElevationRequest(req), false, approver)
		h.bot.EditMessageText(ctx, cq.Message.Chat.ID, cq.Message.MessageID, msg, nil)
	}

	// Notify external system if configured
	if h.notifyURL != "" {
		go h.notifyExternal(ctx, req.ID, "denied", username)
	}
}

// handleStart handles /start command
func (h *WebhookHandler) handleStart(ctx context.Context, chatID int64) {
	text := `<b>Welcome to KingCrab PAM! 🦀</b>

This bot handles elevation request approvals for your systems.

<b>Commands:</b>
/start - Show this message
/status - Check bot status

You'll receive elevation requests here when they need approval.`
	h.bot.SendMessage(ctx, chatID, text, nil)
}

// handleStatus handles /status command
func (h *WebhookHandler) handleStatus(ctx context.Context, chatID int64) {
	// Count pending requests
	pending, err := h.requestStore.ListPending(ctx)
	status := "✅ Bot is running"

	if err == nil && len(pending) > 0 {
		status = fmt.Sprintf("✅ Bot running - %d pending request(s)", len(pending))
	}

	h.bot.SendMessage(ctx, chatID, status, nil)
}

// notifyExternal notifies an external system of status change
func (h *WebhookHandler) notifyExternal(ctx context.Context, requestID, status, approver string) {
	// Build JSON payload
	payload := map[string]interface{}{
		"request_id": requestID,
		"status":     status,
		"approver":   approver,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal notification payload: %v", err)
		return
	}

	// Retry logic: 3 attempts with exponential backoff
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create context with 5 second timeout for each attempt
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

		// Create HTTP request
		req, err := http.NewRequestWithContext(reqCtx, "POST", h.notifyURL, bytes.NewReader(payloadBytes))
		if err != nil {
			log.Printf("Failed to create notification request (attempt %d/%d): %v", attempt, maxRetries, err)
			cancel()
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		// Execute request
		client := &http.Client{}
		resp, err := client.Do(req)
		cancel() // Always cancel context after request

		if err != nil {
			log.Printf("Failed to send notification (attempt %d/%d): %v", attempt, maxRetries, err)
			if attempt < maxRetries {
				// Exponential backoff: 100ms, 200ms, 400ms
				backoff := time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
				time.Sleep(backoff)
				continue
			}
			return
		}

		resp.Body.Close()

		// Check response status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Successfully notified external system: %s - %s", requestID, status)
			return
		}

		log.Printf("External notification returned status %d (attempt %d/%d)", resp.StatusCode, attempt, maxRetries)

		// Retry on 5xx errors, fail on 4xx
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			log.Printf("External notification failed with client error %d, not retrying", resp.StatusCode)
			return
		}

		if attempt < maxRetries {
			backoff := time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	log.Printf("Failed to notify external system after %d attempts", maxRetries)
}

// toElevationRequest converts internal request to bot request
func toElevationRequest(req *pam.ElevationRequest) *ElevationRequest {
	return &ElevationRequest{
		ID:           req.ID,
		TargetSystem: req.TargetSystem,
		Command:      req.Command,
		Reason:       req.Reason,
		Requester:    req.Requester,
		ExpiresAt:    req.ExpiresAt,
	}
}

// Compile-time check
var _ http.Handler = (*WebhookHandler)(nil)

// ==================== Types ====================

// Update represents a Telegram update
type Update struct {
	UpdateID      int            `json:"update_id"`
	Message      *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// Message represents a Telegram message
type Message struct {
	MessageID int           `json:"message_id"`
	Chat      *Chat         `json:"chat"`
	From      *User         `json:"from,omitempty"`
	Text      string        `json:"text,omitempty"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID int64 `json:"id"`
}

// User represents a Telegram user
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// CallbackQuery represents a callback query
type CallbackQuery struct {
	ID           string   `json:"id"`
	From         *User    `json:"from"`
	Message      *Message `json:"message,omitempty"`
	Data         string   `json:"data,omitempty"`
	ChatInstance string   `json:"chat_instance"`
}