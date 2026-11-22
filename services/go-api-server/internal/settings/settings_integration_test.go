//go:build integration

package settings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/settings"
	"ownwave/api/internal/testutil/testdb"
)

func TestGetSet_RoundTripsAndOverwrites(t *testing.T) {
	ctx := context.Background()
	svc := settings.NewService(testdb.NewPool(t))
	key := settings.Key("settings_test") // not seeded by any migration

	_, err := svc.Get(ctx, key)
	require.True(t, errors.Is(err, pgx.ErrNoRows), "missing key: %v", err)

	require.NoError(t, svc.Set(ctx, key, map[string]interface{}{"completed": false}))
	require.NoError(t, svc.Set(ctx, key, map[string]interface{}{"completed": true}))
	got, err := svc.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"completed": true}, got)
}
