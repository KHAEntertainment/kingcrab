# KingCrab 🦀

**Privileged Access Management (PAM) for OpenClaw**

KingCrab provides secure, chat-based approval workflows for elevated commands. Instead of giving agents sudo access, they submit requests that humans approve—providing true separation of duties and audit trails.

## Why KingCrab?

- **Security**: Agents never get sudo. They request, humans approve.
- **Audit Trail**: Every command logged with who approved what and when.
- **Biometric 2FA**: Approvals use Telegram's native biometric authentication.
- **Database-Backed**: PostgreSQL storage for persistence and audit logging.
- **Defense in Depth**: Multiple layers—command allowlists, reason required, human-in-the-loop.

## Architecture

KingCrab v2 uses a hybrid architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                      AGENT WORKSPACE                             │
│  ┌────────────┐        ┌────────────────────────────────────┐  │
│  │   Agent    │───────▶│         OpenClaw Core              │  │
│  │  (Claude)  │        │  - Conversation Manager           │  │
│  └────────────┘        │  - Plugin System                  │  │
│                        │  - Telegram Channel               │  │
│                        └──────────────┬─────────────────────┘  │
│                                       │                         │
│                        ┌──────────────▼─────────────────────┐  │
│                        │      KingCrab Plugin (TS)         │  │
│                        │  - kingcrab_request               │  │
│                        │  - kingcrab_list                  │  │
│                        │  - kingcrab_approve               │  │
│                        └──────────────┬─────────────────────┘  │
└───────────────────────────────────────┼─────────────────────────┘
                                        │ HTTP
└───────────────────────────────────────┼─────────────────────────┘
                                        │ Unix Socket / HTTP
                                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                      SYSTEM LEVEL (Privileged)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  KingCrab Daemon (Go)                   │   │
