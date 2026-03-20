#!/bin/bash
# KingCrab v2 Installer
# Installs the KingCrab daemon as a systemd service

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
VERSION="1.0.0"
BINARY_NAME="kingcrab"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/kingcrab"
DATA_DIR="/var/lib/kingcrab"
LOG_DIR="/var/log/kingcrab"
RUN_DIR="/var/run/kingcrab"
USER="kingcrab"
GROUP="kingcrab"
SERVICE_FILE="/etc/systemd/system/kingcrab.service"

# log_info prints an informational message prefixed with a green `[INFO]` tag followed by the provided text.
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

# log_warn prints a warning message prefixed with `[WARN]` in yellow to stdout.
log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# log_error prints an error message prefixed with a red `[ERROR]` tag.
log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# check_root verifies the script runs as root and exits with status 1 if not.
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# check_prerequisites verifies presence of required tooling: detects Go (logs a warning if missing), detects the PostgreSQL client (logs a warning if missing), and ensures systemd is available (logs an error and exits with status 1 if missing).
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for Go installation if building from source
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
        GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

        if [[ "$GO_MAJOR" -gt 1 ]] || [[ "$GO_MAJOR" -eq 1 && "$GO_MINOR" -ge 22 ]]; then
            log_info "Go 1.22+ found (version $GO_VERSION)"
        else
            log_warn "Go $GO_VERSION found but 1.22+ required. Please upgrade Go 1.22+ or use pre-built binary"
        fi
    else
        log_warn "Go not found. Please install Go 1.22+ or use pre-built binary"
    fi

    # Check for PostgreSQL
    if command -v psql &> /dev/null; then
        log_info "PostgreSQL client found"
    else
        log_warn "PostgreSQL client not found. Please install postgresql-client"
    fi

    # Check for systemd
    if command -v systemctl &> /dev/null; then
        log_info "systemd found"
    else
        log_error "systemd not found. This installer requires systemd"
        exit 1
    fi
}

# create_user creates the system user specified by $USER (system account with no login and home set to $DATA_DIR) if it does not already exist.
create_user() {
    log_info "Creating system user..."

    if id "$USER" &>/dev/null; then
        log_warn "User $USER already exists"
    else
        useradd -r -U -s /bin/false -d "$DATA_DIR" -M "$USER"
        log_info "User $USER created"
    fi
}

# create_directories creates required config, data, log, run, and install directories and sets ownership and permissions appropriate for the kingcrab system user and group.
create_directories() {
    log_info "Creating directories..."

    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$RUN_DIR"
    mkdir -p "$INSTALL_DIR"

    # Set ownership
    chown -R "$USER:$GROUP" "$DATA_DIR"
    chown -R "$USER:$GROUP" "$LOG_DIR"
    chown -R "$USER:$GROUP" "$RUN_DIR"

    # Set permissions
    chmod 755 "$CONFIG_DIR"
    chmod 700 "$DATA_DIR"
    chmod 755 "$LOG_DIR"
    chmod 755 "$RUN_DIR"
}

# install_binary installs the KingCrab binary into "$INSTALL_DIR" by copying it from "./$BINARY_NAME" or "./bin/$BINARY_NAME", makes the installed file executable, and exits with status 1 if no source binary is found.
install_binary() {
    log_info "Installing binary..."

    # Check if binary exists in current directory
    if [[ -f "./$BINARY_NAME" ]]; then
        cp "./$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
        log_info "Binary installed to $INSTALL_DIR/$BINARY_NAME"
    elif [[ -f "./bin/$BINARY_NAME" ]]; then
        cp "./bin/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
        log_info "Binary installed to $INSTALL_DIR/$BINARY_NAME"
    else
        log_error "Binary not found. Please build first: go build -o $BINARY_NAME ./cmd/$BINARY_NAME"
        exit 1
    fi
}

# install_config installs the daemon configuration at "$CONFIG_DIR/config.json".
# If a config already exists it is backed up to "config.json.bak". If a project example config exists it is copied into place; otherwise a sensible default JSON config is created with embedded version, runtime paths, allowed commands, and OpenClaw settings. The resulting file is owned by root:"$GROUP" and set to mode 640.
install_config() {
    log_info "Installing configuration..."

    if [[ -f "$CONFIG_DIR/config.json" ]]; then
        log_warn "Config file already exists. Backing up..."
        cp "$CONFIG_DIR/config.json" "$CONFIG_DIR/config.json.bak"
    fi

    # Copy example config if exists, otherwise create default
    if [[ -f "./config/config.example.json" ]]; then
        cp "./config/config.example.json" "$CONFIG_DIR/config.json"
    else
        cat > "$CONFIG_DIR/config.json" <<EOF
{
  "version": "$VERSION",
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
  "logDir": "$LOG_DIR",
  "dataDir": "$DATA_DIR",
  "openclaw": {
    "webhookUrl": "",
    "enabled": false
  }
}
EOF
    fi

    chmod 640 "$CONFIG_DIR/config.json"
    chown root:"$GROUP" "$CONFIG_DIR/config.json"
}

