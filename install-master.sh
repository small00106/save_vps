#!/usr/bin/env bash
set -euo pipefail

REPO="small00106/save_vps"
VERSION="latest"
INSTALL_DIR="/opt/cloudnest"
DATA_DIR="/var/lib/cloudnest"
ENV_FILE="/etc/cloudnest/cloudnest.env"
LISTEN_ADDR="0.0.0.0:8800"
PUBLIC_BASE_URL=""
SERVICE_NAME="cloudnest"

usage() {
    cat <<'EOF'
CloudNest Master installer

Usage:
  bash install-master.sh [options]

Options:
  --repo <owner/repo>            GitHub repository (default: small00106/save_vps)
  --version <tag|latest>         Release version to install (default: latest)
  --install-dir <path>           Install directory (default: /opt/cloudnest)
  --data-dir <path>              Data directory (default: /var/lib/cloudnest)
  --listen <host:port>           Listen address written to env file if missing (default: 0.0.0.0:8800)
  --public-base-url <url>        Public base URL for generated Agent install script
  --help                         Show this help
EOF
}

require_root() {
    if [[ "$(id -u)" -ne 0 ]]; then
        echo "Error: please run as root, for example: sudo bash install-master.sh" >&2
        exit 1
    fi
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Error: missing required command: $1" >&2
        exit 1
    fi
}

append_env_if_missing() {
    local key="$1"
    local value="$2"
    if ! grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
        printf '%s=%s\n' "$key" "$value" >>"$ENV_FILE"
    fi
}

set_env_value() {
    local key="$1"
    local value="$2"
    local tmp_file
    tmp_file="$(mktemp)"
    if [[ -f "$ENV_FILE" ]]; then
        grep -v "^${key}=" "$ENV_FILE" >"$tmp_file" || true
    fi
    printf '%s=%s\n' "$key" "$value" >>"$tmp_file"
    mv "$tmp_file" "$ENV_FILE"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo)
            REPO="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --listen)
            LISTEN_ADDR="$2"
            shift 2
            ;;
        --public-base-url)
            PUBLIC_BASE_URL="$2"
            shift 2
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Error: unknown option $1" >&2
            usage
            exit 1
            ;;
    esac
done

require_root
require_command curl
require_command tar
require_command systemctl

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture ${ARCH}, expected amd64 or arm64" >&2
        exit 1
        ;;
esac

if [[ "$OS" != "linux" ]]; then
    echo "Error: install-master.sh currently supports Linux only" >&2
    exit 1
fi

PUBLIC_BASE_URL="${PUBLIC_BASE_URL%/}"
ENV_DIR="$(dirname "$ENV_FILE")"
ASSET_NAME="cloudnest-master-linux-${ARCH}.tar.gz"
TMP_DIR="$(mktemp -d)"
TAR_PATH="${TMP_DIR}/${ASSET_NAME}"
EXTRACT_DIR="${TMP_DIR}/extract"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
if [[ "$VERSION" != "latest" ]]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
fi

cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo "=== CloudNest Master Installer ==="
echo "Repo:        ${REPO}"
echo "Version:     ${VERSION}"
echo "Platform:    ${OS}/${ARCH}"
echo "Install dir: ${INSTALL_DIR}"
echo "Data dir:    ${DATA_DIR}"
echo

mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$ENV_DIR"

if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    systemctl stop "$SERVICE_NAME" || true
fi

echo "Downloading ${DOWNLOAD_URL}"
curl -fsSL -o "$TAR_PATH" "$DOWNLOAD_URL"

mkdir -p "$EXTRACT_DIR"
tar -xzf "$TAR_PATH" -C "$EXTRACT_DIR"

install -m 0755 "${EXTRACT_DIR}/cloudnest" "${INSTALL_DIR}/cloudnest"
mkdir -p "${INSTALL_DIR}/data/binaries"
install -m 0755 "${EXTRACT_DIR}/data/binaries/cloudnest-agent-linux-amd64" "${INSTALL_DIR}/data/binaries/cloudnest-agent-linux-amd64"
install -m 0755 "${EXTRACT_DIR}/data/binaries/cloudnest-agent-linux-arm64" "${INSTALL_DIR}/data/binaries/cloudnest-agent-linux-arm64"
if [[ -f "${EXTRACT_DIR}/VERSION" ]]; then
    install -m 0644 "${EXTRACT_DIR}/VERSION" "${INSTALL_DIR}/VERSION"
fi

touch "$ENV_FILE"
append_env_if_missing "CLOUDNEST_LISTEN" "$LISTEN_ADDR"
append_env_if_missing "CLOUDNEST_DB_TYPE" "sqlite"
append_env_if_missing "CLOUDNEST_DB_DSN" "${DATA_DIR}/cloudnest.db"
if [[ -n "$PUBLIC_BASE_URL" ]]; then
    set_env_value "CLOUDNEST_PUBLIC_BASE_URL" "$PUBLIC_BASE_URL"
fi
chmod 600 "$ENV_FILE"

cat >"/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=CloudNest Master
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${INSTALL_DIR}/cloudnest server
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo
echo "=== Installation Complete ==="
echo "Service:    systemctl status ${SERVICE_NAME}"
echo "Logs:       journalctl -u ${SERVICE_NAME} -f"
echo "Env file:   ${ENV_FILE}"
echo "Data dir:   ${DATA_DIR}"
echo "Secrets:    ${DATA_DIR}/secrets/"
if [[ -n "$PUBLIC_BASE_URL" ]]; then
    echo "Agent URL:  ${PUBLIC_BASE_URL}/install.sh"
else
    echo "Agent URL:  http://<server>:8800/install.sh"
fi
