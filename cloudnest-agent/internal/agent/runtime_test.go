package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestStartDataPlaneReturnsStartupError(t *testing.T) {
	restoreStart := swapStartDataPlaneFunc(func(addr string, rateLimit int64) error {
		return errors.New("listen tcp 0.0.0.0:8801: bind failed")
	})
	defer restoreStart()

	restoreProbe := swapProbeDataPlaneFunc(func(addr string) error {
		return errors.New("not ready")
	})
	defer restoreProbe()

	errCh, err := startDataPlane(&Config{Port: 8801})
	if errCh != nil {
		t.Fatalf("expected nil error channel on startup failure")
	}
	if err == nil || !strings.Contains(err.Error(), "bind failed") {
		t.Fatalf("expected bind failure, got %v", err)
	}
}

func TestStartDataPlaneReportsServerExitAfterReady(t *testing.T) {
	serverExited := make(chan struct{})

	restoreStart := swapStartDataPlaneFunc(func(addr string, rateLimit int64) error {
		<-serverExited
		return errors.New("server crashed")
	})
	defer restoreStart()

	restoreProbe := swapProbeDataPlaneFunc(func(addr string) error {
		return nil
	})
	defer restoreProbe()

	errCh, err := startDataPlane(&Config{Port: 8801})
	if err != nil {
		t.Fatalf("expected successful startup, got %v", err)
	}

	close(serverExited)
	runErr := <-errCh
	if runErr == nil || !strings.Contains(runErr.Error(), "server crashed") {
		t.Fatalf("expected runtime server error, got %v", runErr)
	}
}
