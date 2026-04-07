package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	adminAPI "github.com/cloudnest/cloudnest/internal/api/admin"
	agentAPI "github.com/cloudnest/cloudnest/internal/api/agent"
	alertsAPI "github.com/cloudnest/cloudnest/internal/api/alerts"
	authAPI "github.com/cloudnest/cloudnest/internal/api/auth"
	commandAPI "github.com/cloudnest/cloudnest/internal/api/command"
	filesAPI "github.com/cloudnest/cloudnest/internal/api/files"
	nodesAPI "github.com/cloudnest/cloudnest/internal/api/nodes"
	pingAPI "github.com/cloudnest/cloudnest/internal/api/ping"
	terminalAPI "github.com/cloudnest/cloudnest/internal/api/terminal"
	"github.com/cloudnest/cloudnest/internal/server/middleware"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/cloudnest/cloudnest/public"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const publicBaseURLEnvKey = "CLOUDNEST_PUBLIC_BASE_URL"

var executablePathFunc = os.Executable

func SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Ensure default admin user
	authAPI.EnsureDefaultAdmin()

	api := r.Group("/api")
	{
		// === Auth (public) ===
		api.POST("/auth/login", authAPI.Login)
		api.POST("/auth/logout", authAPI.Logout)

		// === Agent endpoints ===
		api.POST("/agent/register", agentAPI.Register)
		api.GET("/agent/ws", agentAPI.WebSocketHandler)

		// === Authenticated routes ===
		authed := api.Group("")
		authed.Use(middleware.AuthRequired())
		{
			authed.GET("/auth/me", authAPI.Me)
			authed.POST("/auth/change-password", authAPI.ChangePassword)
			authed.POST("/auth/default-password-notice/ack", authAPI.AcknowledgeDefaultPasswordNotice)

			// Nodes
			authed.GET("/nodes", nodesAPI.List)
			authed.GET("/nodes/:uuid", nodesAPI.Get)
			authed.GET("/nodes/:uuid/metrics", nodesAPI.GetMetrics)
			authed.GET("/nodes/:uuid/traffic", nodesAPI.GetTraffic)
			authed.PUT("/nodes/:uuid/tags", nodesAPI.UpdateTags)

			// Node files
			authed.GET("/nodes/:uuid/files", filesAPI.BrowseNodeFiles)
			authed.GET("/nodes/:uuid/download", filesAPI.NodeDownloadURL)

			// Files (virtual)
			authed.POST("/files/upload", filesAPI.InitUpload)
			authed.GET("/files/download/:id", filesAPI.GetDownloadURL)
			authed.GET("/files", filesAPI.ListFiles)
			authed.POST("/files/mkdir", filesAPI.CreateDir)
			authed.DELETE("/files/:id", filesAPI.DeleteFile)
			authed.PUT("/files/:id/move", filesAPI.MoveFile)
			authed.GET("/files/search", filesAPI.Search)

			// Proxy (data plane through Master for HTTPS compatibility)
			authed.PUT("/proxy/upload/:file_id", filesAPI.ProxyUpload)
			authed.GET("/proxy/download/:file_id", filesAPI.ProxyDownload)
			authed.GET("/proxy/browse", filesAPI.ProxyBrowse)

			// Dashboard WebSocket
			authed.GET("/ws/dashboard", dashboardWS)

			// Remote operations
			authed.POST("/nodes/:uuid/exec", commandAPI.Exec)
			authed.GET("/commands/:id", commandAPI.GetTask)
			authed.GET("/ws/terminal/:uuid", terminalAPI.HandleTerminal)

			// Ping
			authed.GET("/ping/tasks", pingAPI.ListTasks)
			authed.POST("/ping/tasks", pingAPI.CreateTask)
			authed.GET("/ping/tasks/:id/results", pingAPI.GetResults)
			authed.DELETE("/ping/tasks/:id", pingAPI.DeleteTask)

			// Alerts
			authed.GET("/alerts/rules", alertsAPI.ListRules)
			authed.POST("/alerts/rules", alertsAPI.CreateRule)
			authed.PUT("/alerts/rules/:id", alertsAPI.UpdateRule)
			authed.DELETE("/alerts/rules/:id", alertsAPI.DeleteRule)
			authed.GET("/alerts/channels", alertsAPI.ListChannels)
			authed.POST("/alerts/channels", alertsAPI.CreateChannel)
			authed.PUT("/alerts/channels/:id", alertsAPI.UpdateChannel)

			// Admin
			authed.GET("/admin/settings", adminAPI.GetSettings)
			authed.PUT("/admin/settings", adminAPI.UpdateSettings)
			authed.GET("/admin/audit", adminAPI.GetAuditLogs)
		}
	}

	// === Agent download endpoints (public, no auth) ===
	r.GET("/healthz", healthz)
	r.GET("/install.sh", serveInstallScript)
	r.GET("/download/agent/:os/:arch", serveAgentBinary)

	// === Embedded SPA frontend ===
	setupFrontend(r)

	return r
}

