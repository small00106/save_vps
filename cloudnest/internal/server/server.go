package server

import (
	"io/fs"
	"net/http"
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
	masterURL := "http://" + c.Request.Host
	script := generateInstallScript(masterURL)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(script))
}

func generateInstallScript(masterURL string) string {
	return `#!/bin/bash
set -e

# CloudNest Agent One-Click Installer
# Usage: curl -sSL ` + masterURL + `/install.sh | bash -s -- --token <registration_token> --secret <signing_secret>

MASTER_URL="` + masterURL + `"
INSTALL_DIR="/opt/cloudnest-agent"
SERVICE_NAME="cloudnest-agent"
REG_TOKEN=""
SIGNING_SECRET=""
PORT=8801
SCAN_DIRS="/data"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --token) REG_TOKEN="$2"; shift 2 ;;
        --secret) SIGNING_SECRET="$2"; shift 2 ;;
        --port) PORT="$2"; shift 2 ;;
        --scan-dirs) SCAN_DIRS="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [ -z "$REG_TOKEN" ]; then
    echo "Error: --token is required"
    echo "Usage: curl -sSL ${MASTER_URL}/install.sh | bash -s -- --token <token> --secret <secret>"
    exit 1
fi

if [ -z "$SIGNING_SECRET" ]; then
    echo "Error: --secret is required (the CLOUDNEST_SIGNING_SECRET value from your master)"
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

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$SCAN_DIRS"

# Download agent binary
echo "Downloading agent binary..."
curl -sSL -o "${INSTALL_DIR}/cloudnest-agent" "${MASTER_URL}/download/agent/${OS}/${ARCH}"
chmod +x "${INSTALL_DIR}/cloudnest-agent"

# Register with master
echo "Registering agent..."
"${INSTALL_DIR}/cloudnest-agent" register \
    --master "$MASTER_URL" \
    --token "$REG_TOKEN" \
    --port "$PORT" \
    --scan-dirs "$SCAN_DIRS"

# Create systemd service
echo "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service <<SERVICEEOF
[Unit]
Description=CloudNest Agent
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/cloudnest-agent run
Restart=always
RestartSec=5
LimitNOFILE=65535
Environment=CLOUDNEST_SIGNING_SECRET=${SIGNING_SECRET}

[Install]
WantedBy=multi-user.target
SERVICEEOF

# Enable and start
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl start "$SERVICE_NAME"

echo ""
echo "=== Installation Complete ==="
echo "Service: systemctl status ${SERVICE_NAME}"
echo "Logs:    journalctl -u ${SERVICE_NAME} -f"
echo "Config:  ~/.cloudnest/agent.json"
`
}

// serveAgentBinary serves pre-compiled agent binaries.
// Binaries should be placed in ./data/binaries/cloudnest-agent-{os}-{arch}
func serveAgentBinary(c *gin.Context) {
	osName := c.Param("os")
	arch := c.Param("arch")

	// Look for binary in data directory
	binaryName := "cloudnest-agent-" + osName + "-" + arch
	binaryPath := "./data/binaries/" + binaryName

	c.FileAttachment(binaryPath, "cloudnest-agent")
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
