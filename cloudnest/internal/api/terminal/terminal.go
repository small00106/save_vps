package terminal

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/transfer"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleTerminal proxies browser WebSocket to Agent's /api/terminal endpoint.
func HandleTerminal(c *gin.Context) {
	nodeUUID := c.Param("uuid")

	var node models.Node
	if err := dbcore.DB().Where("uuid = ? AND status = ?", nodeUUID, "online").First(&node).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not connected"})
		return
	}

	// Upgrade browser connection
	browserConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[Terminal] Browser upgrade failed: %v", err)
		return
	}
	defer browserConn.Close()

	agentConn, err := dialAgentTerminal(&node, nodeUUID, c.Request.URL.Query())
	if err != nil {
		log.Printf("[Terminal] Failed to connect to agent %s: %v", nodeUUID, err)
		_ = browserConn.WriteMessage(websocket.TextMessage, []byte("[cloudnest] failed to connect to agent terminal\r\n"))
		_ = browserConn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "failed to connect to agent terminal"),
			time.Now().Add(5*time.Second),
		)
		return
	}
	defer agentConn.Close()

	done := make(chan struct{})

	// Browser → Agent
	go func() {
		defer func() { close(done) }()
		for {
			msgType, msg, err := browserConn.ReadMessage()
			if err != nil {
				_ = agentConn.Close()
				return
			}
			if err := agentConn.WriteMessage(msgType, msg); err != nil {
				_ = browserConn.Close()
				return
			}
		}
	}()

	// Agent → Browser
	go func() {
		for {
			msgType, msg, err := agentConn.ReadMessage()
			if err != nil {
				_ = browserConn.Close()
				return
			}
			if err := browserConn.WriteMessage(msgType, msg); err != nil {
				_ = agentConn.Close()
				return
			}
		}
	}()

	<-done
}

func dialAgentTerminal(node *models.Node, nodeUUID string, browserQuery url.Values) (*websocket.Conn, error) {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 15 * time.Second
	signedURL := buildAgentTerminalURL(node, nodeUUID, browserQuery)
	conn, _, err := dialer.Dial(signedURL, nil)
	return conn, err
}

func buildAgentTerminalURL(node *models.Node, nodeUUID string, browserQuery url.Values) string {
	terminalID := "terminal:" + nodeUUID
	baseURL := fmt.Sprintf("ws://%s:%d/api/terminal", node.IP, node.Port)
	signedURL := transfer.GenerateSignedURL(baseURL, terminalID, http.MethodGet, 5*time.Minute)

	values := url.Values{}
	values.Set("id", terminalID)
	forwardPositiveTerminalParam(values, browserQuery, "cols")
	forwardPositiveTerminalParam(values, browserQuery, "rows")

	return signedURL + "&" + values.Encode()
}

func forwardPositiveTerminalParam(dst, src url.Values, key string) {
	raw := strings.TrimSpace(src.Get(key))
	if raw == "" {
		return
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return
	}
	dst.Set(key, strconv.Itoa(value))
}
