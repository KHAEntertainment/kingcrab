package pam

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

// Helper to create valid initData for testing
func createValidInitData(botToken string, userID int64, username string) (string, string) {
	authDate := time.Now().Unix()
	user := TGUser{
		ID:        userID,
		FirstName: "Test",
		LastName:  "User",
		Username:  username,
	}
	userJSON, _ := json.Marshal(user)

	params := url.Values{}
	params.Set("query_id", "AAHdF6IQAAAAAN0XohDhrOrc")
	params.Set("user", string(userJSON))
	params.Set("auth_date", fmt.Sprintf("%d", authDate))

	// Build data check string
	keys := []string{"auth_date", "query_id", "user"}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params.Get(k)))
	}
	dataCheckString := strings.Join(parts, "\n")

	// Compute hash per Telegram spec
	secretKey := computeHMACSHA256(botToken, "WebAppData")
	hash := computeHMACSHA256(dataCheckString, secretKey)

	params.Set("hash", hash)

	return params.Encode(), hash
}

func TestValidateInitData_ValidData(t *testing.T) {
	botToken := "test_bot_token_123"
	userID := int64(12345)
	username := "testuser"

	initDataString, _ := createValidInitData(botToken, userID, username)

	result, err := ValidateInitData(initDataString, botToken, 24*time.Hour)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.User == nil {
		t.Fatal("Expected user, got nil")
	}

	if result.User.ID != userID {
		t.Errorf("Expected user ID %d, got %d", userID, result.User.ID)
	}

	if result.User.Username != username {
		t.Errorf("Expected username %s, got %s", username, result.User.Username)
	}
}

func TestValidateInitData_EmptyString(t *testing.T) {
	_, err := ValidateInitData("", "token", 24*time.Hour)
	if err == nil {
		t.Fatal("Expected error for empty initData")
	}

	if !strings.Contains(err.Error(), "empty initData") {
		t.Errorf("Expected 'empty initData' error, got: %v", err)
	}
}

func TestValidateInitData_MissingHash(t *testing.T) {
	initData := "auth_date=1234567890&user=%7B%22id%22%3A123%7D"
	_, err := ValidateInitData(initData, "token", 24*time.Hour)
	if err == nil {
		t.Fatal("Expected error for missing hash")
	}

	if !strings.Contains(err.Error(), "missing hash") {
		t.Errorf("Expected 'missing hash' error, got: %v", err)
	}
}

func TestValidateInitData_MissingAuthDate(t *testing.T) {
	initData := "hash=abc123&user=%7B%22id%22%3A123%7D"
	_, err := ValidateInitData(initData, "token", 24*time.Hour)
	if err == nil {
		t.Fatal("Expected error for missing auth_date")
	}

	if !strings.Contains(err.Error(), "missing auth_date") {
		t.Errorf("Expected 'missing auth_date' error, got: %v", err)
	}
}

func TestValidateInitData_ExpiredData(t *testing.T) {
	botToken := "test_bot_token_123"

	// Create initData with old timestamp
	authDate := time.Now().Add(-25 * time.Hour).Unix()
	params := url.Values{}
	params.Set("query_id", "test")
	params.Set("auth_date", fmt.Sprintf("%d", authDate))
	params.Set("user", `{"id":123,"first_name":"Test"}`)

	dataCheckString := buildDataCheckString(params)
	secretKey := computeHMACSHA256(botToken, "WebAppData")
	hash := computeHMACSHA256(dataCheckString, secretKey)
	params.Set("hash", hash)

	_, err := ValidateInitData(params.Encode(), botToken, 24*time.Hour)
	if err == nil {
		t.Fatal("Expected error for expired initData")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected 'expired' error, got: %v", err)
	}
}

func TestValidateInitData_InvalidHash(t *testing.T) {
	botToken := "test_bot_token_123"
	authDate := time.Now().Unix()

	params := url.Values{}
	params.Set("query_id", "test")
	params.Set("auth_date", fmt.Sprintf("%d", authDate))
	params.Set("user", `{"id":123,"first_name":"Test"}`)
	params.Set("hash", "invalid_hash_123")

	_, err := ValidateInitData(params.Encode(), botToken, 24*time.Hour)
	if err == nil {
		t.Fatal("Expected error for invalid hash")
	}

	if !strings.Contains(err.Error(), "invalid hash") {
		t.Errorf("Expected 'invalid hash' error, got: %v", err)
	}
}

