package agent

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HeartbeatParams is the params for agent.heartbeat.
type HeartbeatParams struct {
	UUID         string  `json:"uuid"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemPercent   float64 `json:"mem_percent"`
	SwapUsed     int64   `json:"swap_used"`
	SwapTotal    int64   `json:"swap_total"`
	DiskTotal    int64   `json:"disk_total"`
	DiskUsed     int64   `json:"disk_used"`
	DiskPercent  float64 `json:"disk_percent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
	NetInSpeed   int64   `json:"net_in_speed"`
	NetOutSpeed  int64   `json:"net_out_speed"`
	NetInTotal   int64   `json:"net_in_total"`
	NetOutTotal  int64   `json:"net_out_total"`
	TCPConns     int     `json:"tcp_conns"`
	UDPConns     int     `json:"udp_conns"`
	ProcessCount int     `json:"process_count"`
	Uptime       int64   `json:"uptime"`
}

// FileEntry represents a single file/dir in the agent's file tree.
type FileEntry struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
}

// FileTreeParams is the params for agent.fileTree.
type FileTreeParams struct {
	UUID    string      `json:"uuid"`
	Full    bool        `json:"full"` // true = full scan, false = incremental
	Entries []FileEntry `json:"entries,omitempty"`
	Added   []FileEntry `json:"added,omitempty"`
	Removed []string    `json:"removed,omitempty"`
}

// WebSocketHandler handles GET /api/agent/ws
func WebSocketHandler(c *gin.Context) {
	// Authenticate agent token
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")

	var node models.Node
	if err := dbcore.DB().Where("token = ?", token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed for %s: %v", node.UUID, err)
		return
	}

	safeConn := ws.NewSafeConn(conn)
	hub := ws.GetHub()

	hub.Register(node.UUID, &ws.AgentInfo{
		Conn:     safeConn,
		UUID:     node.UUID,
		Hostname: node.Hostname,
		JoinedAt: time.Now(),
	})

	// Update node status
	dbcore.DB().Model(&node).Updates(map[string]interface{}{
		"status":    "online",
		"last_seen": time.Now(),
	})

	// Notify dashboard clients
	ws.GetDashboardHub().Broadcast(gin.H{
		"type":   "status",
		"node":   node.UUID,
		"status": "online",
	})

	// Push all enabled ping tasks to this agent
	go pushPingTasks(safeConn)

	defer func() {
		// If this connection has already been replaced by a newer one, do not
		// unregister or mark node offline.
		if !hub.UnregisterIfCurrent(node.UUID, safeConn) {
			return
		}
		dbcore.DB().Model(&node).Update("status", "offline")
		ws.GetDashboardHub().Broadcast(gin.H{
			"type":   "status",
			"node":   node.UUID,
			"status": "offline",
		})
	}()

	// Read loop
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] Read error from %s: %v", node.UUID, err)
			break
		}

		var msg ws.RPCMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("[WS] Invalid message from %s: %v", node.UUID, err)
			continue
		}

		handleAgentMessage(node.UUID, &msg)
	}
}

func handleAgentMessage(uuid string, msg *ws.RPCMessage) {
	switch msg.Method {
	case "agent.heartbeat":
		handleHeartbeat(uuid, msg.Params)
	case "agent.fileTree":
		handleFileTree(uuid, msg.Params)
	case "agent.fileStored":
		handleFileStored(uuid, msg.Params)
	case "agent.fileDeleted":
		handleFileDeleted(uuid, msg.Params)
	case "agent.pingResult":
		handlePingResult(uuid, msg.Params)
	case "agent.commandResult":
		handleCommandResult(uuid, msg.Params)
	case "agent.verifyResult":
		handleVerifyResult(uuid, msg.Params)
	case "agent.replicateResult":
		handleReplicateResult(uuid, msg.Params)
	default:
		log.Printf("[WS] Unknown method from %s: %s", uuid, msg.Method)
	}
}

func handleHeartbeat(connUUID string, params json.RawMessage) {
	var hb HeartbeatParams
	if err := json.Unmarshal(params, &hb); err != nil {
		log.Printf("[Heartbeat] Parse error from %s: %v", connUUID, err)
		return
	}

	// Always use the authenticated UUID from the WS connection, not from params
	uuid := connUUID

	// Update node last_seen and disk info
	dbcore.DB().Model(&models.Node{}).Where("uuid = ?", uuid).Updates(map[string]interface{}{
		"last_seen":  time.Now(),
		"status":     "online",
		"disk_total": hb.DiskTotal,
		"disk_used":  hb.DiskUsed,
	})

	// Cache the metric for batch DB insert
	metric := models.NodeMetric{
		NodeUUID:     uuid,
		CPUPercent:   hb.CPUPercent,
		MemPercent:   hb.MemPercent,
		SwapUsed:     hb.SwapUsed,
		SwapTotal:    hb.SwapTotal,
		DiskPercent:  hb.DiskPercent,
		Load1:        hb.Load1,
		Load5:        hb.Load5,
		Load15:       hb.Load15,
		NetInSpeed:   hb.NetInSpeed,
		NetOutSpeed:  hb.NetOutSpeed,
		NetInTotal:   hb.NetInTotal,
		NetOutTotal:  hb.NetOutTotal,
		TCPConns:     hb.TCPConns,
		UDPConns:     hb.UDPConns,
		ProcessCount: hb.ProcessCount,
		Uptime:       hb.Uptime,
		Timestamp:    time.Now(),
	}

	// Store in buffer for batch flush
	cache.PushMetric(&metric)

	// Also keep latest metric for real-time dashboard queries
	cache.FileTreeCache.Set("metric:"+uuid, &metric, 0)

	// Push to dashboard WebSocket clients
	ws.GetDashboardHub().Broadcast(gin.H{
		"type": "heartbeat",
		"node": uuid,
		"data": hb,
	})
}

