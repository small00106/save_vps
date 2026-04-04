package agent

import (
	"fmt"
	"net"
	"time"

	agentServer "github.com/cloudnest/cloudnest-agent/internal/server"
)

const dataPlaneReadyTimeout = 5 * time.Second
const dataPlaneStartupErrorGrace = 100 * time.Millisecond

var startDataPlaneFunc = func(addr string, rateLimit int64) error {
	return agentServer.Start(addr, rateLimit)
}

var probeDataPlaneFunc = waitForDataPlaneReady

func startDataPlane(cfg *Config) (<-chan error, error) {
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	errCh := make(chan error, 1)

	go func() {
		if err := startDataPlaneFunc(addr, cfg.RateLimit); err != nil {
			errCh <- fmt.Errorf("data plane server failed: %w", err)
			return
		}
		errCh <- fmt.Errorf("data plane server stopped unexpectedly")
	}()

	if err := waitForDataPlaneStartup(addr, errCh); err != nil {
		return nil, err
	}

	return errCh, nil
}

func waitForDataPlaneStartup(addr string, errCh <-chan error) error {
	select {
	case err := <-errCh:
		if err == nil {
			return fmt.Errorf("data plane server failed before startup completed")
		}
		return err
	default:
	}

	readyCh := make(chan error, 1)
	go func() {
		readyCh <- probeDataPlaneFunc(addr)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			return fmt.Errorf("data plane server failed before startup completed")
		}
		return err
	case err := <-readyCh:
		if err != nil {
			timer := time.NewTimer(dataPlaneStartupErrorGrace)
			defer timer.Stop()
			select {
			case serverErr := <-errCh:
				if serverErr == nil {
					return err
				}
				return serverErr
			case <-timer.C:
			}
			return err
		}
		return nil
	}
}

func waitForDataPlaneReady(addr string) error {
	target := dataPlaneProbeAddr(addr)
	deadline := time.Now().Add(dataPlaneReadyTimeout)
	var lastErr error

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", target, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for %s", target)
	}
	return fmt.Errorf("data plane failed to become ready on %s: %w", target, lastErr)
}

func dataPlaneProbeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
