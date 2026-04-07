#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

usage() {
    cat <<'EOF'
Usage:
  scripts/build-assets.sh frontend --output <dir>
  scripts/build-assets.sh agent --output <dir>
EOF
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Error: missing required command: $1" >&2
        exit 1
    fi
}

copy_dir_contents() {
    src_dir=$1
    dst_dir=$2
    mkdir -p "$dst_dir"
    cp -a "$src_dir"/. "$dst_dir"/
}

MODE=""
OUTPUT_DIR=""

while [ $# -gt 0 ]; do
    case "$1" in
        frontend|agent)
            MODE="$1"
            shift
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

if [ -z "$MODE" ] || [ -z "$OUTPUT_DIR" ]; then
    usage
    exit 1
fi

case "$MODE" in
    frontend)
        require_command npm
        cd "${ROOT_DIR}/cloudnest-web"
        npm ci
        npm run build
        copy_dir_contents "${ROOT_DIR}/cloudnest-web/dist" "$OUTPUT_DIR"
        ;;
    agent)
        require_command go
        mkdir -p "$OUTPUT_DIR"
        cd "${ROOT_DIR}/cloudnest-agent"
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "${OUTPUT_DIR}/cloudnest-agent-linux-amd64" .
        CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o "${OUTPUT_DIR}/cloudnest-agent-linux-arm64" .
        ;;
esac