│  │                  /usr/local/bin/kingcrab                │   │
│  │                  systemd: kingcrab.service             │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌─────────────────┐   │   │
│  │  │ HTTP API   │  │   Request  │  │   Command       │   │   │
│  │  │            │  │   Store    │  │   Executor      │   │   │
│  │  │ /api/v1/*  │  │ (PostgreSQL) │ │                 │   │   │
│  │  │ /api/pam/* │  │            │  │ - Validate      │   │   │
│  │  └────────────┘  └────────────┘  │ - Execute       │   │   │
│  │                                  │ - Log Result    │   │   │
│  │  ┌────────────────────────────┐  └─────────────────┘   │   │
│  │  │   Notification Service    │                          │   │
│  │  │   (via OpenClaw Webhook)  │                          │   │
│  │  └────────────────────────────┘                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              PostgreSQL Database                         │   │
│  │  - elevation_requests (audit trail)                     │   │
│  │  - authorized_users                                     │   │
│  │  - enrolled_devices (biometric tokens)                  │   │
│  │  - approval_audit                                       │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      USER'S PHONE                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Telegram App                          │   │
│  │  🔐 KingCrab Request #abc12345                          │   │
│  │     Command: apt install golang-go                      │   │
│  │     Reason: Need Go for building CLI                    │   │
│  │                                                         │   │
│  │     [✅ Approve]  [🚫 Deny]                             │   │
│  │                                                         │   │
│  │  (Biometric auth required for approval)                 │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Components

| Component | Language | Purpose |
|-----------|----------|---------|
| Daemon | Go | Runs as root, executes approved commands |
| Plugin | TypeScript | OpenClaw plugin with tool registration |
| Database | PostgreSQL | Request storage and audit logging |
| PAM Module | Go | Biometric authentication via Telegram |

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- OpenClaw installed
- Root/sudo access

### Install Daemon

```bash
# Clone repository
git clone https://github.com/KHAEntertainment/kingcrab.git
cd kingcrab

# Build daemon
go build -o kingcrab ./cmd/kingcrab

# Run installer
sudo ./installer/install-v2.sh

# Configure database
export KINGCRAB_DB_PASSWORD="your_password"
sudo -u postgres createdb -O kingcrab kingcrab

# Start daemon
sudo systemctl start kingcrab
sudo systemctl enable kingcrab
```

### Install Plugin

```bash
# Copy plugin to OpenClaw extensions
cd plugin
mkdir -p ~/.openclaw/extensions/kingcrab
cp -r . ~/.openclaw/extensions/kingcrab/

# Install dependencies
cd ~/.openclaw/extensions/kingcrab
npm install
npm run build

# Restart OpenClaw
systemctl --user restart openclaw
```

## Usage

### Request Elevated Access

```
/kc request "apt install golang-go" --reason "Need Go for building"
```

### List Pending Requests

```
/kc list
```

### Approve via Telegram

1. Receive notification in Telegram
2. Click "Approve" button
3. Authenticate with biometric (FaceID/Fingerprint)
4. Command executes

## Documentation

- **[Installation Guide](docs/INSTALL.md)** - Detailed installation instructions
- **[Configuration Guide](docs/CONFIG.md)** - All configuration options
- **[Hybrid Architecture](memory/HYBRID_ARCHITECTURE.md)** - Architecture details

## Security Model

| Layer | Protection |
|-------|------------|
| **Daemon Isolation** | Runs as root via systemd, separate from agent |
| **Command Allowlist** | Only pre-approved commands can execute |
| **Reason Required** | Agent must justify every request |
| **Biometric 2FA** | Telegram biometric auth for approvals |
| **Audit Trail** | Every request logged to PostgreSQL |
| **Request Expiration** | Requests expire after 5 minutes (default) |

## Configuration

### Daemon Config: `/etc/kingcrab/config.json`

```json
{
  "version": "1.0.0",
  "listen": {
    "type": "unix",
    "path": "/var/run/kingcrab.sock"
  },
  "database": {
    "host": "localhost",
    "port": 5432,
    "user": "kingcrab",
    "passwordEnv": "KINGCRAB_DB_PASSWORD",
    "dbname": "kingcrab",
    "sslmode": "disable"
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
  "openclaw": {
    "webhookUrl": "http://localhost:3000/api/kingcrab/notify",
    "enabled": true
  }
}
```

### Plugin Config: `~/.openclaw/openclaw.json`

```json
{
  "plugins": {
    "entries": {
      "kingcrab": {
        "enabled": true,
        "config": {
          "daemonUrl": "http://localhost:8080",
          "timeout": 30000
        }
      }
    }
  }
}
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/health` | Health check |
| POST | `/api/v1/request` | Create elevation request |
| GET | `/api/v1/requests` | List all requests |
| GET | `/api/v1/request/:id` | Get request status |
| POST | `/api/v1/request/:id/approve` | Approve request |
| POST | `/api/v1/request/:id/deny` | Deny request |
| POST | `/api/pam/enroll` | Enroll biometric device |
| POST | `/api/pam/approve` | Approve with biometric |

## Development

### Project Structure

```
kingcrab/
├── cmd/kingcrab/       # Daemon entrypoint
├── internal/
│   ├── api/           # v1 HTTP API handlers
│   ├── config/        # Configuration loading
│   ├── daemon/        # Server and executor
│   ├── db/            # Database connection
│   ├── executor/      # Command execution
│   ├── logger/        # Structured logging
│   ├── notifications/ # OpenClaw webhook integration
│   └── pam/           # Biometric authentication module
├── plugin/            # TypeScript plugin for OpenClaw
├── skill/             # Python skill (legacy)
├── docs/              # Documentation
└── installer/         # Installation scripts
```

### Building

```bash
# Build daemon
go build -o kingcrab ./cmd/kingcrab

# Build plugin
cd plugin && npm run build
```

### Testing

```bash
# Test daemon health
curl http://localhost:8080/api/v1/health

# Create request
curl -X POST http://localhost:8080/api/v1/request \
  -H "Content-Type: application/json" \
  -d '{"command":"echo test","reason":"testing"}'

# List requests
curl http://localhost:8080/api/v1/requests
```

## Roadmap

- [x] Daemon with v1 REST API
- [x] PostgreSQL database backend
- [x] TypeScript plugin for OpenClaw
- [x] Biometric authentication (Telegram)
- [ ] Web UI for approval management
- [ ] Time-based access windows
- [ ] Multi-approval workflows
- [ ] Metrics dashboard

## License

MIT

## Author

KHAEntertainment

## Related Projects

- [OpenClaw](https://github.com/openclaw/openclaw) - Home automation AI assistant
- [ClawVault](https://github.com/KHAEntertainment/clawvault) - Secret management
- [OCBS](https://github.com/KHAEntertainment/ocbs) - Backup system