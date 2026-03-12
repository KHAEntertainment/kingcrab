# KingCrab - Privileged Access Management

**Security-First PAM for OpenClaw**

## ⚠️ POC Status

This is the POC version using Telegram polls for approval.
V2 will use a dedicated Telegram bot for proper 2FA.

---

## Installation (Human Admin Only)

**Must be installed by a human with sudo - NOT by the agent.**

### Quick Install

```bash
# SSH to your OpenClaw host as admin user
# Copy binary
sudo cp kingcrab /usr/local/bin/
sudo chmod +x /usr/local/bin/kingcrab

# Create config
sudo mkdir -p /etc/kingcrab
sudo cp config/config.json /etc/kingcrab/

# Create service account
sudo useradd -r -s /bin/false -d /var/lib/kingcrab -M kingcrab

# Create systemd service
sudo tee /etc/systemd/system/kingcrab.service > /dev/null << 'EOF'
[Unit]
Description=KingCrab PAM Daemon
After=network.target

[Service]
Type=simple
User=kingcrab
Group=kingcrab
ExecStart=/usr/local/bin/kingcrab
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Start
sudo systemctl daemon-reload
sudo systemctl enable kingcrab
sudo systemctl start kingcrab
```

### Verify

```bash
curl http://localhost:8080/health
# {"status":"ok","version":"0.1.0"}
```

---

## Usage (POC)

### Agent submits request
```
/kc request "sudo apt install golang-go" --reason "Need Go for building"
```

### User approves via poll
- A Telegram poll appears with "Approve" / "Deny" options
- User votes
- (POC limitation: poll results not yet auto-processed)

### Alternative: Direct API

For POC testing, approve via API:
```bash
# List pending requests
curl http://localhost:8080/requests

# Approve
curl -X POST http://localhost:8080/approve/<request_id>

# Deny
curl -X POST http://localhost:8080/deny/<request_id>
```

---

## Configuration

Edit `/etc/kingcrab/config.json`:

```json
{
  "allowedCommands": [
    "apt install *",
    "apt update",
    "systemctl restart *"
  ],
  "requireReason": true
}
```

---

## Security Model

| Component | Who Can Access |
|-----------|---------------|
| Binary | Anyone (read) / Root (write) |
| Config | Root only |
| Daemon | localhost HTTP only |
| Approve/Deny | Human via poll (POC) / Bot (V2) |

---

## V2 (Planned)

- Dedicated Telegram bot
- Inline keyboard buttons (not polls)
- Proper 2FA via Telegram
- Separate from OpenClaw's Telegram

---

*KingCrab v0.1.0 - POC*
