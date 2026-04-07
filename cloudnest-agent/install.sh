#!/bin/bash
set -euo pipefail

# CloudNest Agent Installation Script
# Usage:
#   bash install.sh --master https://master.example.com
#   bash install.sh --master https://master.example.com --token-file /path/to/reg_token --secret-file /path/to/signing_secret
#
# Token/secret source:
#   1) --token-file / --secret-file (automation)
#   2) interactive prompt (default)

INSTALL_DIR="/opt/cloudnest-agent"
SERVICE_NAME="cloudnest-agent"
TMP_BINARY="${INSTALL_DIR}/cloudnest-agent.tmp"
MASTER_URL=""
REG_TOKEN_FILE=""
SECRET_FILE=""
PORT=8801
AGENT_HOME="$(getent passwd root | cut -d: -f6 2>/dev/null || true)"
[ -n "$AGENT_HOME" ] || AGENT_HOME="/root"
SCAN_DIRS="${AGENT_HOME}/data_save/files"
AGENT_ETC_DIR="/etc/cloudnest-agent"
AGENT_ENV_FILE="${AGENT_ETC_DIR}/agent.env"
SIGNING_SECRET_PATH="${AGENT_ETC_DIR}/signing_secret"
REG_TOKEN_TMP_FILE=""

cleanup() {
    if [ -n "$REG_TOKEN_TMP_FILE" ] && [ -f "$REG_TOKEN_TMP_FILE" ]; then
        rm -f "$REG_TOKEN_TMP_FILE"
    fi
}
trap cleanup EXIT

die() {
    echo "Error: $*" >&2
    exit 1
}

prompt_secret() {
    local prompt="$1"
    local __var_name="$2"
    local value=""
    if [ ! -e /dev/tty ]; then
        die "${prompt} requires interactive input, but /dev/tty is unavailable. Use --token-file/--secret-file."
    fi
    read -r -s -p "$prompt" value < /dev/tty
    echo "" > /dev/tty
    value="$(echo "$value" | tr -d '\r')"
    if [ -z "$value" ]; then
        die "input cannot be empty"
    fi
    printf -v "$__var_name" '%s' "$value"
}

read_secret_file() {
    local path="$1"
    local label="$2"
    local __var_name="$3"
    [ -f "$path" ] || die "${label} file not found: ${path}"
    local value
    value="$(tr -d '\r' < "$path" | tr -d '\n')"
    value="$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [ -n "$value" ] || die "${label} file is empty: ${path}"
    printf -v "$__var_name" '%s' "$value"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --master) MASTER_URL="$2"; shift 2 ;;
        --token-file) REG_TOKEN_FILE="$2"; shift 2 ;;
        --secret-file) SECRET_FILE="$2"; shift 2 ;;
        --port) PORT="$2"; shift 2 ;;
        --scan-dirs) SCAN_DIRS="$2"; shift 2 ;;
        --token|--secret)
            die "$1 is no longer supported for security reasons. Use --token-file/--secret-file or interactive input."
            ;;
        *)
            die "unknown option: $1"
            ;;
    esac
done

if [ -z "$MASTER_URL" ]; then
    echo "Usage: install.sh --master <master_url> [--token-file <path>] [--secret-file <path>] [--port <port>] [--scan-dirs <dirs>]"
    die "--master is required"
fi

REG_TOKEN=""
SIGNING_SECRET=""
if [ -n "$REG_TOKEN_FILE" ]; then
    read_secret_file "$REG_TOKEN_FILE" "registration token" REG_TOKEN
else
    prompt_secret "Registration token: " REG_TOKEN
fi
if [ -n "$SECRET_FILE" ]; then
    read_secret_file "$SECRET_FILE" "signing secret" SIGNING_SECRET
else
    prompt_secret "Signing secret: " SIGNING_SECRET
fi

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l)  ARCH="arm" ;;
esac

echo "=== CloudNest Agent Installer ==="
echo "Master:  ${MASTER_URL}"
echo "OS/Arch: ${OS}/${ARCH}"
echo ""

# Create install directory
mkdir -p "$INSTALL_DIR"
mkdir -p "$SCAN_DIRS"
mkdir -p "$AGENT_ETC_DIR"
chmod 700 "$AGENT_ETC_DIR"

# Stop existing service before replacing the binary.
if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    systemctl stop "$SERVICE_NAME" || true
fi

# Download agent binary
echo "Downloading agent binary..."
curl -sSLf -o "${TMP_BINARY}" "${MASTER_URL}/download/agent/${OS}/${ARCH}" || {
    echo "Error: failed to download agent binary for ${OS}/${ARCH}"
    echo "Supported: linux/amd64, linux/arm64"
    exit 1
}
chmod +x "$TMP_BINARY"
mv "$TMP_BINARY" "$INSTALL_DIR/cloudnest-agent"

# Register agent
echo "Registering agent..."
REG_TOKEN_TMP_FILE="$(mktemp)"
chmod 600 "$REG_TOKEN_TMP_FILE"
printf '%s\n' "$REG_TOKEN" > "$REG_TOKEN_TMP_FILE"
HOME="$AGENT_HOME" "$INSTALL_DIR/cloudnest-agent" register \
    --master "$MASTER_URL" \
    --token-file "$REG_TOKEN_TMP_FILE" \
    --port "$PORT" \
    --scan-dirs "$SCAN_DIRS"

# Persist runtime secrets and env file
printf '%s\n' "$SIGNING_SECRET" > "$SIGNING_SECRET_PATH"
chmod 600 "$SIGNING_SECRET_PATH"
cat > "$AGENT_ENV_FILE" <<EOF
HOME=${AGENT_HOME}
CLOUDNEST_SIGNING_SECRET_FILE=${SIGNING_SECRET_PATH}
EOF
chmod 600 "$AGENT_ENV_FILE"

# Create systemd service
echo "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=CloudNest Agent
After=network.target

[Service]
WorkingDirectory=${AGENT_HOME}
EnvironmentFile=${AGENT_ENV_FILE}
Type=simple
ExecStart=${INSTALL_DIR}/cloudnest-agent run
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo ""
echo "=== Installation Complete ==="
echo "Service: systemctl status ${SERVICE_NAME}"
echo "Logs:    journalctl -u ${SERVICE_NAME} -f"
echo "Config:  ~/.cloudnest/agent.json"
