#!/bin/bash
set -e

# CloudNest Agent Installation Script
# Usage: curl -sSL https://your-server/install.sh | bash -s -- --master http://master:8800 --token your-token

INSTALL_DIR="/opt/cloudnest-agent"
SERVICE_NAME="cloudnest-agent"
MASTER_URL=""
REG_TOKEN=""
PORT=8801
SCAN_DIRS="${HOME}/data_save/files"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --master) MASTER_URL="$2"; shift 2 ;;
        --token) REG_TOKEN="$2"; shift 2 ;;
        --port) PORT="$2"; shift 2 ;;
        --scan-dirs) SCAN_DIRS="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [ -z "$MASTER_URL" ] || [ -z "$REG_TOKEN" ]; then
    echo "Usage: install.sh --master <master_url> --token <registration_token>"
    exit 1
fi

echo "=== CloudNest Agent Installer ==="

# Create install directory
mkdir -p "$INSTALL_DIR"
mkdir -p "$SCAN_DIRS"

# Download binary (placeholder - replace with actual download URL)
if [ ! -f "$INSTALL_DIR/cloudnest-agent" ]; then
    echo "Please place the cloudnest-agent binary in $INSTALL_DIR/"
    exit 1
fi

chmod +x "$INSTALL_DIR/cloudnest-agent"

# Register agent
echo "Registering agent with master..."
"$INSTALL_DIR/cloudnest-agent" register \
    --master "$MASTER_URL" \
    --token "$REG_TOKEN" \
    --port "$PORT" \
    --scan-dirs "$SCAN_DIRS"

# Create systemd service
cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=CloudNest Agent
After=network.target

[Service]
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
systemctl start "$SERVICE_NAME"

echo "=== CloudNest Agent installed and started ==="
echo "Service: systemctl status $SERVICE_NAME"
echo "Logs:    journalctl -u $SERVICE_NAME -f"
