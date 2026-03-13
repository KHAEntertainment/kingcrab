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

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			StoragePath:      tmpDir,
			EncryptionKeyEnv: keyEnv,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("failed to create PAM: %v", err)
	}

	store := NewInMemoryRequestStore()
	allowedUsers := []User{{TelegramID: 12345, Name: "Test User"}}
	allowedCommands := []string{"systemctl restart *", "docker restart *"}

	handler := NewHandler(pam, store, "test-token", allowedUsers, allowedCommands, 5)
	return handler, store
}

func TestNewHandler(t *testing.T) {
	tmpDir := t.TempDir()
	keyEnv := "TEST_NEW_HANDLER_KEY"
	key := make([]byte, 32)
	rand.Read(key)
	os.Setenv(keyEnv, hex.EncodeToString(key))
	defer os.Unsetenv(keyEnv)

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			StoragePath:      tmpDir,
			EncryptionKeyEnv: keyEnv,
		},
	}

	pam, _ := NewPAM(config)
	store := NewInMemoryRequestStore()
	botToken := "test-token"
	users := []User{{TelegramID: 12345}}
	commands := []string{"*"}

	t.Run("with valid parameters", func(t *testing.T) {
		handler := NewHandler(pam, store, botToken, users, commands, 10)
		if handler == nil {
			t.Fatal("expected handler, got nil")
		}

		if handler.pam != pam {
			t.Error("PAM mismatch")
		}
		if handler.requestStore != store {
			t.Error("store mismatch")
		}
		if handler.botToken != botToken {
			t.Error("botToken mismatch")
		}
		if handler.requestTTL != 10*time.Minute {
			t.Error("TTL mismatch")
		}
	})

	t.Run("with zero TTL uses default", func(t *testing.T) {
		handler := NewHandler(pam, store, botToken, users, commands, 0)
		if handler.requestTTL != 5*time.Minute {
			t.Errorf("expected default TTL 5m, got %v", handler.requestTTL)
		}
	})

	t.Run("with negative TTL uses default", func(t *testing.T) {
		handler := NewHandler(pam, store, botToken, users, commands, -1)
		if handler.requestTTL != 5*time.Minute {
			t.Errorf("expected default TTL 5m, got %v", handler.requestTTL)
		}
	})
}

