#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/release/dist"
WORK_DIR="$(mktemp -d)"
VERSION="${VERSION:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty)}"

cleanup() {
    rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Error: missing required command: $1" >&2
        exit 1
    fi
}

host_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)
            echo "unsupported"
            ;;
    esac
}

cc_for_arch() {
    local arch="$1"
    local current_arch
    current_arch="$(host_arch)"
    case "$arch" in
        amd64)
            if [[ "$current_arch" == "amd64" ]]; then
                echo "gcc"
            else
                echo "x86_64-linux-gnu-gcc"
            fi
            ;;
        arm64)
            if [[ "$current_arch" == "arm64" ]]; then
                echo "gcc"
            else
                echo "aarch64-linux-gnu-gcc"
            fi
            ;;
        *)
            return 1
            ;;
    esac
}

build_agent() {
    local arch="$1"
    local output="$2"
    (
        cd "${ROOT_DIR}/cloudnest-agent"
        CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$output" .
    )
}

build_master() {
    local arch="$1"
    local output="$2"
    local cc
    cc="$(cc_for_arch "$arch")"
    require_command "$cc"
    (
        cd "${MASTER_SRC_DIR}"
        CGO_ENABLED=1 GOOS=linux GOARCH="$arch" CC="$cc" go build -trimpath -o "$output" .
    )
}

package_master() {
    local arch="$1"
    local master_binary="$2"
    local pkg_dir="${WORK_DIR}/pkg-${arch}"
    rm -rf "$pkg_dir"
    mkdir -p "${pkg_dir}/data/binaries"

    install -m 0755 "$master_binary" "${pkg_dir}/cloudnest"
    install -m 0755 "${AGENT_DIST_DIR}/cloudnest-agent-linux-amd64" "${pkg_dir}/data/binaries/cloudnest-agent-linux-amd64"
    install -m 0755 "${AGENT_DIST_DIR}/cloudnest-agent-linux-arm64" "${pkg_dir}/data/binaries/cloudnest-agent-linux-arm64"
    printf '%s\n' "$VERSION" >"${pkg_dir}/VERSION"

    tar -C "$pkg_dir" -czf "${DIST_DIR}/cloudnest-master-linux-${arch}.tar.gz" .
}

require_command git
require_command go
require_command npm
require_command tar
require_command sha256sum
require_command gcc

if [[ "$(uname -s | tr '[:upper:]' '[:lower:]')" != "linux" ]]; then
    echo "Error: release/build-release.sh currently supports Linux only" >&2
    exit 1
fi

CURRENT_ARCH="$(host_arch)"
if [[ "$CURRENT_ARCH" == "unsupported" ]]; then
    echo "Error: unsupported host architecture $(uname -m)" >&2
    exit 1
fi

if [[ "$CURRENT_ARCH" != "arm64" ]]; then
    require_command aarch64-linux-gnu-gcc
else
    require_command x86_64-linux-gnu-gcc
fi

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "==> Building frontend"
(
    cd "${ROOT_DIR}/cloudnest-web"
    npm ci
    npm run build
)

MASTER_SRC_DIR="${WORK_DIR}/cloudnest"
AGENT_DIST_DIR="${WORK_DIR}/agent"
MASTER_DIST_DIR="${WORK_DIR}/master"
mkdir -p "$MASTER_SRC_DIR" "$AGENT_DIST_DIR" "$MASTER_DIST_DIR"

echo "==> Preparing master source tree"
cp -a "${ROOT_DIR}/cloudnest/." "${MASTER_SRC_DIR}/"
rm -rf "${MASTER_SRC_DIR}/public/dist"
mkdir -p "${MASTER_SRC_DIR}/public/dist"
cp -a "${ROOT_DIR}/cloudnest-web/dist/." "${MASTER_SRC_DIR}/public/dist/"

echo "==> Building agent binaries"
build_agent amd64 "${AGENT_DIST_DIR}/cloudnest-agent-linux-amd64"
build_agent arm64 "${AGENT_DIST_DIR}/cloudnest-agent-linux-arm64"
install -m 0755 "${AGENT_DIST_DIR}/cloudnest-agent-linux-amd64" "${DIST_DIR}/cloudnest-agent-linux-amd64"
install -m 0755 "${AGENT_DIST_DIR}/cloudnest-agent-linux-arm64" "${DIST_DIR}/cloudnest-agent-linux-arm64"

echo "==> Building master binaries"
build_master amd64 "${MASTER_DIST_DIR}/cloudnest-linux-amd64"
build_master arm64 "${MASTER_DIST_DIR}/cloudnest-linux-arm64"

echo "==> Packaging release tarballs"
package_master amd64 "${MASTER_DIST_DIR}/cloudnest-linux-amd64"
package_master arm64 "${MASTER_DIST_DIR}/cloudnest-linux-arm64"

echo "==> Writing checksums"
(
    cd "${DIST_DIR}"
    sha256sum \
        cloudnest-master-linux-amd64.tar.gz \
        cloudnest-master-linux-arm64.tar.gz \
        cloudnest-agent-linux-amd64 \
        cloudnest-agent-linux-arm64 \
        > checksums.txt
)

echo "==> Done"
echo "Release artifacts:"
find "${DIST_DIR}" -maxdepth 1 -type f -printf '  %f\n'