func handleFileTree(uuid string, params json.RawMessage) {
	var ft FileTreeParams
	if err := json.Unmarshal(params, &ft); err != nil {
		log.Printf("[FileTree] Parse error from %s: %v", uuid, err)
		return
	}

	if ft.Full {
		// Full replacement
		cache.FileTreeCache.Set("filetree:"+uuid, ft.Entries, 0)
	} else {
		// Incremental update
		existing, found := cache.FileTreeCache.Get("filetree:" + uuid)
		if !found {
			// No existing tree, treat as full
			cache.FileTreeCache.Set("filetree:"+uuid, ft.Added, 0)
			return
		}

		entries := existing.([]FileEntry)

		// Build set of paths to remove (explicitly removed + modified ones in Added)
		removeSet := make(map[string]bool, len(ft.Removed)+len(ft.Added))
		for _, path := range ft.Removed {
			removeSet[path] = true
		}
		for _, e := range ft.Added {
			removeSet[e.Path] = true
		}
		filtered := make([]FileEntry, 0, len(entries))
		for _, e := range entries {
			if !removeSet[e.Path] {
				filtered = append(filtered, e)
			}
		}

		// Add new/modified entries
		filtered = append(filtered, ft.Added...)
		cache.FileTreeCache.Set("filetree:"+uuid, filtered, 0)
	}

	log.Printf("[FileTree] Updated for %s (full=%v)", uuid, ft.Full)
}

func handleFileStored(uuid string, params json.RawMessage) {
	var data struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}

	dbcore.DB().Model(&models.FileReplica{}).
		Where("file_id = ? AND node_uuid = ?", data.FileID, uuid).
		Update("status", "stored")

	// Check if all replicas are stored
	var pendingCount int64
	dbcore.DB().Model(&models.FileReplica{}).
		Where("file_id = ? AND status != ?", data.FileID, "stored").
		Count(&pendingCount)

	if pendingCount == 0 {
		dbcore.DB().Model(&models.File{}).
			Where("file_id = ?", data.FileID).
			Update("status", "ready")
	}

	log.Printf("[FileStored] %s on node %s", data.FileID, uuid)
}

func handleFileDeleted(uuid string, params json.RawMessage) {
	var data struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}

	dbcore.DB().Where("file_id = ? AND node_uuid = ?", data.FileID, uuid).
		Delete(&models.FileReplica{})

	log.Printf("[FileDeleted] %s from node %s", data.FileID, uuid)
}

func handlePingResult(uuid string, params json.RawMessage) {
	var result models.PingResult
	if err := json.Unmarshal(params, &result); err != nil {
		return
	}
	result.NodeUUID = uuid
	result.Timestamp = time.Now()
	dbcore.DB().Create(&result)
}

func handleCommandResult(uuid string, params json.RawMessage) {
	var data struct {
		TaskID   uint   `json:"task_id"`
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}

	dbcore.DB().Model(&models.CommandTask{}).Where("id = ?", data.TaskID).Updates(map[string]interface{}{
		"output":    data.Output,
		"exit_code": data.ExitCode,
		"status":    "done",
	})
}

func handleVerifyResult(uuid string, params json.RawMessage) {
	var data struct {
		FileID   string `json:"file_id"`
		Checksum string `json:"checksum"`
		Match    bool   `json:"match"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}

	if data.Match {
		dbcore.DB().Model(&models.FileReplica{}).
			Where("file_id = ? AND node_uuid = ?", data.FileID, uuid).
			Updates(map[string]interface{}{"status": "verified", "verified_at": time.Now()})
	} else {
		dbcore.DB().Model(&models.FileReplica{}).
			Where("file_id = ? AND node_uuid = ?", data.FileID, uuid).
			Update("status", "lost")
	}

	if data.Checksum != "" {
		dbcore.DB().Model(&models.File{}).Where("file_id = ? AND checksum = ?", data.FileID, "").
			Update("checksum", data.Checksum)
	}

	log.Printf("[Verify] %s on %s: match=%v", data.FileID, uuid, data.Match)
}

func handleReplicateResult(uuid string, params json.RawMessage) {
	var data struct {
		FileID  string `json:"file_id"`
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}

	if !data.Success {
		log.Printf("[Replicate] Failed for %s on %s: %s", data.FileID, uuid, data.Error)
	}
}

func pushPingTasks(conn *ws.SafeConn) {
	var tasks []models.PingTask
	dbcore.DB().Where("enabled = ?", true).Find(&tasks)
	for _, task := range tasks {
		params, _ := json.Marshal(task)
		conn.WriteJSON(&ws.RPCMessage{
			JSONRPC: "2.0",
			Method:  "master.startPing",
			Params:  params,
		})
	}
}
