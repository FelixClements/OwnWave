//go:build integration

package auth_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"ownwave/api/internal/auth"
	"ownwave/api/internal/testdb"
)

func setupAuthDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx, "pgvector/pgvector:pg15",
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(ctx)) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationsPath := filepath.Join(filepath.Dir(filename), "../../../../database/migrations")
	require.NoError(t, testdb.RunMigrations(connStr, migrationsPath))

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestRegisterFirstAdminAndInviteFlow(t *testing.T) {
	pool := setupAuthDB(t)
	svc := auth.NewService(pool)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, "alice", "password123", "")
	require.NoError(t, err)

	users, err := svc.ListUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.True(t, users[0].IsAdmin)

	_, _, err = svc.Register(ctx, "bob", "password123", "")
	require.ErrorIs(t, err, auth.ErrRegistrationClosed)

	rawInvite, _, err := svc.CreateInvite(ctx, users[0].ID, nil, 24*time.Hour)
	require.NoError(t, err)

	_, invited, err := svc.Register(ctx, "bob", "password123", rawInvite)
	require.NoError(t, err)
	require.False(t, invited.IsAdmin)

	_, _, err = svc.Register(ctx, "carol", "password123", rawInvite)
	require.ErrorIs(t, err, auth.ErrInvalidInvite)
}
