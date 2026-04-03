package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudnest/cloudnest-agent/internal/reporter"
	agentServer "github.com/cloudnest/cloudnest-agent/internal/server"
	"github.com/cloudnest/cloudnest-agent/internal/ws"
)

// currentClient holds the active WS client so the HTTP server can use it.
var (
	clientMu      sync.Mutex
	currentClient *ws.Client
	currentUUID   string
)

func setCurrentClient(c *ws.Client) {
	clientMu.Lock()
	currentClient = c
	clientMu.Unlock()
}

func getCurrentClient() *ws.Client {
	clientMu.Lock()
	defer clientMu.Unlock()
	return currentClient
}

func Run(cfg *Config) error {
	log.Printf("Starting CloudNest Agent (UUID: %s)", cfg.UUID)
	currentUUID = cfg.UUID

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wire up the file-stored callback so HTTP upload handler can notify master
	agentServer.OnFileStored = func(fileID string) {
		if c := getCurrentClient(); c != nil {
			c.Send("agent.fileStored", map[string]interface{}{
				"file_id": fileID,
			})
		}
	}

	// Start data plane HTTP server
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
		log.Printf("Data plane HTTP server on %s", addr)
		if err := agentServer.Start(addr, cfg.RateLimit); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// WebSocket connection loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			client := ws.NewClient(cfg.MasterURL, cfg.Token)
			if err := client.Connect(); err != nil {
				log.Printf("WS connect error: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			setCurrentClient(client)

			heartbeatStop := make(chan struct{})
			fileTreeStop := make(chan struct{})

			go heartbeatLoop(client, cfg, heartbeatStop)
			go fileTreeLoop(client, cfg, fileTreeStop)

			// Track active ping tasks so they stop on disconnect
			pingCancel := make(chan struct{})

			client.OnMessage = func(msg *ws.RPCMessage) {
				handleMasterCommand(client, cfg, msg, pingCancel)
			}

			if err := client.ReadLoop(); err != nil {
				log.Printf("WS disconnected: %v, reconnecting...", err)
			}

			setCurrentClient(nil)
			close(heartbeatStop)
			close(fileTreeStop)
			close(pingCancel) // stops all running ping goroutines
			client.Close()
		}
	}()

	<-sigCh
	fmt.Println()
	log.Println("Agent shutting down...")
	cancel()
	return nil
}

func heartbeatLoop(client *ws.Client, cfg *Config, stop chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	sendHeartbeat(client, cfg)

	for {
		select {
		case <-ticker.C:
			sendHeartbeat(client, cfg)
		case <-stop:
			return
		}
	}
}

func sendHeartbeat(client *ws.Client, cfg *Config) {
	metrics := reporter.Collect()
	params := map[string]interface{}{
		"uuid":          cfg.UUID,
		"cpu_percent":   metrics.CPUPercent,
		"mem_percent":   metrics.MemPercent,
		"swap_used":     metrics.SwapUsed,
		"swap_total":    metrics.SwapTotal,
		"disk_total":    metrics.DiskTotal,
		"disk_used":     metrics.DiskUsed,
		"disk_percent":  metrics.DiskPercent,
		"load1":         metrics.Load1,
		"load5":         metrics.Load5,
		"load15":        metrics.Load15,
		"net_in_speed":  metrics.NetInSpeed,
		"net_out_speed": metrics.NetOutSpeed,
		"net_in_total":  metrics.NetInTotal,
		"net_out_total": metrics.NetOutTotal,
		"tcp_conns":     metrics.TCPConns,
		"udp_conns":     metrics.UDPConns,
		"process_count": metrics.ProcessCount,
		"uptime":        metrics.Uptime,
	}

	if err := client.Send("agent.heartbeat", params); err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	}
}

