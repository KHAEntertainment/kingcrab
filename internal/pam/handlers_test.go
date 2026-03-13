package pam

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// Helper to create valid test initData
func createTestInitData(t *testing.T, botToken string, userID int64) string {
	t.Helper()

	user := &TGUser{
		ID:        userID,
		FirstName: "Test",
		LastName:  "User",
		Username:  "testuser",
	}

	userJSON, _ := json.Marshal(user)
	params := url.Values{}
	params.Set("query_id", "test-query-id")
	params.Set("user", string(userJSON))
	params.Set("auth_date", fmt.Sprintf("%d", time.Now().Unix()))

	keys := []string{"query_id", "user", "auth_date"}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params.Get(k)))
	}
	dataCheckString := strings.Join(parts, "\n")

	secretKey := computeHMACSHA256(botToken, "WebAppData")
	hash := computeHMACSHA256(dataCheckString, secretKey)
	params.Set("hash", hash)

	return params.Encode()
}

func setupTestHandler(t *testing.T) (*Handler, *PAM, RequestStore, string) {
	t.Helper()

	// Setup encryption key
	key := make([]byte, 32)
	rand.Read(key)
	keyEnv := "TEST_PAM_KEY"
	os.Setenv(keyEnv, hex.EncodeToString(key))
	t.Cleanup(func() { os.Unsetenv(keyEnv) })

	tempDir := t.TempDir()

	config := &PAMConfig{
		UseClawVault: "false",
		Fallback: FallbackConfig{
			EncryptionKeyEnv: keyEnv,
			StoragePath:      tempDir,
		},
	}

	pam, err := NewPAM(config)
	if err != nil {
		t.Fatalf("NewPAM() error = %v", err)
	}

	requestStore := NewInMemoryRequestStore()
	botToken := "test-bot-token"
	allowedUsers := []User{
		{TelegramID: 123456, Name: "TestUser"},
	}
	allowedCommands := []string{"systemctl restart *", "ls", "*"}

	handler := NewHandler(pam, requestStore, botToken, allowedUsers, allowedCommands, 5)

	return handler, pam, requestStore, botToken
}

func TestHandler_ServeHTTP_CORS(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/pam/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS request status = %d, want %d", w.Code, http.StatusOK)
	}

	headers := []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
	}

	for _, h := range headers {
		if w.Header().Get(h) == "" {
			t.Errorf("Missing CORS header: %s", h)
		}
	}
}