// setupFrontend serves the embedded frontend SPA.
// Static assets (js/css/svg) served directly; all other paths get index.html.
func setupFrontend(r *gin.Engine) {
	distFS, err := fs.Sub(public.DistFS, "dist")
	if err != nil {
		return
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API paths
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Try to serve the exact file (js, css, svg, ico, png, etc.)
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		if f, err := distFS.(fs.ReadFileFS).ReadFile(cleanPath); err == nil {
			// Detect content type
			ct := "application/octet-stream"
			switch {
			case strings.HasSuffix(cleanPath, ".html"):
				ct = "text/html; charset=utf-8"
			case strings.HasSuffix(cleanPath, ".js"):
				ct = "application/javascript"
			case strings.HasSuffix(cleanPath, ".css"):
				ct = "text/css"
			case strings.HasSuffix(cleanPath, ".svg"):
				ct = "image/svg+xml"
			case strings.HasSuffix(cleanPath, ".png"):
				ct = "image/png"
			case strings.HasSuffix(cleanPath, ".json"):
				ct = "application/json"
			}
			c.Data(http.StatusOK, ct, f)
			return
		}

		// SPA fallback: serve index.html for all routes
		indexData, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "frontend not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})
}

// serveInstallScript serves the agent install script.
func serveInstallScript(c *gin.Context) {
	masterURL := resolvePublicBaseURL(c.Request, os.Getenv)
	script := generateInstallScript(masterURL)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(script))
}

func resolvePublicBaseURL(req *http.Request, getenv func(string) string) string {
	if getenv != nil {
		if configured := strings.TrimSpace(getenv(publicBaseURLEnvKey)); configured != "" {
			return strings.TrimRight(configured, "/")
		}
	}

	scheme := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + req.Host
}

func generateInstallScript(masterURL string) string {
	return `#!/bin/bash
set -euo pipefail

# CloudNest Agent One-Click Installer
# Usage:
#   curl -sSL ` + masterURL + `/install.sh | bash -s --
#   curl -sSL ` + masterURL + `/install.sh | bash -s -- --token-file /path/to/reg_token --secret-file /path/to/signing_secret

MASTER_URL="` + masterURL + `"
INSTALL_DIR="/opt/cloudnest-agent"
SERVICE_NAME="cloudnest-agent"
TMP_BINARY="${INSTALL_DIR}/cloudnest-agent.tmp"
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
        --token-file) REG_TOKEN_FILE="$2"; shift 2 ;;
        --secret-file) SECRET_FILE="$2"; shift 2 ;;
        --port) PORT="$2"; shift 2 ;;
        --scan-dirs) SCAN_DIRS="$2"; shift 2 ;;
        --token|--secret)
            die "$1 is no longer supported for security reasons. Use --token-file/--secret-file or interactive input."
            ;;
        *) die "unknown option: $1" ;;
    esac
done

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

# Create directories
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
chmod +x "${TMP_BINARY}"
mv "${TMP_BINARY}" "${INSTALL_DIR}/cloudnest-agent"

# Register with master
echo "Registering agent..."
REG_TOKEN_TMP_FILE="$(mktemp)"
chmod 600 "$REG_TOKEN_TMP_FILE"
printf '%s\n' "$REG_TOKEN" > "$REG_TOKEN_TMP_FILE"
HOME="$AGENT_HOME" "${INSTALL_DIR}/cloudnest-agent" register \
    --master "$MASTER_URL" \
    --token-file "$REG_TOKEN_TMP_FILE" \
    --port "$PORT" \
    --scan-dirs "$SCAN_DIRS"

# Persist runtime secrets and env file
printf '%s\n' "$SIGNING_SECRET" > "$SIGNING_SECRET_PATH"
chmod 600 "$SIGNING_SECRET_PATH"
cat > "$AGENT_ENV_FILE" <<ENVEOF
HOME=${AGENT_HOME}
CLOUDNEST_SIGNING_SECRET_FILE=${SIGNING_SECRET_PATH}
ENVEOF
chmod 600 "$AGENT_ENV_FILE"

# Create systemd service
echo "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service <<SERVICEEOF
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
SERVICEEOF

# Enable and start
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo ""
echo "=== Installation Complete ==="
echo "Service: systemctl status ${SERVICE_NAME}"
echo "Logs:    journalctl -u ${SERVICE_NAME} -f"
echo "Config:  ~/.cloudnest/agent.json"
`
}

// serveAgentBinary serves pre-compiled agent binaries.
// Binaries should be placed in <executable-dir>/data/binaries/cloudnest-agent-{os}-{arch}
func serveAgentBinary(c *gin.Context) {
	osName := c.Param("os")
	arch := c.Param("arch")

	// Whitelist validation to prevent path traversal
	allowedOS := map[string]bool{"linux": true}
	allowedArch := map[string]bool{"amd64": true, "arm64": true}
	if !allowedOS[osName] || !allowedArch[arch] {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "unsupported platform",
			"hint":  "supported: linux/amd64, linux/arm64",
		})
		return
	}

	binaryPath, err := resolveAgentBinaryPath(osName, arch)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent binary not found for " + osName + "/" + arch,
			"hint":  "supported: linux/amd64, linux/arm64",
		})
		return
	}

	c.FileAttachment(binaryPath, "cloudnest-agent")
}

func resolveAgentBinaryPath(osName, arch string) (string, error) {
	executablePath, err := executablePathFunc()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	binaryPath := filepath.Join(filepath.Dir(executablePath), "data", "binaries", "cloudnest-agent-"+osName+"-"+arch)
	if _, err := os.Stat(binaryPath); err != nil {
		return "", err
	}
	return binaryPath, nil
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func dashboardWS(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	hub := ws.GetDashboardHub()
	sc := hub.Register(conn)
	defer hub.Unregister(sc)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
