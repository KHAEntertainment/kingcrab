#!/bin/bash
set -e

VERSION="0.1.0"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/kingcrab"
DATA_DIR="/var/lib/kingcrab"
LOG_DIR="/var/log/kingcrab"
SERVICE_USER="kingcrab"
SERVICE_NAME="kingcrab"

echo "🦀 KingCrab v${VERSION} Installer"
echo "=============================="

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "❌ Please run as root: sudo $0"
    exit 1
fi

# Find the binary (assume we're in the project directory)
if [ -f "./kingcrab" ]; then
    BINARY="./kingcrab"
elif [ -f "/usr/local/bin/kingcrab" ]; then
    BINARY="/usr/local/bin/kingcrab"
else
    echo "❌ KingCrab binary not found. Build with: go build -o kingcrab ./cmd/kingcrab"
    exit 1
fi

echo "📦 Installing KingCrab binary..."
install -m 755 "$BINARY" "${INSTALL_DIR}/kingcrab"

# Create config directory
echo "⚙️ Creating configuration directory..."
mkdir -p "$CONFIG_DIR"

# Install default config if not exists
if [ ! -f "${CONFIG_DIR}/config.json" ]; then
    if [ -f "./config/config.json" ]; then
        cp ./config/config.json "${CONFIG_DIR}/config.json"
    else
        echo '{"version":"0.1.0","listen":{"type":"unix","path":"/var/run/kingcrab.sock"},"allowedCommands":["apt install *","systemctl restart *"],"requireReason":true}' > "${CONFIG_DIR}/config.json"
    fi
    echo "✅ Config installed to ${CONFIG_DIR}/config.json"
else
    echo "✅ Config already exists at ${CONFIG_DIR}/config.json"
fi

# Create data directory
echo "📁 Creating data directory..."
mkdir -p "$DATA_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" 2>/dev/null || true

# Create log directory
echo "📝 Creating log directory..."
mkdir -p "$LOG_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$LOG_DIR" 2>/dev/null || true

# Create service account if not exists
if ! id "$SERVICE_USER" &>/dev/null; then
    echo "👤 Creating service account '${SERVICE_USER}'..."
    useradd -r -s /bin/false -d "$DATA_DIR" -M "$SERVICE_USER" || true
    echo "✅ Service account created"
else
    echo "✅ Service account already exists"
fi

# Fix ownership
chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$LOG_DIR" 2>/dev/null || true
chmod 755 "$CONFIG_DIR"

# Install systemd service
echo "🔧 Installing systemd service..."
cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=KingCrab PAM Daemon
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
ExecStart=/usr/local/bin/kingcrab
Restart=always
RestartSec=5
Environment=KINGCRAB_CONFIG=$CONFIG_DIR/config.json

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

echo ""
echo "🎉 Installation complete!"
echo ""
echo "Next steps:"
echo "  1. Review config: sudo nano ${CONFIG_DIR}/config.json"
echo "  2. Start daemon:  sudo systemctl start kingcrab"
echo "  3. Check status:  sudo systemctl status kingcrab"
echo "  4. Enable on boot: sudo systemctl enable kingcrab"
echo ""
