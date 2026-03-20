# KingCrab Configuration Guide

## Daemon Configuration

The daemon configuration is stored in `/etc/kingcrab/config.json`.

### Configuration Schema

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
    "webhookUrl": "",
    "enabled": false
  }
}
```

### Configuration Options

#### `version` (string, required)
Configuration format version. Currently `"1.0.0"`.

#### `listen` (object, required)
How the daemon listens for connections.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `"unix"` | Listen type: `"unix"` or `"tcp"` |
| `path` | string | `"/var/run/kingcrab.sock"` | Unix socket path (when type=`unix`) |
| `port` | int | `8080` | TCP port (when type=`tcp`) |

**Recommendation:** Use Unix socket for local communication, TCP for remote.

#### `database` (object, required)
PostgreSQL database connection.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `"localhost"` | Database host |
| `port` | int | `5432` | Database port |
| `user` | string | `"kingcrab"` | Database user |
| `passwordEnv` | string | - | Environment variable containing password |
| `dbname` | string | `"kingcrab"` | Database name |
| `sslmode` | string | `"disable"` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |

**Security:** Set `sslmode: "require"` for production.

#### `allowedCommands` (array of strings, required)
Commands that agents are allowed to request.

Supports wildcards (`*`) for flexible matching:
- `"apt install *"` - Allows `apt install <any-package>`
- `"apt update"` - Exact match only
- `"systemctl restart *"` - Allows restarting any service

**Examples:**
```json
{
  "allowedCommands": [
    "apt install *",
    "apt update",
    "apt upgrade",
    "systemctl restart *",
    "systemctl start *",
    "systemctl stop *",
    "systemctl status *",
    "docker restart *",
    "docker-compose -f /opt/* restart"
  ]
}
```

**Warning:** Avoid overly permissive patterns like `"*"` or `"sudo *"`.

#### `requireReason` (boolean)
Whether agents must provide a reason for their request.

**Recommendation:** Set to `true` for audit purposes.

#### `logLevel` (string)
Logging verbosity: `"debug"`, `"info"`, `"warn"`, `"error"`.

**Recommendation:** Use `"info"` for production, `"debug"` for troubleshooting.

#### `telegram` (object)
Telegram bot integration for approvals.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `botToken` | string | `""` | Telegram bot token |
| `allowedUsers` | array | `[]` | Telegram user IDs allowed to approve |
| `webhookUrl` | string | `""` | Webhook URL for notifications |

**Note:** Leave empty if using OpenClaw's Telegram integration.

#### `pam` (object)
Biometric authentication settings for PAM (Privileged Access Management).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `use_clawvault` | string | `"auto"` | ClawVault usage: `auto`, `true`, `false` |
| `clawvault` | object | - | ClawVault connection settings |
| `fallback` | object | - | Local encrypted storage settings |

**ClawVault settings:**
- `socket`: Unix socket path to ClawVault
- `host`: Network endpoint for ClawVault
- `token_prefix`: Key prefix for token storage
- `timeout_seconds`: Connection timeout

**Fallback settings:**
- `encryption_key_env`: Environment variable with encryption key
- `storage_path`: Path to store encrypted tokens (defaults to `~/.config/kingcrab/tokens`)
- `ttl_minutes`: Token time-to-live in minutes
- `authorized_users`: List of pre-authorized users

#### `openclaw` (object)
OpenClaw integration settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `webhookUrl` | string | `""` | OpenClaw webhook URL for notifications |
| `enabled` | boolean | `false` | Enable OpenClaw integration |

## Plugin Configuration

The plugin is configured in `~/.openclaw/openclaw.json`.

### Plugin Configuration Schema

```json
{
  "plugins": {
    "entries": {
      "kingcrab": {
        "enabled": true,
        "config": {
          "daemonUrl": "http://localhost:8080",
          "daemonToken": "",
          "timeout": 30000,
          "maxRetries": 3,
          "allowedCommands": [],
          "requireReason": true,
          "notifications": {
            "enabled": true,
            "defaultUserId": null
          },
          "webhookUrl": "",
          "webhookSecret": ""
        }
      }
    }
  }
}
```

### Plugin Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `daemonUrl` | string | `"http://localhost:8080"` | KingCrab daemon URL |
| `daemonToken` | string | `""` | Shared secret for daemon authentication |
| `timeout` | integer | `30000` | HTTP request timeout (ms) |
| `maxRetries` | integer | `3` | Number of retries on connection failure |
| `allowedCommands` | array | (daemon config) | Override daemon allowlist |
| `requireReason` | boolean | `true` | Require reason for requests |
| `notifications` | object | - | Notification settings |
| `webhookUrl` | string | `""` | Webhook URL for daemon callbacks |
| `webhookSecret` | string | `""` | Secret for webhook validation |

**Note:** The plugin's `allowedCommands` overrides the daemon's config if set.

## Environment Variables

### Daemon Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `KINGCRAB_CONFIG` | No | Path to config file (default: `/etc/kingcrab/config.json`) |
| `KINGCRAB_DB_PASSWORD` | Yes* | Database password (unless set in config) |
| `KINGCRAB_OPENCLAW_WEBHOOK` | No | OpenClaw webhook URL |
| `KINGCRAB_LOG_LEVEL` | No | Override log level |
| `KINGCRAB_PORT` | No | Override listen port (TCP mode) |

*Required if `database.passwordEnv` is set to `KINGCRAB_DB_PASSWORD`.

### Plugin Environment Variables

No environment variables are currently used by the plugin.

## Security Best Practices

### 1. Database Security

```json
{
  "database": {
    "sslmode": "require",
    "passwordEnv": "KINGCRAB_DB_PASSWORD"
  }
}
```

Set password in `/etc/kingcrab/db.password` with mode `0600`:
```bash
echo "your_secure_password" > /etc/kingcrab/db.password
chmod 0600 /etc/kingcrab/db.password
chown kingcrab:kingcrab /etc/kingcrab/db.password
```

### 2. Command Allowlist

Be specific with allowed commands:
- ✅ `"apt install golang-go"` - Specific package
- ✅ `"systemctl restart nginx"` - Specific service
- ⚠️ `"apt install *"` - Any package (use carefully)
- ❌ `"*"` - Everything (never use)

### 3. Unix Socket Permissions

```bash
# Socket directory
chmod 0750 /var/run/kingcrab
chown kingcrab:kingcrab /var/run/kingcrab

