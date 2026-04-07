#!/usr/bin/env bash
set -euo pipefail

BUNDLE=""
PORT="18880"

usage() {
    cat <<'EOF'
Usage:
  scripts/smoke-test-release.sh --bundle <tar.gz> [--port <port>]
EOF
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Error: missing required command: $1" >&2
        exit 1
    fi
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --bundle)
            BUNDLE="$2"
            shift 2
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

if [[ -z "$BUNDLE" ]]; then
    usage
    exit 1
fi

require_command curl
require_command tar

TMP_DIR="$(mktemp -d)"
cleanup() {
    if [[ -n "${SERVER_PID:-}" ]]; then
        kill "$SERVER_PID" >/dev/null 2>&1 || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

tar -xzf "$BUNDLE" -C "$TMP_DIR"

export CLOUDNEST_LISTEN="127.0.0.1:${PORT}"
export CLOUDNEST_DB_TYPE="sqlite"
export CLOUDNEST_DB_DSN="${TMP_DIR}/cloudnest.db"
export CLOUDNEST_PUBLIC_BASE_URL="http://127.0.0.1:${PORT}"

"${TMP_DIR}/cloudnest" server >"${TMP_DIR}/cloudnest.log" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null; then
        break
    fi
    sleep 1
done

curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null
curl -fsS "http://127.0.0.1:${PORT}/install.sh" | grep -q "/download/agent/"
curl -fsS -o /dev/null "http://127.0.0.1:${PORT}/download/agent/linux/amd64"
curl -fsS -o /dev/null "http://127.0.0.1:${PORT}/download/agent/linux/arm64"
