package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/cloudnest/cloudnest-agent/internal/reporter"
	agentServer "github.com/cloudnest/cloudnest-agent/internal/server"
	"github.com/cloudnest/cloudnest-agent/internal/ws"
)

// currentClient holds the active WS client so the HTTP server can use it.
var currentClient *ws.Client
var currentUUID string

func Run(cfg *Config) error {
	log.Printf("Starting CloudNest Agent (UUID: %s)", cfg.UUID)
	currentUUID = cfg.UUID

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wire up the file-stored callback so HTTP upload handler can notify master
	agentServer.OnFileStored = func(fileID string) {
		if currentClient != nil {
			currentClient.Send("agent.fileStored", map[string]interface{}{
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
			currentClient = client

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

			currentClient = nil
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
	entries := reporter.ScanDirectories(cfg.ScanDirs)
	client.Send("agent.fileTree", map[string]interface{}{
		"uuid":    cfg.UUID,
		"full":    true,
		"entries": entries,
	})

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			entries := reporter.ScanDirectories(cfg.ScanDirs)
			client.Send("agent.fileTree", map[string]interface{}{
				"uuid":    cfg.UUID,
				"full":    true,
				"entries": entries,
			})
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
		log.Printf("[FILE] Received replication request (not yet implemented)")
	case "master.verifyFile":
		log.Printf("[FILE] Received verify request (not yet implemented)")
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
