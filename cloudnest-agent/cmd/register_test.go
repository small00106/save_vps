package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRegistrationTokenPrefersInlineToken(t *testing.T) {
	token, err := resolveRegistrationToken("inline-token", "")
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if token != "inline-token" {
		t.Fatalf("expected inline token, got %q", token)
	}
}

func TestResolveRegistrationTokenFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(path, []byte("file-token\n"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	token, err := resolveRegistrationToken("", path)
	if err != nil {
		t.Fatalf("resolve token from file: %v", err)
	}
	if token != "file-token" {
		t.Fatalf("expected file token, got %q", token)
	}
}

func TestResolveRegistrationTokenRequiresTokenOrFile(t *testing.T) {
	if _, err := resolveRegistrationToken("", ""); err == nil {
		t.Fatal("expected error when both token and token-file are empty")
	}
}