# install_systemd_service writes a hardened systemd unit for the KingCrab daemon to $SERVICE_FILE (including environment, logging, and security hardening) then reloads systemd and logs completion.
install_systemd_service() {
    log_info "Installing systemd service..."

    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=KingCrab PAM Daemon v${VERSION}
Documentation=https://github.com/KHAEntertainment/kingcrab
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=root
Group=root
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=5s

# Security hardening
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR $LOG_DIR $RUN_DIR /var/run
RuntimeDirectory=kingcrab
RuntimeDirectoryMode=0755

# Environment
Environment="KINGCRAB_CONFIG=$CONFIG_DIR/config.json"

# Logging
StandardOutput=append:$LOG_DIR/daemon.log
StandardError=append:$LOG_DIR/daemon.log
SyslogIdentifier=kingcrab

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    log_info "Systemd service installed"
}

# setup_database prompts for PostgreSQL credentials, creates the database and user if needed, writes a secure systemd override with the database environment variables, reloads systemd, and runs the bundled migration SQL if present.
setup_database() {
    log_info "Setting up database..."

    # Prompt for database configuration
    read -p "Database host [localhost]: " DB_HOST
    DB_HOST=${DB_HOST:-localhost}

    read -p "Database port [5432]: " DB_PORT
    DB_PORT=${DB_PORT:-5432}

    read -p "Database name [kingcrab]: " DB_NAME
    DB_NAME=${DB_NAME:-kingcrab}

    read -p "Database user [kingcrab]: " DB_USER
    DB_USER=${DB_USER:-kingcrab}

    read -sp "Database password: " DB_PASSWORD
    echo

    if [[ -z "$DB_PASSWORD" ]]; then
        log_error "Database password is required"
        exit 1
    fi

    # Check if PostgreSQL is running
    if ! command -v psql &> /dev/null; then
        log_warn "psql not found. Skipping database setup. Please setup manually."
        return
    fi

    # Sanitize credentials
    DB_USER_ESCAPED="${DB_USER//\"/\\\"}"
    DB_PASSWORD_ESCAPED="${DB_PASSWORD//\'/\'\'}"

    # Create database and user if they don't exist
    log_info "Creating database and user..."

    if sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw "$DB_NAME"; then
        log_warn "Database $DB_NAME already exists"
    else
        sudo -u postgres createdb -O "$DB_USER_ESCAPED" "$DB_NAME" 2>/dev/null || {
            # Try creating user first
            sudo -u postgres psql -c "CREATE USER \"$DB_USER_ESCAPED\" WITH PASSWORD '$DB_PASSWORD_ESCAPED';"
            sudo -u postgres createdb -O "$DB_USER_ESCAPED" "$DB_NAME"
        }
        log_info "Database $DB_NAME created"
    fi

    # Store database password in systemd override with secure permissions
    mkdir -p "/etc/systemd/system/kingcrab.service.d"
    DB_CONF_FILE="/etc/systemd/system/kingcrab.service.d/database.conf"

    # Write to temporary file first
    TMP_CONF=$(mktemp)
    cat > "$TMP_CONF" <<EOF
[Service]
Environment="KINGCRAB_DB_HOST=$DB_HOST"
Environment="KINGCRAB_DB_PORT=$DB_PORT"
Environment="KINGCRAB_DB_NAME=$DB_NAME"
Environment="KINGCRAB_DB_USER=$DB_USER"
Environment="KINGCRAB_DB_PASSWORD=$DB_PASSWORD"
Environment="KINGCRAB_DB_SSLMODE=require"
EOF

    # Set secure permissions and move into place
    chown root:root "$TMP_CONF"
    chmod 600 "$TMP_CONF"
    mv "$TMP_CONF" "$DB_CONF_FILE"

    systemctl daemon-reload

    # Run migrations
    log_info "Running database migrations..."
    if command -v kingcrab &> /dev/null || [[ -f "$INSTALL_DIR/$BINARY_NAME" ]]; then
        # Use daemon migration command if available
        # Temporarily disable errexit to capture exit status
        set +e
        KINGCRAB_DB_HOST="$DB_HOST" KINGCRAB_DB_PORT="$DB_PORT" KINGCRAB_DB_USER="$DB_USER" KINGCRAB_DB_NAME="$DB_NAME" KINGCRAB_DB_PASSWORD="$DB_PASSWORD" "$INSTALL_DIR/$BINARY_NAME" --migrate 2>/dev/null
        MIGRATION_EXIT=$?
        set -e

        if [[ $MIGRATION_EXIT -eq 0 ]]; then
            log_info "Database migrations completed via daemon"
        else
            log_warn "Migration command failed, falling back to psql"
            if [[ -f "./internal/db/migrations/001_pam_schema.sql" ]]; then
                PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "./internal/db/migrations/001_pam_schema.sql"
                log_info "Database migrations completed via psql"
            else
                log_warn "Migration file not found. Daemon will run migrations on first start."
            fi
        fi
    elif [[ -f "./internal/db/migrations/001_pam_schema.sql" ]]; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "./internal/db/migrations/001_pam_schema.sql"
        log_info "Database migrations completed via psql"
    else
        log_warn "Migration file not found. Daemon will run migrations on first start."
    fi
}

