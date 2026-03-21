package pam

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteRequestStore is a SQLite-backed persistent implementation of RequestStore.
type SQLiteRequestStore struct {
	db *sql.DB
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS elevation_requests (
	id              TEXT PRIMARY KEY,
	requester       TEXT    NOT NULL,
	target_system   TEXT    NOT NULL,
	command         TEXT    NOT NULL,
	reason          TEXT    NOT NULL,
	status          TEXT    NOT NULL DEFAULT 'pending',
	created_at      TEXT    NOT NULL,
	expires_at      TEXT    NOT NULL,
	approved_by     TEXT,
	approved_at     TEXT,
	ip_address      TEXT,
	user_agent      TEXT,
	notify_chat_id  INTEGER NOT NULL DEFAULT 0,
	output          TEXT,
	exit_code       INTEGER
);

CREATE INDEX IF NOT EXISTS idx_er_status  ON elevation_requests(status);
CREATE INDEX IF NOT EXISTS idx_er_expires ON elevation_requests(expires_at);
`

// NewSQLiteRequestStore opens (or creates) a SQLite database at path and
// initialises the schema. Pass ":memory:" for an ephemeral in-process database.
func NewSQLiteRequestStore(path string) (*SQLiteRequestStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	// SQLite supports only one concurrent writer; cap the pool to one connection.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialise sqlite schema: %w", err)
	}

	return &SQLiteRequestStore{db: db}, nil
}

// Close releases the underlying database connection.
func (s *SQLiteRequestStore) Close() error {
	return s.db.Close()
}

// Create inserts a new elevation request.
func (s *SQLiteRequestStore) Create(ctx context.Context, req *ElevationRequest) error {
	const query = `
		INSERT INTO elevation_requests
		(id, requester, target_system, command, reason, status,
		 created_at, expires_at, ip_address, user_agent, notify_chat_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		req.ID, req.Requester, req.TargetSystem, req.Command, req.Reason,
		req.Status,
		req.CreatedAt.UTC().Format(time.RFC3339Nano),
		req.ExpiresAt.UTC().Format(time.RFC3339Nano),
		req.IPAddress, req.UserAgent, req.NotifyChatID,
	)
	return err
}

// Get retrieves a request by ID. Returns nil, nil when the record is not found.
func (s *SQLiteRequestStore) Get(ctx context.Context, id string) (*ElevationRequest, error) {
	const query = `
		SELECT id, requester, target_system, command, reason, status,
		       created_at, expires_at, approved_by, approved_at,
		       ip_address, user_agent, notify_chat_id
		FROM elevation_requests WHERE id = ?
	`
	req, err := scanSQLiteRequest(s.db.QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return req, err
}

// Update modifies the mutable fields of an existing request.
func (s *SQLiteRequestStore) Update(ctx context.Context, req *ElevationRequest) error {
	const query = `
		UPDATE elevation_requests
		SET status      = ?,
		    approved_by = ?,
		    approved_at = ?,
		    output      = ?,
		    exit_code   = ?
		WHERE id = ?
	`
	var approvedBy *string
	if req.ApprovedBy != "" {
		approvedBy = &req.ApprovedBy
	}
	var approvedAt *string
	if req.ApprovedAt != nil {
		s := req.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &s
	}
	_, err := s.db.ExecContext(ctx, query,
		req.Status, approvedBy, approvedAt,
		req.Output, req.ExitCode,
		req.ID,
	)
	return err
}

// UpdateStateIf atomically updates status only when the current status matches
// expectedState, mirroring the semantics of DBRequestStore.UpdateStateIf.
func (s *SQLiteRequestStore) UpdateStateIf(ctx context.Context, id, expectedState, newState, approvedBy string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var (
		result sql.Result
		err    error
	)

	switch newState {
	case "approved", "denied", "failed":
		// Record the actor and timestamp; reject if already expired.
		const query = `
			UPDATE elevation_requests
			SET status = ?, approved_by = ?, approved_at = ?
			WHERE id = ? AND status = ? AND expires_at > ?
		`
		var ab *string
		if approvedBy != "" {
			ab = &approvedBy
		}
		result, err = s.db.ExecContext(ctx, query, newState, ab, now, id, expectedState, now)

	case "expired":
		// Expiration is not gated on expires_at (it may already have passed).
		const query = `
			UPDATE elevation_requests
			SET status = ?
			WHERE id = ? AND status = ?
		`
		result, err = s.db.ExecContext(ctx, query, newState, id, expectedState)

	default:
		// Other non-terminal states (e.g. "executing"): update status only.
		const query = `
			UPDATE elevation_requests
			SET status = ?
			WHERE id = ? AND status = ?
		`
		result, err = s.db.ExecContext(ctx, query, newState, id, expectedState)
	}

	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ListPending returns all requests with status 'pending' that have not yet expired.
func (s *SQLiteRequestStore) ListPending(ctx context.Context) ([]*ElevationRequest, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	const query = `
		SELECT id, requester, target_system, command, reason, status,
		       created_at, expires_at, approved_by, approved_at,
		       ip_address, user_agent, notify_chat_id
		FROM elevation_requests
		WHERE status = 'pending' AND expires_at > ?
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*ElevationRequest
	for rows.Next() {
		req, err := scanSQLiteRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

// Delete removes a request by ID.
func (s *SQLiteRequestStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM elevation_requests WHERE id = ?", id)
	return err
}

// ExpireOldRequests marks all expired pending requests as 'expired'.
func (s *SQLiteRequestStore) ExpireOldRequests(ctx context.Context) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	const query = `
		UPDATE elevation_requests
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at <= ?
	`
	result, err := s.db.ExecContext(ctx, query, now)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

// sqliteScanner is satisfied by both *sql.Row and *sql.Rows.
type sqliteScanner interface {
	Scan(dest ...any) error
}

// scanSQLiteRequest reads an ElevationRequest from a row/rows scanner.
func scanSQLiteRequest(row sqliteScanner) (*ElevationRequest, error) {
	var req ElevationRequest
	var createdAt, expiresAt string
	var approvedBy, approvedAtStr sql.NullString

	if err := row.Scan(
		&req.ID, &req.Requester, &req.TargetSystem, &req.Command, &req.Reason,
		&req.Status, &createdAt, &expiresAt,
		&approvedBy, &approvedAtStr,
		&req.IPAddress, &req.UserAgent, &req.NotifyChatID,
	); err != nil {
		return nil, err
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		req.CreatedAt = t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, expiresAt); err == nil {
		req.ExpiresAt = t.UTC()
	}
	if approvedBy.Valid {
		req.ApprovedBy = approvedBy.String
	}
	if approvedAtStr.Valid {
		if t, err := time.Parse(time.RFC3339Nano, approvedAtStr.String); err == nil {
			utc := t.UTC()
			req.ApprovedAt = &utc
		}
	}

	return &req, nil
}

// Compile-time interface check.
var _ RequestStore = (*SQLiteRequestStore)(nil)
