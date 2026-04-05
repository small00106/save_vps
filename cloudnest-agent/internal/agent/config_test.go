package agent

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestConfigPathFallsBackToCurrentUserHome(t *testing.T) {
	originalHome, hadHome := os.LookupEnv("HOME")
	if hadHome {
		t.Cleanup(func() { _ = os.Setenv("HOME", originalHome) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("HOME") })
	}
	_ = os.Unsetenv("HOME")

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("lookup current user: %v", err)
	}
	if currentUser.HomeDir == "" {
		t.Fatal("current user home dir is empty")
	}

	got := configPath()
	want := filepath.Join(currentUser.HomeDir, ".cloudnest", "agent.json")
	if got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}
