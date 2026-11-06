//go:build integration

package playlists_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
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
	images   map[string][]byte // cover URL -> image; others fail
	fetched  []string          // image URLs requested
}

func (f *fakeSpotify) FetchImage(_ context.Context, url string) ([]byte, string, error) {
	f.fetched = append(f.fetched, url)
	img, ok := f.images[url]
	if !ok {
		return nil, "", errors.New("spotify answered 404")
	}
	return img, "image/png", nil
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

	items, err := svc.Items(ctx, p.ID, playlists.AllItems)
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
	items, err = svc.Items(ctx, p.ID, playlists.AllItems)
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
	items, err = svc.Items(ctx, p.ID, playlists.AllItems)
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
	_, err = pool.Exec(ctx, `
		WITH s AS (INSERT INTO stations (name) VALUES ('Rock Station') RETURNING id)
		INSERT INTO station_playlists (station_id, playlist_id) SELECT id, $1::uuid FROM s`, p.ID)
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

func TestSyncDownloadsCoverOnceAndKeepsItOnFailure(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	fake := &fakeSpotify{
		playlist: spotify.Playlist{Name: "Mix", Tracks: []spotify.Track{track("t1", "Song", "Band")}},
		images:   map[string][]byte{"https://i.scdn.co/image/a": []byte("cover-a"), "https://i.scdn.co/image/b": []byte("cover-b")},
	}
	svc := playlists.NewService(pool, fake)

	// No cover listed: none stored.
	p, err := svc.Add(ctx, spotifyID)
	require.NoError(t, err)
	require.False(t, p.HasCover)
	img, _, err := svc.Cover(ctx, p.ID)
	require.NoError(t, err)
	require.Nil(t, img)

	fake.playlist.CoverURL = "https://i.scdn.co/image/a"
	p, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	require.True(t, p.HasCover)
	img, kind, err := svc.Cover(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("cover-a"), img)
	require.Equal(t, "image/png", kind)

	// Same cover: not downloaded again.
	_, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"https://i.scdn.co/image/a"}, fake.fetched)

	// A new cover that fails to download keeps the old one; the sync succeeds.
	fake.playlist.CoverURL = "https://i.scdn.co/image/broken"
	p, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	require.Nil(t, p.LastSyncError)
	img, _, err = svc.Cover(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("cover-a"), img)

	// A new cover replaces the old one.
	fake.playlist.CoverURL = "https://i.scdn.co/image/b"
	_, err = svc.Sync(ctx, p.ID)
	require.NoError(t, err)
	img, _, err = svc.Cover(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("cover-b"), img)

	_, _, err = svc.Cover(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, playlists.ErrNotFound)
}

func TestGroupCopiesRecordsAndFollowsTheLibrary(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewPool(t)
	svc := playlists.NewService(pool, &fakeSpotify{})
	add := func(path, title string, rate int) string {
		var artist, id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO artists (name) VALUES ('X')
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id::text`).Scan(&artist))
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, title, artist_id, duration_seconds, sample_rate)
			VALUES ($1, $2, $3::uuid, 200, $4) RETURNING id::text`, path, title, artist, rate).Scan(&id))
		return id
	}
	copyOf := func(id string) string {
		var v string
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(copy_of::text, '') FROM tracks WHERE id = $1::uuid`, id).Scan(&v))
		return v
	}
	low := add("/m/low.mp3", "Song", 44100)
	high := add("/m/high.flac", "Song", 96000)
	add("/m/other.flac", "Other Song", 44100)

	n, err := svc.GroupCopies(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, high, copyOf(low))
	require.Equal(t, "", copyOf(high))

	n, err = svc.GroupCopies(ctx)
	require.NoError(t, err)
	require.Zero(t, n, "nothing changed")

	// A better file arrives: the song's best copy moves, in one step.
	best := add("/m/best.flac", "Song", 192000)
	n, err = svc.GroupCopies(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, best, copyOf(low))
	require.Equal(t, best, copyOf(high))

	// The best file goes: the others are their own song until the next run.
	_, err = pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1::uuid`, best)
	require.NoError(t, err)
	require.Equal(t, "", copyOf(low))
	_, err = svc.GroupCopies(ctx)
	require.NoError(t, err)
	require.Equal(t, high, copyOf(low))
}

