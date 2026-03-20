package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/executor"
	"github.com/KHAEntertainment/kingcrab/internal/logger"
	"github.com/KHAEntertainment/kingcrab/internal/notifications"
	"github.com/KHAEntertainment/kingcrab/internal/pam"
	"github.com/google/uuid"
)

// V1Handler handles v1 API requests
type V1Handler struct {
	store      pam.RequestStore
	executor   *executor.Executor
	notifier   *notifications.OpenClawNotifier
	allowlist  []string
}

// NewV1Handler creates a new v1 API handler
func NewV1Handler(store pam.RequestStore, exec *executor.Executor, notifier *notifications.OpenClawNotifier, allowlist []string) *V1Handler {
	return &V1Handler{
		store:     store,
		executor:  exec,
		notifier:  notifier,
		allowlist: allowlist,
	}
}

// RegisterRoutes registers all v1 API routes
func (h *V1Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/health", h.handleHealth)
	mux.HandleFunc("POST /api/v1/request", h.handleCreateRequest)
	mux.HandleFunc("GET /api/v1/request/", h.handleGetRequest)
	mux.HandleFunc("GET /api/v1/requests", h.handleListRequests)
	mux.HandleFunc("POST /api/v1/request/", h.handleApproveOrDeny)
}

// handleHealth returns health status
func (h *V1Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// CreateRequestRequest represents a request creation request
type CreateRequestRequest struct {
	Command      string `json:"command"`
	Reason       string `json:"reason"`
	Requester    string `json:"requester"`
	TargetSystem string `json:"target_system"`
	NotifyChatID int64  `json:"notify_chat_id"`
}

// CreateRequestResponse represents a request creation response
type CreateRequestResponse struct {
	Success bool        `json:"success"`
	Request *RequestRef `json:"request,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// RequestRef represents a request reference
type RequestRef struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleCreateRequest handles POST /api/v1/request
func (h *V1Handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate command
	if req.Command == "" {
		respondError(w, http.StatusBadRequest, "command is required")
		return
	}

	// Check against allowlist
	if !h.isCommandAllowed(req.Command) {
		respondError(w, http.StatusForbidden, "command not in allowlist")
		return
	}

	// Set defaults
	if req.Requester == "" {
		req.Requester = r.RemoteAddr
	}
	if req.TargetSystem == "" {
		hostname, _ := os.Hostname()
		req.TargetSystem = hostname
	}

	// Create request
	request := &pam.ElevationRequest{
		ID:          uuid.New().String(),
		Command:     req.Command,
		Reason:      req.Reason,
		Requester:   req.Requester,
		TargetSystem: req.TargetSystem,
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute),
		NotifyChatID: req.NotifyChatID,
		IPAddress:   r.RemoteAddr,
		UserAgent:   r.UserAgent(),
	}

	ctx := context.Background()
	if err := h.store.Create(ctx, request); err != nil {
		logger.Error("Failed to create request", map[string]interface{}{
			"error":   err.Error(),
			"command": req.Command,
		})
		respondError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	// Send notification via OpenClaw
	if err := h.notifier.Notify(ctx, notifications.NotificationRequest{
		RequestID:    request.ID,
		Command:      request.Command,
		Reason:       request.Reason,
		Requester:    request.Requester,
		TargetSystem: request.TargetSystem,
		ExpiresAt:    request.ExpiresAt,
	}); err != nil {
		logger.Warn("Failed to send notification", map[string]interface{}{
			"error":      err.Error(),
			"request_id": request.ID,
		})
		// Don't fail the request if notification fails
	}

	logger.Info("Request created", map[string]interface{}{
		"request_id": request.ID,
		"command":    request.Command,
		"requester":  request.Requester,
	})

	respondJSON(w, CreateRequestResponse{
		Success: true,
		Request: &RequestRef{
			ID:        request.ID,
			Status:    request.Status,
			ExpiresAt: request.ExpiresAt,
		},
	})
}

// handleGetRequest handles GET /api/v1/request/:id
func (h *V1Handler) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	id := r.URL.Path[len("/api/v1/request/"):]
	if id == "" {
		respondError(w, http.StatusBadRequest, "request ID required")
		return
	}

	ctx := context.Background()
	req, err := h.store.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get request")
		return
	}
	if req == nil {
		respondError(w, http.StatusNotFound, "request not found")
		return
	}

	respondJSON(w, req)
}

// handleListRequests handles GET /api/v1/requests
func (h *V1Handler) handleListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get filter from query params
	status := r.URL.Query().Get("status")

	var requests []*pam.ElevationRequest
	var err error

	if status != "" {
		// Filter by status
		if status == "pending" {
			requests, err = h.store.(*pam.DBRequestStore).ListPending(ctx)
		} else {
			// For other statuses, we'd need a ListByStatus method
			// For now, return all pending
			requests, err = h.store.(*pam.DBRequestStore).ListPending(ctx)
		}
	} else {
		requests, err = h.store.(*pam.DBRequestStore).ListPending(ctx)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	respondJSON(w, map[string]interface{}{
		"requests": requests,
		"count":    len(requests),
	})
}

// handleApproveOrDeny handles POST /api/v1/request/:id/approve and POST /api/v1/request/:id/deny
func (h *V1Handler) handleApproveOrDeny(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/v1/request/{id}/approve or /api/v1/request/{id}/deny
	path := r.URL.Path[len("/api/v1/request/"):]
	parts := splitPath(path)

	if len(parts) < 2 {
		respondError(w, http.StatusBadRequest, "invalid request path")
		return
	}

	requestID := parts[0]
	action := parts[1]

	if action != "approve" && action != "deny" {
		respondError(w, http.StatusBadRequest, "invalid action")
		return
	}

	ctx := context.Background()

	// Get request
	request, err := h.store.Get(ctx, requestID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get request")
		return
	}
	if request == nil {
		respondError(w, http.StatusNotFound, "request not found")
		return
	}

	// Check if request is still pending
	if request.Status != "pending" {
		respondJSON(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("request already %s", request.Status),
		})
		return
	}

	// Check if request is expired
	if time.Now().UTC().After(request.ExpiresAt) {
		// Mark as expired
		_, _ = h.store.UpdateStateIf(ctx, requestID, "pending", "expired", "")
		respondError(w, http.StatusGone, "request expired")
		return
	}

	// Parse approval/deny request
	var body struct {
		BiometricToken string `json:"biometric_token"`
		UserID         string `json:"user_id"`
		Reason         string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Body is optional for approve/deny
		body.UserID = r.Header.Get("X-User-ID")
	}

	if action == "approve" {
		// TODO: Verify biometric token
		// For now, proceed with approval

		// Update request to approved
		success, err := h.store.UpdateStateIf(ctx, requestID, "pending", "approved", body.UserID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to approve request")
			return
		}
		if !success {
			respondJSON(w, map[string]interface{}{
				"success": false,
				"error":   "request already processed",
			})
			return
		}

		// Execute command
		result, err := h.executor.Execute(request.Command)
		if err != nil {
			logger.Error("Command execution failed", map[string]interface{}{
				"request_id": requestID,
				"command":    request.Command,
				"error":      err.Error(),
			})
			// Update to failed
			outputMsg := err.Error()
			exitCode := 1
			_ = h.store.Update(ctx, &pam.ElevationRequest{
				ID:        requestID,
				Status:    "failed",
				Output:    &outputMsg,
				ExitCode:  &exitCode,
			})
			respondError(w, http.StatusInternalServerError, "command execution failed")
			return
		}

		logger.Info("Command executed successfully", map[string]interface{}{
			"request_id": requestID,
			"command":    request.Command,
			"exit_code":  result.ExitCode,
		})

		// Update request with result
		_ = h.store.Update(ctx, &pam.ElevationRequest{
			ID:        requestID,
			Status:    "completed",
			Output:    &result.Stdout,
			ExitCode:  &result.ExitCode,
		})

		// Send notification
		_ = h.notifier.NotifyExecuted(ctx, requestID, request.Command, result.ExitCode, result.Stdout)

		respondJSON(w, map[string]interface{}{
			"success":   true,
			"status":    "completed",
			"exit_code": result.ExitCode,
			"output":    result.Stdout,
		})
	} else {
		// Deny request
		success, err := h.store.UpdateStateIf(ctx, requestID, "pending", "denied", body.UserID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to deny request")
			return
		}
		if !success {
			respondJSON(w, map[string]interface{}{
				"success": false,
				"error":   "request already processed",
			})
			return
		}

		logger.Info("Request denied", map[string]interface{}{
			"request_id": requestID,
			"command":    request.Command,
			"reason":     body.Reason,
		})

		// Send notification
		_ = h.notifier.NotifyDenied(ctx, requestID, request.Command, body.Reason)

		respondJSON(w, map[string]interface{}{
			"success": true,
			"status":  "denied",
		})
	}
}

// isCommandAllowed checks if a command matches the allowlist
func (h *V1Handler) isCommandAllowed(command string) bool {
	for _, pattern := range h.allowlist {
		if matchCommand(pattern, command) {
			return true
		}
	}
	return false
}

// matchCommand checks if a command matches a pattern
func matchCommand(pattern, command string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == command {
		return true
	}

	// Simple wildcard matching
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		if len(command) >= len(prefix) && command[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// Helper functions
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func splitPath(path string) []string {
	var parts []string
	var current strings.Builder

	for _, ch := range path {
		if ch == '/' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
