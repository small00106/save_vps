package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var commandTestDBOnce sync.Once

func initCommandTestDB(t *testing.T) {
	t.Helper()

	commandTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-command-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	db := dbcore.DB()
	for _, table := range []string{"audit_logs", "command_tasks", "sessions", "users", "nodes"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("clear %s: %v", table, err)
		}
	}
	ws.GetHub().Unregister("node-1")
}

func createCommandTestUser(t *testing.T, username string) models.User {
	t.Helper()

	user := models.User{Username: username, PasswordHash: "unused"}
	if err := dbcore.DB().Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestExecWritesAuditLogWhenDispatchSucceeds(t *testing.T) {
	initCommandTestDB(t)
	user := createCommandTestUser(t, "admin")
	if err := dbcore.DB().Create(&models.Node{UUID: "node-1", Hostname: "node-1"}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	conn, received, cleanup := openAgentConn(t)
	defer cleanup()
	ws.GetHub().Register("node-1", &ws.AgentInfo{
		UUID:     "node-1",
		Hostname: "node-1",
		Conn:     ws.NewSafeConn(conn),
		JoinedAt: time.Now(),
	})
	defer ws.GetHub().Unregister("node-1")

	body, err := json.Marshal(map[string]string{"command": "uptime"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "uuid", Value: "node-1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/nodes/node-1/exec", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "203.0.113.30:4567"
	c.Set("user_id", user.ID)

	Exec(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	select {
	case msg := <-received:
		if msg.Method != "master.execCommand" {
			t.Fatalf("expected master.execCommand, got %q", msg.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected command RPC message to be delivered")
	}

	var resp struct {
		TaskID uint `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var auditLog models.AuditLog
	if err := dbcore.DB().Order("id DESC").First(&auditLog).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	if auditLog.Action != "command_exec_requested" {
		t.Fatalf("expected command_exec_requested action, got %q", auditLog.Action)
	}
	if auditLog.Actor != "admin" {
		t.Fatalf("expected actor admin, got %q", auditLog.Actor)
	}
	if auditLog.Status != "success" {
		t.Fatalf("expected success status, got %q", auditLog.Status)
	}
	if auditLog.TargetType != "command_task" {
		t.Fatalf("expected command_task target_type, got %q", auditLog.TargetType)
	}
	if auditLog.TargetID != fmt.Sprintf("%d", resp.TaskID) {
		t.Fatalf("expected target_id %d, got %q", resp.TaskID, auditLog.TargetID)
	}
	if auditLog.NodeUUID != "node-1" {
		t.Fatalf("expected node_uuid node-1, got %q", auditLog.NodeUUID)
	}
}

func TestExecWritesAuditLogWhenDispatchFails(t *testing.T) {
	initCommandTestDB(t)
	user := createCommandTestUser(t, "admin")

	body, err := json.Marshal(map[string]string{"command": "uptime"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "uuid", Value: "node-1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/nodes/node-1/exec", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "203.0.113.31:4567"
	c.Set("user_id", user.ID)

	Exec(c)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var auditLog models.AuditLog
	if err := dbcore.DB().Order("id DESC").First(&auditLog).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	if auditLog.Action != "command_exec_rejected" {
		t.Fatalf("expected command_exec_rejected action, got %q", auditLog.Action)
	}
	if auditLog.Actor != "admin" {
		t.Fatalf("expected actor admin, got %q", auditLog.Actor)
	}
	if auditLog.Status != "failed" {
		t.Fatalf("expected failed status, got %q", auditLog.Status)
	}
	if auditLog.NodeUUID != "node-1" {
		t.Fatalf("expected node_uuid node-1, got %q", auditLog.NodeUUID)
	}
}

func openAgentConn(t *testing.T) (*websocket.Conn, <-chan ws.RPCMessage, func()) {
	t.Helper()

	received := make(chan ws.RPCMessage, 1)
	done := make(chan struct{})
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err == nil {
			var msg ws.RPCMessage
			if err := json.Unmarshal(data, &msg); err == nil {
				received <- msg
			}
		}

		<-done
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}

	cleanup := func() {
		close(done)
		_ = conn.Close()
		server.Close()
	}
	return conn, received, cleanup
}