func TestHandlerServeHTTP(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("CORS preflight", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/pam/enroll", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("expected CORS headers")
		}
	})

	t.Run("health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pam/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		if response["status"] != "healthy" {
			t.Error("expected healthy status")
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pam/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestHandleCreateRequest(t *testing.T) {
	handler, store := setupTestHandler(t)

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pam/request", nil)
		w := httptest.NewRecorder()

		handler.handleCreateRequest(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/pam/request", bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()

		handler.handleCreateRequest(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("empty command", func(t *testing.T) {
		body := CreateRequestRequest{
			Command:      "",
			Reason:       "test",
			Requester:    "user",
			TargetSystem: "server",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/pam/request", bytes.NewBuffer(jsonBody))
		w := httptest.NewRecorder()

		handler.handleCreateRequest(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("command not in allowlist", func(t *testing.T) {
		body := CreateRequestRequest{
			Command:      "rm -rf /",
			Reason:       "test",
			Requester:    "user",
			TargetSystem: "server",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/pam/request", bytes.NewBuffer(jsonBody))
		w := httptest.NewRecorder()

		handler.handleCreateRequest(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("successful request creation", func(t *testing.T) {
		body := CreateRequestRequest{
			Command:      "systemctl restart nginx",
			Reason:       "deploy update",
			Requester:    "admin",
			TargetSystem: "web-server",
			NotifyChatID: 12345,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/pam/request", bytes.NewBuffer(jsonBody))
		w := httptest.NewRecorder()

		handler.handleCreateRequest(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response CreateRequestResponse
		json.NewDecoder(w.Body).Decode(&response)

		if response.RequestID == "" {
			t.Error("expected request ID")
		}

		if response.Status != "pending" {
			t.Errorf("expected status 'pending', got %s", response.Status)
		}

		// Verify stored in database
		stored, _ := store.Get(context.Background(), response.RequestID)
		if stored == nil {
			t.Error("expected request to be stored")
		}
	})
}

func TestHandleGetRequest(t *testing.T) {
	handler, store := setupTestHandler(t)

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/pam/request/123", nil)
		w := httptest.NewRecorder()

		handler.handleGetRequest(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("missing request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pam/request/", nil)
		w := httptest.NewRecorder()

		handler.handleGetRequest(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("nonexistent request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pam/request/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.handleGetRequest(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("successful get", func(t *testing.T) {
		// Create a request first
		elevReq := NewElevationRequest("test cmd", "reason", "user", "system", 5*time.Minute, 12345)
		store.Create(context.Background(), elevReq)

		req := httptest.NewRequest("GET", "/api/pam/request/"+elevReq.ID, nil)
		w := httptest.NewRecorder()

		handler.handleGetRequest(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response ElevationRequest
		json.NewDecoder(w.Body).Decode(&response)

		if response.ID != elevReq.ID {
			t.Error("request ID mismatch")
		}
	})
}

func TestMatchCommand(t *testing.T) {
	tests := []struct {
		pattern string
		command string
		match   bool
	}{
		{"*", "anything", true},
		{"systemctl restart nginx", "systemctl restart nginx", true},
		{"systemctl restart *", "systemctl restart nginx", true},
		{"systemctl restart *", "systemctl restart apache", true},
		{"systemctl restart *", "systemctl stop nginx", false},
		{"docker restart *", "docker restart web", true},
		{"docker restart *", "docker stop web", false},
		{"exact-match", "exact-match", true},
		{"exact-match", "not-exact", false},
		{"prefix*", "prefix-anything", true},
		{"prefix*", "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_vs_"+tt.command, func(t *testing.T) {
			result := matchCommand(tt.pattern, tt.command)
			if result != tt.match {
				t.Errorf("pattern %s vs command %s: expected %v, got %v",
					tt.pattern, tt.command, tt.match, result)
			}
		})
	}
}

func TestIsCommandAllowed(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("allowed command", func(t *testing.T) {
		if !handler.isCommandAllowed("systemctl restart nginx") {
			t.Error("expected command to be allowed")
		}
	})

	t.Run("disallowed command", func(t *testing.T) {
		if handler.isCommandAllowed("rm -rf /") {
			t.Error("expected command to be disallowed")
		}
	})

	t.Run("wildcard match", func(t *testing.T) {
		if !handler.isCommandAllowed("docker restart api") {
			t.Error("expected wildcard match")
		}
	})
}

func TestLoadAllowedUsers(t *testing.T) {
	t.Run("in-memory store returns nil", func(t *testing.T) {
		store := NewInMemoryRequestStore()
		users := loadAllowedUsers(store)
		if users != nil {
			t.Error("expected nil for in-memory store")
		}
	})
}

func TestRespondHelpers(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("respond with JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"status": "ok"}

		handler.respond(w, data)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Error("expected JSON content type")
		}

		var response map[string]string
		json.NewDecoder(w.Body).Decode(&response)

		if response["status"] != "ok" {
			t.Error("response data mismatch")
		}
	})

	t.Run("respond with error", func(t *testing.T) {
		w := httptest.NewRecorder()

		handler.respondError(w, http.StatusBadRequest, "test error")

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]string
		json.NewDecoder(w.Body).Decode(&response)

		if response["error"] != "test error" {
			t.Error("error message mismatch")
		}
	})
}

func TestRequestTypes(t *testing.T) {
	t.Run("EnrollRequest", func(t *testing.T) {
		req := EnrollRequest{
			InitData:       "test-init-data",
			DeviceInfo:     "iPhone 14",
			BiometricToken: "token-123",
		}

		data, _ := json.Marshal(req)
		var unmarshaled EnrollRequest
		json.Unmarshal(data, &unmarshaled)

		if unmarshaled.BiometricToken != req.BiometricToken {
			t.Error("BiometricToken mismatch")
		}
	})

	t.Run("ApproveRequest", func(t *testing.T) {
		req := ApproveRequest{
			InitData:       "test-init",
			BiometricToken: "token",
			RequestID:      "req-123",
		}

		if req.RequestID != "req-123" {
			t.Error("RequestID mismatch")
		}
	})

	t.Run("CreateRequestRequest", func(t *testing.T) {
		req := CreateRequestRequest{
			Command:      "test command",
			Reason:       "test reason",
			Requester:    "user",
			TargetSystem: "system",
			NotifyChatID: 12345,
		}

		if req.NotifyChatID != 12345 {
			t.Error("NotifyChatID mismatch")
		}
	})
}