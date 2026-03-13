package pam

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	if os.Getenv("KINGCRAB_TEST_DB") != "1" {
		t.Skip("Skipping database test - set KINGCRAB_TEST_DB=1 to run")
	}

	host := os.Getenv("KINGCRAB_DB_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("KINGCRAB_DB_PORT")
	if port == "" {
		port = "5432"
	}

	user := os.Getenv("KINGCRAB_DB_USER")
	if user == "" {
		user = "kingcrab"
	}

	password := os.Getenv("KINGCRAB_DB_PASSWORD")
	if password == "" {
		t.Skip("KINGCRAB_DB_PASSWORD not set")
	}

	dbname := os.Getenv("KINGCRAB_DB_NAME")
	if dbname == "" {
		dbname = "kingcrab_test"
	}

	dsn := "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=disable"

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Clean up tables
	ctx := context.Background()
	db.ExecContext(ctx, "TRUNCATE TABLE approval_audit, elevation_requests, enrolled_devices, authorized_users CASCADE")

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestNewDBRequestStore(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)

	if store == nil {
		t.Fatal("NewDBRequestStore() returned nil")
	}

	if store.db != db {
		t.Error("Store db should be set")
	}
}

func TestDBRequestStore_Create(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	req := NewElevationRequest("systemctl restart nginx", "Deploy", "admin", "web-01", 5*time.Minute, 123456)

	err := store.Create(ctx, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify it was created
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM elevation_requests WHERE id = $1", req.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestDBRequestStore_Get(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	req := NewElevationRequest("test command", "test reason", "admin", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	t.Run("get existing request", func(t *testing.T) {
		retrieved, err := store.Get(ctx, req.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved == nil {
			t.Fatal("Get() returned nil")
		}

		if retrieved.ID != req.ID {
			t.Errorf("ID = %s, want %s", retrieved.ID, req.ID)
		}

		if retrieved.Command != req.Command {
			t.Errorf("Command = %s, want %s", retrieved.Command, req.Command)
		}

		if retrieved.Status != req.Status {
			t.Errorf("Status = %s, want %s", retrieved.Status, req.Status)
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

func TestDBRequestStore_Update(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	// Update request
	now := time.Now()
	req.Status = "approved"
	req.ApprovedBy = "tg:789"
	req.ApprovedAt = &now

	err := store.Update(ctx, req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(ctx, req.ID)
	if retrieved.Status != "approved" {
		t.Errorf("Status = %s, want 'approved'", retrieved.Status)
	}

	if retrieved.ApprovedBy != "tg:789" {
		t.Errorf("ApprovedBy = %s, want 'tg:789'", retrieved.ApprovedBy)
	}

	if retrieved.ApprovedAt == nil {
		t.Error("ApprovedAt should be set")
	}
}

func TestDBRequestStore_ListPending(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
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
		t.Errorf("ListPending()[0].ID = %s, want %s", pending[0].ID, req1.ID)
	}
}

func TestDBRequestStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
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

func TestDBRequestStore_LogAudit(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	// Create a request first
	req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	details := map[string]interface{}{
		"action": "test",
		"value":  123,
	}

	err := store.LogAudit(ctx, "approval", &req.ID, nil, nil, "127.0.0.1", "test-agent", details)
	if err != nil {
		t.Fatalf("LogAudit() error = %v", err)
	}

	// Verify audit log was created
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM approval_audit WHERE request_id = $1", req.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query audit: %v", err)
	}

	if count != 1 {
		t.Errorf("Audit count = %d, want 1", count)
	}
}

func TestDBRequestStore_GetAuthorizedUsers(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	// Insert test users
	_, err := db.ExecContext(ctx, `
		INSERT INTO authorized_users (telegram_id, display_name, is_active)
		VALUES (123, 'User 1', true), (456, 'User 2', true), (789, 'User 3', false)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	users, err := store.GetAuthorizedUsers(ctx)
	if err != nil {
		t.Fatalf("GetAuthorizedUsers() error = %v", err)
	}

	// Should only return active users
	if len(users) != 2 {
		t.Errorf("User count = %d, want 2 (only active users)", len(users))
	}

	// Verify user data
	found := false
	for _, u := range users {
		if u.TelegramID == 123 && u.Name == "User 1" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Should find user with telegram_id 123")
	}
}

func TestDBRequestStore_RequestWithNullFields(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	// Create request without approved fields
	req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
	store.Create(ctx, req)

	retrieved, err := store.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.ApprovedBy != "" {
		t.Errorf("ApprovedBy should be empty, got %s", retrieved.ApprovedBy)
	}

	if retrieved.ApprovedAt != nil {
		t.Error("ApprovedAt should be nil")
	}
}

func TestDBRequestStore_ConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	// Create requests concurrently
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			req := NewElevationRequest("cmd", "reason", "user", "system", 5*time.Minute, 123)
			store.Create(ctx, req)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify all were created
	pending, _ := store.ListPending(ctx)
	if len(pending) != 5 {
		t.Errorf("After concurrent creates, count = %d, want 5", len(pending))
	}
}

func TestMustDBRequestStore(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)

	t.Run("returns DB store for DB-backed store", func(t *testing.T) {
		result := MustDBRequestStore(store)
		if result != store {
			t.Error("MustDBRequestStore() should return the same store")
		}
	})

	t.Run("panics for non-DB store", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustDBRequestStore() should panic for non-DB store")
			}
		}()

		inMemStore := NewInMemoryRequestStore()
		MustDBRequestStore(inMemStore)
	})
}

func TestDBRequestStore_CompleteWorkflow(t *testing.T) {
	db := setupTestDB(t)
	store := NewDBRequestStore(db)
	ctx := context.Background()

	// 1. Create request
	req := NewElevationRequest("systemctl restart nginx", "Deploy v2.0", "alice", "web-01", 10*time.Minute, 123456)
	req.IPAddress = "192.168.1.100"
	req.UserAgent = "KingCrab/1.0"

	err := store.Create(ctx, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 2. List pending
	pending, _ := store.ListPending(ctx)
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(pending))
	}

	// 3. Approve request
	now := time.Now()
	req.Status = "approved"
	req.ApprovedBy = "tg:789"
	req.ApprovedAt = &now

	err = store.Update(ctx, req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// 4. Log audit
	details := map[string]interface{}{
		"method": "biometric",
	}
	err = store.LogAudit(ctx, "request_approved", &req.ID, nil, nil, "192.168.1.100", "KingCrab/1.0", details)
	if err != nil {
		t.Fatalf("LogAudit() error = %v", err)
	}

	// 5. Verify final state
	final, _ := store.Get(ctx, req.ID)
	if final.Status != "approved" {
		t.Errorf("Final status = %s, want 'approved'", final.Status)
	}

	// 6. Verify no longer in pending
	pending, _ = store.ListPending(ctx)
	if len(pending) != 0 {
		t.Errorf("Should have no pending requests, got %d", len(pending))
	}
}