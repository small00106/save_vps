package terminal

import (
	"log"
	"net/http"

	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleTerminal proxies WebSocket between browser and agent for terminal access.
func HandleTerminal(c *gin.Context) {
	nodeUUID := c.Param("uuid")

	hub := ws.GetHub()
	agentInfo := hub.Get(nodeUUID)
	if agentInfo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not connected"})
		return
	}

	browserConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[Terminal] Browser upgrade failed: %v", err)
		return
	}

	done := make(chan struct{})

	// Browser → Agent
	go func() {
		defer close(done)
		for {
			msgType, msg, err := browserConn.ReadMessage()
			if err != nil {
				return
			}
			if err := agentInfo.Conn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()

	// Wait until browser disconnects
	<-done
	browserConn.Close()
}
