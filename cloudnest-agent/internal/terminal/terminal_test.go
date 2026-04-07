package terminal

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestHandleTerminalUsesControlMessageProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var fake *fakeTerminalSession
	restore := swapNewTerminalSessionFunc(func(cols, rows int) (terminalSession, error) {
		fake = newFakeTerminalSession(cols, rows)
		return fake, nil
	})
	defer restore()

	router := gin.New()
	router.GET("/api/terminal", HandleTerminal)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/terminal?cols=120&rows=40"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial terminal ws: %v", err)
	}
	defer conn.Close()

	if fake == nil {
		t.Fatal("expected terminal session to be created")
	}
	if fake.initialCols != 120 || fake.initialRows != 40 {
		t.Fatalf("expected initial size 120x40, got %dx%d", fake.initialCols, fake.initialRows)
	}

	fake.EmitOutput("ready$ ")
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read terminal output: %v", err)
	}
	if string(msg) != "ready$ " {
		t.Fatalf("expected ready prompt, got %q", string(msg))
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"input","data":"ls\n"}`)); err != nil {
		t.Fatalf("write input control message: %v", err)
	}
	select {
	case got := <-fake.writeCh:
		if got != "ls\n" {
			t.Fatalf("expected ls input, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal input")
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":150,"rows":50}`)); err != nil {
		t.Fatalf("write resize control message: %v", err)
	}
	select {
	case size := <-fake.resizeCh:
		if size.cols != 150 || size.rows != 50 {
			t.Fatalf("expected resize to 150x50, got %dx%d", size.cols, size.rows)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal resize")
	}
}

type fakeTerminalSession struct {
	initialCols int
	initialRows int
	reader      *io.PipeReader
	writer      *io.PipeWriter
	writeCh     chan string
	resizeCh    chan fakeResize
}

type fakeResize struct {
	cols int
	rows int
}

func newFakeTerminalSession(cols, rows int) *fakeTerminalSession {
	reader, writer := io.Pipe()
	return &fakeTerminalSession{
		initialCols: cols,
		initialRows: rows,
		reader:      reader,
		writer:      writer,
		writeCh:     make(chan string, 4),
		resizeCh:    make(chan fakeResize, 4),
	}
}

func (f *fakeTerminalSession) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *fakeTerminalSession) Write(p []byte) (int, error) {
	f.writeCh <- string(p)
	return len(p), nil
}

func (f *fakeTerminalSession) Resize(cols, rows int) error {
	f.resizeCh <- fakeResize{cols: cols, rows: rows}
	return nil
}

func (f *fakeTerminalSession) Close() error {
	_ = f.writer.Close()
	return f.reader.Close()
}

func (f *fakeTerminalSession) Wait() error {
	return nil
}

func (f *fakeTerminalSession) EmitOutput(text string) {
	_, _ = f.writer.Write([]byte(text))
}
