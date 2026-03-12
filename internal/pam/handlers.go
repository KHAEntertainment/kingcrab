package pam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ==================== HTTP Handlers ====================

// Handler handles PAM HTTP requests
type Handler struct {
	pam           *PAM
	requestStore  RequestStore
	botToken      string
	allowedUsers  []User
	requestTTL    time.Duration
	notifyWebhook string // URL to call on approval
}

// NewHandler creates a new PAM handler
func NewHandler(pam *PAM, requestStore RequestStore, botToken string, allowedUsers []User, ttlMinutes int) *Handler {
	// If no allowed users provided, try to load from store
	if len(allowedUsers) == 0 {
		allowedUsers = loadAllowedUsers(requestStore)
	}

	return &Handler{
		pam:          pam,
		requestStore: requestStore,
		botToken:     botToken,
		allowedUsers: allowedUsers,
		requestTTL:   time.Duration(ttlMinutes) * time.Minute,
	}
}

// loadAllowedUsers tries to load users from the store
func loadAllowedUsers(store RequestStore) []User {
	// Try DB store first
	if dbStore, ok := store.(*DBRequestStore); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if users, err := dbStore.GetAuthorizedUsers(ctx); err == nil && len(users) > 0 {
			return users
		}
	}
	return nil
}

// Request types
type EnrollRequest struct {
	InitData     string `json:"initData"`
	DeviceInfo   string `json:"device_info"`
	BiometricToken string `json:"biometric_token"`
}

type EnrollResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"`
}

type ApproveRequest struct {
	InitData        string `json:"initData"`
	BiometricToken  string `json:"biometric_token"`
	RequestID       string `json:"request_id"`
}

type ApproveResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type CreateRequestRequest struct {
	Command      string `json:"command"`
	Reason       string `json:"reason"`
	Requester    string `json:"requester"`
	TargetSystem string `json:"target_system"`
	// Telegram-specific
	NotifyChatID int64 `json:"notify_chat_id"`
}

type CreateRequestResponse struct {
	RequestID    string    `json:"request_id"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	ApprovalURL  string    `json:"approval_url,omitempty"`
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for Mini App
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Telegram-Init-Data")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Route requests
	switch r.URL.Path {
	case "/api/pam/enroll":
		h.handleEnroll(w, r)
	case "/api/pam/approve":
		h.handleApprove(w, r)
	case "/api/pam/request":
		h.handleCreateRequest(w, r)
	case "/api/pam/health":
		h.handleHealth(w, r)
	default:
		// Check for /api/pam/request/{id} pattern
		if len(r.URL.Path) > len("/api/pam/request/") && r.URL.Path[:len("/api/pam/request/")] == "/api/pam/request/" {
			h.handleGetRequest(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

// handleEnroll handles biometric enrollment
func (h *Handler) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate initData
	initData, err := ValidateInitDataFromRequest(req.InitData, h.botToken)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, fmt.Sprintf("invalid initData: %v", err))
		return
	}

	// Check authorization
	if err := CheckAuthorization(initData, h.allowedUsers); err != nil {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	}

	// Store the biometric token
	userID := InitDataToUserID(initData)
	token := Token{
		Value:        req.BiometricToken,
		DeviceInfo:   req.DeviceInfo,
		EnrolledAt:   time.Now(),
		LastUsedAt:   time.Now(),
		TokenStorage: "local", // or "clawvault" based on PAM config
	}

	if err := h.pam.StoreToken(context.Background(), userID, token); err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to store token: %v", err))
		return
	}

	h.respond(w, EnrollResponse{
		Success: true,
		Message: "Device enrolled successfully",
		UserID:  userID,
	})
}

// handleApprove handles elevation request approval
func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate initData
	initData, err := ValidateInitDataFromRequest(req.InitData, h.botToken)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, fmt.Sprintf("invalid initData: %v", err))
		return
	}

	// Check authorization
	if err := CheckAuthorization(initData, h.allowedUsers); err != nil {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	}

	// Verify biometric token
	userID := InitDataToUserID(initData)
	storedToken, err := h.pam.RetrieveToken(context.Background(), userID)
	if err != nil || storedToken == nil {
		h.respondError(w, http.StatusUnauthorized, "no enrolled device found")
		return
	}

	// Verify token matches (in production, would do proper token comparison)
	if storedToken.Value != req.BiometricToken {
		h.respondError(w, http.StatusUnauthorized, "biometric token mismatch")
		return
	}

	// Update token last used
	storedToken.LastUsedAt = time.Now()
	h.pam.StoreToken(context.Background(), userID, *storedToken)

	// Find and update the pending request via store
	ctx := context.Background()
	pending, err := h.requestStore.Get(ctx, req.RequestID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("store error: %v", err))
		return
	}
	if pending == nil {
		h.respondError(w, http.StatusNotFound, "request not found")
		return
	}

	// Check expiration
	if time.Now().After(pending.ExpiresAt) {
		pending.Status = "expired"
		h.requestStore.Update(ctx, pending)
		h.respondError(w, http.StatusGone, "request expired")
		return
	}

	// Approve the request
	now := time.Now()
	pending.Status = "approved"
	pending.ApprovedBy = userID
	pending.ApprovedAt = &now
	h.requestStore.Update(ctx, pending)

	// TODO: Trigger actual command execution
	// TODO: Call notify webhook

	h.respond(w, ApproveResponse{
		Success:   true,
		Message:   "Elevation request approved",
		RequestID: req.RequestID,
	})
}

// handleCreateRequest creates a new elevation request
func (h *Handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate command is allowed (would check against allowlist)
	// For now, accept any command

	pending := NewElevationRequest(
		req.Command,
		req.Reason,
		req.Requester,
		req.TargetSystem,
		h.requestTTL,
		req.NotifyChatID,
	)

	ctx := context.Background()
	if err := h.requestStore.Create(ctx, pending); err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create request: %v", err))
		return
	}

	// Build approval URL for Mini App
	approvalURL := fmt.Sprintf("/approve?request_id=%s", pending.ID)

	h.respond(w, CreateRequestResponse{
		RequestID:   pending.ID,
		Status:      "pending",
		ExpiresAt:   pending.ExpiresAt,
		ApprovalURL: approvalURL,
	})
}

// handleGetRequest gets request status
func (h *Handler) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract request ID from path
	requestID := r.URL.Path[len("/api/pam/request/"):]
	if requestID == "" {
		http.Error(w, "request ID required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	pending, err := h.requestStore.Get(ctx, requestID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("store error: %v", err))
		return
	}
	if pending == nil {
		http.NotFound(w, r)
		return
	}

	h.respond(w, pending)
}

// handleHealth returns health status
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.respond(w, map[string]interface{}{
		"status": "healthy",
		"mode":   h.pam.GetStatus()["mode"],
	})
}

// Helper functions
func (h *Handler) respond(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// Compile-time check
var _ http.Handler = (*Handler)(nil)