func fileTreeLoop(client *ws.Client, cfg *Config, stop chan struct{}) {
	// First scan: full
	prevEntries := reporter.ScanDirectories(cfg.ScanDirs)
	client.Send("agent.fileTree", map[string]interface{}{
		"uuid":    cfg.UUID,
		"full":    true,
		"entries": prevEntries,
	})

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currEntries := reporter.ScanDirectories(cfg.ScanDirs)
			diff := reporter.DiffFileTrees(prevEntries, currEntries)

			if len(diff.Added) == 0 && len(diff.Removed) == 0 {
				// No changes, skip
				prevEntries = currEntries
				continue
			}

			client.Send("agent.fileTree", map[string]interface{}{
				"uuid":    cfg.UUID,
				"full":    false,
				"added":   diff.Added,
				"removed": diff.Removed,
			})
			prevEntries = currEntries
		case <-stop:
			return
		}
	}
}

func handleMasterCommand(client *ws.Client, cfg *Config, msg *ws.RPCMessage, pingCancel chan struct{}) {
	switch msg.Method {
	case "master.execCommand":
		go executeCommand(client, msg.Params)
	case "master.deleteFile":
		go deleteFile(client, msg.Params)
	case "master.startPing":
		go executePing(client, msg.Params, pingCancel)
	case "master.replicateFile":
		go replicateFile(client, msg.Params)
	case "master.verifyFile":
		go verifyFile(client, msg.Params)
	default:
		log.Printf("[WS] Unknown command: %s", msg.Method)
	}
}

// === Command Execution ===

func executeCommand(client *ws.Client, params json.RawMessage) {
	var data struct {
		TaskID  uint   `json:"task_id"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		log.Printf("[CMD] Parse error: %v", err)
		return
	}

	log.Printf("[CMD] Executing: %s", data.Command)

	shell := "/bin/sh"
	flag := "-c"
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/c"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, flag, data.Command)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	client.Send("agent.commandResult", map[string]interface{}{
		"task_id":   data.TaskID,
		"output":    string(output),
		"exit_code": exitCode,
	})

	log.Printf("[CMD] Done (exit=%d): %s", exitCode, data.Command)
}

// === File Delete ===

func deleteFile(client *ws.Client, params json.RawMessage) {
	var data struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}
	if len(data.FileID) < 2 {
		log.Printf("[FILE] Delete error: invalid file_id %q", data.FileID)
		return
	}

	path := fmt.Sprintf("/data/files/%s/%s", data.FileID[:2], data.FileID)
	if err := os.Remove(path); err != nil {
		log.Printf("[FILE] Delete error: %v", err)
		return
	}

	client.Send("agent.fileDeleted", map[string]interface{}{
		"file_id": data.FileID,
	})
	log.Printf("[FILE] Deleted: %s", data.FileID)
}

// === Ping Execution ===

func executePing(client *ws.Client, params json.RawMessage, stop chan struct{}) {
	var task struct {
		ID       uint   `json:"id"`
		Type     string `json:"type"`
		Target   string `json:"target"`
		Interval int    `json:"interval"`
	}
	if err := json.Unmarshal(params, &task); err != nil {
		log.Printf("[PING] Parse error: %v", err)
		return
	}

	if task.Interval < 5 {
		task.Interval = 60
	}

	log.Printf("[PING] Starting: %s %s every %ds", task.Type, task.Target, task.Interval)

	ticker := time.NewTicker(time.Duration(task.Interval) * time.Second)
	defer ticker.Stop()

	// Run once immediately, then on ticker
	doPing(client, &task)
	for {
		select {
		case <-ticker.C:
			doPing(client, &task)
		case <-stop:
			log.Printf("[PING] Stopped: %s", task.Target)
			return
		}
	}
}

func doPing(client *ws.Client, task *struct {
	ID       uint   `json:"id"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	Interval int    `json:"interval"`
}) {
	start := time.Now()
	success := false
	var latency float64

	switch task.Type {
	case "tcp":
		conn, err := net.DialTimeout("tcp", task.Target, 5*time.Second)
		if err == nil {
			conn.Close()
			success = true
			latency = float64(time.Since(start).Milliseconds())
		}
	case "http":
		target := task.Target
		if !strings.HasPrefix(target, "http") {
			target = "http://" + target
		}
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(target)
		if err == nil {
			resp.Body.Close()
			success = true
			latency = float64(time.Since(start).Milliseconds())
		}
	default: // icmp fallback to TCP port 80
		conn, err := net.DialTimeout("tcp", task.Target+":80", 5*time.Second)
		if err == nil {
			conn.Close()
			success = true
			latency = float64(time.Since(start).Milliseconds())
		}
	}

	client.Send("agent.pingResult", map[string]interface{}{
		"task_id":   task.ID,
		"node_uuid": currentUUID,
		"latency":   latency,
		"success":   success,
	})
}

