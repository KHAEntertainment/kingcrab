package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/KHAEntertainment/kingcrab/internal/pam"
)

// WebhookHandler handles Telegram webhook callbacks
type WebhookHandler struct {
	bot          *Bot
	requestStore pam.RequestStore
	notifyURL    string // URL to call when request is approved/denied
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(bot *Bot, requestStore pam.RequestStore, notifyURL string) *WebhookHandler {
	return &WebhookHandler{
		bot:          bot,
		requestStore: requestStore,
		notifyURL:    notifyURL,
	}
}

// Handle processes incoming webhook requests
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	// Get request
	req, err := h.requestStore.Get(ctx, requestID)
	if err != nil || req == nil {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Request not found", true)
		return
	}

	// Check if already processed
	if req.Status != "pending" {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Request already processed", false)
		return
	}

	// Update request status
	req.Status = "denied"
	if err := h.requestStore.Update(ctx, req); err != nil {
		h.bot.AnswerCallbackQuery(ctx, cq.ID, "Error updating request", true)
		return
	}

	// Send confirmation
	h.bot.AnswerCallbackQuery(ctx, cq.ID, "Request denied", false)

	// Update message
	msg := h.bot.BuildApprovalResultMessage(toElevationRequest(req), false, cq.From.Username)
	h.bot.EditMessageText(ctx, cq.Message.Chat.ID, cq.Message.MessageID, msg, nil)

	// Notify external system if configured
	if h.notifyURL != "" {
		go h.notifyExternal(ctx, req.ID, "denied", "")
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
	// Simple POST to notify URL
	// In production, would use proper HTTP client with retries
	fmt.Printf("Notify external: %s %s %s\n", h.notifyURL, requestID, status)
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
	ID       string   `json:"id"`
	From     *User    `json:"from"`
	Message  *Message `json:"message,omitempty"`
	Data     string   `json:"data,omitempty"`
	ChatInstance int64 `json:"chat_instance"`
}
