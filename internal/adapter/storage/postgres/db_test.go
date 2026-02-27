package postgres

import (
	"testing"
	"time"

	"secure-payment-gateway/config"

	"github.com/stretchr/testify/assert"
)

func TestDSN_Format(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	assert.Equal(t, expected, cfg.DSN())
}

func TestDefaultPoolConfig(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "testuser",
		Password:        "testpass",
		DBName:          "testdb",
		SSLMode:         "disable",
		MaxConns:        20,
		MinConns:        5,
		ConnMaxLifetime: 30 * time.Minute,
	}

	// Verify DSN is constructed correctly with all fields.
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "testuser")
	assert.Contains(t, dsn, "testpass")
	assert.Contains(t, dsn, "localhost")
	assert.Contains(t, dsn, "5432")
	assert.Contains(t, dsn, "testdb")
	assert.Contains(t, dsn, "disable")

	// Verify pool-specific config values.
	assert.Equal(t, int32(20), cfg.MaxConns)
	assert.Equal(t, int32(5), cfg.MinConns)
	assert.Equal(t, 30*time.Minute, cfg.ConnMaxLifetime)
}

// NOTE: Integration test (requires running PostgreSQL) should be placed in a
// separate file with build tag: //go:build integration
// For unit tests, we verify config parsing only. The actual NewPool function
// is tested via integration tests.
