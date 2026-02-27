package dto

import (
"testing"

"github.com/stretchr/testify/assert"
)

// --- SanitizeStruct tests ---

func TestSanitizeStruct_TrimsWhitespace(t *testing.T) {
req := RegisterRequest{
Username:     "  alice  ",
Password:     "  pass1234  ",
MerchantName: " My Shop ",
}
SanitizeStruct(&req)

assert.Equal(t, "alice", req.Username)
assert.Equal(t, "pass1234", req.Password)
assert.Equal(t, "My Shop", req.MerchantName)
}

func TestSanitizeStruct_EscapesHTML(t *testing.T) {
reason := "customer <script>alert('x')</script> request"
req := RefundRequest{
OriginalReferenceID: "ref-001",
Reason:              reason,
}
SanitizeStruct(&req)

assert.Contains(t, req.Reason, "&lt;script&gt;")
assert.NotContains(t, req.Reason, "<script>")
}

func TestSanitizeStruct_HandlesPointerString(t *testing.T) {
url := "  https://example.com/webhook  "
req := RegisterRequest{
Username:     "bob",
Password:     "password123",
MerchantName: "Bob Shop",
WebhookURL:   &url,
}
SanitizeStruct(&req)

assert.Equal(t, "https://example.com/webhook", *req.WebhookURL)
}

func TestSanitizeStruct_NilPointerIsNoOp(t *testing.T) {
req := RegisterRequest{
Username:     "carol",
Password:     "password123",
MerchantName: "Carol Shop",
WebhookURL:   nil,
}
SanitizeStruct(&req)
assert.Nil(t, req.WebhookURL)
}

func TestSanitizeStruct_NonPointerIsNoOp(t *testing.T) {
s := "hello"
SanitizeStruct(s) // should not panic
}

// --- Custom Validator tests ---

func TestSafeID_Valid(t *testing.T) {
cases := []string{
"ref-001",
"REF_002",
"a.b.c",
"simple123",
"ABC-def_GHI.123",
}
for _, tc := range cases {
assert.True(t, safeStringRe.MatchString(tc), "expected valid: %s", tc)
}
}

func TestSafeID_Invalid(t *testing.T) {
cases := []string{
"ref 001",     // space
"ref<001>",    // angle brackets
"ref;DROP",    // semicolon
"",            // empty
"hello world", // space
"ref\n001",    // newline
}
for _, tc := range cases {
assert.False(t, safeStringRe.MatchString(tc), "expected invalid: %s", tc)
}
}

func TestSanitizeStruct_PaymentRequest(t *testing.T) {
extra := "  some notes <b>bold</b>  "
req := PaymentRequest{
ReferenceID: "  ref-001  ",
Currency:    " VND ",
ExtraData:   &extra,
}
SanitizeStruct(&req)

assert.Equal(t, "ref-001", req.ReferenceID)
assert.Equal(t, "VND", req.Currency)
assert.Equal(t, "some notes &lt;b&gt;bold&lt;/b&gt;", *req.ExtraData)
}
