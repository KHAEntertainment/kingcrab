---
name: kingcrab
version: 1.0.0
description: Privileged Access Management (PAM) for OpenClaw - Chat-based sudo approval workflows with biometric authentication
author: KHAEntertainment
tags: [pam, sudo, security, approval]
---

# KingCrab - Privileged Access Management

**Hybrid PAM System for OpenClaw**

KingCrab provides secure, chat-based approval workflows for elevated commands. Instead of giving agents sudo access, they submit requests that humans approve via Telegram with biometric authentication.

## Architecture

KingCrab uses a **hybrid architecture**:

1. **Go Daemon** - Systemd service running as root for privilege escalation
2. **OpenClaw Plugin** - TypeScript plugin that registers tools for agents
3. **PostgreSQL** - Database-backed request store with complete audit trail
4. **Telegram Integration** - Native OpenClaw Telegram for notifications and approvals

```
Agent → Plugin Tool → HTTP → Daemon (Go) → Database
                                    ↓
                              OpenClaw Webhook
                                    ↓
                              Telegram Notification
                                    ↓
                         Biometric Approval → Execute
```

## Prerequisites

1. **OpenClaw** installed and configured
2. **PostgreSQL** running with `kingcrab` database
3. **KingCrab Daemon** installed and running (see Installation below)

## Installation

### 1. Install Daemon

```bash
# Clone repository
git clone https://github.com/KHAEntertainment/kingcrab.git
cd kingcrab

# Build daemon
go build -o kingcrab ./cmd/kingcrab

# Run installer (requires sudo)
sudo ./installer/install.sh

# Set database password
sudo systemctl edit kingcrab
# Add: [Service]
#      Environment="KINGCRAB_DB_PASSWORD=your_password"

# Start service
sudo systemctl start kingcrab
sudo systemctl enable kingcrab

# Verify
curl http://localhost:8080/api/v1/health
```

### 2. Configure Database

```bash
# Create database and user
sudo -u postgres createuser kingcrab
sudo -u postgres createdb -O kingcrab kingcrab

# Set password
sudo -u postgres psql -c "ALTER USER kingcrab PASSWORD 'your_password';"

# Run migrations (daemon does this automatically on first start)
# Or manually:
psql -U kingcrab -d kingcrab -f /usr/local/share/kingcrab/migrations/001_pam_schema.sql
```

### 3. Install Plugin

```bash
# Copy to OpenClaw extensions
mkdir -p ~/.openclaw/extensions/kingcrab
cp -r plugin/* ~/.openclaw/extensions/kingcrab/

# Install dependencies
cd ~/.openclaw/extensions/kingcrab
npm install

# Build
npm run build
```

### 4. Configure OpenClaw

Edit `~/.openclaw/openclaw.json`:

```json
{
  "extensions": {
    "kingcrab": {
      "enabled": true,
      "daemonUrl": "http://localhost:8080"
    }
  }
}
```

### 5. Enroll Biometric Device

```bash
# Via OpenClaw skill
/kc enroll

# Follow prompts in Telegram to authorize device
```

## Usage

### Agent Tools

The plugin registers these tools for agents:

#### `kingcrab_request`
Create a privileged command request requiring approval.

**Input:**
- `command` (string, required): The command to execute
- `reason` (string, optional): Explanation for why this command is needed

**Example:**
```json
{
  "command": "apt install golang-go",
  "reason": "Need Go for building CLI tool"
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
  },
  "message": "Request created: abc123. Waiting for approval via Telegram..."
}
```

#### `kingcrab_list`
List all KingCrab elevation requests, optionally filtered by status.

**Input:**
- `status` (string, optional): Filter by status (pending, approved, denied, completed, failed, expired)

**Example:**
```json
{
  "status": "pending"
}
```

#### `kingcrab_get`
Get details of a specific KingCrab request by ID.

**Input:**
- `id` (string, required): The request ID

### Approval Flow

1. **Agent creates request** via `kingcrab_request` tool
2. **Notification sent** to user's Telegram via OpenClaw
3. **User approves** by clicking inline button in Telegram
4. **Biometric auth** required (FaceID/Fingerprint)
5. **Command executes** as root
6. **Result returned** to agent

## Configuration

### Daemon Config: `/etc/kingcrab/config.json`

```json
{
  "version": "1.0.0",
  "listen": {
    "type": "tcp",
    "port": 8080
  },
  "allowedCommands": [
    "apt install *",
    "apt update",
    "systemctl restart *",
    "systemctl start *",
    "systemctl stop *",
    "systemctl status *"
  ],
  "requireReason": true,
  "openclaw": {
    "webhookUrl": "http://localhost:3000/api/kingcrab/notify",
    "enabled": true
  }
}
```

### Plugin Config: `~/.openclaw/openclaw.json`

```json
{
  "extensions": {
    "kingcrab": {
      "enabled": true,
      "daemonUrl": "http://localhost:8080",
      "timeout": 10000
    }
  }
}
```

## Security Model

| Layer | Protection |
|-------|------------|
| **Daemon Isolation** | Runs as root via systemd, separate from agent |
| **Database Persistence** | All requests logged with audit trail |
| **Command Allowlist** | Only pre-approved commands can execute |
| **Biometric 2FA** | Telegram biometric auth required for approvals |
| **Request Expiration** | Pending requests expire after 5 minutes |
| **Privilege Separation** | Plugin never has root access, only daemon does |

## Troubleshooting

### Daemon not starting

```bash
# Check logs
sudo journalctl -u kingcrab -n 50

# Check database connection
psql -U kingcrab -d kingcrab -c "SELECT 1;"

# Verify config
sudo kingcrab --check-config
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
# Check daemon logs for notification errors
sudo journalctl -u kingcrab -f | grep notify

# Verify webhook URL
sudo cat /etc/kingcrab/config.json | jq '.openclaw.webhookUrl'
```

## API Reference

### Daemon HTTP API

#### POST /api/v1/request
Create a new elevation request.

#### GET /api/v1/requests
List all requests (with optional filters).

#### GET /api/v1/request/:id
Get details of a specific request.

#### POST /api/v1/request/:id/approve
Approve a request with biometric authentication.

#### POST /api/v1/request/:id/deny
Deny a request.

#### GET /api/v1/health
Health check endpoint.

## License

MIT

## Author

KHAEntertainment

## Related

- [OpenClaw](https://github.com/openclaw/openclaw) - Home automation AI assistant
- [ClawVault](https://github.com/KHAEntertainment/clawvault) - Secret management
