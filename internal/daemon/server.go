package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/KHAEntertainment/kingcrab/internal/config"
	"github.com/KHAEntertainment/kingcrab/internal/logger"
	"github.com/google/uuid"
)

// RequestStatus represents the status of a request
type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved"
	StatusDenied   RequestStatus = "denied"
	StatusComplete RequestStatus = "completed"
	StatusFailed   RequestStatus = "failed"
)

// Request represents a privileged access request
type Request struct {
	ID        string         `json:"id"`
	Command   string         `json:"command"`
	Reason    string         `json:"reason"`
	Status    RequestStatus  `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	Result    *ExecuteResult `json:"result,omitempty"`
}

// Server is the KingCrab HTTP server
type Server struct {
	config    *config.Config
	executor  *Executor
	queue     map[string]*Request
	mu        sync.RWMutex
	httpServer *http.Server
}

// ErrCommandNotAllowed is returned when command is not in allowlist
var ErrCommandNotAllowed = fmt.Errorf("command not allowed")

// NewServer creates a new KingCrab server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config:   cfg,
		executor: NewExecutor(cfg.AllowedCommands),
		queue:    make(map[string]*Request),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", s.config.Port),
		Handler: s.router(),
	}

	logger.Info("Starting KingCrab server", map[string]interface{}{
		"port": s.config.Port,
	})

	return s.httpServer.ListenAndServe()
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func (s *Server) router() http.Handler {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/requests", s.handleRequests)
	mux.HandleFunc("/request", s.handleRequest)
	mux.HandleFunc("/approve/", s.handleApprove)
	mux.HandleFunc("/deny/", s.handleDeny)
	
	return mux
}

// handleHealth returns health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	})
}

// handleRequests returns all requests
func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	
	requests := make([]*Request, 0, len(s.queue))
	for _, req := range s.queue {
		requests = append(requests, req)
	}
	
	json.NewEncoder(w).Encode(requests)
}

// handleRequest creates a new request
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command string `json:"command"`
		Reason  string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate command
	if req.Command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}

	// Check allowlist
	if !s.executor.IsAllowed(req.Command) {
		http.Error(w, "command not in allowlist", http.StatusForbidden)
		return
	}

	// Check reason required
	if s.config.RequireReason && req.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	// Create request
	request := &Request{
		ID:        uuid.New().String(),
		Command:   req.Command,
		Reason:    req.Reason,
		Status:    StatusPending,
		Timestamp: time.Now(),
	}

	s.mu.Lock()
	s.queue[request.ID] = request
	s.mu.Unlock()

	logger.Info("Request created", map[string]interface{}{
		"request_id": request.ID,
		"command":    request.Command,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(request)
}

// handleApprove approves a request
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/approve/")
	if id == "" {
		http.Error(w, "request id required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.queue[id]
	if !ok {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	if req.Status != StatusPending {
		http.Error(w, "request already processed", http.StatusBadRequest)
		return
	}

	// Execute command
	result, err := s.executor.Execute(req.Command)
	if err != nil {
		req.Status = StatusFailed
		req.Result = result
		logger.Error("Command execution failed", map[string]interface{}{
			"request_id": req.ID,
			"command":    req.Command,
			"error":      err.Error(),
		})
	} else {
		req.Status = StatusComplete
		req.Result = result
		logger.Info("Request completed", map[string]interface{}{
			"request_id":  req.ID,
			"command":    req.Command,
			"exit_code":  result.ExitCode,
			"duration_ms": result.Duration,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// handleDeny denies a request
func (s *Server) handleDeny(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/deny/")
	if id == "" {
		http.Error(w, "request id required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.queue[id]
	if !ok {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	if req.Status != StatusPending {
		http.Error(w, "request already processed", http.StatusBadRequest)
		return
	}

	req.Status = StatusDenied

	logger.Info("Request denied", map[string]interface{}{
		"request_id": req.ID,
		"command":    req.Command,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}
