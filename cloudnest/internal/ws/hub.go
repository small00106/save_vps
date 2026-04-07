package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// RPCMessage represents a JSON-RPC 2.0 message.
type RPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	ID      string          `json:"id,omitempty"`
}

// AgentInfo holds connection and metadata for a connected agent.
type AgentInfo struct {
	Conn     *SafeConn
	UUID     string
	Hostname string
	JoinedAt time.Time
}

// Hub manages all connected agent WebSocket connections.
type Hub struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo // keyed by UUID
}

var defaultHub = &Hub{
	agents: make(map[string]*AgentInfo),
}

func GetHub() *Hub {
	return defaultHub
}

func (h *Hub) Register(uuid string, info *AgentInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close existing connection if any
	if old, ok := h.agents[uuid]; ok {
		old.Conn.Close()
		log.Printf("[Hub] Replaced existing connection for agent %s", uuid)
	}

	h.agents[uuid] = info
	log.Printf("[Hub] Agent registered: %s (%s)", uuid, info.Hostname)
}

func (h *Hub) Unregister(uuid string) {
	h.UnregisterIfCurrent(uuid, nil)
}

// UnregisterIfCurrent removes an agent connection only when it still matches
// the current connection in the hub. If conn is nil, it unregisters unconditionally.
func (h *Hub) UnregisterIfCurrent(uuid string, conn *SafeConn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if info, ok := h.agents[uuid]; ok {
		if conn != nil && info.Conn != conn {
			return false
		}
		info.Conn.Close()
		delete(h.agents, uuid)
		log.Printf("[Hub] Agent unregistered: %s", uuid)
		return true
	}
	return false
}

func (h *Hub) Get(uuid string) *AgentInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.agents[uuid]
}

func (h *Hub) GetAll() map[string]*AgentInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[string]*AgentInfo, len(h.agents))
	for k, v := range h.agents {
		result[k] = v
	}
	return result
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.agents)
}

// SendToAgent sends a JSON-RPC message to a specific agent.
func (h *Hub) SendToAgent(uuid string, msg *RPCMessage) error {
	h.mu.RLock()
	info, ok := h.agents[uuid]
	h.mu.RUnlock()

	if !ok {
		return ErrAgentNotFound
	}
	return info.Conn.WriteJSON(msg)
}

// BroadcastToAgents sends a JSON-RPC message to all connected agents.
func (h *Hub) BroadcastToAgents(msg *RPCMessage) {
	agents := h.GetAll()
	for uuid, info := range agents {
		if err := info.Conn.WriteJSON(msg); err != nil {
			log.Printf("[Hub] Failed to send to agent %s: %v", uuid, err)
		}
	}
}
