package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudnest/cloudnest-agent/internal/storage"
)

func TestRegisterWithMasterDefaultsScanDirsToFilesDir(t *testing.T) {
	restoreEnv := withRegisterEnv(t)
	defer restoreEnv()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"uuid":  "node-1",
			"token": "token-1",
		})
	}))
	defer srv.Close()

	cfg := &Config{
		MasterURL: srv.URL,
		Port:      8801,
	}

	if err := RegisterWithMaster(cfg, "reg-token"); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if len(cfg.ScanDirs) != 1 || cfg.ScanDirs[0] != storage.FilesDir() {
		t.Fatalf("expected default scan dir %q, got %#v", storage.FilesDir(), cfg.ScanDirs)
	}
}

func TestRegisterWithMasterRejectsEmptyToken(t *testing.T) {
	restoreEnv := withRegisterEnv(t)
	defer restoreEnv()

	cfg := &Config{MasterURL: "http://127.0.0.1:8800", Port: 8801}
	err := RegisterWithMaster(cfg, "")
	if err == nil {
		t.Fatal("expected empty token to be rejected")
	}
	if !strings.Contains(err.Error(), "registration token") {
		t.Fatalf("expected registration token error, got %v", err)
	}
}

func TestRegisterWithMasterRespectsHTTPTimeout(t *testing.T) {
	restoreEnv := withRegisterEnv(t)
	defer restoreEnv()

	prevClient := registrationHTTPClient
	registrationHTTPClient = &http.Client{Timeout: 20 * time.Millisecond}
	defer func() {
		registrationHTTPClient = prevClient
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{
		MasterURL: srv.URL,
		Port:      8801,
	}

	err := RegisterWithMaster(cfg, "reg-token")
	if err == nil {
		t.Fatal("expected registration timeout")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Fatalf("expected connect failure, got %v", err)
	}
}

func withRegisterEnv(t *testing.T) func() {
	t.Helper()
	tempHome := t.TempDir()
	prevHome := os.Getenv("HOME")
	prevUserProfile := os.Getenv("USERPROFILE")
	prevStorage := os.Getenv("CLOUDNEST_DATA_SAVE_DIR")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	if err := os.Setenv("USERPROFILE", tempHome); err != nil {
		t.Fatalf("set USERPROFILE: %v", err)
	}
	if err := os.Setenv("CLOUDNEST_DATA_SAVE_DIR", filepath.Join(tempHome, "data_save")); err != nil {
		t.Fatalf("set CLOUDNEST_DATA_SAVE_DIR: %v", err)
	}
	return func() {
		_ = restoreEnvVar("HOME", prevHome)
		_ = restoreEnvVar("USERPROFILE", prevUserProfile)
		_ = restoreEnvVar("CLOUDNEST_DATA_SAVE_DIR", prevStorage)
	}
}

func restoreEnvVar(key, value string) error {
	if value == "" {
		return os.Unsetenv(key)
	}
	return os.Setenv(key, value)
}
