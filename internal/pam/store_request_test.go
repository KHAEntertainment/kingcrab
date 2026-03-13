package pam

import (
	"context"
	"testing"
	"time"
)

func TestNewElevationRequest(t *testing.T) {
	command := "systemctl restart nginx"
	reason := "service update"
	requester := "admin"
	targetSystem := "web-server"
	ttl := 5 * time.Minute
	chatID := int64(12345)

	req := NewElevationRequest(command, reason, requester, targetSystem, ttl, chatID)

	if req == nil {
		t.Fatal("expected request, got nil")
	}

	if req.ID == "" {
		t.Error("expected ID to be set")
	}

	if req.Command != command {
		t.Errorf("expected command %s, got %s", command, req.Command)
	}

	if req.Reason != reason {
		t.Errorf("expected reason %s, got %s", reason, req.Reason)
	}

	if req.Requester != requester {
		t.Errorf("expected requester %s, got %s", requester, req.Requester)
	}

	if req.TargetSystem != targetSystem {
		t.Errorf("expected target system %s, got %s", targetSystem, req.TargetSystem)
	}

	if req.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", req.Status)
	}

	if req.NotifyChatID != chatID {
		t.Errorf("expected chat ID %d, got %d", chatID, req.NotifyChatID)
	}

	if req.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	if req.ExpiresAt.IsZero() {
		t.Error("expected ExpiresAt to be set")
	}

	// Verify TTL is approximately correct (within 1 second)
	expectedExpiry := time.Now().Add(ttl)
	if req.ExpiresAt.Sub(expectedExpiry) > time.Second {
		t.Error("expected ExpiresAt to be approximately CreatedAt + TTL")
	}
}

func TestInMemoryRequestStore(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	t.Run("create and get request", func(t *testing.T) {
		req := NewElevationRequest("ls -la", "testing", "admin", "server1", 5*time.Minute, 12345)

		err := store.Create(ctx, req)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		retrieved, err := store.Get(ctx, req.ID)
		if err != nil {
			t.Fatalf("failed to get request: %v", err)
		}

		if retrieved == nil {
			t.Fatal("expected request, got nil")
		}

		if retrieved.ID != req.ID {
			t.Errorf("expected ID %s, got %s", req.ID, retrieved.ID)
		}

		if retrieved.Command != req.Command {
			t.Errorf("expected command %s, got %s", req.Command, retrieved.Command)
		}
	})

	t.Run("get non-existent request", func(t *testing.T) {
		retrieved, err := store.Get(ctx, "non-existent-id")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil for non-existent request")
		}
	})

	t.Run("update request", func(t *testing.T) {
		req := NewElevationRequest("pwd", "test", "user", "server2", 5*time.Minute, 67890)
		store.Create(ctx, req)

		// Update the request
		req.Status = "approved"
		approvedBy := "approver1"
		now := time.Now()
		req.ApprovedBy = approvedBy
		req.ApprovedAt = &now

		err := store.Update(ctx, req)
		if err != nil {
			t.Fatalf("failed to update request: %v", err)
		}

		// Retrieve and verify
		updated, _ := store.Get(ctx, req.ID)
		if updated.Status != "approved" {
			t.Errorf("expected status 'approved', got %s", updated.Status)
		}

		if updated.ApprovedBy != approvedBy {
			t.Errorf("expected approver %s, got %s", approvedBy, updated.ApprovedBy)
		}

		if updated.ApprovedAt == nil {
			t.Error("expected ApprovedAt to be set")
		}
	})

	t.Run("list pending requests", func(t *testing.T) {
		store := NewInMemoryRequestStore() // Fresh store

		// Create pending requests
		req1 := NewElevationRequest("cmd1", "reason1", "user1", "srv1", 10*time.Minute, 111)
		req2 := NewElevationRequest("cmd2", "reason2", "user2", "srv2", 10*time.Minute, 222)
		store.Create(ctx, req1)
		store.Create(ctx, req2)

		// Create an approved request
		req3 := NewElevationRequest("cmd3", "reason3", "user3", "srv3", 10*time.Minute, 333)
		req3.Status = "approved"
		store.Create(ctx, req3)

		// Create an expired request
		req4 := NewElevationRequest("cmd4", "reason4", "user4", "srv4", -1*time.Minute, 444)
		store.Create(ctx, req4)

		pending, err := store.ListPending(ctx)
		if err != nil {
			t.Fatalf("failed to list pending: %v", err)
		}

		// Should only return pending and non-expired requests
		if len(pending) != 2 {
			t.Errorf("expected 2 pending requests, got %d", len(pending))
		}

		// Verify they're the right ones
		foundIDs := make(map[string]bool)
		for _, req := range pending {
			foundIDs[req.ID] = true
		}

		if !foundIDs[req1.ID] || !foundIDs[req2.ID] {
			t.Error("expected to find req1 and req2 in pending list")
		}

		if foundIDs[req3.ID] || foundIDs[req4.ID] {
			t.Error("should not find approved or expired requests in pending list")
		}
	})

	t.Run("delete request", func(t *testing.T) {
		req := NewElevationRequest("rm test", "cleanup", "admin", "server", 5*time.Minute, 555)
		store.Create(ctx, req)

		err := store.Delete(ctx, req.ID)
		if err != nil {
			t.Fatalf("failed to delete request: %v", err)
		}

		// Verify deleted
		retrieved, _ := store.Get(ctx, req.ID)
		if retrieved != nil {
			t.Error("expected request to be deleted")
		}
	})

	t.Run("delete non-existent request", func(t *testing.T) {
		err := store.Delete(ctx, "non-existent")
		if err != nil {
			t.Errorf("expected no error deleting non-existent request, got: %v", err)
		}
	})
}

func TestElevationRequestFields(t *testing.T) {
	t.Run("all fields are set correctly", func(t *testing.T) {
		req := &ElevationRequest{
			ID:           "test-id",
			Command:      "test command",
			Reason:       "test reason",
			Requester:    "test-user",
			TargetSystem: "test-system",
			Status:       "pending",
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(5 * time.Minute),
			NotifyChatID: 12345,
			IPAddress:    "192.168.1.1",
			UserAgent:    "test-agent",
		}

		approvedBy := "approver"
		approvedAt := time.Now()
		req.ApprovedBy = approvedBy
		req.ApprovedAt = &approvedAt

		// Verify all fields
		if req.ID != "test-id" {
			t.Error("ID mismatch")
		}
		if req.Command != "test command" {
			t.Error("Command mismatch")
		}
		if req.Reason != "test reason" {
			t.Error("Reason mismatch")
		}
		if req.Requester != "test-user" {
			t.Error("Requester mismatch")
		}
		if req.TargetSystem != "test-system" {
			t.Error("TargetSystem mismatch")
		}
		if req.Status != "pending" {
			t.Error("Status mismatch")
		}
		if req.NotifyChatID != 12345 {
			t.Error("NotifyChatID mismatch")
		}
		if req.IPAddress != "192.168.1.1" {
			t.Error("IPAddress mismatch")
		}
		if req.UserAgent != "test-agent" {
			t.Error("UserAgent mismatch")
		}
		if req.ApprovedBy != approvedBy {
			t.Error("ApprovedBy mismatch")
		}
		if req.ApprovedAt == nil {
			t.Error("ApprovedAt should be set")
		}
	})
}