package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHMACSignatureService_SignAndVerify(t *testing.T) {
	svc := NewHMACSignatureService()
	secretKey := "my-secret-key"
	payload := "POST|/api/v1/payments|1708092000|abc123nonce|{\"amount\":50000}"

	signature := svc.Sign(secretKey, payload)

	// Should be lowercase hex
	assert.Regexp(t, `^[0-9a-f]{64}$`, signature, "signature should be 64-char lowercase hex (SHA-256)")

	// Verify with correct key
	assert.True(t, svc.Verify(secretKey, payload, signature))
}

func TestHMACSignatureService_VerifyFails_WrongKey(t *testing.T) {
	svc := NewHMACSignatureService()
	payload := "test payload"

	signature := svc.Sign("correct-key", payload)
	assert.False(t, svc.Verify("wrong-key", payload, signature))
}

func TestHMACSignatureService_VerifyFails_WrongPayload(t *testing.T) {
	svc := NewHMACSignatureService()
	secretKey := "my-key"

	signature := svc.Sign(secretKey, "original payload")
	assert.False(t, svc.Verify(secretKey, "tampered payload", signature))
}

func TestHMACSignatureService_VerifyFails_WrongSignature(t *testing.T) {
	svc := NewHMACSignatureService()
	assert.False(t, svc.Verify("key", "payload", "invalidsignature"))
}

func TestHMACSignatureService_DeterministicSign(t *testing.T) {
	svc := NewHMACSignatureService()

	sig1 := svc.Sign("key", "data")
	sig2 := svc.Sign("key", "data")

	assert.Equal(t, sig1, sig2, "same key+payload should produce same signature")
}

func TestHMACSignatureService_BuildCanonicalString(t *testing.T) {
	svc := NewHMACSignatureService()

	result := svc.BuildCanonicalString("POST", "/api/v1/payments", 1708092000, "abc123", `{"amount":50000}`)

	expected := "POST|/api/v1/payments|1708092000|abc123|{\"amount\":50000}"
	assert.Equal(t, expected, result)
}

func TestHMACSignatureService_EmptyBody(t *testing.T) {
	svc := NewHMACSignatureService()

	result := svc.BuildCanonicalString("GET", "/api/v1/balance", 1708092000, "nonce1", "")
	expected := "GET|/api/v1/balance|1708092000|nonce1|"
	assert.Equal(t, expected, result)
}