func TestParseUser_Valid(t *testing.T) {
	userData := `{"id":123,"first_name":"John","last_name":"Doe","username":"johndoe"}`
	user, err := parseUser(userData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if user.ID != 123 {
		t.Errorf("Expected ID 123, got %d", user.ID)
	}
	if user.FirstName != "John" {
		t.Errorf("Expected FirstName 'John', got '%s'", user.FirstName)
	}
	if user.LastName != "Doe" {
		t.Errorf("Expected LastName 'Doe', got '%s'", user.LastName)
	}
	if user.Username != "johndoe" {
		t.Errorf("Expected Username 'johndoe', got '%s'", user.Username)
	}
}

func TestParseUser_URLEncoded(t *testing.T) {
	userData := url.QueryEscape(`{"id":456,"first_name":"Jane"}`)
	user, err := parseUser(userData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if user.ID != 456 {
		t.Errorf("Expected ID 456, got %d", user.ID)
	}
	if user.FirstName != "Jane" {
		t.Errorf("Expected FirstName 'Jane', got '%s'", user.FirstName)
	}
}

func TestParseUser_EmptyData(t *testing.T) {
	_, err := parseUser("")
	if err == nil {
		t.Fatal("Expected error for empty user data")
	}
}

func TestParseUser_InvalidJSON(t *testing.T) {
	_, err := parseUser("not valid json")
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestBuildDataCheckString(t *testing.T) {
	params := url.Values{}
	params.Set("query_id", "test123")
	params.Set("user", "userdata")
	params.Set("auth_date", "1234567890")
	params.Set("hash", "should_be_excluded")

	result := buildDataCheckString(params)

	// Should be sorted alphabetically and exclude hash
	expected := "auth_date=1234567890\nquery_id=test123\nuser=userdata"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestComputeHMACSHA256(t *testing.T) {
	data := "test data"
	key := "test key"

	result := computeHMACSHA256(data, key)

	// Verify it's valid hex
	_, err := hex.DecodeString(result)
	if err != nil {
		t.Errorf("Result is not valid hex: %v", err)
	}

	// Verify it's consistent
	result2 := computeHMACSHA256(data, key)
	if result != result2 {
		t.Error("HMAC computation not consistent")
	}

	// Verify expected output
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	expected := hex.EncodeToString(h.Sum(nil))

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestInitDataToUserID(t *testing.T) {
	tests := []struct {
		name     string
		initData *InitData
		expected string
	}{
		{
			name: "valid user",
			initData: &InitData{
				User: &TGUser{ID: 12345},
			},
			expected: "tg:12345",
		},
		{
			name:     "nil initData",
			initData: nil,
			expected: "",
		},
		{
			name: "nil user",
			initData: &InitData{
				User: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InitDataToUserID(tt.initData)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsAuthorizedUser(t *testing.T) {
	allowedUsers := []User{
		{TelegramID: 123, Name: "Alice"},
		{TelegramID: 456, Name: "Bob"},
	}

	tests := []struct {
		name     string
		initData *InitData
		expected bool
	}{
		{
			name: "authorized user",
			initData: &InitData{
				User: &TGUser{ID: 123},
			},
			expected: true,
		},
		{
			name: "unauthorized user",
			initData: &InitData{
				User: &TGUser{ID: 999},
			},
			expected: false,
		},
		{
			name:     "nil initData",
			initData: nil,
			expected: false,
		},
		{
			name: "nil user",
			initData: &InitData{
				User: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAuthorizedUser(tt.initData, allowedUsers)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckAuthorization(t *testing.T) {
	allowedUsers := []User{
		{TelegramID: 123, Name: "Alice"},
	}

	tests := []struct {
		name         string
		initData     *InitData
		allowedUsers []User
		shouldError  bool
		errorMessage string
	}{
		{
			name: "authorized user",
			initData: &InitData{
				User: &TGUser{ID: 123},
			},
			allowedUsers: allowedUsers,
			shouldError:  false,
		},
		{
			name: "unauthorized user",
			initData: &InitData{
				User: &TGUser{ID: 999},
			},
			allowedUsers: allowedUsers,
			shouldError:  true,
			errorMessage: "not authorized",
		},
		{
			name:         "nil initData",
			initData:     nil,
			allowedUsers: allowedUsers,
			shouldError:  true,
			errorMessage: "missing user data",
		},
		{
			name: "no allowed users",
			initData: &InitData{
				User: &TGUser{ID: 123},
			},
			allowedUsers: []User{},
			shouldError:  true,
			errorMessage: "no authorized users configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckAuthorization(tt.initData, tt.allowedUsers)
			if tt.shouldError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMessage, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestValidateInitDataFromRequest(t *testing.T) {
	botToken := "test_bot_token_123"
	initDataString, _ := createValidInitData(botToken, 12345, "testuser")

	result, err := ValidateInitDataFromRequest(initDataString, botToken)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
}

// Test that old data is rejected with default 24h max age
func TestValidateInitDataFromRequest_VeryOldData(t *testing.T) {
	botToken := "test_bot_token_123"

	// Create data that's 25 hours old
	authDate := time.Now().Add(-25 * time.Hour).Unix()
	params := url.Values{}
	params.Set("query_id", "test")
	params.Set("auth_date", fmt.Sprintf("%d", authDate))
	params.Set("user", `{"id":123,"first_name":"Test"}`)

	dataCheckString := buildDataCheckString(params)
	secretKey := computeHMACSHA256(botToken, "WebAppData")
	hash := computeHMACSHA256(dataCheckString, secretKey)
	params.Set("hash", hash)

	_, err := ValidateInitDataFromRequest(params.Encode(), botToken)
	if err == nil {
		t.Fatal("Expected error for old data")
	}
}