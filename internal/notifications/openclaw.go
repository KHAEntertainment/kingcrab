package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OpenClawNotifier sends notifications via OpenClaw's webhook API
type OpenClawNotifier struct {
	webhookURL string
	httpClient *http.Client
	enabled    bool
}

// NewOpenClawNotifier creates a new OpenClaw notification service
func NewOpenClawNotifier(webhookURL string) *OpenClawNotifier {
	return &OpenClawNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		enabled: webhookURL != "",
	}
}

// Notification represents a notification payload
type Notification struct {
	Type      string                 `json:"type"`
	RequestID string                 `json:"request_id"`
	Command   string                 `json:"command"`
	Reason    string                 `json:"reason"`
	Requester string                 `json:"requester"`
	ExpiresAt time.Time              `json:"expires_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Notify sends a notification about a new request
func (n *OpenClawNotifier) Notify(ctx context.Context, req NotificationRequest) error {
	if !n.enabled {
		return nil // Silently skip if notifications disabled
	}

	payload := Notification{
		Type:      "new_request",
		RequestID: req.RequestID,
		Command:   req.Command,
		Reason:    req.Reason,
		Requester: req.Requester,
		ExpiresAt: req.ExpiresAt,
		Metadata: map[string]interface{}{
			"target_system": req.TargetSystem,
		},
	}

	return n.send(ctx, payload)
}

// NotifyApproved sends notification when request is approved
func (n *OpenClawNotifier) NotifyApproved(ctx context.Context, reqID, command string) error {
	if !n.enabled {
		return nil
	}

	payload := Notification{
		Type:      "request_approved",
		RequestID: reqID,
		Command:   command,
	}

	return n.send(ctx, payload)
}

// NotifyDenied sends notification when request is denied
func (n *OpenClawNotifier) NotifyDenied(ctx context.Context, reqID, command, reason string) error {
	if !n.enabled {
		return nil
	}

	payload := Notification{
		Type:      "request_denied",
		RequestID: reqID,
		Command:   command,
		Metadata: map[string]interface{}{
			"reason": reason,
		},
	}

	return n.send(ctx, payload)
}

// NotifyExecuted sends notification when command execution completes
func (n *OpenClawNotifier) NotifyExecuted(ctx context.Context, reqID, command string, exitCode int, output string) error {
	if !n.enabled {
		return nil
	}

	payload := Notification{
		Type:      "request_executed",
		RequestID: reqID,
		Command:   command,
		Metadata: map[string]interface{}{
			"exit_code": exitCode,
			"output":    truncateOutput(output, 500),
		},
	}

	return n.send(ctx, payload)
}

// send sends a notification to the OpenClaw webhook
func (n *OpenClawNotifier) send(ctx context.Context, payload Notification) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "KingCrab/1.0")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notification failed: status %d", resp.StatusCode)
	}

	return nil
}

// NotificationRequest represents a request notification payload
type NotificationRequest struct {
	RequestID    string
	Command      string
	Reason       string
	Requester    string
	TargetSystem string
	ExpiresAt    time.Time
}

// truncateOutput limits output length for notifications
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
