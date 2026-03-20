# KingCrab Hybrid Architecture

## Executive Summary

KingCrab v2 uses a **hybrid architecture** combining:
1. **Go Daemon** - Systemd service running as root for privilege escalation
2. **OpenClaw Plugin** - TypeScript plugin leveraging OpenClaw's Telegram integration
3. **PostgreSQL** - Database-backed request store with audit logging

This architecture provides:
- Security: Daemon isolation, privilege separation, audit trails
- Usability: Native OpenClaw Telegram integration with inline buttons
- Production-ready: Database persistence, structured logging, systemd integration

---

## Component Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AGENT WORKSPACE                                     │
│  ┌──────────────┐         ┌──────────────────────────────────────────────┐ │
│  │   Agent      │────────▶│           OpenClaw Core                       │ │
│  │  (Claude)    │         │    - Conversation Manager                     │ │
│  └──────────────┘         │    - Plugin System                            │ │
│                           │    - Telegram Channel (linked session)        │ │
│                           └───────────────────┬──────────────────────────┐ │
│                                               │                              │
│                           ┌───────────────────▼──────────────────────────┐ │
│                           │    KingCrab Plugin (TypeScript)               │ │
│                           │    ~/.openclaw/extensions/kingcrab/           │ │
│                           │    ┌────────────────────────────────────────┐ │ │
│                           │    │ Tools:                                 │ │ │
│                           │    │  - kingcrab_request                    │ │ │
│                           │    │  - kingcrab_list                       │ │ │
│                           │    │  - kingcrab_approve                    │ │ │
│                           │    └────────────────────────────────────────┘ │ │
│                           └───────────────────┬──────────────────────────┘ │
│                                               │ HTTP (localhost:8080)       │
└───────────────────────────────────────────────┼─────────────────────────────┘
                                                │
                                                │ Unix Socket / HTTP
                                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SYSTEM LEVEL (Privileged)                            │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    KingCrab Daemon (Go)                               │  │
