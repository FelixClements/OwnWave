//go:build integration

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestInviteUsernameMismatchDoesNotBurnInvite(t *testing.T) {
	pool := setupAuthDB(t)
	svc := auth.NewService(pool)
	ctx := context.Background()

	_, admin, err := svc.Register(ctx, "alice", "password123", "")
	require.NoError(t, err)

	preset := "bob"
	rawInvite, _, err := svc.CreateInvite(ctx, admin.ID, &preset, 24*time.Hour)
	require.NoError(t, err)

	_, _, err = svc.Register(ctx, "not-bob", "password123", rawInvite)
	require.ErrorIs(t, err, auth.ErrInvalidInvite)

	_, invited, err := svc.Register(ctx, "bob", "password123", rawInvite)
	require.NoError(t, err)
	require.Equal(t, "bob", invited.Username)
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	pool := setupAuthDB(t)
	svc := auth.NewService(pool)
	ctx := context.Background()

	token1, user, err := svc.Register(ctx, "alice", "password123", "")
	require.NoError(t, err)

	token2, _, err := svc.Login(ctx, "alice", "password123")
	require.NoError(t, err)

	loaded, ok := svc.UserFromRequest(requestWithBearer(token1))
	require.True(t, ok)
	require.Equal(t, user.ID, loaded.ID)

	newToken, err := svc.ChangePassword(ctx, loaded, "password123", "password456")
	require.NoError(t, err)
	require.NotEmpty(t, newToken)

	_, ok = svc.UserFromRequest(requestWithBearer(token1))
	require.False(t, ok)
	_, ok = svc.UserFromRequest(requestWithBearer(token2))
	require.False(t, ok)
	_, ok = svc.UserFromRequest(requestWithBearer(newToken))
	require.True(t, ok)
}

func requestWithBearer(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
