//go:build integration

package playlists_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/playlists"
	"ownwave/api/internal/spotify"
	"ownwave/api/internal/stations"
	"ownwave/api/internal/testdb"
)

func addLocalTrack(t *testing.T, pool *pgxpool.Pool, title, artist string) string {
	t.Helper()
	ctx := context.Background()
	var artistID, id string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO artists (name) VALUES ($1) RETURNING id::text`, artist).Scan(&artistID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, artist_id, title, duration_seconds)
		VALUES ($1, $2::uuid, $3, 200) RETURNING id::text`, "/m/"+artist+"/"+title, artistID, title).Scan(&id))
	return id
}

func itemsByID(t *testing.T, svc *playlists.Service, playlistID string, filter playlists.ItemFilter) map[string]playlists.Item {
	t.Helper()
	items, err := svc.Items(context.Background(), playlistID, filter)
	require.NoError(t, err)
	out := map[string]playlists.Item{}
	for _, it := range items {
		out[it.SpotifyTrackID] = it
	}
	return out
}

func TestManualMatchesSurviveSyncsAndCanBeReset(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	domino := addLocalTrack(t, pool, "Domino (Space 92 Remix)", "Oxia")
	other := addLocalTrack(t, pool, "Some Other Song", "Somebody Else")

	fake := &fakeSpotify{playlist: spotify.Playlist{Name: "TECHNO BUNKER", Tracks: []spotify.Track{
		track("t-domino", "Domino - Space 92 Remix", "Oxia", "Space 92"),
		track("t-obsidian", "Obsidian", "Kevin de Vries"),
	}}}
	svc := playlists.NewService(pool, fake)
	p, err := svc.Add(ctx, "spotify:playlist:"+spotifyID)
	require.NoError(t, err)
	require.Equal(t, 1, p.MatchedCount)

	// Pick a file by hand for the missing track.
	require.NoError(t, svc.SetMatch(ctx, p.ID, "t-obsidian", other))
	items := itemsByID(t, svc, p.ID, playlists.AllItems)
	require.Equal(t, other, *items["t-obsidian"].TrackID)
	require.Equal(t, "manual", *items["t-obsidian"].MatchMethod)
	require.True(t, items["t-obsidian"].Manual)
	require.False(t, items["t-domino"].Manual)

	// A wrong automatic match is cleared: it is missing and stays missing,
	// although its file still exists.
	require.NoError(t, svc.SetMatch(ctx, p.ID, "t-domino", ""))
	items = itemsByID(t, svc, p.ID, playlists.MissingItems)
	require.Contains(t, items, "t-domino")
	require.Nil(t, items["t-domino"].TrackID)
	require.Nil(t, items["t-domino"].MatchMethod)
	require.True(t, items["t-domino"].Manual, "marked as having no local file")

	// A sync, and a file the matcher would pick, change neither.
	addLocalTrack(t, pool, "Obsidian (Extended Mix)", "Kevin de Vries")
	_, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	require.NoError(t, svc.SyncStale(ctx, 0))
	items = itemsByID(t, svc, p.ID, playlists.AllItems)
	require.Equal(t, other, *items["t-obsidian"].TrackID)
	require.Nil(t, items["t-domino"].TrackID)
	p, err = svc.Get(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, 1, p.MatchedCount)

	// Reset hands it back to the matcher, which finds the file again.
	require.NoError(t, svc.ResetMatch(ctx, p.ID, "t-domino"))
	items = itemsByID(t, svc, p.ID, playlists.AllItems)
	require.Equal(t, domino, *items["t-domino"].TrackID)
	require.False(t, items["t-domino"].Manual)

	// Resetting an automatic match changes nothing.
	require.NoError(t, svc.ResetMatch(ctx, p.ID, "t-domino"))
	require.Equal(t, domino, *itemsByID(t, svc, p.ID, playlists.AllItems)["t-domino"].TrackID)

	// A manual match whose file is deleted goes back to the matcher.
	_, err = pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1::uuid`, other)
	require.NoError(t, err)
	_, err = svc.MatchPending(ctx)
	require.NoError(t, err)
	items = itemsByID(t, svc, p.ID, playlists.AllItems)
	require.NotNil(t, items["t-obsidian"].TrackID, "matched automatically to the Extended Mix")
	require.False(t, items["t-obsidian"].Manual)
}

func TestHiddenSongsLeaveStationsAndStayHidden(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	addLocalTrack(t, pool, "Domino", "Oxia")
	addLocalTrack(t, pool, "Obsidian", "Kevin de Vries")

	fake := &fakeSpotify{playlist: spotify.Playlist{Name: "Bunker", Tracks: []spotify.Track{
		track("t-domino", "Domino", "Oxia"),
		track("t-obsidian", "Obsidian", "Kevin de Vries"),
		track("t-missing", "Not Here", "Nobody"),
	}}}
	svc := playlists.NewService(pool, fake)
	p, err := svc.Add(ctx, "spotify:playlist:"+spotifyID)
	require.NoError(t, err)
	require.Equal(t, 3, p.TrackCount)
	require.Equal(t, 2, p.MatchedCount)

	poolSize := func() int {
		var args stations.Args
		where := stations.Definition{PlaylistIDs: []string{p.ID}}.Where(&args)
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM tracks t WHERE `+where, args...).Scan(&n))
		return n
	}
	require.Equal(t, 2, poolSize())

	require.NoError(t, svc.SetHidden(ctx, p.ID, "t-obsidian", true))
	require.NoError(t, svc.SetHidden(ctx, p.ID, "t-missing", true))
	require.Equal(t, 1, poolSize(), "the hidden song left the station")

	p, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, 1, p.TrackCount, "hidden songs are not counted")
	require.Equal(t, 1, p.MatchedCount)
	require.Equal(t, 2, p.HiddenCount)

	hidden := itemsByID(t, svc, p.ID, playlists.HiddenItems)
	require.Len(t, hidden, 2, "still hidden after a sync")
	require.True(t, hidden["t-obsidian"].Hidden)
	require.NotContains(t, itemsByID(t, svc, p.ID, playlists.MatchedItems), "t-obsidian")
	require.NotContains(t, itemsByID(t, svc, p.ID, playlists.MissingItems), "t-missing")

	require.NoError(t, svc.SetHidden(ctx, p.ID, "t-obsidian", false))
	require.Equal(t, 2, poolSize())
	require.Contains(t, itemsByID(t, svc, p.ID, playlists.MatchedItems), "t-obsidian")
}

func TestManualEditsRejectUnknownIDs(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	fake := &fakeSpotify{playlist: spotify.Playlist{Name: "Bunker", Tracks: []spotify.Track{track("t-1", "One", "A")}}}
	svc := playlists.NewService(pool, fake)
	p, err := svc.Add(ctx, "spotify:playlist:"+spotifyID)
	require.NoError(t, err)
	unknown := "00000000-0000-0000-0000-000000000001"

	require.ErrorIs(t, svc.SetMatch(ctx, unknown, "t-1", ""), playlists.ErrNotFound)
	require.ErrorIs(t, svc.SetMatch(ctx, p.ID, "t-other", ""), playlists.ErrItemNotFound)
	require.ErrorIs(t, svc.SetMatch(ctx, p.ID, "t-1", unknown), playlists.ErrTrackNotFound)
	require.ErrorIs(t, svc.ResetMatch(ctx, p.ID, "t-other"), playlists.ErrItemNotFound)
	require.ErrorIs(t, svc.SetHidden(ctx, p.ID, "t-other", true), playlists.ErrItemNotFound)
	require.ErrorIs(t, svc.SetHidden(ctx, unknown, "t-1", true), playlists.ErrNotFound)
}
