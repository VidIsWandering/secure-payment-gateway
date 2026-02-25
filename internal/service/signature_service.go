package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HMACSignatureService implements ports.SignatureService using HMAC-SHA256.
type HMACSignatureService struct{}

// NewHMACSignatureService creates a new HMAC-SHA256 signature service.
func NewHMACSignatureService() *HMACSignatureService {
	return &HMACSignatureService{}
}

// Sign computes HMAC-SHA256 of payload using secretKey.
// Returns lowercase hex-encoded signature.
func (s *HMACSignatureService) Sign(secretKey string, payload string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks if signature matches HMAC-SHA256(secretKey, payload).
// Uses constant-time comparison to prevent timing attacks.
func (s *HMACSignatureService) Verify(secretKey string, payload string, signature string) bool {
	expected := s.Sign(secretKey, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// BuildCanonicalString constructs the canonical payload for signing.
// Format: METHOD|PATH|TIMESTAMP|NONCE|BODY
func (s *HMACSignatureService) BuildCanonicalString(method, path string, timestamp int64, nonce string, body string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s", method, path, timestamp, nonce, body)
}
