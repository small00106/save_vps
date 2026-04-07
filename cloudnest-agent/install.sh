#!/bin/bash
set -e

# CloudNest Agent Installation Script
# Usage:
#   bash install.sh --master https://master.example.com --token <registration_token> --secret <signing_secret>
# Token/secret come from the master's secrets directory:
#   reg_token      -> data/secrets/reg_token
#   signing_secret -> data/secrets/signing_secret

INSTALL_DIR="/opt/cloudnest-agent"
SERVICE_NAME="cloudnest-agent"
TMP_BINARY="${INSTALL_DIR}/cloudnest-agent.tmp"
MASTER_URL=""
REG_TOKEN=""
SIGNING_SECRET=""
PORT=8801
AGENT_HOME="$(getent passwd root | cut -d: -f6 2>/dev/null || true)"
[ -n "$AGENT_HOME" ] || AGENT_HOME="/root"
SCAN_DIRS="${AGENT_HOME}/data_save/files"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --master) MASTER_URL="$2"; shift 2 ;;
        --token) REG_TOKEN="$2"; shift 2 ;;
        --secret) SIGNING_SECRET="$2"; shift 2 ;;
        --port) PORT="$2"; shift 2 ;;
        --scan-dirs) SCAN_DIRS="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [ -z "$MASTER_URL" ]; then
    echo "Error: --master is required"
    echo "Usage: install.sh --master <master_url> --token <token> --secret <secret>"
    exit 1
fi

if [ -z "$REG_TOKEN" ]; then
    echo "Error: --token is required (read it from the master's secrets/reg_token)"
    echo "Usage: install.sh --master <master_url> --token <token> --secret <secret>"
    exit 1
fi

if [ -z "$SIGNING_SECRET" ]; then
    echo "Error: --secret is required (read it from the master's secrets/signing_secret)"
    exit 1
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
HOME="$AGENT_HOME" "$INSTALL_DIR/cloudnest-agent" register \
    --master "$MASTER_URL" \
    --token "$REG_TOKEN" \
    --port "$PORT" \
    --scan-dirs "$SCAN_DIRS"

# Create systemd service
echo "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=CloudNest Agent
After=network.target

[Service]
WorkingDirectory=${AGENT_HOME}
Environment=HOME=${AGENT_HOME}
Type=simple
ExecStart=${INSTALL_DIR}/cloudnest-agent run
Restart=always
RestartSec=5
LimitNOFILE=65535
Environment=CLOUDNEST_SIGNING_SECRET=${SIGNING_SECRET}

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
