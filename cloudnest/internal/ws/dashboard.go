package ws

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// DashboardHub manages frontend WebSocket connections for real-time updates.
type DashboardHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

var dashboardHub = &DashboardHub{
	clients: make(map[*websocket.Conn]bool),
}

func GetDashboardHub() *DashboardHub {
	return dashboardHub
}

func (dh *DashboardHub) Register(conn *websocket.Conn) {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	dh.clients[conn] = true
}

func (dh *DashboardHub) Unregister(conn *websocket.Conn) {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	delete(dh.clients, conn)
	conn.Close()
}

// Broadcast sends a message to all connected dashboard clients.
func (dh *DashboardHub) Broadcast(msg interface{}) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	for conn := range dh.clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("[Dashboard] Failed to send: %v", err)
			go dh.Unregister(conn)
		}
	}
}
