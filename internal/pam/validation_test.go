package pam

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestValidateInitData(t *testing.T) {
	botToken := "test-bot-token"

	// Helper to create valid initData
	createValidInitData := func(authDate time.Time) string {
		user := TGUser{
			ID:        12345,
			FirstName: "Test",
			LastName:  "User",
			Username:  "testuser",
		}
		userJSON, _ := json.Marshal(user)

		params := url.Values{
			"query_id":  []string{"test-query"},
			"user":      []string{string(userJSON)},
			"auth_date": []string{fmt.Sprintf("%d", authDate.Unix())},
		}

		// Build data check string
		keys := []string{"query_id", "user", "auth_date"}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, params.Get(k)))
		}
		dataCheckString := strings.Join(parts, "\n")

		// Derive secret key
		secretKey := computeHMACSHA256(botToken, "WebAppData")
		hash := computeHMACSHA256(dataCheckString, secretKey)
		params.Set("hash", hash)

		return params.Encode()
	}

	t.Run("valid initData", func(t *testing.T) {
		authDate := time.Now()
		initData := createValidInitData(authDate)

		result, err := ValidateInitData(initData, botToken, 1*time.Hour)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result == nil {
			t.Fatal("expected result, got nil")
		}

		if result.User == nil {
			t.Fatal("expected user, got nil")
		}

		if result.User.ID != 12345 {
			t.Errorf("expected user ID 12345, got %d", result.User.ID)
		}
	})

	t.Run("empty initData", func(t *testing.T) {
		_, err := ValidateInitData("", botToken, 1*time.Hour)
		if err == nil {
			t.Fatal("expected error for empty initData")
		}

		if !strings.Contains(err.Error(), "empty initData") {
			t.Errorf("expected 'empty initData' error, got: %v", err)
		}
	})

	t.Run("missing hash", func(t *testing.T) {
		initData := "query_id=test&auth_date=1234567890"
		_, err := ValidateInitData(initData, botToken, 1*time.Hour)
		if err == nil {
			t.Fatal("expected error for missing hash")
		}

		if !strings.Contains(err.Error(), "missing hash") {
			t.Errorf("expected 'missing hash' error, got: %v", err)
		}
	})

	t.Run("missing auth_date", func(t *testing.T) {
		initData := "query_id=test&hash=abc123"
		_, err := ValidateInitData(initData, botToken, 1*time.Hour)
		if err == nil {
			t.Fatal("expected error for missing auth_date")
		}

		if !strings.Contains(err.Error(), "missing auth_date") {
			t.Errorf("expected 'missing auth_date' error, got: %v", err)
		}
	})

	t.Run("expired initData", func(t *testing.T) {
		authDate := time.Now().Add(-2 * time.Hour)
		initData := createValidInitData(authDate)

		_, err := ValidateInitData(initData, botToken, 1*time.Hour)
		if err == nil {
			t.Fatal("expected error for expired initData")
		}

		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("expected 'expired' error, got: %v", err)
		}
	})

	t.Run("invalid hash", func(t *testing.T) {
		authDate := time.Now()
		initData := createValidInitData(authDate)
		// Tamper with the hash
		initData = strings.ReplaceAll(initData, "hash=", "hash=invalid")

		_, err := ValidateInitData(initData, botToken, 1*time.Hour)
		if err == nil {
			t.Fatal("expected error for invalid hash")
		}

		if !strings.Contains(err.Error(), "invalid hash") {
			t.Errorf("expected 'invalid hash' error, got: %v", err)
		}
	})
}

