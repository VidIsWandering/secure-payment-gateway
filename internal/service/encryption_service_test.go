package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Valid 32-byte key in hex (64 chars)
const testAESKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestAESEncryptionService_NewInvalidKey(t *testing.T) {
	_, err := NewAESEncryptionService("shortkey")
	assert.Error(t, err)
}

func TestAESEncryptionService_EncryptDecrypt(t *testing.T) {
	svc, err := NewAESEncryptionService(testAESKey)
	require.NoError(t, err)

	plaintext := "1000000"
	ciphertext, err := svc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := svc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESEncryptionService_DifferentNonces(t *testing.T) {
	svc, err := NewAESEncryptionService(testAESKey)
	require.NoError(t, err)

	plaintext := "test_value"
	c1, err := svc.Encrypt(plaintext)
	require.NoError(t, err)
	c2, err := svc.Encrypt(plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, c1, c2, "same plaintext should produce different ciphertext due to random nonce")

	d1, _ := svc.Decrypt(c1)
	d2, _ := svc.Decrypt(c2)
	assert.Equal(t, d1, d2)
}

func TestAESEncryptionService_TamperedCiphertext(t *testing.T) {
	svc, err := NewAESEncryptionService(testAESKey)
	require.NoError(t, err)

	ciphertext, err := svc.Encrypt("secret")
	require.NoError(t, err)

	tampered := ciphertext[:len(ciphertext)-2] + "ff"
	_, err = svc.Decrypt(tampered)
	assert.Error(t, err)
}

func TestAESEncryptionService_WrongKey(t *testing.T) {
	svc1, _ := NewAESEncryptionService(testAESKey)
	otherKey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	svc2, _ := NewAESEncryptionService(otherKey)

	ciphertext, err := svc1.Encrypt("balance_100")
	require.NoError(t, err)

	_, err = svc2.Decrypt(ciphertext)
	assert.Error(t, err)
}

func TestAESEncryptionService_InvalidCiphertext(t *testing.T) {
	svc, _ := NewAESEncryptionService(testAESKey)

	_, err := svc.Decrypt("not-hex-at-all!!!")
	assert.Error(t, err)

	_, err = svc.Decrypt("abcdef")
	assert.Error(t, err)
}
