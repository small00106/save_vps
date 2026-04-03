package ws

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// DashboardHub manages frontend WebSocket connections for real-time updates.
type DashboardHub struct {
	mu      sync.RWMutex
	clients map[*SafeConn]bool
}

var dashboardHub = &DashboardHub{
	clients: make(map[*SafeConn]bool),
}

func GetDashboardHub() *DashboardHub {
	return dashboardHub
}

func (dh *DashboardHub) Register(conn *websocket.Conn) *SafeConn {
	sc := NewSafeConn(conn)
	dh.mu.Lock()
	defer dh.mu.Unlock()
	dh.clients[sc] = true
	return sc
}

func (dh *DashboardHub) Unregister(sc *SafeConn) {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	if dh.clients[sc] {
		delete(dh.clients, sc)
		sc.Close()
	}
}

// Broadcast sends a message to all connected dashboard clients.
func (dh *DashboardHub) Broadcast(msg interface{}) {
	dh.mu.RLock()
	clients := make([]*SafeConn, 0, len(dh.clients))
	for sc := range dh.clients {
		clients = append(clients, sc)
	}
	dh.mu.RUnlock()

	for _, sc := range clients {
		if err := sc.WriteJSON(msg); err != nil {
			log.Printf("[Dashboard] Failed to send: %v", err)
			dh.Unregister(sc)
		}
	}
}
