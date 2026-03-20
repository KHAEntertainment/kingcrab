package pam

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RequestStore defines the interface for storing elevation requests
type RequestStore interface {
	Create(ctx context.Context, req *ElevationRequest) error
	Get(ctx context.Context, id string) (*ElevationRequest, error)
	Update(ctx context.Context, req *ElevationRequest) error
	UpdateStateIf(ctx context.Context, id string, expectedState string, newState string, approvedBy string) (bool, error)
	ListPending(ctx context.Context) ([]*ElevationRequest, error)
	Delete(ctx context.Context, id string) error
	ExpireOldRequests(ctx context.Context) (int, error)
}

// ElevationRequest represents an elevation request
type ElevationRequest struct {
	ID           string     `json:"id"`
	Command      string     `json:"command"`
	Reason       string     `json:"reason"`
	Requester    string     `json:"requester"`
	TargetSystem string     `json:"target_system"`
	Status       string     `json:"status"` // pending/approved/denied/expired
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	ApprovedBy   string     `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	NotifyChatID int64      `json:"notify_chat_id"`
	IPAddress    string     `json:"ip_address,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	Output       *string    `json:"output,omitempty"`
	ExitCode     *int       `json:"exit_code,omitempty"`
}

// NewElevationRequest creates a new elevation request
func NewElevationRequest(command, reason, requester, targetSystem string, ttl time.Duration, notifyChatID int64) *ElevationRequest {
	return &ElevationRequest{
		ID:           uuid.New().String(),
		Command:      command,
		Reason:       reason,
		Requester:    requester,
		TargetSystem: targetSystem,
		Status:       "pending",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(ttl),
		NotifyChatID: notifyChatID,
	}
}

// deepCopyElevationRequest creates a deep copy of an ElevationRequest
func deepCopyElevationRequest(req *ElevationRequest) *ElevationRequest {
	if req == nil {
		return nil
	}

	copy := &ElevationRequest{
		ID:           req.ID,
		Command:      req.Command,
		Reason:       req.Reason,
		Requester:    req.Requester,
		TargetSystem: req.TargetSystem,
		Status:       req.Status,
		CreatedAt:    req.CreatedAt,
		ExpiresAt:    req.ExpiresAt,
		ApprovedBy:   req.ApprovedBy,
		NotifyChatID: req.NotifyChatID,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
	}

	// Copy pointer fields
	if req.ApprovedAt != nil {
		approvedAt := *req.ApprovedAt
		copy.ApprovedAt = &approvedAt
	}
	if req.Output != nil {
		output := *req.Output
		copy.Output = &output
	}
	if req.ExitCode != nil {
		exitCode := *req.ExitCode
		copy.ExitCode = &exitCode
	}

	return copy
}

// InMemoryRequestStore is a simple in-memory implementation for development
type InMemoryRequestStore struct {
	mu       sync.RWMutex
	requests map[string]*ElevationRequest
}

// NewInMemoryRequestStore creates a new in-memory store
func NewInMemoryRequestStore() *InMemoryRequestStore {
	return &InMemoryRequestStore{
		requests: make(map[string]*ElevationRequest),
	}
}

// Create adds a new request
func (s *InMemoryRequestStore) Create(ctx context.Context, req *ElevationRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Store a deep copy to prevent caller mutations
	s.requests[req.ID] = deepCopyElevationRequest(req)
	return nil
}

// Get retrieves a request by ID
func (s *InMemoryRequestStore) Get(ctx context.Context, id string) (*ElevationRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a deep copy to prevent caller mutations
	return deepCopyElevationRequest(s.requests[id]), nil
}

// Update modifies an existing request
func (s *InMemoryRequestStore) Update(ctx context.Context, req *ElevationRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Store a deep copy to prevent caller mutations
	s.requests[req.ID] = deepCopyElevationRequest(req)
	return nil
}

// UpdateStateIf atomically updates state only if current state matches expected
func (s *InMemoryRequestStore) UpdateStateIf(ctx context.Context, id string, expectedState string, newState string, approvedBy string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, exists := s.requests[id]
	if !exists {
		return false, nil
	}

	if req.Status != expectedState {
		return false, nil
	}

	// Reject expired requests - cannot approve/deny expired requests
	if time.Now().After(req.ExpiresAt) {
		return false, nil
	}

	// Make a deep copy for mutation to avoid modifying the stored instance
	reqCopy := deepCopyElevationRequest(req)

	// Perform state transition
	reqCopy.Status = newState

	// Only set approval metadata when transitioning to "approved" state
	if newState == "approved" && approvedBy != "" {
		reqCopy.ApprovedBy = approvedBy
		now := time.Now()
		reqCopy.ApprovedAt = &now
	} else {
		// Clear approval metadata for non-approved states
		reqCopy.ApprovedBy = ""
		reqCopy.ApprovedAt = nil
	}

	s.requests[id] = reqCopy
	return true, nil
}

// ListPending returns all pending requests
func (s *InMemoryRequestStore) ListPending(ctx context.Context) ([]*ElevationRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ElevationRequest
	for _, req := range s.requests {
		if req.Status == "pending" && time.Now().Before(req.ExpiresAt) {
			// Return deep copies to prevent caller mutations
			result = append(result, deepCopyElevationRequest(req))
		}
	}
	return result, nil
}

// Delete removes a request
func (s *InMemoryRequestStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.requests, id)
	return nil
}

// ExpireOldRequests marks all expired pending requests as expired
func (s *InMemoryRequestStore) ExpireOldRequests(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for id, req := range s.requests {
		// Only expire requests that are still pending and past their expiration time
		if req.Status == "pending" && now.After(req.ExpiresAt) {
			// Create a copy and update its status
			reqCopy := deepCopyElevationRequest(req)
			reqCopy.Status = "expired"
			s.requests[id] = reqCopy
			expiredCount++
		}
	}

	return expiredCount, nil
}

// Compile-time check
var _ RequestStore = (*InMemoryRequestStore)(nil)