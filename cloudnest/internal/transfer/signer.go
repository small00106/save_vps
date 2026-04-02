package transfer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

var signingSecret = "cloudnest-default-secret" // Override via config

func SetSigningSecret(secret string) {
	signingSecret = secret
}

// GenerateSignedURL creates a time-limited signed URL for file access.
func GenerateSignedURL(baseURL, fileID string, expiry time.Duration) string {
	expires := time.Now().Add(expiry).Unix()
	sig := sign(fileID, expires)
	return fmt.Sprintf("%s?expires=%d&sig=%s", baseURL, expires, sig)
}

// ValidateSignature checks whether a signature is valid and not expired.
func ValidateSignature(fileID, expiresStr, sig string) bool {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	expected := sign(fileID, expires)
	return hmac.Equal([]byte(sig), []byte(expected))
}

func sign(fileID string, expires int64) string {
	payload := fmt.Sprintf("%s:%d", fileID, expires)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
