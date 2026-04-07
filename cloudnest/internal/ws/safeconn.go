package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultWriteWait = 10 * time.Second

// SafeConn wraps a websocket.Conn with a mutex for concurrent write safety.
type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{conn: conn}
}

func (sc *SafeConn) WriteJSON(v interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err := sc.conn.SetWriteDeadline(time.Now().Add(defaultWriteWait)); err != nil {
		return err
	}
	return sc.conn.WriteJSON(v)
}

func (sc *SafeConn) ReadJSON(v interface{}) error {
	return sc.conn.ReadJSON(v)
}

func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	return sc.conn.ReadMessage()
}

func (sc *SafeConn) WriteMessage(msgType int, data []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err := sc.conn.SetWriteDeadline(time.Now().Add(defaultWriteWait)); err != nil {
		return err
	}
	return sc.conn.WriteMessage(msgType, data)
}

func (sc *SafeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteControl(messageType, data, deadline)
}

func (sc *SafeConn) Close() error {
	return sc.conn.Close()
}

func (sc *SafeConn) Raw() *websocket.Conn {
	return sc.conn
}
