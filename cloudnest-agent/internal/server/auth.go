package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	signingSecretEnvKey     = "CLOUDNEST_SIGNING_SECRET"
	signingSecretFileEnvKey = "CLOUDNEST_SIGNING_SECRET_FILE"
	legacySigningSecret     = "cloudnest-default-secret"
)

var signingSecret string

func LoadSigningSecretFromEnv() error {
	secret, err := resolveSigningSecret()
	if err != nil {
		return err
	}
	signingSecret = secret
	return nil
}

func resolveSigningSecret() (string, error) {
	if path := strings.TrimSpace(os.Getenv(signingSecretFileEnvKey)); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from %q: %w", signingSecretFileEnvKey, path, err)
		}
		secret := strings.TrimSpace(string(data))
		switch {
		case secret == "":
			return "", fmt.Errorf("%s file %q is empty", signingSecretFileEnvKey, path)
		case secret == legacySigningSecret:
			return "", fmt.Errorf("%s cannot use legacy default value %q", signingSecretFileEnvKey, legacySigningSecret)
		}
		return secret, nil
	}

	secret := strings.TrimSpace(os.Getenv(signingSecretEnvKey))
	switch {
	case secret == "":
		return "", fmt.Errorf("either %s or %s is required", signingSecretFileEnvKey, signingSecretEnvKey)
	case secret == legacySigningSecret:
		return "", fmt.Errorf("%s cannot use legacy default value %q", signingSecretEnvKey, legacySigningSecret)
	}
	return secret, nil
}

// validateSignature checks HMAC-SHA256 signature, same algorithm as master's transfer.signer.
func validateSignature(resourceID, method, expiresStr, sig string) bool {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}

	if method == "" {
		method = "GET"
	}
	expected := signResource(resourceID, method, expires)

	return hmac.Equal([]byte(sig), []byte(expected))
}

func signResource(resourceID, method string, expires int64) string {
	if method == "" {
		method = "GET"
	}
	payload := fmt.Sprintf("%s:%s:%d", strings.ToUpper(method), resourceID, expires)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func signedUploadResource(fileID, path, name, overwrite string) string {
	cleanPath := strings.TrimSpace(path)
	cleanName := strings.TrimSpace(name)
	if cleanName == "" && cleanPath == "" && strings.TrimSpace(overwrite) == "" {
		return fileID
	}
	if cleanPath == "" {
		cleanPath = "/"
	}
	return fmt.Sprintf("%s|%s|%s|%t", fileID, cleanPath, cleanName, strings.EqualFold(strings.TrimSpace(overwrite), "true"))
}

func signedMoveResource(fromPath, toPath string) string {
	return fmt.Sprintf("%s|%s", strings.TrimSpace(fromPath), strings.TrimSpace(toPath))
}
