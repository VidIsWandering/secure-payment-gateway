package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Unset any SPG_* env vars injected by CI so that pure defaults are tested.
	// t.Setenv("") would set to empty string, which viper still treats as an override;
	// os.Unsetenv removes the variable entirely, restoring via t.Cleanup.
	unsetSPGEnvVars(t)

	// When no config file and no env vars, defaults should apply.
	cfg, err := Load("/non/existent/path/config.yaml")
	// File not found is OK — we fall back to defaults.
	// But viper returns error for explicit path that doesn't exist.
	// So we test with empty path to use search paths.
	_ = cfg
	_ = err

	cfg, err = Load("")
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "debug", cfg.Server.Mode)

	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "postgres", cfg.Database.User)
	assert.Equal(t, "payment_gateway", cfg.Database.DBName)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, int32(20), cfg.Database.MaxConns)
	assert.Equal(t, int32(5), cfg.Database.MinConns)

	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, 0, cfg.Redis.DB)

	assert.Equal(t, 24*time.Hour, cfg.JWT.Expiry)
	assert.Equal(t, "secure-payment-gateway", cfg.JWT.Issuer)

	assert.Equal(t, "info", cfg.Log.Level)
	assert.False(t, cfg.Log.Pretty)
}

func TestLoad_FromYAMLFile(t *testing.T) {
	// Unset any SPG_* env vars injected by CI so that only the YAML file values are tested.
	unsetSPGEnvVars(t)

	// Create a temporary YAML config.
	content := []byte(`
server:
  host: "127.0.0.1"
  port: 9090
  mode: "release"
database:
  host: "db.example.com"
  port: 5433
  user: "appuser"
  password: "secret123"
  dbname: "testdb"
  sslmode: "require"
redis:
  host: "redis.example.com"
  port: 6380
  password: "redispwd"
  db: 2
jwt:
  secret: "my-jwt-secret"
  expiry: "12h"
  issuer: "test-gateway"
aes:
  key: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
log:
  level: "debug"
  pretty: true
`)
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, content, 0600))

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "release", cfg.Server.Mode)

	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "appuser", cfg.Database.User)
	assert.Equal(t, "secret123", cfg.Database.Password)
	assert.Equal(t, "testdb", cfg.Database.DBName)
	assert.Equal(t, "require", cfg.Database.SSLMode)

	assert.Equal(t, "redis.example.com", cfg.Redis.Host)
	assert.Equal(t, 6380, cfg.Redis.Port)
	assert.Equal(t, "redispwd", cfg.Redis.Password)
	assert.Equal(t, 2, cfg.Redis.DB)

	assert.Equal(t, "my-jwt-secret", cfg.JWT.Secret)
	assert.Equal(t, 12*time.Hour, cfg.JWT.Expiry)
	assert.Equal(t, "test-gateway", cfg.JWT.Issuer)

	assert.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", cfg.AES.Key)
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.True(t, cfg.Log.Pretty)
}

func TestLoad_EnvOverride(t *testing.T) {
	// Environment variables should override defaults.
	t.Setenv("SPG_SERVER_PORT", "3000")
	t.Setenv("SPG_DATABASE_HOST", "env-db-host")
	t.Setenv("SPG_JWT_SECRET", "env-secret")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "env-db-host", cfg.Database.Host)
	assert.Equal(t, "env-secret", cfg.JWT.Secret)
}

func TestDatabaseConfig_DSN(t *testing.T) {
	dbCfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "myuser",
		Password: "mypass",
		DBName:   "mydb",
		SSLMode:  "disable",
	}

	expected := "postgres://myuser:mypass@localhost:5432/mydb?sslmode=disable"
	assert.Equal(t, expected, dbCfg.DSN())
}

func TestRedisConfig_Addr(t *testing.T) {
	redisCfg := RedisConfig{
		Host: "redis.local",
		Port: 6380,
	}

	assert.Equal(t, "redis.local:6380", redisCfg.Addr())
}

// unsetSPGEnvVars removes all SPG_* environment variables that CI injects for
// integration-test services. It registers a t.Cleanup to restore each variable
// to its original value once the test finishes, so sibling tests are unaffected.
func unsetSPGEnvVars(t *testing.T) {
	t.Helper()
	keys := []string{
		"SPG_DATABASE_HOST", "SPG_DATABASE_PORT", "SPG_DATABASE_USER",
		"SPG_DATABASE_PASSWORD", "SPG_DATABASE_DBNAME",
		"SPG_REDIS_HOST", "SPG_REDIS_PORT", "SPG_REDIS_PASSWORD", "SPG_REDIS_DB",
		"SPG_SERVER_HOST", "SPG_SERVER_PORT", "SPG_SERVER_MODE",
		"SPG_JWT_SECRET", "SPG_JWT_EXPIRY", "SPG_JWT_ISSUER",
	}
	for _, key := range keys {
		original, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to unset %s: %v", key, err)
		}
		if existed {
			t.Cleanup(func() { os.Setenv(key, original) }) //nolint:errcheck
		}
	}
}
