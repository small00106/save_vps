#!/usr/bin/env bash
set -euo pipefail

IMAGE=""
PORT="18881"
CONTAINER_NAME="cloudnest-smoke-$$"

usage() {
    cat <<'EOF'
Usage:
  scripts/smoke-test-docker.sh --image <image> [--port <port>]
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
        --image)
            IMAGE="$2"
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

if [[ -z "$IMAGE" ]]; then
    usage
    exit 1
fi

require_command docker
require_command curl

cleanup() {
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER_NAME" -p "${PORT}:8800" "$IMAGE" >/dev/null

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
