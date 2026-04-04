package transfer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var signingSecret = "cloudnest-default-secret" // Override via config

func SetSigningSecret(secret string) {
	signingSecret = secret
}

// GenerateSignedURL creates a time-limited signed URL for file access.
// Signature binds method + resourceID + expires to prevent cross-method replay.
func GenerateSignedURL(baseURL, resourceID, method string, expiry time.Duration) string {
	expires := time.Now().Add(expiry).Unix()
	sig := sign(resourceID, method, expires)
	return fmt.Sprintf("%s?expires=%d&sig=%s", baseURL, expires, sig)
}

// ValidateSignature checks whether a signature is valid and not expired.
func ValidateSignature(resourceID, method, expiresStr, sig string) bool {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	expected := sign(resourceID, method, expires)
	return hmac.Equal([]byte(sig), []byte(expected))
}

func sign(resourceID, method string, expires int64) string {
	if method == "" {
		method = "GET"
	}
	payload := fmt.Sprintf("%s:%s:%d", strings.ToUpper(method), resourceID, expires)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
