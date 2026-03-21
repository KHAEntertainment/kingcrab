# KingCrab Hybrid Implementation Tasks

## Phase 1: Daemon Core (Go)

### 1.1 Database Layer

**File:** `internal/database/db.go`
```go
// DB manages PostgreSQL connection
type DB struct {
    pool *pgxpool.Pool
}

// NewDB creates a new database connection
func NewDB(ctx context.Context, cfg Config) (*DB, error)

// Close closes the database connection
func (db *DB) Close()
```

**File:** `internal/database/store.go`
```go
// RequestStore manages elevation requests
type RequestStore interface {
    Create(ctx context.Context, req *ElevationRequest) error
    Get(ctx context.Context, id string) (*ElevationRequest, error)
    List(ctx context.Context, filter RequestFilter) ([]*ElevationRequest, error)
    UpdateStatus(ctx context.Context, id, status string) error
    UpdateStatusIf(ctx context.Context, id, currentStatus, newStatus, approvedBy string) (bool, error)
}

// DBRequestStore implements RequestStore with PostgreSQL
type DBRequestStore struct {
    db *DB
}
```

**Tasks:**
- [ ] Implement `NewDB()` with pgxpool connection
- [ ] Implement `DBRequestStore.Create()` with INSERT
- [ ] Implement `DBRequestStore.Get()` with SELECT
- [ ] Implement `DBRequestStore.List()` with filtering
- [ ] Implement `DBRequestStore.UpdateStatusIf()` atomic update
- [ ] Add connection pooling configuration
- [ ] Add database migration runner

### 1.2 HTTP Server

**File:** `internal/daemon/server.go`
```go
// Server is the KingCrab HTTP server
type Server struct {
    config    *config.Config
    store     database.RequestStore
    executor  *executor.Executor
    notifer   *notifier.OpenClawNotifier
    server    *http.Server
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, store database.RequestStore) (*Server, error)

// Start starts the HTTP server
func (s *Server) Start() error

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error
```

**File:** `internal/daemon/handlers.go`
```go
// Handlers implements HTTP request handlers
type Handlers struct {
    server *Server
}

// RegisterRoutes registers all HTTP routes
func (h *Handlers) RegisterRoutes(mux *http.ServeMux)

// HandleCreateRequest handles POST /api/v1/request
func (h *Handlers) HandleCreateRequest(w http.ResponseWriter, r *http.Request)

// HandleGetRequest handles GET /api/v1/request/:id
func (h *Handlers) HandleGetRequest(w http.ResponseWriter, r *http.Request)

// HandleApprove handles POST /api/v1/request/:id/approve
func (h *Handlers) HandleApprove(w http.ResponseWriter, r *http.Request)

// HandleDeny handles POST /api/v1/request/:id/deny
func (h *Handlers) HandleDeny(w http.ResponseWriter, r *http.Request)
```

**Tasks:**
- [ ] Implement `NewServer()` with dependencies
- [ ] Implement `Start()` with graceful shutdown
- [ ] Implement `RegisterRoutes()` with all endpoints
- [ ] Add middleware (logging, recovery, CORS)
- [ ] Add authentication middleware for /approve and /deny
- [ ] Implement request validation
- [ ] Add rate limiting

### 1.3 Command Executor

**File:** `internal/executor/executor.go`
```go
// Executor executes approved commands
type Executor struct {
    allowlist    *security.Allowlist
    maxDuration  time.Duration
}

// NewExecutor creates a new command executor
func NewExecutor(allowlist []string, timeout time.Duration) *Executor

// Execute runs a command if allowed
func (e *Executor) Execute(ctx context.Context, cmd string) (*Result, error)

// Result contains command execution results
type Result struct {
    ExitCode int
    Stdout   string
    Stderr   string
    Duration time.Duration
}
```

**Tasks:**
- [ ] Implement `Execute()` with timeout
- [ ] Add allowlist validation
- [ ] Add shell metacharacter filtering
- [ ] Add stdout/stderr capture
- [ ] Add execution logging

### 1.4 OpenClaw Notification Integration

**File:** `internal/notifications/openclaw.go`
```go
// OpenClawNotifier sends notifications via OpenClaw
type OpenClawNotifier struct {
    webhookURL string
    httpClient *http.Client
}

// Notify sends a notification about a new request
func (n *OpenClawNotifier) Notify(ctx context.Context, req *ElevationRequest) error

// NotifyApproved sends notification when request is approved
func (n *OpenClawNotifier) NotifyApproved(ctx context.Context, req *ElevationRequest) error

// NotifyDenied sends notification when request is denied
func (n *OpenClawNotifier) NotifyDenied(ctx context.Context, req *ElevationRequest) error
```

**Tasks:**
- [ ] Implement `Notify()` with webhook call
- [ ] Add retry logic with exponential backoff
- [ ] Add message formatting for Telegram
- [ ] Add inline keyboard button generation
- [ ] Handle webhook failures gracefully

### 1.5 Configuration

**File:** `internal/config/config.go`
```go
// Config holds daemon configuration
type Config struct {
    Version     string              `json:"version"`
    Server      ServerConfig        `json:"server"`
    Database    DatabaseConfig      `json:"database"`
    Security    SecurityConfig      `json:"security"`
    Notifications NotificationsConfig `json:"notifications"`
    Logging     LoggingConfig       `json:"logging"`
}

// Load loads configuration from file
func Load(path string) (*Config, error)

// Validate validates the configuration
func (c *Config) Validate() error
```

**Tasks:**
- [ ] Implement `Load()` with JSON parsing
- [ ] Implement `Validate()` with schema validation
- [ ] Add environment variable overrides
- [ ] Add default values
- [ ] Add config validation command

