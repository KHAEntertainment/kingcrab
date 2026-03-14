package pam

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DBRequestStore is a PostgreSQL-backed implementation
type DBRequestStore struct {
	db *sql.DB
}

// NewDBRequestStore creates a new database-backed store
func NewDBRequestStore(db *sql.DB) *DBRequestStore {
	return &DBRequestStore{db: db}
}

// Create inserts a new request
func (s *DBRequestStore) Create(ctx context.Context, req *ElevationRequest) error {
	query := `
		INSERT INTO elevation_requests 
		(id, requester, target_system, command, reason, status, created_at, expires_at, ip_address, user_agent, notify_chat_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := s.db.ExecContext(ctx, query,
		req.ID, req.Requester, req.TargetSystem, req.Command, req.Reason,
		req.Status, req.CreatedAt, req.ExpiresAt, req.IPAddress, req.UserAgent, req.NotifyChatID,
	)
	return err
}

// Get retrieves a request by ID
func (s *DBRequestStore) Get(ctx context.Context, id string) (*ElevationRequest, error) {
	query := `
		SELECT id, requester, target_system, command, reason, status, 
		       created_at, expires_at, approved_by, approved_at, 
		       ip_address, user_agent, notify_chat_id
		FROM elevation_requests WHERE id = $1
	`
	var req ElevationRequest
	var approvedBy sql.NullString
	var approvedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&req.ID, &req.Requester, &req.TargetSystem, &req.Command, &req.Reason,
		&req.Status, &req.CreatedAt, &req.ExpiresAt, &approvedBy, &approvedAt,
		&req.IPAddress, &req.UserAgent, &req.NotifyChatID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if approvedBy.Valid {
		req.ApprovedBy = approvedBy.String
	}
	if approvedAt.Valid {
		req.ApprovedAt = &approvedAt.Time
	}

	return &req, nil
}

// Update modifies an existing request
func (s *DBRequestStore) Update(ctx context.Context, req *ElevationRequest) error {
	query := `
		UPDATE elevation_requests SET
			status = $2,
			approved_by = $3,
			approved_at = $4,
			output = $5,
			exit_code = $6
		WHERE id = $1
	`
	var approvedBy, output *string
	var exitCode *int
	var approvedAt *time.Time

	if req.ApprovedBy != "" {
		approvedBy = &req.ApprovedBy
	}
	if req.ApprovedAt != nil {
		approvedAt = req.ApprovedAt
	}

	_, err := s.db.ExecContext(ctx, query, req.ID, req.Status, approvedBy, approvedAt, output, exitCode)
	return err
}

// UpdateStateIf atomically updates state only if current state matches expected
func (s *DBRequestStore) UpdateStateIf(ctx context.Context, id string, expectedState string, newState string, approvedBy string) (bool, error) {
	query := `
		UPDATE elevation_requests
		SET status = $2, approved_by = $3, approved_at = NOW()
		WHERE id = $1 AND status = $4
	`

	var approvedByPtr *string
	if approvedBy != "" {
		approvedByPtr = &approvedBy
	}

	result, err := s.db.ExecContext(ctx, query, id, newState, approvedByPtr, expectedState)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

// ListPending returns all pending requests
func (s *DBRequestStore) ListPending(ctx context.Context) ([]*ElevationRequest, error) {
	query := `
		SELECT id, requester, target_system, command, reason, status, 
		       created_at, expires_at, approved_by, approved_at, 
		       ip_address, user_agent, notify_chat_id
		FROM elevation_requests 
		WHERE status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*ElevationRequest
	for rows.Next() {
		var req ElevationRequest
		var approvedBy sql.NullString
		var approvedAt sql.NullTime

		err := rows.Scan(
			&req.ID, &req.Requester, &req.TargetSystem, &req.Command, &req.Reason,
			&req.Status, &req.CreatedAt, &req.ExpiresAt, &approvedBy, &approvedAt,
			&req.IPAddress, &req.UserAgent, &req.NotifyChatID,
		)
		if err != nil {
			return nil, err
		}

		if approvedBy.Valid {
			req.ApprovedBy = approvedBy.String
		}
		if approvedAt.Valid {
			req.ApprovedAt = &approvedAt.Time
		}

		result = append(result, &req)
	}

	return result, rows.Err()
}

// Delete removes a request
func (s *DBRequestStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM elevation_requests WHERE id = $1", id)
	return err
}

// LogAudit logs an approval action
func (s *DBRequestStore) LogAudit(ctx context.Context, action string, requestID *string, deviceID *int, userID *int, ipAddress, userAgent string, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal details: %w", err)
	}

	query := `
		INSERT INTO approval_audit 
		(request_id, device_id, user_id, action, ip_address, user_agent, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = s.db.ExecContext(ctx, query, requestID, deviceID, userID, action, ipAddress, userAgent, detailsJSON)
	return err
}

// GetAuthorizedUsers returns all authorized users
func (s *DBRequestStore) GetAuthorizedUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT telegram_id, display_name FROM authorized_users WHERE is_active = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.TelegramID, &u.Name); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Compile-time check
var _ RequestStore = (*DBRequestStore)(nil)

// MustDBRequestStore panics if the store isn't a DB-backed one
func MustDBRequestStore(store RequestStore) *DBRequestStore {
	if db, ok := store.(*DBRequestStore); ok {
		return db
	}
	panic("RequestStore is not DB-backed")
}