// vec is a unit embedding at the given angle (degrees) in the first plane:
// the cosine similarity of two is the cosine of their angle's difference.
func vec(deg float64) string {
	parts := make([]string, 1280)
	for i := range parts {
		parts[i] = "0"
	}
	parts[0] = fmt.Sprint(math.Cos(deg * math.Pi / 180))
	parts[1] = fmt.Sprint(math.Sin(deg * math.Pi / 180))
	return "[" + strings.Join(parts, ",") + "]"
}

func TestRebuildPicksFindsSimilarLibrarySongs(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewPool(t)
	svc := playlists.NewService(pool, &fakeSpotify{})
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	add := func(title string, deg float64) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, title) VALUES ($1, $2) RETURNING id::text`, "/m/"+title, title).Scan(&id))
		exec(`INSERT INTO track_audio_genres (track_id, model_version, activations, embedding) VALUES ($1::uuid, 'test', '{}', $2::vector)`, id, vec(deg))
		return id
	}
	var playlist string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO playlists (spotify_id, name) VALUES ('s', 'List') RETURNING id::text`).Scan(&playlist))
	seed := func(id, sp string) {
		exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title) VALUES ($1::uuid, $2, $2)`, playlist, sp)
		exec(`INSERT INTO spotify_track_matches (spotify_track_id, track_id, method) VALUES ($1, $2::uuid, 'exact')`, sp, id)
	}
	picked := func() []string {
		t.Helper()
		list, err := svc.Picks(ctx, playlist)
		require.NoError(t, err)
		out := []string{}
		for _, p := range list {
			out = append(out, p.Title)
		}
		return out
	}

	one, two := add("seed-1", 0), add("seed-2", 10)
	seed(one, "sp1")
	seed(two, "sp2")
	add("near", 4)
	add("close", 15)
	add("third", 20)
	add("far", 80) // below the similarity floor: cos(70) is about 0.34

	n, err := svc.RebuildPicks(ctx, true)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, []string{"near", "close"}, picked(), "no more picks than the playlist has songs")

	n, err = svc.RebuildPicks(ctx, true)
	require.NoError(t, err)
	require.Zero(t, n, "same input, no rebuild")

	// A library change waits for a background run; a playlist's own does not.
	add("later", 170)
	n, err = svc.RebuildPicks(ctx, false)
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = svc.RebuildPicks(ctx, true)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got, err := svc.Get(ctx, playlist)
	require.NoError(t, err)
	require.Equal(t, 2, got.PickCount)

	// An admin's removal lasts, and the next song takes the place.
	var near string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM tracks WHERE title = 'near'`).Scan(&near))
	require.NoError(t, svc.RemovePick(ctx, playlist, near))
	require.ErrorIs(t, svc.RemovePick(ctx, playlist, near), playlists.ErrNotFound)
	exec(`UPDATE playlists SET picks_input = NULL`)
	_, err = svc.RebuildPicks(ctx, true)
	require.NoError(t, err)
	require.Equal(t, []string{"close", "third"}, picked())

	// A copy of a pick plays as that pick; a copy never is a pick itself.
	var close string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM tracks WHERE title = 'close'`).Scan(&close))
	third := ""
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM tracks WHERE title = 'third'`).Scan(&third))
	exec(`UPDATE tracks SET copy_of = $1::uuid WHERE id = $2::uuid`, close, third)
	_, err = svc.RebuildPicks(ctx, true)
	require.NoError(t, err)
	require.Equal(t, []string{"close"}, picked())

	// New matches change the seeds: a pick that became a seed stops being a pick.
	seed(close, "sp3")
	n, err = svc.RebuildPicks(ctx, false)
	require.NoError(t, err)
	require.Equal(t, 1, n, "new seeds rebuild even in a request")
	require.NotContains(t, picked(), "close")
}
