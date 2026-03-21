package pam

import (
	"context"
	"testing"
	"time"
)

// TestSQLiteRequestStore_ImplementsInterface checks the compile-time assertion.
func TestSQLiteRequestStore_ImplementsInterface(t *testing.T) {
	var _ RequestStore = (*SQLiteRequestStore)(nil)
}

// newTestSQLiteStore opens an in-memory SQLite store for testing.
func newTestSQLiteStore(t *testing.T) *SQLiteRequestStore {
	t.Helper()
	store, err := NewSQLiteRequestStore(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSQLiteRequestStore_CreateAndGet(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	req := NewElevationRequest("ls -la", "testing", "alice", "server1", 5*time.Minute, 0)
	if err := store.Create(ctx, req); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.ID != req.ID {
		t.Errorf("ID mismatch: want %s, got %s", req.ID, got.ID)
	}
	if got.Command != req.Command {
		t.Errorf("Command mismatch: want %s, got %s", req.Command, got.Command)
	}
	if got.Status != "pending" {
		t.Errorf("Status: want pending, got %s", got.Status)
	}
}

func TestSQLiteRequestStore_Get_NotFound(t *testing.T) {
	store := newTestSQLiteStore(t)
	got, err := store.Get(context.Background(), "nonexistent-id")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("Expected nil for missing request")
	}
}

func TestSQLiteRequestStore_Update(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	req := NewElevationRequest("uptime", "monitoring", "bob", "host2", 5*time.Minute, 0)
	if err := store.Create(ctx, req); err != nil {
		t.Fatalf("Create: %v", err)
	}

	req.Status = "approved"
	req.ApprovedBy = "admin"
	now := time.Now().UTC()
	req.ApprovedAt = &now

	if err := store.Update(ctx, req); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Status != "approved" {
		t.Errorf("Status: want approved, got %s", got.Status)
	}
	if got.ApprovedBy != "admin" {
		t.Errorf("ApprovedBy: want admin, got %s", got.ApprovedBy)
	}
	if got.ApprovedAt == nil {
		t.Error("ApprovedAt should not be nil after approval")
	}
}

func TestSQLiteRequestStore_Delete(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	req := NewElevationRequest("id", "testing", "carol", "server3", 5*time.Minute, 0)
	if err := store.Create(ctx, req); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete(ctx, req.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Error("Expected nil after delete")
	}
}

func TestSQLiteRequestStore_ListPending(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	req1 := NewElevationRequest("cmd1", "r", "u1", "s", 5*time.Minute, 0)
	req2 := NewElevationRequest("cmd2", "r", "u2", "s", 5*time.Minute, 0)
	req3 := NewElevationRequest("cmd3", "r", "u3", "s", 5*time.Minute, 0)

	for _, r := range []*ElevationRequest{req1, req2, req3} {
		if err := store.Create(ctx, r); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Mark req3 approved so it shouldn't appear in pending
	req3.Status = "approved"
	if err := store.Update(ctx, req3); err != nil {
		t.Fatalf("Update: %v", err)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending requests, got %d", len(pending))
	}
}

func TestSQLiteRequestStore_UpdateStateIf(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	req := NewElevationRequest("sudo apt update", "patch", "dave", "prod1", 5*time.Minute, 0)
	if err := store.Create(ctx, req); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Should fail: wrong expected state
	ok, err := store.UpdateStateIf(ctx, req.ID, "approved", "denied", "eve")
	if err != nil {
		t.Fatalf("UpdateStateIf (bad expected): %v", err)
	}
	if ok {
		t.Error("UpdateStateIf should return false when expectedState doesn't match")
	}

	// Should succeed
	ok, err = store.UpdateStateIf(ctx, req.ID, "pending", "approved", "admin")
	if err != nil {
		t.Fatalf("UpdateStateIf (approve): %v", err)
	}
	if !ok {
		t.Error("UpdateStateIf should return true on successful transition")
	}

	got, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "approved" {
		t.Errorf("Status: want approved, got %s", got.Status)
	}
}

func TestSQLiteRequestStore_ExpireOldRequests(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	// Create an already-expired request by setting a past TTL
	req := &ElevationRequest{
		ID:           "expired-test",
		Command:      "echo test",
		Reason:       "test",
		Requester:    "eve",
		TargetSystem: "local",
		Status:       "pending",
		CreatedAt:    time.Now().Add(-10 * time.Minute),
		ExpiresAt:    time.Now().Add(-1 * time.Minute), // already past
	}
	if err := store.Create(ctx, req); err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err := store.ExpireOldRequests(ctx)
	if err != nil {
		t.Fatalf("ExpireOldRequests: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 expired request, got %d", count)
	}

	got, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "expired" {
		t.Errorf("Status: want expired, got %s", got.Status)
	}
}
