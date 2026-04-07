package terminal

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var terminalTestDBOnce sync.Once

func initTerminalTestDB(t *testing.T) {
	t.Helper()

	terminalTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)

		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-terminal-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	db := dbcore.DB()
	if err := db.Exec("DELETE FROM nodes").Error; err != nil {
		t.Fatalf("clear nodes: %v", err)
	}
}

func seedTerminalNode(t *testing.T, uuid, ip string, port int) {
	t.Helper()

	if err := dbcore.DB().Create(&models.Node{
		UUID:     uuid,
		Token:    uuid + "-token",
		Hostname: uuid,
		IP:       ip,
		Port:     port,
		Status:   "online",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
}

func TestHandleTerminalForwardsInitialSizeToAgent(t *testing.T) {
	initTerminalTestDB(t)

	var capturedCols string
	var capturedRows string
	var capturedMessage string
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCols = r.URL.Query().Get("cols")
		capturedRows = r.URL.Query().Get("rows")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade agent ws: %v", err)
		}
		defer conn.Close()

		if err := conn.WriteMessage(websocket.TextMessage, []byte("agent-ready")); err != nil {
			t.Fatalf("write ready: %v", err)
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read proxied input: %v", err)
		}
		capturedMessage = string(msg)
	}))
	defer agentServer.Close()

	u, err := neturl.Parse(agentServer.URL)
	if err != nil {
		t.Fatalf("parse agent server url: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	seedTerminalNode(t, "node-1", host, port)

	router := gin.New()
	router.GET("/ws/terminal/:uuid", HandleTerminal)
	masterServer := httptest.NewServer(router)
	defer masterServer.Close()

	wsURL := "ws" + strings.TrimPrefix(masterServer.URL, "http") + "/ws/terminal/node-1?cols=120&rows=40"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial browser ws: %v", err)
	}
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read agent output: %v", err)
	}
	if string(msg) != "agent-ready" {
		t.Fatalf("expected agent-ready, got %q", string(msg))
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"input","data":"pwd\n"}`)); err != nil {
		t.Fatalf("write browser input: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && capturedMessage == "" {
		time.Sleep(10 * time.Millisecond)
	}

	if capturedCols != "120" || capturedRows != "40" {
		t.Fatalf("expected forwarded cols/rows 120x40, got cols=%q rows=%q", capturedCols, capturedRows)
	}
	if capturedMessage != `{"type":"input","data":"pwd\n"}` {
		t.Fatalf("expected proxied browser control message, got %q", capturedMessage)
	}
}

func TestHandleTerminalShowsReadableErrorWhenAgentDialFails(t *testing.T) {
	initTerminalTestDB(t)
	seedTerminalNode(t, "node-unreachable", "127.0.0.1", 1)

	router := gin.New()
	router.GET("/ws/terminal/:uuid", HandleTerminal)
	masterServer := httptest.NewServer(router)
	defer masterServer.Close()

	wsURL := "ws" + strings.TrimPrefix(masterServer.URL, "http") + "/ws/terminal/node-unreachable?cols=100&rows=30"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		status := ""
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("expected browser websocket upgrade, got err=%v status=%s", err, status)
	}
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected readable error message before close, got %v", err)
	}
	if !strings.Contains(strings.ToLower(string(msg)), "failed to connect to agent terminal") {
		t.Fatalf("expected connection failure message, got %q", string(msg))
	}
}
