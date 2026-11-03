//go:build integration

package auth_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/auth"
)

func TestDeviceTokensLoginListSlideAndRevoke(t *testing.T) {
	pool := setupAuthDB(t)
	svc := auth.NewService(pool)
	ctx := context.Background()
	_, _, err := svc.Register(ctx, "alice", "password123", "")
	require.NoError(t, err)

	_, _, _, err = svc.LoginDevice(ctx, "alice", "wrong-password", "Pixel")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
	_, _, _, err = svc.LoginDevice(ctx, "nobody", "password123", "Pixel")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)

	token, user, device, err := svc.LoginDevice(ctx, "alice", "password123", "  Pixel   8  ")
	require.NoError(t, err)
	require.Equal(t, "alice", user.Username)
	require.Empty(t, user.PasswordHash)
	require.Equal(t, "Pixel 8", device.Name)
	require.WithinDuration(t, time.Now().Add(365*24*time.Hour), device.ExpiresAt, time.Minute)

	// The token authenticates as a Bearer header.
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	got, ok := svc.UserFromRequest(req)
	require.True(t, ok)
	require.Equal(t, user.ID, got.ID)

	// Use slides the expiry once the last use is over an hour old.
	_, err = pool.Exec(ctx, `UPDATE sessions SET expires_at = NOW() + INTERVAL '1 day', last_used_at = NOW() - INTERVAL '2 hours' WHERE id = $1::uuid`, device.ID)
	require.NoError(t, err)
	_, ok = svc.UserFromRequest(req)
	require.True(t, ok)
	var expires time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT expires_at FROM sessions WHERE id = $1::uuid`, device.ID).Scan(&expires))
	require.WithinDuration(t, time.Now().Add(365*24*time.Hour), expires, time.Minute)

	// Web sessions are not devices and never slide.
	webToken, _, err := svc.Login(ctx, "alice", "password123")
	require.NoError(t, err)
	devices, err := svc.ListDevices(ctx, user.ID, token)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.True(t, devices[0].Current)
	devices, err = svc.ListDevices(ctx, user.ID, webToken)
	require.NoError(t, err)
	require.False(t, devices[0].Current)

	// Another user cannot revoke it; the owner can, and the token stops working.
	other, err := svc.CreateUser(ctx, "bob", "password123")
	require.NoError(t, err)
	require.ErrorIs(t, svc.RevokeDevice(ctx, other.ID, device.ID), auth.ErrDeviceNotFound)
	require.NoError(t, svc.RevokeDevice(ctx, user.ID, device.ID))
	_, ok = svc.UserFromRequest(req)
	require.False(t, ok)
	require.ErrorIs(t, svc.RevokeDevice(ctx, user.ID, device.ID), auth.ErrDeviceNotFound)
}