│  │                    /usr/local/bin/kingcrab                            │  │
│  │                    systemd: kingcrab.service                          │  │
│  │                    Runs as: root (via capability)                     │  │
│  │                                                                      │  │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │  │
│  │  │   HTTP Server   │  │   Request       │  │    Command          │  │  │
│  │  │   (API)         │  │   Store         │  │    Executor         │  │  │
│  │  │                 │  │   (PostgreSQL)  │  │                     │  │  │
│  │  │  POST /api/v1/request        │  │                 │  │  - Validate         │  │  │
│  │  │  GET  /api/v1/requests       │  │  - Requests     │  │  - Execute          │  │  │
│  │  │  POST /api/v1/request/{id}/approve  │  - Users  │  │  - Log result       │  │  │
│  │  │  POST /api/v1/request/{id}/deny     │  - Devices│  │                     │  │  │
│  │  │                 │  │  - Audit        │  │                     │  │  │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────┘  │  │
│  │                                                                      │  │
│  │  ┌──────────────────────────────────────────────────────────────┐  │  │
│  │  │              Notification Service                             │  │  │
│  │  │              (via OpenClaw Telegram)                         │  │  │
│  │  └──────────────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    PostgreSQL Database                                │  │
│  │                    kingcrab database                                  │  │
│  │                                                                      │  │
│  │  - elevation_requests                                                │  │
│  │  - authorized_users                                                  │  │
│  │  - enrolled_devices                                                  │  │
│  │  - approval_audit                                                    │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                         USER'S PHONE                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    Telegram App                                       │  │
│  │                                                                      │  │
│  │  🔐 KingCrab Request #abc12345                                       │  │
│  │     Command: apt install golang-go                                   │  │
│  │     Reason: Need Go for building CLI                                 │  │
│  │                                                                      │  │
│  │     [✅ Approve]  [🚫 Deny]                                          │  │
│  │                                                                      │  │
│  │  (Inline buttons trigger OpenClaw's Telegram channel)               │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Communication Flow

### 1. Agent Request Flow

```
Agent                   Plugin                  Daemon                  Database
  │                       │                       │                        │
  │ "sudo apt install"    │                       │                        │
  ├─────────────────────▶│                       │                        │
  │                       │ POST /api/v1/request  │                        │
  │                       │ (command, reason)     │                        │
  │                       ├─────────────────────▶│                        │
  │                       │                       │ Validate allowlist     │
  │                       │                       │ Create request         │
  │                       │                       ├─────────────────────▶│
  │                       │                       │ INSERT request         │
  │                       │                       │◀─────────────────────┤
  │                       │                       │                        │
  │                       │                       │ Send notification      │
  │                       │                       │ to OpenClaw            │
  │                       │◀─────────────────────┤                        │
  │ ◀─────────────────────┤ {request_id, status}  │                        │
  │                       │                       │                        │
```

### 2. User Approval Flow (via OpenClaw Telegram)

```
User's Phone             OpenClaw                Plugin                  Daemon
     │                       │                       │                       │
     │ Click "Approve"       │                       │                       │
     ├─────────────────────▶│                       │                       │
     │                       │ Callback to plugin    │                       │
     │                       ├─────────────────────▶│                       │
     │                       │                       │ POST /api/v1/request/{id}/approve │
     │                       │                       │ (request_id, token)    │
     │                       │                       ├─────────────────────▶│
     │                       │                       │                       │
     │                       │                       │ Verify biometric      │
     │                       │                       │ Update request state  │
     │                       │                       │ Execute command       │
     │                       │◀─────────────────────┤ {success, result}     │
     │                       │ Update UI             │                       │
     │◀──────────────────────┤ "Command executed"    │                       │
     │                       │                       │                       │
```

---

## File Structure

### Daemon (Go)

```
kingcrab/
├── cmd/
│   └── kingcrab/
│       └── main.go                 # Daemon entrypoint
├── internal/
│   ├── api/
│   │   └── v1.go                  # API v1 handlers
│   ├── config/
│   │   └── config.go              # Config loading
│   ├── daemon/
│   │   └── server_v2.go           # HTTP server v2
│   ├── executor/
│   │   └── executor.go            # Command execution
│   ├── db/
│   │   ├── migrations/
│   │   │   └── 001_pam_schema.sql
│   │   └── db.go                  # Database connection
│   ├── pam/
│   │   ├── store_request.go       # RequestStore implementation
│   │   └── pam.go                 # PAM module
│   ├── logger/
│   │   └── logger.go              # Structured logging
│   └── notifications/
│       └── openclaw.go            # OpenClaw integration
├── configs/
│   └── config.json.example        # Example configuration
├── installer/
│   └── install-v2.sh              # Installation script v2
├── go.mod
├── go.sum
└── README.md
```

### Plugin (TypeScript)

```
plugin/
├── package.json
├── tsconfig.json
├── index.ts                       # Main plugin entry point
├── types/
│   └── index.d.ts                 # Type definitions
├── SKILL.md                       # OpenClaw skill documentation
└── README.md
```

---

## API Protocol

### Daemon HTTP API

All endpoints return JSON with `Content-Type: application/json`.

#### POST /api/v1/request
Create a new elevation request.

**Request:**
```json
{
  "command": "apt install golang-go",
  "reason": "Need Go for building CLI",
  "requestedBy": "agent@hostname",
  "telegramUserId": 123456789
}
```

**Response:**
```json
{
  "success": true,
  "request": {
    "id": "abc123...",
    "status": "pending",
    "expires_at": "2026-03-19T12:35:00Z"
  }
}
```

#### GET /api/v1/requests
List all requests (with optional filters).

**Query params:**
- `status`: Filter by status (pending, approved, denied, expired)
- `limit`: Max results (default: 50)
- `requester`: Filter by requester

#### GET /api/v1/request/:id
Get details of a specific request.

#### POST /api/v1/request/:id/approve
Approve a request with biometric authentication.

**Request:**
```json
{
  "biometric_token": "encrypted_token",
  "user_id": "tg:123456789"
}
```

#### POST /api/v1/request/:id/deny
Deny a request.

**Request:**
```json
{
  "reason": "Unnecessary package",
  "biometric_token": "encrypted_token",
  "user_id": "tg:123456789"
}
```

#### GET /api/v1/health
Health check endpoint.

---

## Configuration

### Daemon Config: /etc/kingcrab/config.json

```json
{
  "version": "1.0.0",
  "listen": {
    "type": "unix",
    "path": "/var/run/kingcrab.sock"
  },
  "allowedCommands": [
    "apt install *",
    "apt update",
    "systemctl restart *",
    "systemctl start *",
    "systemctl stop *"
  ],
  "requireReason": true,
  "logLevel": "info",
  "telegram": {
    "botToken": "",
    "allowedUsers": [],
    "webhookUrl": ""
  },
  "pam": {
    "use_clawvault": "auto",
    "clawvault": {
      "socket": "",
      "host": "",
      "token_prefix": "kingcrab/pam/tokens",
      "timeout_seconds": 5
    },
    "fallback": {
      "encryption_key_env": "PAM_FALLBACK_ENCRYPTION_KEY",
      "storage_path": "",
      "ttl_minutes": 5,
      "authorized_users": []
    }
  },
  "openclaw": {
    "webhookUrl": "http://localhost:3000/api/kingcrab/notify",
    "enabled": true
  }
}
```

**Note:** Database credentials (host, port, name, user) are read from environment variables (`KINGCRAB_DB_*`), not from the config file.

### Plugin Config: ~/.openclaw/openclaw.json

```json
{
  "plugins": {
    "entries": {
      "kingcrab": {
        "enabled": true,
        "config": {
          "daemonUrl": "http://localhost:8080",
          "timeout": 30000,
          "allowedCommands": []
        }
      }
    }
  }
}
```

### Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `KINGCRAB_DB_PASSWORD` | Database password | Yes |
| `KINGCRAB_BOT_TOKEN` | Telegram bot token (if not using OpenClaw) | No* |
| `KINGCRAB_LOG_LEVEL` | Override log level | No |

\* Only needed if not using OpenClaw's Telegram integration

---

## Installation Procedure

### Prerequisites

1. **OpenClaw installed** (`~/.openclaw/` directory exists)
2. **PostgreSQL** running with `kingcrab` database
3. **Go 1.21+** for building daemon
4. **Node.js 20+** for plugin
5. **systemd** available
6. **Telegram session linked** in OpenClaw

### 1. Install Daemon

```bash
# Clone repository
git clone https://github.com/KHAEntertainment/kingcrab.git
cd kingcrab

# Build daemon
go build -o kingcrab ./cmd/kingcrab

# Run installer (requires sudo)
sudo ./installer/install-v2.sh

# Installer does:
# - Copy binary to /usr/local/bin/
# - Create config directory /etc/kingcrab/
# - Create database and run migrations
# - Create systemd service
# - Start service

# Verify
sudo systemctl status kingcrab
curl http://localhost:8080/api/v1/health
```

### 2. Install Plugin

```bash
# From kingcrab directory
cd plugin

# Copy to OpenClaw extensions
mkdir -p ~/.openclaw/extensions/kingcrab
cp -r . ~/.openclaw/extensions/kingcrab/

# Install dependencies
cd ~/.openclaw/extensions/kingcrab
npm install

# Build
npm run build

# Configure daemon URL
# Edit ~/.openclaw/openclaw.json to add kingcrab extension config
```

### 3. Configure Database

```bash
# Create database and user
sudo -u postgres createuser kingcrab
sudo -u postgres createdb -O kingcrab kingcrab

# Run migrations using daemon migration command
# Set database environment variables
export KINGCRAB_DB_HOST=localhost
export KINGCRAB_DB_PORT=5432
export KINGCRAB_DB_USER=kingcrab
export KINGCRAB_DB_NAME=kingcrab
export KINGCRAB_DB_PASSWORD=your_password

# Run migrations
kingcrab --migrate
```

### 4. Enroll Biometric Device

```bash
# Via OpenClaw skill
/kc enroll

# Follow prompts in Telegram to authorize device
```

---

## Security Considerations

### 1. Privilege Separation

- **Daemon runs as root** (via systemd, for privilege escalation)
- **Database user has limited permissions** (CRUD on tables, no DDL)
- **Unix socket for localhost communication** (vs HTTP on localhost)

### 2. Authentication

- **Telegram Mini App initData** for initial request
- **Biometric tokens** stored encrypted in database
- **Token verification** on every approve/deny action

### 3. Authorization

- **Command allowlist** with wildcard matching
- **Shell metacharacter filtering** to prevent injection
- **Request expiration** (default 5 minutes)

### 4. Audit Trail

- **All approvals logged** to `approval_audit` table
- **Timestamps, IP addresses, user agents** recorded
- **Immutable logs** (APPEND only, no UPDATE on audit records)

### 5. Network Security

- **Localhost-only binding** by default (127.0.0.1)
- **Unix socket option** for IPC
- **TLS optional** for remote deployments

---

## 2FA Implementation Options

### Option 1: Telegram Biometric Auth (Recommended)

Use Telegram's native biometric authentication in Mini Apps:

1. **Enrollment flow:**
   - User opens KingCrab Mini App via OpenClaw
   - Telegram prompts for biometric (FaceID/Fingerprint)
   - Token encrypted and stored in database

2. **Approval flow:**
   - User clicks inline button in OpenClaw Telegram
   - Opens Mini App with biometric prompt
   - Approval sent only after biometric verified

**Pros:**
- Native Telegram UX
- No additional infrastructure
- Works with existing OpenClaw session

**Cons:**
- Dependent on Telegram's biometric support
- Requires Mini App development

### Option 2: Standalone Bot with 2FA

Create a separate Telegram bot dedicated to approvals:

1. **Separate bot** (`@KingCrabApprovalBot`)
2. **User authenticates** via separate mechanism (e.g., code to linked Telegram)
3. **Approvals happen** in dedicated bot chat

**Pros:**
- Complete control over auth flow
- Independent of OpenClaw's Telegram
- Can implement custom 2FA

**Cons:**
- Additional bot to manage
- Separate auth mechanism
- More infrastructure to maintain

### Recommended Approach

**Start with Option 1** (Telegram Biometric via Mini App):
- Leverage OpenClaw's existing Telegram integration
- Native user experience
- Falls back to Option 2 if biometric not available

---

## Testing Strategy

### 1. Unit Tests

- **Daemon:** Go tests for each component
- **Plugin:** Jest tests for plugin logic
- **Coverage target:** 80%+

### 2. Integration Tests

- Test daemon-database communication
- Test plugin-daemon HTTP communication
- Test notification delivery via OpenClaw

### 3. End-to-End Tests

```bash
# 1. Create request
curl -X POST http://localhost:8080/api/v1/request \
  -d '{"command":"echo test","reason":"test"}'

# 2. Verify in database
psql -U kingcrab -d kingcrab -c "SELECT * FROM elevation_requests;"

# 3. Approve via plugin
# (via OpenClaw skill or direct API)

# 4. Verify execution
# Check audit logs
```

### 4. Security Tests

- Command injection attempts
- SQL injection attempts
- Biometric token manipulation
- Expiration enforcement

---

## Upgrade Path

### From v1 (POC) to v2 (Hybrid)

1. **Stop old daemon:**
   ```bash
   sudo systemctl stop kingcrab
   ```

2. **Backup database:**
   ```bash
   pg_dump -U kingcrab kingcrab > kingcrab_backup.sql
   ```

3. **Run migrations:**
   ```bash
   # Set database credentials and run migration command
   export KINGCRAB_DB_PASSWORD=your_password
   kingcrab --migrate
   ```

4. **Install new daemon:**
   ```bash
   sudo ./installer/install-v2.sh --upgrade
   ```

5. **Migrate plugin:**
   ```bash
   rm -rf ~/.openclaw/extensions/kingcrab
   cp -r plugin/ ~/.openclaw/extensions/kingcrab/
   cd ~/.openclaw/extensions/kingcrab && npm install
   ```

6. **Restart services:**
   ```bash
   sudo systemctl start kingcrab
   ```

---

## Troubleshooting

### Daemon not starting

```bash
# Check logs
sudo journalctl -u kingcrab -n 50

# Check config
sudo kingcrab --check-config

# Check database connection
psql -U kingcrab -d kingcrab -c "SELECT 1;"
```

### Plugin not loading

```bash
# Check OpenClaw logs
tail -f ~/.openclaw/openclaw.log

# Verify plugin structure
ls -la ~/.openclaw/extensions/kingcrab/

# Check config
cat ~/.openclaw/openclaw.json | jq '.extensions.kingcrab'
```

### Notifications not arriving

```bash
# Check OpenClaw webhook
curl http://localhost:3000/api/kingcrab/notify

# Verify daemon notification config
sudo cat /etc/kingcrab/config.json | jq '.notifications'

# Check daemon logs for notification errors
sudo journalctl -u kingcrab -f | grep notify
```

---

## Future Enhancements

1. **Time-based access windows** - Approvals valid only during certain hours
2. **Multi-approval required** - Require N approvers for high-risk commands
3. **Command sandboxing** - Run commands in containers
4. **Approval delegation** - Temporary transfer of approval rights
5. **Metrics dashboard** - Request stats, approval times, common commands