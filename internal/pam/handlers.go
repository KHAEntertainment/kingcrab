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
	pam             *PAM
	requestStore    RequestStore
	botToken        string
	allowedUsers    []User
	allowedCommands []string
	requestTTL      time.Duration
	notifyWebhook   string // URL to call on approval
}

// NewHandler creates a new PAM handler
func NewHandler(pam *PAM, requestStore RequestStore, botToken string, allowedUsers []User, allowedCommands []string, ttlMinutes int) *Handler {
	// If no allowed users provided, try to load from store
	if len(allowedUsers) == 0 {
		allowedUsers = loadAllowedUsers(requestStore)
	}

	if ttlMinutes <= 0 {
		ttlMinutes = 5 // default 5 minutes
	}

	return &Handler{
		pam:             pam,
		requestStore:    requestStore,
		botToken:        botToken,
		allowedUsers:    allowedUsers,
		allowedCommands: allowedCommands,
		requestTTL:      time.Duration(ttlMinutes) * time.Minute,
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

type DenyRequest struct {
	InitData        string `json:"initData"`
	BiometricToken  string `json:"biometric_token"`
	RequestID       string `json:"request_id"`
	Reason          string `json:"reason"`
}

type DenyResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type CreateRequestRequest struct {
	InitData     string `json:"initData"`
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
	case "/api/pam/deny":
		h.handleDeny(w, r)
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

	// Validate biometric token is not empty
	if req.BiometricToken == "" {
		h.respondError(w, http.StatusBadRequest, "biometric token required")
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
		TokenStorage: "local",
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

	// Validate biometric token is not empty
	if req.BiometricToken == "" {
		h.respondError(w, http.StatusBadRequest, "biometric token required")
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

	// Verify token matches
	if storedToken.Value != req.BiometricToken {
		h.respondError(w, http.StatusUnauthorized, "biometric token mismatch")
		return
	}

	// Update token last used
	storedToken.LastUsedAt = time.Now()
	if err := h.pam.StoreToken(context.Background(), userID, *storedToken); err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update token usage: %v", err))
		return
	}

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
		// Try to atomically set to expired if still pending
		h.requestStore.UpdateStateIf(ctx, pending.ID, "pending", "expired", "")
		h.respondError(w, http.StatusGone, "request expired")
		return
	}

	// Atomically approve the request (only if still pending)
	success, err := h.requestStore.UpdateStateIf(ctx, req.RequestID, "pending", "approved", userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update request: %v", err))
		return
	}

	if !success {
		h.respondError(w, http.StatusConflict, "request already processed")
		return
	}

	// TODO: Trigger actual command execution
	// TODO: Call notify webhook

	h.respond(w, ApproveResponse{
		Success:   true,
		Message:   "Elevation request approved",
		RequestID: req.RequestID,
	})
}

// handleDeny handles elevation request denial
func (h *Handler) handleDeny(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate biometric token is not empty
	if req.BiometricToken == "" {
		h.respondError(w, http.StatusBadRequest, "biometric token required")
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

	// Verify token matches
	if storedToken.Value != req.BiometricToken {
		h.respondError(w, http.StatusUnauthorized, "biometric token mismatch")
		return
	}

	// Update token last used
	storedToken.LastUsedAt = time.Now()
	if err := h.pam.StoreToken(context.Background(), userID, *storedToken); err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update token usage: %v", err))
		return
	}

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
		// Try to atomically set to expired if still pending
		h.requestStore.UpdateStateIf(ctx, pending.ID, "pending", "expired", "")
		h.respondError(w, http.StatusGone, "request expired")
		return
	}

	// Atomically deny the request (only if still pending)
	success, err := h.requestStore.UpdateStateIf(ctx, req.RequestID, "pending", "denied", userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update request: %v", err))
		return
	}

	if !success {
		h.respondError(w, http.StatusConflict, "request already processed")
		return
	}

	// TODO: Audit log the denial with reason
	// TODO: Call notify webhook

	h.respond(w, DenyResponse{
		Success:   true,
		Message:   "Elevation request denied",
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

	// Authenticate caller via initData
	initDataHeader := r.Header.Get("X-Telegram-Init-Data")
	if initDataHeader == "" {
		// Fallback to JSON field for backwards compatibility
		initDataHeader = req.InitData
	}

	initData, err := ValidateInitDataFromRequest(initDataHeader, h.botToken)
	if err != nil {
		// Log authentication failure for auditing
		if dbStore, ok := h.requestStore.(*DBRequestStore); ok {
			ctx := context.Background()
			dbStore.LogAudit(ctx, "create_request_denied", nil, nil, nil, r.RemoteAddr, r.UserAgent(), map[string]interface{}{
				"reason": "invalid_initdata",
				"error":  err.Error(),
			})
		}
		h.respondError(w, http.StatusUnauthorized, fmt.Sprintf("invalid initData: %v", err))
		return
	}

	// Authorize caller
	if err := CheckAuthorization(initData, h.allowedUsers); err != nil {
		// Log authorization failure for auditing
		if dbStore, ok := h.requestStore.(*DBRequestStore); ok {
			ctx := context.Background()
			userID := int(initData.User.ID)
			dbStore.LogAudit(ctx, "create_request_denied", nil, nil, &userID, r.RemoteAddr, r.UserAgent(), map[string]interface{}{
				"reason": "unauthorized",
				"error":  err.Error(),
			})
		}
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	}

	// Validate command is not empty
	if req.Command == "" {
		h.respondError(w, http.StatusBadRequest, "command required")
		return
	}

	// Validate command against allowlist
	if !h.isCommandAllowed(req.Command) {
		h.respondError(w, http.StatusForbidden, "command not in allowlist")
		return
	}

	// Use server-validated identity instead of client-supplied requester
	requester := InitDataToUserID(initData)

	pending := NewElevationRequest(
		req.Command,
		req.Reason,
		requester,
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

	// Authenticate caller via initData
	initDataHeader := r.Header.Get("X-Telegram-Init-Data")
	if initDataHeader == "" {
		h.respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	initData, err := ValidateInitDataFromRequest(initDataHeader, h.botToken)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "invalid authentication")
		return
	}

	// Authorize caller
	if err := CheckAuthorization(initData, h.allowedUsers); err != nil {
		h.respondError(w, http.StatusForbidden, "access denied")
		return
	}

	ctx := context.Background()
	pending, err := h.requestStore.Get(ctx, requestID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "error retrieving request")
		return
	}
	if pending == nil {
		h.respondError(w, http.StatusNotFound, "request not found")
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

// isCommandAllowed checks if a command matches the allowlist
func (h *Handler) isCommandAllowed(command string) bool {
	// If no allowlist configured, reject all (fail closed)
	if len(h.allowedCommands) == 0 {
		return false
	}

	for _, pattern := range h.allowedCommands {
		if matchCommand(pattern, command) {
			return true
		}
	}
	return false
}

// matchCommand checks if a command matches a pattern (* = wildcard)
func matchCommand(pattern, command string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == command {
		return true
	}
	// Simple wildcard support: "systemctl restart *"
	if len(pattern) > 1 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(command) >= len(prefix) && command[:len(prefix)] == prefix
	}
	return false
}

// Compile-time check
var _ http.Handler = (*Handler)(nil)