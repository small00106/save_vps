package terminal

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleTerminal starts a shell process and pipes it over WebSocket.
// GET /api/terminal
func HandleTerminal(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[Terminal] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	if runtime.GOOS == "windows" {
		shell = "cmd.exe"
	}

	cmd := exec.Command(shell)
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[Terminal] StdinPipe error: %v", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[Terminal] StdoutPipe error: %v", err)
		return
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		log.Printf("[Terminal] Start error: %v", err)
		return
	}

	done := make(chan struct{})

	// stdout → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if wErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); wErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		close(done)
	}()

	// WebSocket → stdin
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				stdin.Close()
				return
			}
			stdin.Write(msg)
		}
	}()

	<-done
	cmd.Process.Kill()
	cmd.Wait()
}