func TestHandler_HandleEnroll(t *testing.T) {
	handler, _, _, botToken := setupTestHandler(t)

	t.Run("successful enrollment", func(t *testing.T) {
		initData := createTestInitData(t, botToken, 123456)

		reqBody := EnrollRequest{
			InitData:       initData,
			DeviceInfo:     "Test Device",
			BiometricToken: "test-token-123",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/enroll", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp EnrollResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if !resp.Success {
			t.Error("Response success = false, want true")
		}

		if resp.UserID == "" {
			t.Error("UserID should not be empty")
		}
	})

	t.Run("missing biometric token", func(t *testing.T) {
		initData := createTestInitData(t, botToken, 123456)

		reqBody := EnrollRequest{
			InitData:       initData,
			DeviceInfo:     "Test Device",
			BiometricToken: "",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/enroll", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid initData", func(t *testing.T) {
		reqBody := EnrollRequest{
			InitData:       "invalid-init-data",
			DeviceInfo:     "Test Device",
			BiometricToken: "test-token",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/enroll", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("unauthorized user", func(t *testing.T) {
		initData := createTestInitData(t, botToken, 999999) // Not in allowed list

		reqBody := EnrollRequest{
			InitData:       initData,
			DeviceInfo:     "Test Device",
			BiometricToken: "test-token",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/enroll", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/pam/enroll", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("wrong HTTP method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/pam/enroll", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestHandler_HandleApprove(t *testing.T) {
	handler, pam, requestStore, botToken := setupTestHandler(t)

	// Enroll a device first
	ctx := context.Background()
	userID := "tg:123456"
	token := Token{
		Value:      "enrolled-token",
		DeviceInfo: "Test Device",
		EnrolledAt: time.Now(),
		LastUsedAt: time.Now(),
	}
	pam.StoreToken(ctx, userID, token)

	// Create a pending request
	req := NewElevationRequest("systemctl restart nginx", "Deploy", "admin", "web-01", 5*time.Minute, 123)
	requestStore.Create(ctx, req)

	t.Run("successful approval", func(t *testing.T) {
		initData := createTestInitData(t, botToken, 123456)

		approveReq := ApproveRequest{
			InitData:       initData,
			BiometricToken: "enrolled-token",
			RequestID:      req.ID,
		}

		body, _ := json.Marshal(approveReq)
		httpReq := httptest.NewRequest(http.MethodPost, "/api/pam/approve", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp ApproveResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if !resp.Success {
			t.Error("Response success = false, want true")
		}

		// Verify request was updated
		updated, _ := requestStore.Get(ctx, req.ID)
		if updated.Status != "approved" {
			t.Errorf("Request status = %s, want 'approved'", updated.Status)
		}
	})

	t.Run("wrong biometric token", func(t *testing.T) {
		req2 := NewElevationRequest("ls", "List", "admin", "web-01", 5*time.Minute, 123)
		requestStore.Create(ctx, req2)

		initData := createTestInitData(t, botToken, 123456)

		approveReq := ApproveRequest{
			InitData:       initData,
			BiometricToken: "wrong-token",
			RequestID:      req2.ID,
		}

		body, _ := json.Marshal(approveReq)
		httpReq := httptest.NewRequest(http.MethodPost, "/api/pam/approve", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("request not found", func(t *testing.T) {
		initData := createTestInitData(t, botToken, 123456)

		approveReq := ApproveRequest{
			InitData:       initData,
			BiometricToken: "enrolled-token",
			RequestID:      "nonexistent-id",
		}

		body, _ := json.Marshal(approveReq)
		httpReq := httptest.NewRequest(http.MethodPost, "/api/pam/approve", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("expired request", func(t *testing.T) {
		expiredReq := NewElevationRequest("cmd", "reason", "user", "system", -1*time.Minute, 123)
		requestStore.Create(ctx, expiredReq)

		initData := createTestInitData(t, botToken, 123456)

		approveReq := ApproveRequest{
			InitData:       initData,
			BiometricToken: "enrolled-token",
			RequestID:      expiredReq.ID,
		}

		body, _ := json.Marshal(approveReq)
		httpReq := httptest.NewRequest(http.MethodPost, "/api/pam/approve", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusGone {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusGone)
		}
	})

	t.Run("request already processed", func(t *testing.T) {
		processedReq := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
		processedReq.Status = "approved"
		requestStore.Create(ctx, processedReq)

		initData := createTestInitData(t, botToken, 123456)

		approveReq := ApproveRequest{
			InitData:       initData,
			BiometricToken: "enrolled-token",
			RequestID:      processedReq.ID,
		}

		body, _ := json.Marshal(approveReq)
		httpReq := httptest.NewRequest(http.MethodPost, "/api/pam/approve", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusConflict {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusConflict)
		}
	})
}

func TestHandler_HandleCreateRequest(t *testing.T) {
	handler, _, requestStore, _ := setupTestHandler(t)
	ctx := context.Background()

	t.Run("successful request creation", func(t *testing.T) {
		reqBody := CreateRequestRequest{
			Command:      "systemctl restart nginx",
			Reason:       "Deploy new version",
			Requester:    "admin",
			TargetSystem: "web-01",
			NotifyChatID: 123456,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/request", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp CreateRequestResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if resp.RequestID == "" {
			t.Error("RequestID should not be empty")
		}

		if resp.Status != "pending" {
			t.Errorf("Status = %s, want 'pending'", resp.Status)
		}

		// Verify request was stored
		stored, _ := requestStore.Get(ctx, resp.RequestID)
		if stored == nil {
			t.Error("Request should be stored")
		}
	})

	t.Run("empty command", func(t *testing.T) {
		reqBody := CreateRequestRequest{
			Command:      "",
			Reason:       "reason",
			Requester:    "admin",
			TargetSystem: "system",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/request", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("command not in allowlist", func(t *testing.T) {
		// Create handler with restrictive allowlist
		handler2 := NewHandler(handler.pam, handler.requestStore, handler.botToken,
			handler.allowedUsers, []string{"systemctl restart nginx"}, 5)

		reqBody := CreateRequestRequest{
			Command:      "rm -rf /",
			Reason:       "malicious",
			Requester:    "attacker",
			TargetSystem: "system",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pam/request", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler2.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})
}

func TestHandler_HandleGetRequest(t *testing.T) {
	handler, _, requestStore, _ := setupTestHandler(t)
	ctx := context.Background()

	req := NewElevationRequest("test", "reason", "user", "system", 5*time.Minute, 123)
	requestStore.Create(ctx, req)

	t.Run("get existing request", func(t *testing.T) {
		httpReq := httptest.NewRequest(http.MethodGet, "/api/pam/request/"+req.ID, nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}

		var retrieved ElevationRequest
		json.NewDecoder(w.Body).Decode(&retrieved)

		if retrieved.ID != req.ID {
			t.Errorf("ID = %s, want %s", retrieved.ID, req.ID)
		}
	})

	t.Run("get non-existent request", func(t *testing.T) {
		httpReq := httptest.NewRequest(http.MethodGet, "/api/pam/request/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestHandler_HandleHealth(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pam/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "healthy" {
		t.Errorf("Status = %v, want 'healthy'", resp["status"])
	}
}

func TestMatchCommand(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		command string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "systemctl restart nginx",
			command: "systemctl restart nginx",
			want:    true,
		},
		{
			name:    "wildcard matches all",
			pattern: "*",
			command: "any command",
			want:    true,
		},
		{
			name:    "prefix wildcard matches",
			pattern: "systemctl restart *",
			command: "systemctl restart nginx",
			want:    true,
		},
		{
			name:    "prefix wildcard doesn't match different prefix",
			pattern: "systemctl restart *",
			command: "systemctl stop nginx",
			want:    false,
		},
		{
			name:    "no match",
			pattern: "ls",
			command: "cat",
			want:    false,
		},
		{
			name:    "partial match doesn't count",
			pattern: "ls",
			command: "ls -la",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchCommand(tt.pattern, tt.command)
			if got != tt.want {
				t.Errorf("matchCommand(%q, %q) = %v, want %v", tt.pattern, tt.command, got, tt.want)
			}
		})
	}
}

func TestHandler_IsCommandAllowed(t *testing.T) {
	t.Run("no allowlist configured - fail closed", func(t *testing.T) {
		handler := &Handler{
			allowedCommands: []string{},
		}

		if handler.isCommandAllowed("any command") {
			t.Error("isCommandAllowed() should return false when no allowlist configured")
		}
	})

	t.Run("command in allowlist", func(t *testing.T) {
		handler := &Handler{
			allowedCommands: []string{"ls", "cat", "systemctl restart *"},
		}

		if !handler.isCommandAllowed("ls") {
			t.Error("isCommandAllowed('ls') should return true")
		}

		if !handler.isCommandAllowed("systemctl restart nginx") {
			t.Error("isCommandAllowed('systemctl restart nginx') should return true")
		}
	})

	t.Run("command not in allowlist", func(t *testing.T) {
		handler := &Handler{
			allowedCommands: []string{"ls", "cat"},
		}

		if handler.isCommandAllowed("rm -rf /") {
			t.Error("isCommandAllowed('rm -rf /') should return false")
		}
	})
}

func TestNewHandler_DefaultTTL(t *testing.T) {
	handler := NewHandler(nil, nil, "", []User{}, []string{}, 0)

	if handler.requestTTL != 5*time.Minute {
		t.Errorf("Default TTL = %v, want 5 minutes", handler.requestTTL)
	}
}

func TestNewHandler_CustomTTL(t *testing.T) {
	handler := NewHandler(nil, nil, "", []User{}, []string{}, 10)

	if handler.requestTTL != 10*time.Minute {
		t.Errorf("Custom TTL = %v, want 10 minutes", handler.requestTTL)
	}
}