//go:build integration

package listening_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/listening"
	"ownwave/api/internal/testutil/testdb"
)

type fixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
	user string
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{t: t, pool: testdb.NewPool(t), ctx: context.Background()}
	f.user = f.newUser()
	return f
}

func (f *fixture) scalar(query string, args ...any) string {
	f.t.Helper()
	var v string
	require.NoError(f.t, f.pool.QueryRow(f.ctx, query, args...).Scan(&v))
	return v
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, query, args...)
	require.NoError(f.t, err)
}

func (f *fixture) newUser() string {
	return f.scalar(`INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id::text`, uuid.NewString())
}

func (f *fixture) addTrack(title string) string {
	f.t.Helper()
	return f.scalar(`INSERT INTO tracks (path, title, duration_seconds) VALUES ($1, $2, 180) RETURNING id::text`,
		fmt.Sprintf("/music/%s.flac", uuid.NewString()), title)
}

func TestRecord_WritesHistoryAndChecksStationOwner(t *testing.T) {
	f := newFixture(t)
	trackID := f.addTrack("History")
	station := f.scalar(`INSERT INTO stations (user_id, name) VALUES ($1::uuid, 'Mine') RETURNING id::text`, f.user)
	svc := listening.NewService(f.pool)

	require.NoError(t, svc.Record(f.ctx, f.user, trackID, ""))
	require.NoError(t, svc.Record(f.ctx, f.user, trackID, station))
	assert.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE track_id = $1::uuid`, trackID))
	assert.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE station_id = $1::uuid`, station))

	other := f.newUser()
	require.Error(t, svc.Record(f.ctx, other, trackID, station))
	assert.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE track_id = $1::uuid`, trackID))

	shared := f.scalar(`INSERT INTO stations (name) VALUES ('Shared') RETURNING id::text`)
	require.NoError(t, svc.Record(f.ctx, other, trackID, shared))
}
