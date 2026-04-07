package terminal

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	defaultTerminalCols = 120
	defaultTerminalRows = 40
	maxTerminalCols     = 400
	maxTerminalRows     = 120
)

var ErrTerminalUnsupported = errors.New("remote terminal is unsupported on this operating system")

var newTerminalSessionFunc = newTerminalSession

type terminalSession interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Wait() error
}

type terminalControlMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// HandleTerminal starts an interactive shell session and bridges it over WebSocket.
// GET /api/terminal
func HandleTerminal(c *gin.Context) {
	cols, rows := initialTerminalSize(c)
	session, err := newTerminalSessionFunc(cols, rows)
	if err != nil {
		handleTerminalSessionError(c, err)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		_ = session.Close()
		_ = session.Wait()
		log.Printf("[Terminal] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	defer func() {
		_ = session.Close()
		_ = session.Wait()
	}()

	done := make(chan struct{})

	go func() {
		defer close(done)
		streamTerminalOutput(conn, session)
	}()

	go func() {
		handleTerminalInput(conn, session)
	}()

	<-done
}

func handleTerminalSessionError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "failed to start shell session"
	if errors.Is(err, ErrTerminalUnsupported) {
		status = http.StatusNotImplemented
		message = "remote terminal is unsupported on this agent OS"
	}
	log.Printf("[Terminal] Session start error: %v", err)
	c.JSON(status, gin.H{"error": message})
}

func initialTerminalSize(c *gin.Context) (int, int) {
	return sanitizeTerminalDimension(c.Query("cols"), defaultTerminalCols, maxTerminalCols),
		sanitizeTerminalDimension(c.Query("rows"), defaultTerminalRows, maxTerminalRows)
}

func sanitizeTerminalDimension(raw string, fallback, limit int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	if value > limit {
		return limit
	}
	return value
}

func streamTerminalOutput(conn *websocket.Conn, session terminalSession) {
	buf := make([]byte, 4096)
	for {
		n, err := session.Read(buf)
		if n > 0 {
			if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
				_ = session.Close()
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[Terminal] PTY read error: %v", err)
			}
			return
		}
	}
}

func handleTerminalInput(conn *websocket.Conn, session terminalSession) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			_ = session.Close()
			return
		}
		if err := applyTerminalControlMessage(session, msg); err != nil {
			log.Printf("[Terminal] Input handling error: %v", err)
			_ = session.Close()
			return
		}
	}
}

func applyTerminalControlMessage(session terminalSession, msg []byte) error {
	var control terminalControlMessage
	if err := json.Unmarshal(msg, &control); err != nil || control.Type == "" {
		_, writeErr := session.Write(msg)
		return writeErr
	}

	switch control.Type {
	case "input":
		if control.Data == "" {
			return nil
		}
		_, err := session.Write([]byte(control.Data))
		return err
	case "resize":
		if control.Cols <= 0 || control.Rows <= 0 {
			return nil
		}
		return session.Resize(control.Cols, control.Rows)
	default:
		return nil
	}
}
