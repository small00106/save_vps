package ws

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	ID      string          `json:"id,omitempty"`
}

const (
	clientWSWriteWait      = 10 * time.Second
	clientWSPongWait       = 70 * time.Second
	clientWSPingPeriod     = (clientWSPongWait * 9) / 10
	clientWSMaxMessageSize = 32 << 20
)

var ErrNotConnected = errors.New("websocket not connected")

type Client struct {
	mu        sync.Mutex
	conn      *websocket.Conn
	masterURL string
	token     string
	OnMessage func(msg *RPCMessage)
	pingStop  chan struct{}
}

func NewClient(masterURL, token string) *Client {
	return &Client{
		masterURL: masterURL,
		token:     token,
	}
}

// Connect establishes a WebSocket connection with exponential backoff.
func (c *Client) Connect() error {
	wsURL := strings.Replace(c.masterURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/agent/ws"

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.token)

	backoff := time.Second
	maxBackoff := 60 * time.Second
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 15 * time.Second

	for {
		conn, _, err := dialer.Dial(wsURL, header)
		if err == nil {
			configureConn(conn)
			c.mu.Lock()
			c.conn = conn
			c.pingStop = make(chan struct{})
			c.mu.Unlock()
			go c.pingLoop()
			log.Printf("[WS] Connected to master")
			backoff = time.Second
			return nil
		}

		log.Printf("[WS] Connection failed: %v, retrying in %v", err, backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// SendJSON sends a JSON-RPC message.
func (c *Client) SendJSON(msg *RPCMessage) error {
	c.mu.Lock()
	conn := c.conn
	if conn == nil {
		c.mu.Unlock()
		return ErrNotConnected
	}

	// gorilla/websocket requires a single writer per connection.
	if err := conn.SetWriteDeadline(time.Now().Add(clientWSWriteWait)); err != nil {
		c.mu.Unlock()
		return err
	}
	err := conn.WriteJSON(msg)
	c.mu.Unlock()
	if err != nil {
		c.Close()
	}
	return err
}

// Send is a convenience method for sending method+params.
func (c *Client) Send(method string, params interface{}) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.SendJSON(&RPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	})
}

// ReadLoop reads messages from the WebSocket and dispatches them.
// Returns an error when the connection is lost.
func (c *Client) ReadLoop() error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return ErrNotConnected
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg RPCMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("[WS] Invalid message: %v", err)
			continue
		}

		if c.OnMessage != nil {
			c.OnMessage(&msg)
		}
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pingStop != nil {
		close(c.pingStop)
		c.pingStop = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func configureConn(conn *websocket.Conn) {
	conn.SetReadLimit(clientWSMaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(clientWSPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(clientWSPongWait))
	})
}

func (c *Client) pingLoop() {
	c.mu.Lock()
	stop := c.pingStop
	c.mu.Unlock()

	ticker := time.NewTicker(clientWSPingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.writeControl(websocket.PingMessage, []byte("ping")); err != nil {
				log.Printf("[WS] Ping failed: %v", err)
				c.Close()
				return
			}
		case <-stop:
			return
		}
	}
}

func (c *Client) writeControl(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return ErrNotConnected
	}
	return c.conn.WriteControl(messageType, data, time.Now().Add(clientWSWriteWait))
}
