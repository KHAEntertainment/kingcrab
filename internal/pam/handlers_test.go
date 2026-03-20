package pam

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func setupTestHandler(t *testing.T) (*Handler, *InMemoryRequestStore) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_HANDLER_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	t.Cleanup(func() { os.Unsetenv(keyEnv) })

	cfg := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tmpDir,
		},
	}

	pam, err := NewPAM(cfg)
	if err != nil {
		t.Fatalf("Failed to create PAM: %v", err)
	}

	requestStore := NewInMemoryRequestStore()
	botToken := "test_bot_token_123"
	allowedUsers := []User{
		{TelegramID: 12345, Name: "TestUser"},
	}
	allowedCommands := []string{"systemctl restart *", "ls", "*"}

	handler := NewHandler(pam, requestStore, botToken, allowedUsers, allowedCommands, 5)

	return handler, requestStore
}

func TestNewHandler(t *testing.T) {
	handler, _ := setupTestHandler(t)

	if handler == nil {
		t.Fatal("Expected handler, got nil")
	}

	if handler.pam == nil {
		t.Error("Expected PAM instance")
	}

	if handler.requestStore == nil {
		t.Error("Expected request store")
	}

	if handler.requestTTL != 5*time.Minute {
		t.Errorf("Expected TTL 5 minutes, got %v", handler.requestTTL)
	}
}

func TestNewHandler_DefaultTTL(t *testing.T) {
	handler, _ := setupTestHandler(t)
	handler2 := NewHandler(handler.pam, handler.requestStore, "token", []User{}, []string{}, 0)

	if handler2.requestTTL != 5*time.Minute {
		t.Errorf("Expected default TTL 5 minutes, got %v", handler2.requestTTL)
	}
}

func TestHandler_CORS(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/pam/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
	if corsHeader != "*" {
		t.Errorf("Expected CORS header *, got %s", corsHeader)
	}
}

func TestHandler_Health(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pam/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}
}

func TestHandler_NotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pam/invalid", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandler_CreateRequest_Valid(t *testing.T) {
	handler, store := setupTestHandler(t)

	reqBody := CreateRequestRequest{
		Command:      "systemctl restart nginx",
		Reason:       "Deploy new config",
		Requester:    "admin",
		TargetSystem: "web-01",
		NotifyChatID: 12345,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/pam/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result CreateRequestResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.RequestID == "" {
		t.Error("Expected request ID")
	}

	if result.Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", result.Status)
	}

	// Verify stored
	ctx := context.Background()
	stored, _ := store.Get(ctx, result.RequestID)
	if stored == nil {
		t.Error("Request not stored")
	}
}

func TestHandler_CreateRequest_EmptyCommand(t *testing.T) {
	handler, _ := setupTestHandler(t)

	reqBody := CreateRequestRequest{
		Command:      "",
		Reason:       "Test",
		Requester:    "admin",
		TargetSystem: "web-01",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/pam/request", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_CreateRequest_DisallowedCommand(t *testing.T) {
	handler, _ := setupTestHandler(t)
	// Override allowed commands to restrict
	handler.allowedCommands = []string{"ls"}

	reqBody := CreateRequestRequest{
		Command:      "rm -rf /",
		Reason:       "Test",
		Requester:    "admin",
		TargetSystem: "web-01",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/pam/request", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}
}

func TestHandler_GetRequest(t *testing.T) {
	handler, store := setupTestHandler(t)

	// Create a request
	ctx := context.Background()
	req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Get it via HTTP
	httpReq := httptest.NewRequest(http.MethodGet, "/api/pam/request/"+req.ID, nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result ElevationRequest
	json.NewDecoder(resp.Body).Decode(&result)

	if result.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, result.ID)
	}
}

func TestHandler_GetRequest_NotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pam/request/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestMatchCommand(t *testing.T) {
	tests := []struct {
		pattern  string
		command  string
		expected bool
	}{
		{"*", "anything", true},
		{"ls", "ls", true},
		{"ls", "ls -la", false},
		{"systemctl restart *", "systemctl restart nginx", true},
		{"systemctl restart *", "systemctl stop nginx", false},
		{"docker ps", "docker ps", true},
		{"docker ps", "docker stop", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.command, func(t *testing.T) {
			result := matchCommand(tt.pattern, tt.command)
			if result != tt.expected {
				t.Errorf("matchCommand(%q, %q) = %v, expected %v", tt.pattern, tt.command, result, tt.expected)
			}
		})
	}
}

func TestHandler_IsCommandAllowed(t *testing.T) {
	handler, _ := setupTestHandler(t)

	tests := []struct {
		command  string
		expected bool
	}{
		{"systemctl restart nginx", true},
		{"ls", true},
		{"anything", true}, // because "*" is in allowedCommands
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := handler.isCommandAllowed(tt.command)
			if result != tt.expected {
				t.Errorf("isCommandAllowed(%q) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestHandler_IsCommandAllowed_NoAllowlist(t *testing.T) {
	handler, _ := setupTestHandler(t)
	handler.allowedCommands = []string{} // Empty allowlist

	result := handler.isCommandAllowed("ls")
	if result {
		t.Error("Expected false when no commands allowed")
	}
}

func TestHandler_EnrollRequest_MissingBiometricToken(t *testing.T) {
	handler, _ := setupTestHandler(t)

	botToken := "test_bot_token_123"
	initDataString, _ := createValidInitData(botToken, 12345, "testuser")

	reqBody := EnrollRequest{
		InitData:       initDataString,
		DeviceInfo:     "iPhone 14",
		BiometricToken: "", // Empty
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/pam/enroll", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_ApproveRequest_MissingBiometricToken(t *testing.T) {
	handler, _ := setupTestHandler(t)

	botToken := "test_bot_token_123"
	initDataString, _ := createValidInitData(botToken, 12345, "testuser")

	reqBody := ApproveRequest{
		InitData:       initDataString,
		BiometricToken: "", // Empty
		RequestID:      "test-id",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/pam/approve", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_CreateRequest_InvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/pam/request", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := setupTestHandler(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/pam/enroll"},
		{http.MethodGet, "/api/pam/approve"},
		{http.MethodGet, "/api/pam/request"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405, got %d", resp.StatusCode)
			}
		})
	}
}

func TestLoadAllowedUsers_InMemoryStore(t *testing.T) {
	store := NewInMemoryRequestStore()
	users := loadAllowedUsers(store)

	// InMemoryRequestStore doesn't support GetAuthorizedUsers
	if users != nil {
		t.Error("Expected nil for in-memory store")
	}
}

// Test wildcard command matching edge cases
func TestMatchCommand_EdgeCases(t *testing.T) {
	tests := []struct {
		pattern  string
		command  string
		expected bool
	}{
		{"", "cmd", false},
		{"cmd", "", false},
		{"a*", "a", true},
		{"a*", "ab", true},
		{"a*", "b", false},
		{"*", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.command, func(t *testing.T) {
			result := matchCommand(tt.pattern, tt.command)
			if result != tt.expected {
				t.Errorf("matchCommand(%q, %q) = %v, expected %v", tt.pattern, tt.command, result, tt.expected)
			}
		})
	}
}