package pam

import (
	"context"
	"testing"
	"time"
)

func TestNewElevationRequest(t *testing.T) {
	command := "systemctl restart nginx"
	reason := "Deploy new config"
	requester := "admin"
	targetSystem := "web-server-01"
	ttl := 5 * time.Minute
	chatID := int64(12345)

	req := NewElevationRequest(command, reason, requester, targetSystem, ttl, chatID)

	if req == nil {
		t.Fatal("Expected request, got nil")
	}

	if req.ID == "" {
		t.Error("Expected non-empty ID")
	}

	if req.Command != command {
		t.Errorf("Expected command %s, got %s", command, req.Command)
	}

	if req.Reason != reason {
		t.Errorf("Expected reason %s, got %s", reason, req.Reason)
	}

	if req.Requester != requester {
		t.Errorf("Expected requester %s, got %s", requester, req.Requester)
	}

	if req.TargetSystem != targetSystem {
		t.Errorf("Expected target system %s, got %s", targetSystem, req.TargetSystem)
	}

	if req.Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", req.Status)
	}

	if req.NotifyChatID != chatID {
		t.Errorf("Expected chat ID %d, got %d", chatID, req.NotifyChatID)
	}

	if req.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt")
	}

	if req.ExpiresAt.IsZero() {
		t.Error("Expected non-zero ExpiresAt")
	}

	// Verify TTL is approximately correct
	expectedExpiry := req.CreatedAt.Add(ttl)
	if req.ExpiresAt.Sub(expectedExpiry) > time.Second {
		t.Errorf("ExpiresAt not set correctly: expected ~%v, got %v", expectedExpiry, req.ExpiresAt)
	}
}

func TestNewElevationRequest_UniqueIDs(t *testing.T) {
	req1 := NewElevationRequest("cmd1", "reason1", "user1", "sys1", 5*time.Minute, 123)
	req2 := NewElevationRequest("cmd2", "reason2", "user2", "sys2", 5*time.Minute, 456)

	if req1.ID == req2.ID {
		t.Error("Expected unique IDs for different requests")
	}
}

func TestInMemoryRequestStore_Create(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("test cmd", "test reason", "user", "system", 5*time.Minute, 123)

	err := store.Create(ctx, req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it was stored
	retrieved, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected request, got nil")
	}

	if retrieved.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, retrieved.ID)
	}
}

func TestInMemoryRequestStore_Get_NotFound(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	retrieved, err := store.Get(ctx, "nonexistent-id")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil for nonexistent request")
	}
}

func TestInMemoryRequestStore_Update(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("test cmd", "test reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Update status
	req.Status = "approved"
	approvedBy := "admin"
	req.ApprovedBy = approvedBy
	now := time.Now()
	req.ApprovedAt = &now

	err := store.Update(ctx, req)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(ctx, req.ID)
	if retrieved.Status != "approved" {
		t.Errorf("Expected status 'approved', got %s", retrieved.Status)
	}

	if retrieved.ApprovedBy != approvedBy {
		t.Errorf("Expected approved by %s, got %s", approvedBy, retrieved.ApprovedBy)
	}

	if retrieved.ApprovedAt == nil {
		t.Error("Expected non-nil ApprovedAt")
	}
}

func TestInMemoryRequestStore_Delete(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("test cmd", "test reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Delete
	err := store.Delete(ctx, req.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	retrieved, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil after delete")
	}
}

func TestInMemoryRequestStore_ListPending(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	// Create pending request
	req1 := NewElevationRequest("cmd1", "reason1", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req1)

	// Create approved request
	req2 := NewElevationRequest("cmd2", "reason2", "user", "system", 5*time.Minute, 123)
	req2.Status = "approved"
	store.Create(ctx, req2)

	// Create expired request (expires in past)
	req3 := NewElevationRequest("cmd3", "reason3", "user", "system", -1*time.Minute, 123)
	store.Create(ctx, req3)

	// List pending
	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}

	// Should only return req1 (pending and not expired)
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(pending))
	}

	if pending[0].ID != req1.ID {
		t.Errorf("Expected request ID %s, got %s", req1.ID, pending[0].ID)
	}
}

func TestInMemoryRequestStore_ListPending_Empty(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}

	// Allow nil or empty slice
	if pending != nil && len(pending) != 0 {
		t.Errorf("Expected 0 pending requests, got %d", len(pending))
	}
}

func TestInMemoryRequestStore_Multiple_Operations(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	// Create multiple requests
	for i := 0; i < 5; i++ {
		req := NewElevationRequest(
			"cmd",
			"reason",
			"user",
			"system",
			5*time.Minute,
			int64(i),
		)
		err := store.Create(ctx, req)
		if err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}
	}

	// List pending should return all 5
	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}

	if len(pending) != 5 {
		t.Errorf("Expected 5 pending requests, got %d", len(pending))
	}
}

func TestInMemoryRequestStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	// Create request
	req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Simulate concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := store.Get(ctx, req.ID)
			if err != nil {
				t.Errorf("Concurrent Get failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestElevationRequest_StatusTransitions(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Transition to approved
	req.Status = "approved"
	store.Update(ctx, req)

	retrieved, _ := store.Get(ctx, req.ID)
	if retrieved.Status != "approved" {
		t.Errorf("Expected status 'approved', got %s", retrieved.Status)
	}

	// Transition to denied
	req.Status = "denied"
	store.Update(ctx, req)

	retrieved, _ = store.Get(ctx, req.ID)
	if retrieved.Status != "denied" {
		t.Errorf("Expected status 'denied', got %s", retrieved.Status)
	}
}

func TestElevationRequest_Expiration(t *testing.T) {
	// Create request that expires immediately
	req := NewElevationRequest("cmd", "reason", "user", "system", 1*time.Millisecond, 123)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Should be expired
	if time.Now().Before(req.ExpiresAt) {
		t.Error("Request should be expired")
	}
}