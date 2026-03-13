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

func TestValidateInitData(t *testing.T) {
	botToken := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

	// Helper to create valid initData
	createValidInitData := func(authDate time.Time, user *TGUser) string {
		userJSON, _ := json.Marshal(user)
		params := url.Values{}
		params.Set("query_id", "test-query-id")
		params.Set("user", string(userJSON))
		params.Set("auth_date", fmt.Sprintf("%d", authDate.Unix()))

		// Build data check string
		keys := []string{"query_id", "user", "auth_date"}
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

		return params.Encode()
	}

	user := &TGUser{
		ID:        123456789,
		FirstName: "Test",
		LastName:  "User",
		Username:  "testuser",
	}

	t.Run("valid initData", func(t *testing.T) {
		initData := createValidInitData(time.Now(), user)

		result, err := ValidateInitData(initData, botToken, 24*time.Hour)
		if err != nil {
			t.Fatalf("ValidateInitData() error = %v", err)
		}

		if result.User.ID != user.ID {
			t.Errorf("User ID = %v, want %v", result.User.ID, user.ID)
		}
		if result.User.FirstName != user.FirstName {
			t.Errorf("FirstName = %v, want %v", result.User.FirstName, user.FirstName)
		}
	})

	t.Run("empty initData", func(t *testing.T) {
		_, err := ValidateInitData("", botToken, 24*time.Hour)
		if err == nil {
			t.Error("ValidateInitData() expected error for empty initData, got nil")
		}
	})

	t.Run("missing hash", func(t *testing.T) {
		params := url.Values{}
		params.Set("auth_date", fmt.Sprintf("%d", time.Now().Unix()))

		_, err := ValidateInitData(params.Encode(), botToken, 24*time.Hour)
		if err == nil || !strings.Contains(err.Error(), "missing hash") {
			t.Errorf("ValidateInitData() error = %v, want missing hash error", err)
		}
	})

	t.Run("missing auth_date", func(t *testing.T) {
		params := url.Values{}
		params.Set("hash", "somehash")

		_, err := ValidateInitData(params.Encode(), botToken, 24*time.Hour)
		if err == nil || !strings.Contains(err.Error(), "missing auth_date") {
			t.Errorf("ValidateInitData() error = %v, want missing auth_date error", err)
		}
	})

	t.Run("expired initData", func(t *testing.T) {
		oldDate := time.Now().Add(-25 * time.Hour)
		initData := createValidInitData(oldDate, user)

		_, err := ValidateInitData(initData, botToken, 24*time.Hour)
		if err == nil || !strings.Contains(err.Error(), "expired") {
			t.Errorf("ValidateInitData() error = %v, want expired error", err)
		}
	})

	t.Run("invalid hash", func(t *testing.T) {
		initData := createValidInitData(time.Now(), user)
		// Tamper with the hash
		params, _ := url.ParseQuery(initData)
		params.Set("hash", "invalidhash")

		_, err := ValidateInitData(params.Encode(), botToken, 24*time.Hour)
		if err == nil || !strings.Contains(err.Error(), "invalid hash") {
			t.Errorf("ValidateInitData() error = %v, want invalid hash error", err)
		}
	})

	t.Run("no max age check when zero", func(t *testing.T) {
		oldDate := time.Now().Add(-48 * time.Hour)
		initData := createValidInitData(oldDate, user)

		// Should not error even though data is old, since maxAge is 0
		_, err := ValidateInitData(initData, botToken, 0)
		if err != nil {
			t.Errorf("ValidateInitData() with maxAge=0 should not check expiration, got error: %v", err)
		}
	})
}

