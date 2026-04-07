package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSecretValueUsesEnvWithoutWritingFile(t *testing.T) {
	dataDir := t.TempDir()

	value, path, err := resolveSecretValue(dataDir, "signing_secret", signingSecretEnvKey, legacySigningSecret, func(key string) string {
		if key == signingSecretEnvKey {
			return "env-secret"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("resolve secret from env: %v", err)
	}
	if value != "env-secret" {
		t.Fatalf("expected env secret, got %q", value)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no secret file to be written, stat err=%v", err)
	}
}

func TestResolveSecretValueReusesExistingFile(t *testing.T) {
	dataDir := t.TempDir()
	secretPath := filepath.Join(dataDir, "secrets", "reg_token")
	if err := os.MkdirAll(filepath.Dir(secretPath), 0700); err != nil {
		t.Fatalf("mkdir secrets dir: %v", err)
	}
	if err := os.WriteFile(secretPath, []byte("stored-token"), 0600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}

	value, path, err := resolveSecretValue(dataDir, "reg_token", registrationTokenEnvKey, legacyRegistrationToken, nil)
	if err != nil {
		t.Fatalf("resolve secret from file: %v", err)
	}
	if path != secretPath {
		t.Fatalf("expected path %q, got %q", secretPath, path)
	}
	if value != "stored-token" {
		t.Fatalf("expected stored token, got %q", value)
	}
}

func TestResolveSecretValueGeneratesAndPersistsWhenMissing(t *testing.T) {
	dataDir := t.TempDir()

	value, path, err := resolveSecretValue(dataDir, "signing_secret", signingSecretEnvKey, legacySigningSecret, nil)
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	if len(value) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(value))
	}

	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated secret: %v", err)
	}
	if string(stored) != value {
		t.Fatalf("expected stored secret %q, got %q", value, string(stored))
	}
}

func TestResolveSecretValueRejectsLegacyDefaultFromEnv(t *testing.T) {
	dataDir := t.TempDir()

	_, _, err := resolveSecretValue(dataDir, "reg_token", registrationTokenEnvKey, legacyRegistrationToken, func(string) string {
		return legacyRegistrationToken
	})
	if err == nil {
		t.Fatal("expected legacy default env secret to be rejected")
	}
}

func TestResolveDataDirUsesSQLiteDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "cloudnest.db")

	dataDir, err := resolveDataDir("", "sqlite", dbPath)
	if err != nil {
		t.Fatalf("resolve data dir: %v", err)
	}
	if dataDir != filepath.Dir(dbPath) {
		t.Fatalf("expected data dir %q, got %q", filepath.Dir(dbPath), dataDir)
	}
}
