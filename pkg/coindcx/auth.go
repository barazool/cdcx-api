package coindcx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// generateSignature creates HMAC-SHA256 signature for authentication
func (c *Client) generateSignature(payload string) string {
	h := hmac.New(sha256.New, []byte(c.APISecret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}
