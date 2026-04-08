package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testJWTSecret = "this-is-a-test-jwt-secret-key-32chars"
	testAESKey    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestLoad_Defaults(t *testing.T) {
	unsetSPGEnvVars(t)

	// Set required secrets so validation passes
	t.Setenv("SPG_JWT_SECRET", testJWTSecret)
	t.Setenv("SPG_AES_KEY", testAESKey)

	cfg, err := Load("")
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
	unsetSPGEnvVars(t)

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
  secret: "yaml-jwt-secret-key-at-least-32chars!!"
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

	assert.Equal(t, "yaml-jwt-secret-key-at-least-32chars!!", cfg.JWT.Secret)
	assert.Equal(t, 12*time.Hour, cfg.JWT.Expiry)
	assert.Equal(t, "test-gateway", cfg.JWT.Issuer)

	assert.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", cfg.AES.Key)
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.True(t, cfg.Log.Pretty)
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("SPG_SERVER_PORT", "3000")
	t.Setenv("SPG_DATABASE_HOST", "env-db-host")
	t.Setenv("SPG_JWT_SECRET", testJWTSecret)
	t.Setenv("SPG_AES_KEY", testAESKey)

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "env-db-host", cfg.Database.Host)
	assert.Equal(t, testJWTSecret, cfg.JWT.Secret)
}

func TestLoad_Validation_MissingJWTSecret(t *testing.T) {
	unsetSPGEnvVars(t)
	t.Setenv("SPG_AES_KEY", testAESKey)
	// No JWT_SECRET set, default is empty

	// Load from temp dir so no config.yaml is found
	tmpDir := t.TempDir()
	tmpCfg := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(tmpCfg, []byte("{}\n"), 0600)

	_, err := Load(tmpCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SPG_JWT_SECRET")
}

func TestLoad_Validation_MissingAESKey(t *testing.T) {
	unsetSPGEnvVars(t)
	t.Setenv("SPG_JWT_SECRET", testJWTSecret)
	// No AES_KEY set, default is empty

	// Load from temp dir so no config.yaml is found
	tmpDir := t.TempDir()
	tmpCfg := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(tmpCfg, []byte("{}\n"), 0600)

	_, err := Load(tmpCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SPG_AES_KEY")
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

// unsetSPGEnvVars removes all SPG_* environment variables that CI injects.
func unsetSPGEnvVars(t *testing.T) {
	t.Helper()
	keys := []string{
		"SPG_DATABASE_HOST", "SPG_DATABASE_PORT", "SPG_DATABASE_USER",
		"SPG_DATABASE_PASSWORD", "SPG_DATABASE_DBNAME",
		"SPG_REDIS_HOST", "SPG_REDIS_PORT", "SPG_REDIS_PASSWORD", "SPG_REDIS_DB",
		"SPG_SERVER_HOST", "SPG_SERVER_PORT", "SPG_SERVER_MODE",
		"SPG_JWT_SECRET", "SPG_JWT_EXPIRY", "SPG_JWT_ISSUER",
		"SPG_AES_KEY",
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
