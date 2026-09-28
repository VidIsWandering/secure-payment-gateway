package postgres

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VidIsWandering/secure-payment-gateway/db/migrations"
)

func TestEmbeddedMigrations_ArePaired(t *testing.T) {
	ups, err := fs.Glob(migrations.FS, "*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, ups)
	for _, up := range ups {
		down := up[:len(up)-len(".up.sql")] + ".down.sql"
		_, err := fs.Stat(migrations.FS, down)
		assert.NoError(t, err, "missing down migration for %s", up)
	}
}

func TestPgx5URL(t *testing.T) {
	assert.Equal(t, "pgx5://u:p@h:5432/db?sslmode=disable", pgx5URL("postgres://u:p@h:5432/db?sslmode=disable"))
	assert.Equal(t, "pgx5://u:p@h/db", pgx5URL("postgresql://u:p@h/db"))
	assert.Equal(t, "pgx5://already", pgx5URL("pgx5://already"))
}

// TestMigrate_RealDatabase runs against PostgreSQL when SPG_TEST_DATABASE_URL is
// set (CI provides one); it is skipped otherwise.
func TestMigrate_RealDatabase(t *testing.T) {
	dsn := os.Getenv("SPG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SPG_TEST_DATABASE_URL not set")
	}

	version, err := Migrate(dsn)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, uint(1))

	// Idempotent: a second run is a no-op at the same version.
	again, err := Migrate(dsn)
	require.NoError(t, err)
	assert.Equal(t, version, again)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	for _, table := range []string{"merchants", "wallets", "transactions", "idempotency_logs", "webhook_delivery_logs", "audit_logs"} {
		var exists bool
		require.NoError(t, conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists))
		assert.True(t, exists, "table %s should exist", table)
	}
}
