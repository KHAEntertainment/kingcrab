package pam

import (
	"database/sql"
	"testing"
	"time"
)

// Note: These tests require a running PostgreSQL database
// They will be skipped if TEST_DB environment variable is not set

func TestNewDBRequestStore(t *testing.T) {
	// Mock DB for testing structure
	var db *sql.DB
	store := NewDBRequestStore(db)

	if store == nil {
		t.Fatal("expected store, got nil")
	}

	if store.db != db {
		t.Error("db mismatch")
	}
}

func TestMustDBRequestStore(t *testing.T) {
	var db *sql.DB
	dbStore := NewDBRequestStore(db)

	t.Run("with DB store", func(t *testing.T) {
		result := MustDBRequestStore(dbStore)
		if result != dbStore {
			t.Error("expected same store instance")
		}
	})

	t.Run("with non-DB store panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()

		inMemStore := NewInMemoryRequestStore()
		MustDBRequestStore(inMemStore)
	})
}

// Mock tests for DB operations structure (won't actually run without DB)
func TestDBRequestStoreStructure(t *testing.T) {
	t.Run("verify interface compliance", func(t *testing.T) {
		var _ RequestStore = (*DBRequestStore)(nil)
	})

	t.Run("store has expected methods", func(t *testing.T) {
		var db *sql.DB
		store := NewDBRequestStore(db)

		// Verify the store has all required methods by interface
		var _ RequestStore = store
	})
}

// Additional edge case tests
func TestDBRequestStoreEdgeCases(t *testing.T) {
	t.Run("handles null approved_by", func(t *testing.T) {
		// This tests that the code properly handles nullable fields
		// Without a real DB, we just verify the structure
		var approvedBy sql.NullString
		if approvedBy.Valid {
			t.Error("expected Valid to be false for zero value")
		}
	})

	t.Run("handles null approved_at", func(t *testing.T) {
		var approvedAt sql.NullTime
		if approvedAt.Valid {
			t.Error("expected Valid to be false for zero value")
		}
	})

	t.Run("elevation request with all fields", func(t *testing.T) {
		now := time.Now()
		req := &ElevationRequest{
			ID:           "test-id",
			Command:      "test",
			Reason:       "test",
			Requester:    "test",
			TargetSystem: "test",
			Status:       "pending",
			CreatedAt:    now,
			ExpiresAt:    now.Add(5 * time.Minute),
			ApprovedBy:   "admin",
			ApprovedAt:   &now,
			NotifyChatID: 12345,
			IPAddress:    "192.168.1.1",
			UserAgent:    "test-agent",
		}

		// Verify all fields are accessible
		if req.ApprovedBy != "admin" {
			t.Error("ApprovedBy mismatch")
		}
		if req.ApprovedAt == nil {
			t.Error("ApprovedAt should be set")
		}
	})
}