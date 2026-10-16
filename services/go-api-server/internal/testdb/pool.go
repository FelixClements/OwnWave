package testdb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// NewPool returns a pool on a freshly migrated database for integration tests.
//
// It uses OWNWAVE_TEST_DATABASE_URL when set (the public schema is reset, so
// point it at a throwaway database) and otherwise starts a pgvector container.
// OWNWAVE_TEST_MIGRATIONS overrides the migrations directory.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	connStr := os.Getenv("OWNWAVE_TEST_DATABASE_URL")
	if connStr != "" {
		pool, err := pgxpool.New(ctx, connStr)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`)
		pool.Close()
		require.NoError(t, err)
	} else {
		container, err := postgres.Run(ctx, "pgvector/pgvector:pg15",
			postgres.WithSQLDriver("pgx"),
			postgres.BasicWaitStrategies(),
		)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, container.Terminate(ctx)) })
		connStr, err = container.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
		waitForPostgres(t, ctx, connStr)
	}

	require.NoError(t, RunMigrations(connStr, migrationsPath(t)))

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func migrationsPath(t *testing.T) string {
	if p := os.Getenv("OWNWAVE_TEST_MIGRATIONS"); p != "" {
		return p
	}
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(filename), "../../../../database/migrations")
}

func waitForPostgres(t *testing.T, ctx context.Context, connStr string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		pool, err := pgxpool.New(ctx, connStr)
		if err == nil {
			err = pool.Ping(ctx)
			pool.Close()
		}
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}
	require.NoError(t, lastErr, "postgres not ready")
}
