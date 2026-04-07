package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	registrationTokenEnvKey = "CLOUDNEST_REG_TOKEN"
	signingSecretEnvKey     = "CLOUDNEST_SIGNING_SECRET"

	legacyRegistrationToken = "cloudnest-register"
	legacySigningSecret     = "cloudnest-default-secret"
)

type runtimeSecrets struct {
	RegistrationToken string
	SigningSecret     string
}

func resolveRuntimeSecrets(dataDir string, getenv func(string) string) (runtimeSecrets, error) {
	registrationToken, _, err := resolveSecretValue(dataDir, "reg_token", registrationTokenEnvKey, legacyRegistrationToken, getenv)
	if err != nil {
		return runtimeSecrets{}, err
	}

	signingSecret, _, err := resolveSecretValue(dataDir, "signing_secret", signingSecretEnvKey, legacySigningSecret, getenv)
	if err != nil {
		return runtimeSecrets{}, err
	}

	return runtimeSecrets{
		RegistrationToken: registrationToken,
		SigningSecret:     signingSecret,
	}, nil
}

func resolveDataDir(executablePath, dbType, dbDSN string) (string, error) {
	if dbType == "sqlite" && dbDSN != "" && dbDSN != ":memory:" {
		return filepath.Dir(dbDSN), nil
	}

	execDir, err := executableDir(executablePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(execDir, "data"), nil
}

func resolveSecretValue(dataDir, filename, envKey, legacyDefault string, getenv func(string) string) (string, string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	secretPath := filepath.Join(dataDir, "secrets", filename)

	if value := strings.TrimSpace(getenv(envKey)); value != "" {
		if value == legacyDefault {
			return "", secretPath, fmt.Errorf("%s cannot use legacy default value %q", envKey, legacyDefault)
		}
		return value, secretPath, nil
	}

	if data, err := os.ReadFile(secretPath); err == nil {
		value := strings.TrimSpace(string(data))
		if value == legacyDefault {
			return "", secretPath, fmt.Errorf("%s file %q still uses legacy default value %q", envKey, secretPath, legacyDefault)
		}
		if value != "" {
			return value, secretPath, nil
		}
	} else if !os.IsNotExist(err) {
		return "", secretPath, fmt.Errorf("read secret file %q: %w", secretPath, err)
	}

	value, err := generateSecretValue()
	if err != nil {
		return "", secretPath, err
	}

	if err := os.MkdirAll(filepath.Dir(secretPath), 0700); err != nil {
		return "", secretPath, fmt.Errorf("create secrets directory: %w", err)
	}
	if err := os.WriteFile(secretPath, []byte(value), 0600); err != nil {
		return "", secretPath, fmt.Errorf("write secret file %q: %w", secretPath, err)
	}

	return value, secretPath, nil
}

func generateSecretValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
