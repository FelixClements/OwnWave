//go:build integration

package playlists_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/playlists"
	"ownwave/api/internal/spotify"
	"ownwave/api/internal/testdb"
)

const spotifyID = "37i9dQZF1DX6J5NfMJS675"

// fakeSpotify serves whatever playlist the test sets, or fails.
type fakeSpotify struct {
	playlist spotify.Playlist
	err      error
}

func (f *fakeSpotify) FetchPlaylist(_ context.Context, id string) (spotify.Playlist, error) {
	if f.err != nil {
		return spotify.Playlist{}, f.err
	}
	p := f.playlist
	p.ID = id
	return p, nil
}

func track(id, title string, artists ...string) spotify.Track {
	return spotify.Track{ID: id, Title: title, Artists: artists, DurationMS: 200000}
}

func TestSyncGrowsPlaylistAndMatchesNewFiles(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	addLocal := func(title, artist string) string {
		var artistID, id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO artists (name) VALUES ($1) RETURNING id::text`, artist).Scan(&artistID))
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, artist_id, title, duration_seconds)
			VALUES ($1, $2::uuid, $3, 200) RETURNING id::text`, "/m/"+title, artistID, title).Scan(&id))
		return id
	}
	domino := addLocal("Domino (Space 92 Remix)", "Oxia")

	fake := &fakeSpotify{playlist: spotify.Playlist{Name: "TECHNO BUNKER", Tracks: []spotify.Track{
		track("t-domino", "Domino - Space 92 Remix", "Oxia", "Space 92"),
		track("t-obsidian", "Obsidian", "Kevin de Vries"),
	}}}
	svc := playlists.NewService(pool, fake)

	p, err := svc.Add(ctx, "https://open.spotify.com/playlist/"+spotifyID+"?si=x")
	require.NoError(t, err)
	require.Equal(t, "TECHNO BUNKER", p.Name)
	require.Equal(t, 2, p.TrackCount)
	require.Equal(t, 1, p.MatchedCount)

	// Adding the same playlist again does not duplicate it.
	again, err := svc.Add(ctx, "spotify:playlist:"+spotifyID)
	require.NoError(t, err)
	require.Equal(t, p.ID, again.ID)

	// Spotify rotates: Domino is gone, a new track arrives. Domino stays.
	fake.playlist.Tracks = []spotify.Track{
		track("t-obsidian", "Obsidian", "Kevin de Vries"),
		track("t-new", "New One", "Somebody"),
	}
	p, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, 3, p.TrackCount)
	require.NotNil(t, p.LastSyncedAt)

	items, err := svc.Items(ctx, p.ID)
	require.NoError(t, err)
	byID := map[string]playlists.Item{}
	for _, it := range items {
		byID[it.SpotifyTrackID] = it
	}
	require.Equal(t, domino, *byID["t-domino"].TrackID)
	require.Equal(t, playlists.MethodExact, *byID["t-domino"].MatchMethod)
	require.True(t, byID["t-domino"].LastSeenAt.Before(byID["t-new"].LastSeenAt), "rotated-out track keeps its last_seen_at")
	require.Nil(t, byID["t-obsidian"].TrackID)

	// A failed fetch is recorded and changes nothing else.
	fake.err = &spotify.FetchError{Err: errors.New("spotify answered 500")}
	_, err = svc.Sync(ctx, p.ID)
	var fetchErr *spotify.FetchError
	require.ErrorAs(t, err, &fetchErr)
	p, err = svc.Get(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, 3, p.TrackCount)
	require.Equal(t, "spotify answered 500", *p.LastSyncError)

	// A file added later is matched on the next sync, which also clears the error.
	fake.err = nil
	obsidian := addLocal("Obsidian (Extended Mix)", "Kevin de Vries")
	require.NoError(t, svc.SyncStale(ctx, 0))
	p, err = svc.Get(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, 2, p.MatchedCount)
	require.Nil(t, p.LastSyncError)
	items, err = svc.Items(ctx, p.ID)
	require.NoError(t, err)
	for _, it := range items {
		if it.SpotifyTrackID == "t-obsidian" {
			require.Equal(t, obsidian, *it.TrackID)
		}
	}

	// Fresh playlists are not fetched, but new files still get matched.
	fake.err = errors.New("must not be fetched")
	newFile := addLocal("New One", "Somebody")
	require.NoError(t, svc.SyncStale(ctx, time.Hour))
	p, err = svc.Get(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, 3, p.MatchedCount)
	require.Nil(t, p.LastSyncError)
	items, err = svc.Items(ctx, p.ID)
	require.NoError(t, err)
	for _, it := range items {
		if it.SpotifyTrackID == "t-new" {
			require.Equal(t, newFile, *it.TrackID)
		}
	}
}

func TestDeleteRefusesPlaylistsInUse(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	svc := playlists.NewService(pool, &fakeSpotify{playlist: spotify.Playlist{Name: "Rock", Tracks: []spotify.Track{track("t1", "Song", "Band")}}})

	p, err := svc.Add(ctx, spotifyID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO stations (name, seed_features) VALUES ('Rock Station', $1::jsonb)`,
		`{"type":"playlists","playlist_ids":["`+p.ID+`"]}`)
	require.NoError(t, err)

	usedBy, err := svc.Delete(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Rock Station"}, usedBy)

	_, err = pool.Exec(ctx, `DELETE FROM stations`)
	require.NoError(t, err)
	usedBy, err = svc.Delete(ctx, p.ID)
	require.NoError(t, err)
	require.Empty(t, usedBy)
	_, err = svc.Get(ctx, p.ID)
	require.ErrorIs(t, err, playlists.ErrNotFound)
	_, err = svc.Delete(ctx, p.ID)
	require.ErrorIs(t, err, playlists.ErrNotFound)
}