# Socket itself
chmod 0660 /var/run/kingcrab/kingcrab.sock
```

Add users who need access to the `kingcrab` group:
```bash
sudo usermod -a -G kingcrab $USER
```

### 4. Biometric Token Storage

For production, use ClawVault for secure token storage:
```json
{
  "pam": {
    "use_clawvault": "true",
    "clawvault": {
      "socket": "/var/run/clawvault.sock"
    }
  }
}
```

### 5. Audit Logging

Ensure logs are captured and rotated:
```bash
# /etc/logrotate.d/kingcrab
/var/log/kingcrab/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 kingcrab kingcrab
}
```

## Troubleshooting Configuration

### Test Config Syntax

```bash
# Use jq to validate JSON
jq '.' /etc/kingcrab/config.json

# Or use Python
python3 -m json.tool /etc/kingcrab/config.json
```

### View Active Configuration

```bash
# Daemon logs config on startup
sudo journalctl -u kingcrab | grep "Config loaded"

# Or query via API (if enabled)
curl http://localhost:8080/api/v1/config
```

### Common Issues

**Issue:** "database connection failed"
**Fix:** Verify `KINGCRAB_DB_PASSWORD` is set and database is accessible.

**Issue:** "command not in allowlist"
**Fix:** Add command pattern to `allowedCommands` in config.

**Issue:** "permission denied on socket"
**Fix:** Add user to `kingcrab` group and re-login.

**Issue:** "ClawVault not available"
**Fix:** Either install ClawVault or set `use_clawvault: "false"`.

## Example Configurations

### Development Setup

```json
{
  "listen": {
    "type": "tcp",
    "port": 8080
  },
  "database": {
    "host": "localhost",
    "port": 5432,
    "user": "kingcrab",
    "passwordEnv": "KINGCRAB_DB_PASSWORD",
    "dbname": "kingcrab_dev",
    "sslmode": "disable"
  },
  "allowedCommands": ["*"],
  "requireReason": false,
  "logLevel": "debug",
  "openclaw": {
    "enabled": false
  }
}
```

### Production Setup

```json
{
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
    "sslmode": "require"
  },
  "allowedCommands": [
    "apt install golang-go nodejs npm python3 python3-pip",
    "apt update",
    "systemctl restart nginx",
    "systemctl restart postgresql",
    "systemctl restart kingcrab"
  ],
  "requireReason": true,
  "logLevel": "info",
  "pam": {
    "use_clawvault": "true",
    "clawvault": {
      "socket": "/var/run/clawvault.sock"
    }
  },
  "openclaw": {
    "webhookUrl": "http://localhost:3000/api/kingcrab/notify",
    "enabled": true
  }
}
```

### High-Security Setup

```json
{
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
    "sslmode": "verify-full"
  },
  "allowedCommands": [
    "apt install golang-go",
    "apt install nodejs",
    "systemctl restart nginx"
  ],
  "requireReason": true,
  "logLevel": "info",
  "pam": {
    "use_clawvault": "true",
    "fallback": {
      "ttl_minutes": 2
    }
  },
  "openclaw": {
    "enabled": true
  }
}
```
