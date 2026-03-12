# KingCrab 🦀

**Privileged Access Management (PAM) for OpenClaw**

KingCrab provides secure, chat-based approval workflows for elevated commands. Instead of giving agents sudo access, they submit requests that humans approve via a separate Telegram bot—providing true 2FA.

## Why KingCrab?

- **Security**: Agents never get sudo. They request, humans approve.
- **Audit Trail**: Every command logged with who approved what and when.
- **2FA**: Approvals happen via dedicated Telegram bot, separate from the agent's conversation.
- **Defense in Depth**: Multiple layers—command allowlists, reason required, human-in-the-loop.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Agent     │────▶│   Skill     │────▶│   Daemon    │
│ (OpenClaw)  │     │  (Python)   │     │   (Go)      │
└─────────────┘     └─────────────┘     └─────────────┘
                                                 │
                                                 ▼
                                          ┌─────────────┐
                                          │  Telegram   │
                                          │    Bot      │ ← Separate from OpenClaw!
                                          │ (2FA)       │
                                          └─────────────┘
                                                 │
                                                 ▼
                                          ┌─────────────┐
                                          │   Human     │ ← Clicks Approve/Deny
                                          └─────────────┘
```

## Components

| Component | Language | Purpose |
|-----------|----------|---------|
| Daemon | Go | Runs as root, executes approved commands |
| Skill | Python | OpenClaw skill for agents to submit requests |
| Telegram Bot | Go | Handles inline button approvals (2FA) |

## Installation

### Prerequisites

- Go 1.21+ (to build the daemon)
- Root/sudo access (for installation)
- Telegram bot token (for approval bot)

### Quick Install

```bash
# Clone
git clone https://github.com/KHAEntertainment/kingcrab.git
cd kingcrab

# Build
go build -o kingcrab ./cmd/kingcrab

# Install (as root)
sudo ./installer/install.sh

# Configure Telegram bot (edit /etc/kingcrab/config.json)
sudo nano /etc/kingcrab/config.json
```

### Configuration

Edit `/etc/kingcrab/config.json`:

```json
{
  "version": "0.1.0",
  "allowedCommands": [
    "apt install *",
    "apt update",
    "systemctl restart *",
    "systemctl start *",
    "systemctl stop *"
  ],
  "requireReason": true,
  "autoApproveTimeout": 0,
  "telegram": {
    "botToken": "YOUR_BOT_TOKEN",
    "allowedUsers": ["YOUR_TELEGRAM_USER_ID"]
  }
}
```

### Start

```bash
sudo systemctl start kingcrab
sudo systemctl enable kingcrab  # auto-start on boot

# Verify
curl http://localhost:8080/health
```

## Usage

### Agent Workflow

1. Agent needs elevated access:
   ```
   /kc request "sudo apt install golang-go" --reason "Need Go for building CLI"
   ```

2. Request created with status `pending`

3. KingCrab Telegram bot sends message with inline buttons:
   - ✅ Approve
   - 🚫 Deny

4. User clicks button (2FA—Telegram auth proves human)

5. Command executes as root (if approved)

6. Result returned to agent

### Human Workflow

- Start a chat with your KingCrab bot (@your_bot)
- You'll receive approval requests with buttons
- Click Approve or Deny
- Done!

## Security Model

| Layer | Protection |
|-------|------------|
| **Service Account** | Daemon runs as unprivileged `kingcrab` user |
| **Command Allowlist** | Only pre-approved commands can execute |
| **Reason Required** | Agent must justify every request |
| **No Self-Approval** | Approve/deny disabled in skill—Telegram bot only |
| **Audit Log** | Every request logged with timestamp, command, approver |

### Threat Model

| Threat | Mitigation |
|--------|------------|
| Agent tries arbitrary sudo | Allowlist blocks unapproved commands |
| Agent tries to approve own request | Disabled in skill—bot only |
| Someone else's Telegram | `allowedUsers` config restricts who can approve |
| Daemon compromised | Runs as limited `kingcrab` user, not root |

## Development

### Project Structure

```
kingcrab/
├── cmd/kingcrab/     # Daemon entrypoint
├── internal/
│   ├── config/       # Configuration loading
│   ├── daemon/       # HTTP server + request queue
│   ├── executor/     # Command execution
│   ├── logger/       # Structured logging
│   └── telegram/     # Telegram bot (future)
├── skill/            # OpenClaw Python skill
├── plugin/          # OpenClaw plugin (legacy)
├── installer/       # Installation scripts
├── config/          # Default configuration
└── bin/             # CLI tools
```

### Building

```bash
# Build daemon
go build -o kingcrab ./cmd/kingcrab

# Build with version info
go build -ldflags="-s -w" -o kingcrab ./cmd/kingcrab
```

### Testing

```bash
# Run daemon
./kingcrab

# Test API
curl -X POST http://localhost:8080/request \
  -H "Content-Type: application/json" \
  -d '{"command":"echo test","reason":"testing"}'

# Approve
curl -X POST http://localhost:8080/approve/<request_id>
```

## Roadmap

- [x] Daemon with request queue
- [x] Command allowlist
- [x] Basic audit logging
- [x] Python skill for OpenClaw
- [ ] Telegram bot for 2FA approvals (in progress)
- [ ] Web UI for approval
- [ ] Time-based access windows
- [ ] Two-factor requiring second approver

## License

MIT

## Author

KHAEntertainment

## Related

- [OpenClaw](https://github.com/openclaw/openclaw) - Home automation AI assistant
- [ClawVault](https://github.com/KHAEntertainment/clawvault) - Secret management
- [OCBS](https://github.com/KHAEntertainment/ocbs) - Backup system
