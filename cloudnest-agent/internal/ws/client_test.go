package ws

import "testing"

func TestSendJSONReturnsErrNotConnected(t *testing.T) {
	client := NewClient("http://127.0.0.1:8080", "token")
	err := client.SendJSON(&RPCMessage{JSONRPC: "2.0", Method: "agent.heartbeat"})
	if err != ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestReadLoopReturnsErrNotConnected(t *testing.T) {
	client := NewClient("http://127.0.0.1:8080", "token")
	err := client.ReadLoop()
	if err != ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}
