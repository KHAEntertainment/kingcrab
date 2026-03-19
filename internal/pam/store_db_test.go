package pam

import (
	"testing"
)

// Note: These are unit tests for DBRequestStore functions.
// Full integration tests would require a running PostgreSQL database.

func TestNewDBRequestStore(t *testing.T) {
	// Test with nil DB (will fail on use but constructor should work)
	store := NewDBRequestStore(nil)
	if store == nil {
		t.Fatal("Expected store, got nil")
	}

	if store.db != nil {
		t.Error("Expected db to be nil")
	}
}

func TestMustDBRequestStore_Success(t *testing.T) {
	store := NewDBRequestStore(nil)
	result := MustDBRequestStore(store)
	if result != store {
		t.Error("Expected same store")
	}
}

func TestMustDBRequestStore_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for non-DB store")
		}
	}()

	inMemoryStore := NewInMemoryRequestStore()
	MustDBRequestStore(inMemoryStore)
}

// Test that DBRequestStore implements RequestStore interface
func TestDBRequestStore_ImplementsInterface(t *testing.T) {
	var _ RequestStore = (*DBRequestStore)(nil)
}

// Note: Testing with actual database requires integration tests
// The DBRequestStore methods will panic if called with nil DB,
// so we don't test them directly here. Integration tests would
// use a real or mock database.