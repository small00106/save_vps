package ws

import (
	"encoding/json"
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

type Client struct {
	mu        sync.Mutex
	conn      *websocket.Conn
	masterURL string
	token     string
	OnMessage func(msg *RPCMessage)
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

	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err == nil {
			c.mu.Lock()
			c.conn = conn
			c.mu.Unlock()
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
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	// gorilla/websocket requires a single writer per connection.
	return c.conn.WriteJSON(msg)
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
		return nil
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

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}