---

## Phase 2: Plugin (TypeScript)

### 2.1 Plugin Core

**File:** `plugin/kingcrab-plugin.ts`
```typescript
interface KingCrabPluginConfig {
  daemonUrl: string;
  useUnixSocket: boolean;
  socketPath?: string;
}

class KingCrabPlugin extends Plugin {
  private config: KingCrabPluginConfig;
  private daemonClient: DaemonClient;

  async onLoad(config: PluginConfig): Promise<void> {
    this.config = this.parseConfig(config);
    this.daemonClient = new DaemonClient(this.config);
  }

  registerTools(): Tool[] {
    return [
      this.createRequestTool(),
      this.listRequestsTool(),
      this.approveRequestTool(),
    ];
  }
}
```

**Tasks:**
- [ ] Implement `KingCrabPlugin` class
- [ ] Add tool registration
- [ ] Add configuration parsing
- [ ] Add error handling

### 2.2 Daemon Client

**File:** `plugin/daemon-client.ts`
```typescript
class DaemonClient {
  private baseURL: string;
  private httpClient: HTTPClient;

  async createRequest(request: CreateRequestDto): Promise<Request>;
  async listRequests(filter: RequestFilter): Promise<Request[]>;
  async getRequest(id: string): Promise<Request>;
  async approveRequest(id: string, token: string): Promise<ApprovalResult>;
  async denyRequest(id: string, reason: string, token: string): Promise<void>;
}
```

**Tasks:**
- [ ] Implement `createRequest()` with POST
- [ ] Implement `listRequests()` with GET
- [ ] Implement `approveRequest()` with POST
- [ ] Implement `denyRequest()` with POST
- [ ] Add Unix socket support
- [ ] Add error handling and retries

### 2.3 Tool Definitions

**File:** `plugin/tools/create-request.ts`
```typescript
const createRequestTool: Tool = {
  name: 'kingcrab_request',
  description: 'Create a privileged command request requiring approval',
  inputSchema: {
    type: 'object',
    properties: {
      command: { type: 'string', description: 'Command to execute' },
      reason: { type: 'string', description: 'Reason for request' },
    },
    required: ['command'],
  },
  handler: async (input) => {
    // Implementation
  },
};
```

**Tasks:**
- [ ] Implement `kingcrab_request` tool
- [ ] Implement `kingcrab_list` tool
- [ ] Implement `kingcrab_approve` tool
- [ ] Add input validation
- [ ] Add output formatting

### 2.4 Telegram Integration

**File:** `plugin/telegram/handler.ts`
```typescript
class TelegramHandler {
  async handleCallback(callback: CallbackQuery): Promise<string>;
  async formatRequestMessage(request: Request): Promise<string>;
  async createInlineKeyboard(request: Request): Promise<InlineKeyboard>;
}
```

**Tasks:**
- [ ] Implement callback handler
- [ ] Add message formatting
- [ ] Add inline keyboard generation
- [ ] Handle approval/deny callbacks

---

## Phase 3: Installation & Deployment

### 3.1 Daemon Installer

**File:** `installer/install.sh`
```bash
#!/bin/bash
set -euo pipefail

# Check prerequisites
# Create user
# Install binary
# Create config
# Initialize database
# Install systemd service
# Start service
```

**Tasks:**
- [ ] Add prerequisite checks
- [ ] Add user creation
- [ ] Add binary installation
- [ ] Add database initialization
- [ ] Add systemd service installation
- [ ] Add service start

### 3.2 Plugin Installer

**File:** `plugin/install.sh`
```bash
#!/bin/bash
# Copy to OpenClaw extensions
# Install dependencies
# Build plugin
# Configure OpenClaw
```

**Tasks:**
- [ ] Add extension directory creation
- [ ] Add dependency installation
- [ ] Add build step
- [ ] Add OpenClaw configuration

---

## Phase 4: Testing

### 4.1 Daemon Tests

**File:** `internal/database/store_test.go`
```go
func TestDBRequestStore_Create(t *testing.T)
func TestDBRequestStore_Get(t *testing.T)
func TestDBRequestStore_List(t *testing.T)
func TestDBRequestStore_UpdateStatusIf(t *testing.T)
```

**Tasks:**
- [ ] Add database integration tests
- [ ] Add HTTP handler tests
- [ ] Add executor tests
- [ ] Add end-to-end tests

### 4.2 Plugin Tests

**File:** `plugin/daemon-client.test.ts`
```typescript
describe('DaemonClient', () => {
  it('should create request', async () => {});
  it('should list requests', async () => {});
  it('should approve request', async () => {});
});
```

**Tasks:**
- [ ] Add unit tests for daemon client
- [ ] Add tool handler tests
- [ ] Add Telegram handler tests

---

## Implementation Order

### Sprint 1: Foundation (Week 1)
1. Database layer with migrations
2. Basic HTTP server with health endpoint
3. Configuration system

### Sprint 2: Core Features (Week 2)
1. Request creation endpoint
2. Request store implementation
3. Command executor

### Sprint 3: Integration (Week 3)
1. Approval/deny endpoints
2. OpenClaw notification integration
3. Plugin daemon client

### Sprint 4: Plugin & Polish (Week 4)
1. Plugin tool implementations
2. Telegram inline buttons
3. Installation scripts
4. Documentation

### Sprint 5: Testing & Release (Week 5)
1. Test coverage
2. Security audit
3. Performance testing
4. Release preparation

---

## Milestones

- **M1:** Daemon can store and retrieve requests from database
- **M2:** Agent can create requests via plugin
- **M3:** User can approve/deny via Telegram
- **M4:** Commands execute after approval
- **M5:** Production-ready with installer and docs
