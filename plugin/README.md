# KingCrab Plugin for OpenClaw

This is the OpenClaw plugin for KingCrab, providing a PAM (Privileged Access Management) system with chat-based sudo approval workflows.

## Installation

```bash
# From the plugin directory
cd ~/projects/kingcrab/plugin
npm install
npm run build
```

## Configuration

The plugin accepts the following configuration options (see `skill.json`):

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `daemonUrl` | string | `http://localhost:8080` | URL of the KingCrab daemon HTTP API |
| `allowedCommands` | string[] | See below | Allowed command patterns (supports wildcards) |

### Default Allowed Commands

- `apt install *` - Install packages
- `apt update` - Update package lists
- `systemctl restart *` - Restart services
- `systemctl start *` - Start services
- `systemctl stop *` - Stop services
- `systemctl status *` - Check service status

## Usage

### Agent API Endpoints

The plugin exposes the following HTTP endpoints:

- `POST /kingcrab/request` - Create a new privileged command request
- `GET /kingcrab/requests` - List all requests
- `POST /kingcrab/approve/:id` - Approve a request
- `POST /kingcrab/deny/:id` - Deny a request

### Telegram Commands

- `/kc request <command> [--reason <reason>]` - Create a new request
- `/kc list` - List all pending requests

### Inline Buttons

When a request is created via Telegram, inline buttons are provided for quick approval/deny.

## UI

A web-based approval UI is available at `/kingcrab/ui.html` (served by the plugin).

The UI shows:
- Statistics (pending, approved, denied requests)
- List of all requests with details
- Approve/Deny buttons for pending requests
- Auto-refresh every 10 seconds

## Example Request

```json
POST /kingcrab/request
{
  "command": "apt install golang-go",
  "reason": "Required for notes-cli development"
}
```

Response:
```json
{
  "success": true,
  "request": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "command": "apt install golang-go",
    "reason": "Required for notes-cli development",
    "status": "pending",
    "timestamp": "2026-03-06T04:00:00Z",
    "result": null
  },
  "message": "Request created successfully. Use inline buttons to approve/deny."
}
```

## Architecture

```
┌─────────────┐     /kingcrab/*     ┌──────────────┐     HTTP       ┌─────────────┐
│   Agent     │ ─────────────────▶  │   Plugin     │ ──────────────▶ │   Daemon    │
└─────────────┘                     └──────────────┘                └─────────────┘
                                          │
                                          │ Telegram
                                          ▼
                                    ┌──────────────┐
                                    │   Telegram   │
                                    │   Bot/Chat   │
                                    └──────────────┘
```

## License

MIT
