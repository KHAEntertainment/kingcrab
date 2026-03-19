package pam

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// InitData represents parsed Telegram initData
type InitData struct {
	QueryID    string    `json:"query_id"`
	User       *TGUser   `json:"user"`
	AuthDate   time.Time `json:"auth_date"`
	Hash       string    `json:"hash"`
	Raw        string    `json:"raw"` // Original query string
}

// TGUser represents a Telegram user from initData
type TGUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Language  string `json:"language_code"`
}

// ValidateInitData validates Telegram initData from Mini App
// Per Telegram docs: https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func ValidateInitData(initDataString, botToken string, maxAge time.Duration) (*InitData, error) {
	if initDataString == "" {
		return nil, fmt.Errorf("empty initData")
	}

	// Parse the query string
	params, err := url.ParseQuery(initDataString)
	if err != nil {
		return nil, fmt.Errorf("parse initData: %w", err)
	}

	// Extract hash
	hash := params.Get("hash")
	if hash == "" {
		return nil, fmt.Errorf("missing hash")
	}

	// Extract auth_date and validate freshness
	authDateStr := params.Get("auth_date")
	if authDateStr == "" {
		return nil, fmt.Errorf("missing auth_date")
	}

	authDateUnix, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse auth_date: %w", err)
	}

	authDate := time.Unix(authDateUnix, 0)
	if maxAge > 0 && time.Since(authDate) > maxAge {
		return nil, fmt.Errorf("initData expired (age: %v, max: %v)", time.Since(authDate), maxAge)
	}

	// Extract user data
	var user *TGUser
	if userData := params.Get("user"); userData != "" {
		user, err = parseUser(userData)
		if err != nil {
			return nil, fmt.Errorf("parse user: %w", err)
		}
	}

	// Build data check string (all params except hash, sorted by key)
	dataCheckString := buildDataCheckString(params)

	// Derive secret key: HMAC-SHA256(key="WebAppData", message=botToken)
	// Per Telegram spec: https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
	secretKey := computeHMACSHA256(botToken, "WebAppData")

	// Validate hash using derived secret
	expectedHash := computeHMACSHA256(dataCheckString, secretKey)
	if !hmac.Equal([]byte(expectedHash), []byte(hash)) {
		return nil, fmt.Errorf("invalid hash (possible spoofing attempt)")
	}

	return &InitData{
		QueryID: params.Get("query_id"),
		User:    user,
		AuthDate: authDate,
		Hash:    hash,
		Raw:     initDataString,
	}, nil
}

// parseUser parses JSON-encoded user from initData
func parseUser(userData string) (*TGUser, error) {
	// URL-decode the user data (Telegram URL-encodes special chars)
	decoded, err := url.QueryUnescape(userData)
	if err != nil {
		return nil, err
	}

	if decoded == "" {
		return nil, fmt.Errorf("empty user data")
	}

	// Parse JSON
	var user TGUser
	if err := json.Unmarshal([]byte(decoded), &user); err != nil {
		return nil, fmt.Errorf("parse user JSON: %w", err)
	}

	return &user, nil
}

// buildDataCheckString creates the data check string per Telegram spec
func buildDataCheckString(params url.Values) string {
	// Get all keys except 'hash'
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "hash" {
			keys = append(keys, k)
		}
	}

	// Sort alphabetically
	sort.Strings(keys)

	// Build string: "key=value\nkey=value\n..."
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params.Get(k)))
	}

	return strings.Join(parts, "\n")
}

// computeHMACSHA256 computes HMAC-SHA256 with data as message and key as secret
func computeHMACSHA256(data, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateInitDataFromRequest validates initData from HTTP request
func ValidateInitDataFromRequest(initData string, botToken string) (*InitData, error) {
	// Default max age: 24 hours (Telegram recommendation)
	return ValidateInitData(initData, botToken, 24*time.Hour)
}

// InitDataToUserID extracts user ID from validated initData
func InitDataToUserID(initData *InitData) string {
	if initData == nil || initData.User == nil {
		return ""
	}
	return fmt.Sprintf("tg:%d", initData.User.ID)
}

// IsAuthorizedUser checks if the Telegram user is in the authorized list
func IsAuthorizedUser(initData *InitData, allowedUsers []User) bool {
	if initData == nil || initData.User == nil {
		return false
	}

	for _, user := range allowedUsers {
		if user.TelegramID == initData.User.ID {
			return true
		}
	}

	return false
}

func CheckAuthorization(initData *InitData, allowedUsers []User) error {
	if initData == nil || initData.User == nil {
		return fmt.Errorf("missing user data in initData")
	}

	// Fail closed: no allowed users = no access
	if len(allowedUsers) == 0 {
		return fmt.Errorf("no authorized users configured")
	}

	if !IsAuthorizedUser(initData, allowedUsers) {
		return fmt.Errorf("user %d not authorized", initData.User.ID)
	}

	return nil
}