# start_service enables and starts the kingcrab systemd service, waits briefly, verifies the service is active, logs success or an error (with a journalctl hint), and exits with status 1 if the service failed to start.
start_service() {
    log_info "Starting KingCrab service..."

    systemctl enable kingcrab
    systemctl start kingcrab

    # Wait for service to start
    sleep 2

    if systemctl is-active --quiet kingcrab; then
        log_info "KingCrab service started successfully"
    else
        log_error "Failed to start KingCrab service"
        log_error "Check logs with: journalctl -u kingcrab -n 50"
        exit 1
    fi
}

# print_success prints the installation completion message including service management commands, a health endpoint example, the configuration file path, and brief next-step instructions.
print_success() {
    echo ""
    log_info "KingCrab v${VERSION} installed successfully!"
    echo ""
    echo "Service commands:"
    echo "  sudo systemctl start kingcrab    # Start service"
    echo "  sudo systemctl stop kingcrab     # Stop service"
    echo "  sudo systemctl restart kingcrab  # Restart service"
    echo "  sudo systemctl status kingcrab   # Check status"
    echo "  sudo journalctl -u kingcrab -f   # View logs"
    echo ""
    echo "API test:"
    echo "  curl http://localhost:8080/api/v1/health"
    echo ""
    echo "Configuration:"
    echo "  $CONFIG_DIR/config.json"
    echo ""
    echo "Next steps:"
    echo "  1. Configure OpenClaw plugin (see plugin/README.md)"
    echo "  2. Enroll biometric device: /kc enroll"
    echo ""
}

# perform_upgrade runs the upgrade workflow: installs the new binary, runs database migrations, restarts the service, and logs completion.
perform_upgrade() {
    log_info "Running upgrade workflow..."

    install_binary

    # Prompt for database migration
    read -p "Run database migrations now? [y/N]: " RUN_MIGRATIONS
    if [[ "$RUN_MIGRATIONS" =~ ^[Yy]$ ]]; then
        setup_database
    else
        log_warn "Skipping database migrations. Run manually or service will run them on start."
    fi

    # Restart service to pick up new binary
    log_info "Restarting KingCrab service..."
    systemctl restart kingcrab

    # Wait for service to restart
    sleep 2

    if systemctl is-active --quiet kingcrab; then
        log_info "KingCrab service restarted successfully"
    else
        log_error "Failed to restart KingCrab service"
        log_error "Check logs with: journalctl -u kingcrab -n 50"
        exit 1
    fi

    log_info "Upgrade completed successfully"
}

# main runs the full installation workflow for the KingCrab daemon: performs preflight checks, creates the system user and directories, installs the binary and configuration, writes the systemd service, optionally sets up the database interactively, starts and enables the service, and prints completion instructions.
main() {
    log_info "KingCrab v${VERSION} Installer"
    echo ""

    # Parse arguments for upgrade mode
    UPGRADE=false
    for arg in "$@"; do
        if [[ "$arg" == "--upgrade" || "$arg" == "-u" ]]; then
            UPGRADE=true
            break
        fi
    done

    check_root
    check_prerequisites

    if [[ "$UPGRADE" == "true" ]]; then
        log_info "Running in upgrade mode"
        perform_upgrade
        print_success
        return
    fi

    # Fresh install workflow
    create_user
    create_directories
    install_binary
    install_config
    install_systemd_service

    # Prompt for database setup
    read -p "Setup database now? [y/N]: " SETUP_DB
    if [[ "$SETUP_DB" =~ ^[Yy]$ ]]; then
        setup_database
    else
        log_warn "Skipping database setup. Remember to set KINGCRAB_DB_PASSWORD environment variable."
    fi

    start_service
    print_success
}

# Run main
main "$@"