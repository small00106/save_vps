package terminal

import (
	"fmt"
	"log"
	"net/http"
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

	// Build signed URL for agent terminal
	terminalID := "terminal:" + nodeUUID
	baseURL := fmt.Sprintf("ws://%s:%d/api/terminal", node.IP, node.Port)
	signedURL := transfer.GenerateSignedURL(baseURL, terminalID, http.MethodGet, 5*time.Minute)
	signedURL += "&id=" + terminalID

	agentConn, _, err := websocket.DefaultDialer.Dial(signedURL, nil)
	if err != nil {
		log.Printf("[Terminal] Failed to connect to agent %s: %v", nodeUUID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to agent terminal"})
		return
	}

	// Upgrade browser connection
	browserConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		agentConn.Close()
		log.Printf("[Terminal] Browser upgrade failed: %v", err)
		return
	}

	done := make(chan struct{})

	// Browser → Agent
	go func() {
		defer func() { close(done) }()
		for {
			msgType, msg, err := browserConn.ReadMessage()
			if err != nil {
				agentConn.Close()
				return
			}
			if err := agentConn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()

	// Agent → Browser
	go func() {
		for {
			msgType, msg, err := agentConn.ReadMessage()
			if err != nil {
				browserConn.Close()
				return
			}
			if err := browserConn.WriteMessage(msgType, msg); err != nil {
				agentConn.Close()
				return
			}
		}
	}()

	<-done
	agentConn.Close()
	browserConn.Close()
}
