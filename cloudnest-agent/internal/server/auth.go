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

var signingSecret = "cloudnest-default-secret"

func init() {
	if s := os.Getenv("CLOUDNEST_SIGNING_SECRET"); s != "" {
		signingSecret = s
	}
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
	payload := fmt.Sprintf("%s:%s:%d", strings.ToUpper(method), resourceID, expires)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}
