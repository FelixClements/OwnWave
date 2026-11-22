//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/testdb"
)

// The songs view is the one home of song identity (#111).
func TestSongsViewGivesACopyItsOriginalsSong(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewPool(t)
	one := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v))
		return v
	}
	song := func(trackID string) string {
		t.Helper()
		return one(`SELECT song_id::text || ' best=' || best_copy::text FROM songs WHERE track_id = $1::uuid`, trackID)
	}

	original := one(`INSERT INTO tracks (path, title) VALUES ('/m/song.flac', 'Song') RETURNING id::text`)
	cp := one(`INSERT INTO tracks (path, title, copy_of) VALUES ('/m/song.mp3', 'Song', $1::uuid) RETURNING id::text`, original)
	other := one(`INSERT INTO tracks (path, title) VALUES ('/m/other.flac', 'Other') RETURNING id::text`)

	require.Equal(t, original+" best=true", song(original))
	require.Equal(t, original+" best=false", song(cp), "a copy shares its original's song")
	require.Equal(t, other+" best=true", song(other))
	require.Equal(t, "3", one(`SELECT COUNT(*)::text FROM songs`), "one row per track")

	// The original goes: the copy is its own song, and its best copy.
	_, err := pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1::uuid`, original)
	require.NoError(t, err)
	require.Equal(t, cp+" best=true", song(cp))
	require.Equal(t, "2", one(`SELECT COUNT(*)::text FROM songs`))
}
