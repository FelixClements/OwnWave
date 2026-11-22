//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
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
	cp := one(`INSERT INTO tracks (path, title) VALUES ('/m/song.mp3', 'Song') RETURNING id::text`)
	_, err := pool.Exec(ctx, `INSERT INTO track_copies (track_id, copy_of) VALUES ($1::uuid, $2::uuid)`, cp, original)
	require.NoError(t, err)
	other := one(`INSERT INTO tracks (path, title) VALUES ('/m/other.flac', 'Other') RETURNING id::text`)

	require.Equal(t, original+" best=true", song(original))
	require.Equal(t, original+" best=false", song(cp), "a copy shares its original's song")
	require.Equal(t, other+" best=true", song(other))
	require.Equal(t, "3", one(`SELECT COUNT(*)::text FROM songs`), "one row per track")

	// The original goes: the copy is its own song, and its best copy, as
	// tracks.copy_of's ON DELETE SET NULL had it before #112.
	_, err = pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1::uuid`, original)
	require.NoError(t, err)
	require.Equal(t, cp+" best=true", song(cp))
	require.Equal(t, "2", one(`SELECT COUNT(*)::text FROM songs`))
	require.Equal(t, "0", one(`SELECT COUNT(*)::text FROM track_copies`))
}

// Migration 028 (#112) moves copy_of from tracks into track_copies. The
// songs view gives the same rows before it, after it and after its down.
func TestTrackCopiesMigrationKeepsSongs(t *testing.T) {
	ctx := context.Background()
	pool, migrateTo := testdb.NewPoolAt(t, 27)
	one := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v))
		return v
	}
	add := func(path string, copyOf any) string {
		t.Helper()
		return one(`INSERT INTO tracks (path, title, copy_of) VALUES ($1, 'Song', $2::uuid) RETURNING id::text`, path, copyOf)
	}
	songs := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT track_id::text || ' ' || song_id::text || ' ' || best_copy::text
			FROM songs ORDER BY track_id`)
		require.NoError(t, err)
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		return out
	}
	hasColumn := func() bool {
		t.Helper()
		return one(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_name = 'tracks' AND column_name = 'copy_of')::text`) == "true"
	}

	best := add("/m/a.flac", nil)
	add("/m/a.mp3", best)
	add("/m/a.ogg", best)
	add("/m/b.flac", nil)
	before := songs()
	require.Len(t, before, 4)

	migrateTo(28)
	require.False(t, hasColumn(), "copy_of left tracks")
	require.Equal(t, "2", one(`SELECT COUNT(*)::text FROM track_copies WHERE copy_of = $1::uuid`, best))
	require.Equal(t, before, songs())

	migrateTo(27)
	require.True(t, hasColumn())
	require.Equal(t, before, songs(), "the down migration fills copy_of back")
}
