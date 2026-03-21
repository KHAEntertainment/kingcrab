# KingCrab Installation Guide

## Prerequisites

### Required
- **Go 1.21+** - For building the daemon
- **PostgreSQL 14+** - For request storage and audit logging
- **OpenClaw** - AI assistant framework (https://github.com/openclaw/openclaw)
- **systemd** - For running the daemon as a service
- **Root/sudo access** - For installation and privileged operations

### Optional
- **Telegram bot** - For approval notifications (uses OpenClaw's Telegram if available)
- **Node.js 20+** - For plugin development

## Quick Start

### 1. Clone Repository

```bash
git clone https://github.com/KHAEntertainment/kingcrab.git
cd kingcrab
```

### 2. Install Dependencies

```bash
# On Debian/Ubuntu
sudo apt update
sudo apt install golang-postgresql postgresql systemd

# On Fedora
sudo dnf install golang postgresql-server systemd

# On macOS (development only)
brew install go postgresql
```

### 3. Setup Database

```bash
# Create database user
sudo -u postgres createuser kingcrab

# Create database
sudo -u postgres createdb -O kingcrab kingcrab

# Set password for kingcrab user
sudo -u postgres psql -c "ALTER USER kingcrab PASSWORD 'your_password';"
```

### 4. Configure Environment

```bash
# Set database password
export KINGCRAB_DB_PASSWORD="your_password"

# Optional: Set OpenClaw webhook URL
export KINGCRAB_OPENCLAW_WEBHOOK="http://localhost:3000/api/kingcrab/notify"
```

### 5. Build Daemon

```bash
go build -o kingcrab ./cmd/kingcrab
```

### 6. Install Daemon

```bash
# Run installer
sudo ./installer/install-v2.sh

# This will:
# - Copy binary to /usr/local/bin/kingcrab
# - Create config directory /etc/kingcrab/
# - Create systemd service
# - Run database migrations (optional - only if requested during setup)
# Note: On reruns, the installer backs up /etc/kingcrab/config.json and overwrites it.
# Note: Database migrations are optional; DB setup only runs when requested and will skip SQL execution if the migration file is missing.
```

### 7. Configure Daemon

Edit `/etc/kingcrab/config.json`:

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
  "logLevel": "info"
}
```

### 8. Start Daemon

```bash
sudo systemctl start kingcrab
sudo systemctl enable kingcrab  # Auto-start on boot

# Verify
sudo systemctl status kingcrab
curl --unix-socket /var/run/kingcrab.sock http://localhost/api/v1/health
```

## Plugin Installation

### 1. Install Plugin

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
```

### 2. Configure Plugin

Add to `~/.openclaw/openclaw.json`:

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

### 3. Restart OpenClaw

```bash
systemctl --user restart openclaw
```

## Verification

### Test Daemon Health

```bash
curl --unix-socket /var/run/kingcrab.sock http://localhost/api/v1/health
```

Expected response:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "time": "2026-03-20T12:00:00Z"
}
```

### Test Request Creation

```bash
curl -X POST http://localhost:8080/api/v1/request \
  -H "Content-Type: application/json" \
  -d '{
    "command": "echo test",
    "reason": "testing installation"
  }'
```

### Test Plugin Tools

```bash
# In OpenClaw conversation
/kc request "echo hello" --reason "testing"
/kc list
/kc status
```

## Troubleshooting

### Daemon won't start

```bash
# Check logs
sudo journalctl -u kingcrab -n 50

# Check config
sudo kingcrab --check-config 2>/dev/null || echo "Config error"

# Check database connection
psql -U kingcrab -d kingcrab -c "SELECT 1;"
```

### Plugin not loading

```bash
# Check OpenClaw logs
journalctl --user -u openclaw -n 50

# Verify plugin files
ls -la ~/.openclaw/extensions/kingcrab/

# Check OpenClaw config
cat ~/.openclaw/openclaw.json | jq '.plugins.entries.kingcrab'
```

### Database connection failed

```bash
# Verify PostgreSQL is running
sudo systemctl status postgresql

# Check database exists
sudo -u postgres psql -l | grep kingcrab

# Test connection
psql -U kingcrab -d kingcrab -c "SELECT version();"
```

### Permission denied on socket

```bash
# Check socket permissions
ls -la /var/run/kingcrab.sock

# Add user to kingcrab group
sudo usermod -a -G kingcrab $USER

# Re-login required
```

## Uninstallation

### Remove Daemon

```bash
sudo systemctl stop kingcrab
sudo systemctl disable kingcrab
sudo ./installer/uninstall.sh  # if available
sudo rm -f /usr/local/bin/kingcrab
sudo rm -rf /etc/kingcrab
sudo rm -f /etc/systemd/system/kingcrab.service
sudo systemctl daemon-reload
```

### Remove Plugin

```bash
rm -rf ~/.openclaw/extensions/kingcrab
systemctl --user restart openclaw
```

### Remove Database

```bash
sudo -u postgres dropdb kingcrab
sudo -u postgres dropuser kingcrab
```

## Production Considerations

1. **Use a strong database password** - Store in `/etc/kingcrab/db.password` with mode 0600
2. **Enable SSL/TLS for database** - Set `sslmode: "require"` in config
3. **Configure log rotation** - Add to `/etc/logrotate.d/kingcrab`
4. **Set up monitoring** - Monitor daemon health and database connection
5. **Regular backups** - Backup database and `/etc/kingcrab/` directory
6. **Firewall rules** - Restrict access to daemon port if using TCP

## Upgrade Procedure

### From v1.x to v2.0

1. **Stop daemon:**
   ```bash
   sudo systemctl stop kingcrab
   ```

2. **Backup database:**
   ```bash
   pg_dump -U kingcrab kingcrab > kingcrab_backup.sql
   ```

3. **Run migrations:**
   ```bash
   sudo -u kingcrab /usr/local/bin/kingcrab --migrate
   ```

4. **Update config:**
   - Old config format is incompatible
   - Copy `/etc/kingcrab/config.json.example` and customize

5. **Install new binary:**
   ```bash
   go build -o kingcrab ./cmd/kingcrab
   sudo cp kingcrab /usr/local/bin/
   ```

6. **Update plugin:**
   ```bash
   rm -rf ~/.openclaw/extensions/kingcrab
   cp -r plugin/ ~/.openclaw/extensions/kingcrab/
   cd ~/.openclaw/extensions/kingcrab && npm install
   ```

7. **Start services:**
   ```bash
   sudo systemctl start kingcrab
   systemctl --user restart openclaw
   ```