// === File Replication ===

func replicateFile(client *ws.Client, params json.RawMessage) {
	var data struct {
		FileID    string `json:"file_id"`
		SourceURL string `json:"source_url"` // signed URL to download from source agent
	}
	if err := json.Unmarshal(params, &data); err != nil {
		log.Printf("[FILE] Replicate parse error: %v", err)
		return
	}

	log.Printf("[FILE] Replicating %s from %s", data.FileID, data.SourceURL)

	if len(data.FileID) < 2 {
		log.Printf("[FILE] Replicate error: invalid file_id %q", data.FileID)
		return
	}

	// Download from source agent
	resp, err := http.Get(data.SourceURL)
	if err != nil {
		log.Printf("[FILE] Replicate download error: %v", err)
		client.Send("agent.replicateResult", map[string]interface{}{
			"file_id": data.FileID,
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[FILE] Replicate download status: %d", resp.StatusCode)
		client.Send("agent.replicateResult", map[string]interface{}{
			"file_id": data.FileID,
			"success": false,
			"error":   fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		return
	}

	// Save locally
	dir := fmt.Sprintf("/data/files/%s", data.FileID[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[FILE] Replicate mkdir error: %v", err)
		return
	}

	filePath := fmt.Sprintf("%s/%s", dir, data.FileID)
	f, err := os.Create(filePath)
	if err != nil {
		log.Printf("[FILE] Replicate create error: %v", err)
		return
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(filePath)
		log.Printf("[FILE] Replicate write error: %v", err)
		return
	}
	f.Close()

	// Notify master
	client.Send("agent.fileStored", map[string]interface{}{
		"file_id": data.FileID,
	})
	log.Printf("[FILE] Replicated: %s", data.FileID)
}

// === File Verification ===

func verifyFile(client *ws.Client, params json.RawMessage) {
	var data struct {
		FileID           string `json:"file_id"`
		ExpectedChecksum string `json:"expected_checksum"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		log.Printf("[FILE] Verify parse error: %v", err)
		return
	}
	if len(data.FileID) < 2 {
		log.Printf("[FILE] Verify error: invalid file_id %q", data.FileID)
		return
	}

	filePath := fmt.Sprintf("/data/files/%s/%s", data.FileID[:2], data.FileID)
	f, err := os.Open(filePath)
	if err != nil {
		client.Send("agent.verifyResult", map[string]interface{}{
			"file_id": data.FileID,
			"match":   false,
			"error":   "file not found",
		})
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		client.Send("agent.verifyResult", map[string]interface{}{
			"file_id": data.FileID,
			"match":   false,
			"error":   err.Error(),
		})
		return
	}

	checksum := hex.EncodeToString(h.Sum(nil))
	match := data.ExpectedChecksum == "" || checksum == data.ExpectedChecksum

	client.Send("agent.verifyResult", map[string]interface{}{
		"file_id":  data.FileID,
		"checksum": checksum,
		"match":    match,
	})
	log.Printf("[FILE] Verified %s: checksum=%s match=%v", data.FileID, checksum[:12], match)
}
