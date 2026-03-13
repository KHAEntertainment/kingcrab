package pam

import (
	"context"
	"testing"
	"time"
)

func TestNewElevationRequest(t *testing.T) {
	command := "systemctl restart nginx"
	reason := "Deploy new version"
	requester := "admin"
	targetSystem := "web-01"
	ttl := 5 * time.Minute
	chatID := int64(123456)

	req := NewElevationRequest(command, reason, requester, targetSystem, ttl, chatID)

	if req.ID == "" {
		t.Error("NewElevationRequest() ID should not be empty")
	}

	if req.Command != command {
		t.Errorf("Command = %v, want %v", req.Command, command)
	}

	if req.Reason != reason {
		t.Errorf("Reason = %v, want %v", req.Reason, reason)
	}

	if req.Requester != requester {
		t.Errorf("Requester = %v, want %v", req.Requester, requester)
	}

	if req.TargetSystem != targetSystem {
		t.Errorf("TargetSystem = %v, want %v", req.TargetSystem, targetSystem)
	}

	if req.Status != "pending" {
		t.Errorf("Status = %v, want 'pending'", req.Status)
	}

	if req.NotifyChatID != chatID {
		t.Errorf("NotifyChatID = %v, want %v", req.NotifyChatID, chatID)
	}

	if req.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	if req.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}

	expectedExpiry := req.CreatedAt.Add(ttl)
	if !req.ExpiresAt.Equal(expectedExpiry) {
		// Allow small time difference
		diff := req.ExpiresAt.Sub(expectedExpiry)
		if diff > time.Second || diff < -time.Second {
			t.Errorf("ExpiresAt = %v, want approximately %v", req.ExpiresAt, expectedExpiry)
		}
	}

	if req.ApprovedBy != "" {
		t.Error("ApprovedBy should be empty initially")
	}

	if req.ApprovedAt != nil {
		t.Error("ApprovedAt should be nil initially")
	}
}

func TestInMemoryRequestStore_Create(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("test", "reason", "user", "system", 5*time.Minute, 123)

	err := store.Create(ctx, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify it was stored
	retrieved, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("Get() returned nil")
	}

	if retrieved.ID != req.ID {
		t.Errorf("Retrieved ID = %v, want %v", retrieved.ID, req.ID)
	}
}

func TestInMemoryRequestStore_Get(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	t.Run("get existing request", func(t *testing.T) {
		req := NewElevationRequest("test", "reason", "user", "system", 5*time.Minute, 123)
		store.Create(ctx, req)

		retrieved, err := store.Get(ctx, req.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved.Command != req.Command {
			t.Errorf("Command = %v, want %v", retrieved.Command, req.Command)
		}
	})

	t.Run("get non-existent request", func(t *testing.T) {
		retrieved, err := store.Get(ctx, "nonexistent-id")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved != nil {
			t.Error("Get() should return nil for non-existent request")
		}
	})
}

func TestInMemoryRequestStore_Update(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("test", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Update status
	req.Status = "approved"
	now := time.Now()
	req.ApprovedAt = &now
	req.ApprovedBy = "tg:789"

	err := store.Update(ctx, req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(ctx, req.ID)
	if retrieved.Status != "approved" {
		t.Errorf("Status = %v, want 'approved'", retrieved.Status)
	}

	if retrieved.ApprovedBy != "tg:789" {
		t.Errorf("ApprovedBy = %v, want 'tg:789'", retrieved.ApprovedBy)
	}

	if retrieved.ApprovedAt == nil {
		t.Error("ApprovedAt should be set")
	}
}

func TestInMemoryRequestStore_ListPending(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	// Create pending request
	req1 := NewElevationRequest("cmd1", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req1)

	// Create approved request
	req2 := NewElevationRequest("cmd2", "reason", "user", "system", 5*time.Minute, 123)
	req2.Status = "approved"
	store.Create(ctx, req2)

	// Create expired request
	req3 := NewElevationRequest("cmd3", "reason", "user", "system", -1*time.Minute, 123)
	store.Create(ctx, req3)

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}

	// Should only return req1
	if len(pending) != 1 {
		t.Errorf("ListPending() count = %d, want 1", len(pending))
	}

	if len(pending) > 0 && pending[0].ID != req1.ID {
		t.Errorf("ListPending()[0].ID = %v, want %v", pending[0].ID, req1.ID)
	}
}

func TestInMemoryRequestStore_Delete(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	req := NewElevationRequest("test", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	err := store.Delete(ctx, req.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	retrieved, _ := store.Get(ctx, req.ID)
	if retrieved != nil {
		t.Error("Request should be deleted")
	}
}

func TestInMemoryRequestStore_MultipleRequests(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	// Create multiple requests
	for i := 0; i < 5; i++ {
		req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
		store.Create(ctx, req)
	}

	pending, _ := store.ListPending(ctx)
	if len(pending) != 5 {
		t.Errorf("ListPending() count = %d, want 5", len(pending))
	}
}

func TestInMemoryRequestStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryRequestStore()
	ctx := context.Background()

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
			store.Create(ctx, req)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	pending, _ := store.ListPending(ctx)
	if len(pending) != 10 {
		t.Errorf("After concurrent writes, count = %d, want 10", len(pending))
	}
}

func TestElevationRequest_TTLBehavior(t *testing.T) {
	t.Run("request expires after TTL", func(t *testing.T) {
		shortTTL := 100 * time.Millisecond
		req := NewElevationRequest("cmd", "reason", "user", "system", shortTTL, 123)

		// Should not be expired immediately
		if time.Now().After(req.ExpiresAt) {
			t.Error("Request should not be expired immediately")
		}

		// Wait for expiration
		time.Sleep(shortTTL + 50*time.Millisecond)

		if !time.Now().After(req.ExpiresAt) {
			t.Error("Request should be expired after TTL")
		}
	})

	t.Run("different TTLs produce different expiry times", func(t *testing.T) {
		req1 := NewElevationRequest("cmd", "reason", "user", "system", 1*time.Minute, 123)
		req2 := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)

		diff := req2.ExpiresAt.Sub(req1.ExpiresAt)
		expectedDiff := 4 * time.Minute

		// Allow 1 second tolerance
		if diff < expectedDiff-time.Second || diff > expectedDiff+time.Second {
			t.Errorf("Expiry time difference = %v, want approximately %v", diff, expectedDiff)
		}
	})
}