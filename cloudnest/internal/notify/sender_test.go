package notify

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookSenderReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream failed"))
	}))
	defer server.Close()

	sender := &WebhookSender{cfg: WebhookConfig{URL: server.URL}}
	err := sender.Send("disk", "threshold exceeded")
	if err == nil {
		t.Fatal("expected non-2xx response to fail")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

func TestWebhookSenderRespectsTimeout(t *testing.T) {
	prevClient := notificationHTTPClient
	notificationHTTPClient = &http.Client{Timeout: 20 * time.Millisecond}
	defer func() {
		notificationHTTPClient = prevClient
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &WebhookSender{cfg: WebhookConfig{URL: server.URL}}
	if err := sender.Send("disk", "threshold exceeded"); err == nil {
		t.Fatal("expected timeout error")
	}
}
