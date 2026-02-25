package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgon2HashService_HashAndVerify(t *testing.T) {
	svc := NewArgon2HashService()

	password := "SecureP@ssw0rd!"
	hash, err := svc.Hash(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Format check
	assert.True(t, strings.HasPrefix(hash, "$argon2id$v="), "hash should start with $argon2id$v=")

	// Verify correct password
	match, err := svc.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, match, "correct password should verify")
}

func TestArgon2HashService_VerifyWrongPassword(t *testing.T) {
	svc := NewArgon2HashService()

	hash, err := svc.Hash("correct-password")
	require.NoError(t, err)

	match, err := svc.Verify("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, match, "wrong password should not verify")
}

func TestArgon2HashService_UniqueSalts(t *testing.T) {
	svc := NewArgon2HashService()

	hash1, err := svc.Hash("same-password")
	require.NoError(t, err)

	hash2, err := svc.Hash("same-password")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "same password should produce different hashes (different salts)")
}

func TestArgon2HashService_EmptyPassword(t *testing.T) {
	svc := NewArgon2HashService()

	hash, err := svc.Hash("")
	require.NoError(t, err)

	match, err := svc.Verify("", hash)
	require.NoError(t, err)
	assert.True(t, match)
}

func TestArgon2HashService_VerifyInvalidFormat(t *testing.T) {
	svc := NewArgon2HashService()

	_, err := svc.Verify("password", "not-a-valid-hash")
	assert.Error(t, err)
}

func TestArgon2HashService_HashContainsParams(t *testing.T) {
	svc := NewArgon2HashService()

	hash, err := svc.Hash("test")
	require.NoError(t, err)

	// Verify it contains expected params
	assert.Contains(t, hash, "m=65536,t=1,p=4", "hash should contain Argon2id params")
}

func TestArgon2HashService_LongPassword(t *testing.T) {
	svc := NewArgon2HashService()

	longPassword := strings.Repeat("a", 1000)
	hash, err := svc.Hash(longPassword)
	require.NoError(t, err)

	match, err := svc.Verify(longPassword, hash)
	require.NoError(t, err)
	assert.True(t, match)
}