func TestParseUser(t *testing.T) {
	t.Run("valid user JSON", func(t *testing.T) {
		user := TGUser{
			ID:        12345,
			FirstName: "Test",
			Username:  "testuser",
		}
		userJSON, _ := json.Marshal(user)

		result, err := parseUser(string(userJSON))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result.ID != 12345 {
			t.Errorf("expected ID 12345, got %d", result.ID)
		}

		if result.FirstName != "Test" {
			t.Errorf("expected FirstName 'Test', got %s", result.FirstName)
		}
	})

	t.Run("URL-encoded user data", func(t *testing.T) {
		user := TGUser{
			ID:        12345,
			FirstName: "Test User",
		}
		userJSON, _ := json.Marshal(user)
		encoded := url.QueryEscape(string(userJSON))

		result, err := parseUser(encoded)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result.FirstName != "Test User" {
			t.Errorf("expected FirstName 'Test User', got %s", result.FirstName)
		}
	})

	t.Run("empty user data", func(t *testing.T) {
		_, err := parseUser("")
		if err == nil {
			t.Fatal("expected error for empty user data")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseUser("{invalid json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestComputeHMACSHA256(t *testing.T) {
	data := "test data"
	key := "secret key"

	hash := computeHMACSHA256(data, key)

	// Verify it's a hex string (64 characters for SHA256)
	if len(hash) != 64 {
		t.Errorf("expected 64 character hex string, got %d characters", len(hash))
	}

	// Verify deterministic
	hash2 := computeHMACSHA256(data, key)
	if hash != hash2 {
		t.Error("expected same hash for same inputs")
	}

	// Verify different with different key
	hash3 := computeHMACSHA256(data, "different key")
	if hash == hash3 {
		t.Error("expected different hash for different key")
	}
}

func TestInitDataToUserID(t *testing.T) {
	t.Run("valid initData", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 12345},
		}

		userID := InitDataToUserID(initData)
		if userID != "tg:12345" {
			t.Errorf("expected 'tg:12345', got %s", userID)
		}
	})

	t.Run("nil initData", func(t *testing.T) {
		userID := InitDataToUserID(nil)
		if userID != "" {
			t.Errorf("expected empty string, got %s", userID)
		}
	})

	t.Run("nil user", func(t *testing.T) {
		initData := &InitData{User: nil}
		userID := InitDataToUserID(initData)
		if userID != "" {
			t.Errorf("expected empty string, got %s", userID)
		}
	})
}

func TestIsAuthorizedUser(t *testing.T) {
	allowedUsers := []User{
		{TelegramID: 12345, Name: "User1"},
		{TelegramID: 67890, Name: "User2"},
	}

	t.Run("authorized user", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 12345},
		}

		if !IsAuthorizedUser(initData, allowedUsers) {
			t.Error("expected user to be authorized")
		}
	})

	t.Run("unauthorized user", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 99999},
		}

		if IsAuthorizedUser(initData, allowedUsers) {
			t.Error("expected user to be unauthorized")
		}
	})

	t.Run("nil initData", func(t *testing.T) {
		if IsAuthorizedUser(nil, allowedUsers) {
			t.Error("expected nil initData to be unauthorized")
		}
	})

	t.Run("nil user", func(t *testing.T) {
		initData := &InitData{User: nil}
		if IsAuthorizedUser(initData, allowedUsers) {
			t.Error("expected nil user to be unauthorized")
		}
	})

	t.Run("empty allowed users", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 12345},
		}

		if IsAuthorizedUser(initData, []User{}) {
			t.Error("expected user to be unauthorized with empty list")
		}
	})
}

func TestCheckAuthorization(t *testing.T) {
	allowedUsers := []User{
		{TelegramID: 12345, Name: "User1"},
	}

	t.Run("authorized user", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 12345},
		}

		err := CheckAuthorization(initData, allowedUsers)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("unauthorized user", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 99999},
		}

		err := CheckAuthorization(initData, allowedUsers)
		if err == nil {
			t.Fatal("expected error for unauthorized user")
		}

		if !strings.Contains(err.Error(), "not authorized") {
			t.Errorf("expected 'not authorized' error, got: %v", err)
		}
	})

	t.Run("no allowed users - fail closed", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 12345},
		}

		err := CheckAuthorization(initData, []User{})
		if err == nil {
			t.Fatal("expected error when no users configured")
		}

		if !strings.Contains(err.Error(), "no authorized users") {
			t.Errorf("expected 'no authorized users' error, got: %v", err)
		}
	})
}

func TestBuildDataCheckString(t *testing.T) {
	t.Run("sorts keys alphabetically", func(t *testing.T) {
		params := url.Values{
			"zebra": []string{"z"},
			"alpha": []string{"a"},
			"beta":  []string{"b"},
			"hash":  []string{"should_be_excluded"},
		}

		result := buildDataCheckString(params)
		expected := "alpha=a\nbeta=b\nzebra=z"

		if result != expected {
			t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
		}
	})

	t.Run("excludes hash parameter", func(t *testing.T) {
		params := url.Values{
			"key1": []string{"val1"},
			"hash": []string{"abc123"},
		}

		result := buildDataCheckString(params)
		if strings.Contains(result, "hash") {
			t.Error("expected hash to be excluded")
		}
	})
}

func TestValidateInitDataFromRequest(t *testing.T) {
	// This is a wrapper around ValidateInitData with 24h max age
	// Just verify it calls through correctly
	botToken := "test-token"

	// Test with empty string
	_, err := ValidateInitDataFromRequest("", botToken)
	if err == nil {
		t.Fatal("expected error for empty initData")
	}
}