func TestParseUser(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantID  int64
	}{
		{
			name:    "valid user JSON",
			input:   url.QueryEscape(`{"id":123456,"first_name":"John","last_name":"Doe","username":"johndoe"}`),
			wantErr: false,
			wantID:  123456,
		},
		{
			name:    "minimal user JSON",
			input:   url.QueryEscape(`{"id":789,"first_name":"Jane"}`),
			wantErr: false,
			wantID:  789,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   url.QueryEscape(`{invalid json}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := parseUser(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("parseUser() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseUser() unexpected error = %v", err)
				return
			}

			if user.ID != tt.wantID {
				t.Errorf("parseUser() ID = %v, want %v", user.ID, tt.wantID)
			}
		})
	}
}

func TestBuildDataCheckString(t *testing.T) {
	params := url.Values{}
	params.Set("query_id", "test-id")
	params.Set("user", `{"id":123}`)
	params.Set("auth_date", "1234567890")
	params.Set("hash", "shouldbeignored")

	result := buildDataCheckString(params)

	// Should be sorted alphabetically, hash excluded
	expected := "auth_date=1234567890\nquery_id=test-id\nuser={\"id\":123}"

	if result != expected {
		t.Errorf("buildDataCheckString() = %q, want %q", result, expected)
	}

	// Verify hash is not included
	if strings.Contains(result, "hash") {
		t.Error("buildDataCheckString() should not include hash parameter")
	}
}

func TestComputeHMACSHA256(t *testing.T) {
	data := "test data"
	key := "secret key"

	result := computeHMACSHA256(data, key)

	// Verify it's a valid hex string
	if _, err := hex.DecodeString(result); err != nil {
		t.Errorf("computeHMACSHA256() result is not valid hex: %v", err)
	}

	// Verify it's 64 characters (SHA256 = 32 bytes = 64 hex chars)
	if len(result) != 64 {
		t.Errorf("computeHMACSHA256() length = %d, want 64", len(result))
	}

	// Verify it matches expected HMAC
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	expected := hex.EncodeToString(h.Sum(nil))

	if result != expected {
		t.Errorf("computeHMACSHA256() = %q, want %q", result, expected)
	}

	// Verify different inputs produce different outputs
	result2 := computeHMACSHA256("different data", key)
	if result == result2 {
		t.Error("computeHMACSHA256() should produce different hashes for different inputs")
	}
}

func TestInitDataToUserID(t *testing.T) {
	tests := []struct {
		name     string
		initData *InitData
		want     string
	}{
		{
			name: "valid user",
			initData: &InitData{
				User: &TGUser{ID: 123456789},
			},
			want: "tg:123456789",
		},
		{
			name:     "nil initData",
			initData: nil,
			want:     "",
		},
		{
			name: "nil user",
			initData: &InitData{
				User: nil,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InitDataToUserID(tt.initData)
			if got != tt.want {
				t.Errorf("InitDataToUserID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAuthorizedUser(t *testing.T) {
	allowedUsers := []User{
		{TelegramID: 111, Name: "User1"},
		{TelegramID: 222, Name: "User2"},
		{TelegramID: 333, Name: "User3"},
	}

	tests := []struct {
		name     string
		initData *InitData
		want     bool
	}{
		{
			name: "authorized user",
			initData: &InitData{
				User: &TGUser{ID: 222},
			},
			want: true,
		},
		{
			name: "unauthorized user",
			initData: &InitData{
				User: &TGUser{ID: 999},
			},
			want: false,
		},
		{
			name:     "nil initData",
			initData: nil,
			want:     false,
		},
		{
			name: "nil user",
			initData: &InitData{
				User: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuthorizedUser(tt.initData, allowedUsers)
			if got != tt.want {
				t.Errorf("IsAuthorizedUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckAuthorization(t *testing.T) {
	allowedUsers := []User{
		{TelegramID: 111, Name: "User1"},
	}

	t.Run("no users configured", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 111},
		}

		err := CheckAuthorization(initData, []User{})
		if err == nil {
			t.Error("CheckAuthorization() expected error when no users configured, got nil")
		}
		if !strings.Contains(err.Error(), "no authorized users configured") {
			t.Errorf("CheckAuthorization() error = %v, want 'no authorized users configured'", err)
		}
	})

	t.Run("authorized user", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 111},
		}

		err := CheckAuthorization(initData, allowedUsers)
		if err != nil {
			t.Errorf("CheckAuthorization() unexpected error = %v", err)
		}
	})

	t.Run("unauthorized user", func(t *testing.T) {
		initData := &InitData{
			User: &TGUser{ID: 999},
		}

		err := CheckAuthorization(initData, allowedUsers)
		if err == nil {
			t.Error("CheckAuthorization() expected error for unauthorized user, got nil")
		}
		if !strings.Contains(err.Error(), "not authorized") {
			t.Errorf("CheckAuthorization() error = %v, want 'not authorized'", err)
		}
	})
}

func TestValidateInitDataFromRequest(t *testing.T) {
	// This is a convenience wrapper, so just test it delegates correctly
	botToken := "test-token"

	t.Run("delegates to ValidateInitData with 24h maxAge", func(t *testing.T) {
		// Should fail with empty initData
		_, err := ValidateInitDataFromRequest("", botToken)
		if err == nil {
			t.Error("ValidateInitDataFromRequest() expected error, got nil")
		}
	})
}