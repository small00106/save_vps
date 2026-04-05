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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudnest/cloudnest-agent/internal/reporter"
	agentServer "github.com/cloudnest/cloudnest-agent/internal/server"
	"github.com/cloudnest/cloudnest-agent/internal/storage"
	"github.com/cloudnest/cloudnest-agent/internal/ws"
)

// currentClient holds the active WS client so the HTTP server can use it.
var (
	clientMu      sync.Mutex
	currentClient *ws.Client
	currentUUID   string
)

const fileTreeInterval = 10 * time.Second

func managedScanDirs(cfg *Config) []string {
	filesDir := storage.FilesDir()
	dirs := []string{filesDir}
	for _, d := range cfg.ScanDirs {
		if d != filesDir {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

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

	if err := storage.EnsureDataDirs(); err != nil {
		return fmt.Errorf("failed to initialize data directory: %w", err)
	}
	cfg.ScanDirs = managedScanDirs(cfg)
	log.Printf("File storage root: %s", storage.FilesDir())

	lock, err := acquireDefaultRunLock()
	if err != nil {
		return err
	}
	defer func() {
		if err := lock.Release(); err != nil {
			log.Printf("Failed to release agent lock: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wire up the file-stored callback so HTTP upload handler can notify master
	agentServer.OnFileStored = func(event agentServer.StoredFileEvent) {
		if c := getCurrentClient(); c != nil {
			c.Send("agent.fileStored", map[string]interface{}{
				"file_id":       event.FileID,
				"relative_path": event.RelativePath,
				"store_path":    event.StorePath,
			})
		}
	}

	log.Printf("Data plane HTTP server on 0.0.0.0:%d", cfg.Port)
	dataPlaneErrCh, err := startDataPlane(cfg)
	if err != nil {
		return err
	}

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

			// Track active ping tasks by task ID so they can be stopped individually.
			pingStops := make(map[uint]chan struct{})
			pingStopsMu := &sync.Mutex{}

			client.OnMessage = func(msg *ws.RPCMessage) {
				handleMasterCommand(client, cfg, msg, pingStopsMu, pingStops)
			}

			if err := client.ReadLoop(); err != nil {
				log.Printf("WS disconnected: %v, reconnecting...", err)
			}

			setCurrentClient(nil)
			close(heartbeatStop)
			close(fileTreeStop)
			stopAllPingTasks(pingStopsMu, pingStops)
			client.Close()
		}
	}()

	select {
	case <-sigCh:
		fmt.Println()
		log.Println("Agent shutting down...")
		cancel()
		return nil
	case err := <-dataPlaneErrCh:
		cancel()
		return err
	}
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

	ticker := time.NewTicker(fileTreeInterval)
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

func handleMasterCommand(client *ws.Client, cfg *Config, msg *ws.RPCMessage, pingStopsMu *sync.Mutex, pingStops map[uint]chan struct{}) {
	switch msg.Method {
	case "master.execCommand":
		go executeCommand(client, msg.Params)
	case "master.deleteFile":
		go deleteFile(client, msg.Params)
	case "master.startPing":
		var task struct {
			ID uint `json:"id"`
		}
		if err := json.Unmarshal(msg.Params, &task); err != nil || task.ID == 0 {
			log.Printf("[PING] Invalid startPing payload: %v", err)
			return
		}

		stop := make(chan struct{})
		pingStopsMu.Lock()
		if oldStop, exists := pingStops[task.ID]; exists {
			close(oldStop) // restart existing task with latest config
		}
		pingStops[task.ID] = stop
		pingStopsMu.Unlock()

		go executePing(client, msg.Params, stop, func() {
			finishPingTask(task.ID, stop, pingStopsMu, pingStops)
		})
	case "master.stopPing":
		var payload struct {
			TaskID uint `json:"task_id"`
		}
		if err := json.Unmarshal(msg.Params, &payload); err != nil || payload.TaskID == 0 {
			log.Printf("[PING] Invalid stopPing payload: %v", err)
			return
		}
		if stopped := stopPingTask(payload.TaskID, pingStopsMu, pingStops); stopped {
			log.Printf("[PING] Stop requested for task %d", payload.TaskID)
		}
	case "master.replicateFile":
		go replicateFile(client, msg.Params)
	case "master.verifyFile":
		go verifyFile(client, msg.Params)
	default:
		log.Printf("[WS] Unknown command: %s", msg.Method)
	}
}

func stopPingTask(taskID uint, pingStopsMu *sync.Mutex, pingStops map[uint]chan struct{}) bool {
	pingStopsMu.Lock()
	defer pingStopsMu.Unlock()
	stop, exists := pingStops[taskID]
	if !exists {
		return false
	}
	close(stop)
	delete(pingStops, taskID)
	return true
}

func finishPingTask(taskID uint, stop chan struct{}, pingStopsMu *sync.Mutex, pingStops map[uint]chan struct{}) {
	pingStopsMu.Lock()
	defer pingStopsMu.Unlock()
	current, exists := pingStops[taskID]
	if exists && current == stop {
		delete(pingStops, taskID)
	}
}

func stopAllPingTasks(pingStopsMu *sync.Mutex, pingStops map[uint]chan struct{}) {
	pingStopsMu.Lock()
	defer pingStopsMu.Unlock()
	for taskID, stop := range pingStops {
		close(stop)
		delete(pingStops, taskID)
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
		FileID    string `json:"file_id"`
		StorePath string `json:"store_path"`
	}
	if err := json.Unmarshal(params, &data); err != nil {
		return
	}
	if len(data.FileID) < 2 {
		log.Printf("[FILE] Delete error: invalid file_id %q", data.FileID)
		return
	}

	path := strings.TrimSpace(data.StorePath)
	if path != "" {
		path = filepath.Clean(path)
		if _, err := storage.RelativeManagedPath(path); err != nil {
			log.Printf("[FILE] Delete error: invalid managed path %q: %v", path, err)
			return
		}
	} else {
		var err error
		path, err = storage.FilePath(data.FileID)
		if err != nil {
			log.Printf("[FILE] Delete error: %v", err)
			return
		}
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			// File already gone — still confirm deletion to Master so it can clean up
			client.Send("agent.fileDeleted", map[string]interface{}{
				"file_id": data.FileID,
			})
			log.Printf("[FILE] Already gone, confirmed: %s", data.FileID)
		} else {
			log.Printf("[FILE] Delete error: %v", err)
		}
		return
	}

	client.Send("agent.fileDeleted", map[string]interface{}{
		"file_id": data.FileID,
	})
	log.Printf("[FILE] Deleted: %s", data.FileID)
}

// === Ping Execution ===

func executePing(client *ws.Client, params json.RawMessage, stop chan struct{}, onExit func()) {
	if onExit != nil {
		defer onExit()
	}

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
	case "icmp":
		latency, success = runICMP(task.Target)
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
	default:
		log.Printf("[PING] Unsupported task type: %s", task.Type)
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
	dir, err := storage.EnsureShardDir(data.FileID)
	if err != nil {
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

	filePath, err := storage.FilePath(data.FileID)
	if err != nil {
		client.Send("agent.verifyResult", map[string]interface{}{
			"file_id": data.FileID,
			"match":   false,
			"error":   "invalid file id",
		})
		return
	}
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
