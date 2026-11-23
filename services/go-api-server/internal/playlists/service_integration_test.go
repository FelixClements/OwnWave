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
	"ownwave/api/internal/testutil/testdb"
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
		require.NoError(t, pool.QueryRow(ctx, `SELECT CASE WHEN best_copy THEN '' ELSE song_id::text END FROM songs WHERE track_id = $1::uuid`, id).Scan(&v))
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

// vecIn is a unit embedding at the given angle (degrees) in one plane of
// the embedding space: the cosine similarity of two in the same plane is
// the cosine of their angle's difference, and of two in different planes 0.
func vecIn(plane int, deg float64) string {
	parts := make([]string, 1280)
	for i := range parts {
		parts[i] = "0"
	}
	parts[2*plane] = fmt.Sprint(math.Cos(deg * math.Pi / 180))
	parts[2*plane+1] = fmt.Sprint(math.Sin(deg * math.Pi / 180))
	return "[" + strings.Join(parts, ",") + "]"
}

func vec(deg float64) string { return vecIn(0, deg) }

func TestRebuildPicksFindsSimilarLibrarySongs(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewPool(t)
	svc := playlists.NewService(pool, &fakeSpotify{})
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	addIn := func(plane int, title string, deg float64) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, title) VALUES ($1, $2) RETURNING id::text`, "/m/"+title, title).Scan(&id))
		exec(`INSERT INTO track_audio_genres (track_id, model_version, activations, embedding) VALUES ($1::uuid, 'test', '{}', $2::vector)`, id, vecIn(plane, deg))
		return id
	}
	add := func(title string, deg float64) string { return addIn(0, title, deg) }
	trackID := func(title string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM tracks WHERE title = $1`, title).Scan(&id))
		return id
	}
	newPlaylist := func(name string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO playlists (spotify_id, name) VALUES ($1, $1) RETURNING id::text`, name).Scan(&id))
		return id
	}
	seedIn := func(playlist, id, sp string) {
		exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title) VALUES ($1::uuid, $2, $2)`, playlist, sp)
		exec(`INSERT INTO spotify_track_matches (spotify_track_id, track_id, method) VALUES ($1, $2::uuid, 'exact')`, sp, id)
	}
	pickedIn := func(playlist string) []string {
		t.Helper()
		list, err := svc.Picks(ctx, playlist)
		require.NoError(t, err)
		out := []string{}
		for _, p := range list {
			out = append(out, p.Title)
		}
		return out
	}
	// exactPicks follows the rules by brute force, as rebuilds did before
	// the neighbour lists: each playlist song's nearest songs by an exact
	// scan. Titles here are unique, so choosePicks only applies the limit.
	exactPicks := func(playlist string) []string {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, `SET LOCAL enable_indexscan = off`)
		require.NoError(t, err)
		rows, err := tx.Query(ctx, `
			WITH seeds AS (
				SELECT DISTINCT ms.song_id AS id
				FROM playlist_items pi
				JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
				JOIN songs ms ON ms.track_id = m.track_id
				WHERE pi.playlist_id = $1::uuid AND NOT pi.hidden)
			SELECT t.title, (SELECT COUNT(*) FROM seeds)
			FROM (
				SELECT nb.track_id, COUNT(*) AS hits, MAX(nb.similarity)::real AS best
				FROM seeds s
				JOIN track_audio_genres sg ON sg.track_id = s.id AND sg.embedding IS NOT NULL
				CROSS JOIN LATERAL (
					SELECT c.track_id, 1 - (c.embedding <=> sg.embedding) AS similarity
					FROM track_audio_genres c
					WHERE c.track_id NOT IN (SELECT id FROM seeds)
					  AND c.track_id NOT IN (SELECT r.track_id FROM playlist_pick_removals r WHERE r.playlist_id = $1::uuid)
					  AND NOT EXISTS (SELECT 1 FROM track_copies x WHERE x.track_id = c.track_id)
					ORDER BY c.embedding <=> sg.embedding
					LIMIT 5
				) nb
				WHERE nb.similarity >= 0.7
				GROUP BY nb.track_id
			) n
			JOIN tracks t ON t.id = n.track_id
			ORDER BY n.hits DESC, n.best DESC, t.id
		`, playlist)
		require.NoError(t, err)
		out := []string{}
		var title string
		var limit int
		for rows.Next() {
			require.NoError(t, rows.Scan(&title, &limit))
			if len(out) < limit {
				out = append(out, title)
			}
		}
		require.NoError(t, rows.Err())
		return out
	}
	rebuild := func(followLibrary bool) int {
		t.Helper()
		n, err := svc.RebuildPicks(ctx, followLibrary)
		require.NoError(t, err)
		return n
	}
	input := func(playlist string) string {
		t.Helper()
		var in string
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(picks_input, '') FROM playlists WHERE id = $1::uuid`, playlist).Scan(&in))
		return in
	}

	playlist := newPlaylist("List")
	seed := func(id, sp string) { seedIn(playlist, id, sp) }
	picked := func() []string { return pickedIn(playlist) }
	one, two := add("seed-1", 0), add("seed-2", 10)
	seed(one, "sp1")
	seed(two, "sp2")
	add("near", 4)
	add("close", 15)
	add("third", 20)
	add("far", 80) // below the similarity floor: cos(70) is about 0.34

	// A second playlist sounds different: no song is near both.
	other := newPlaylist("Other")
	seedIn(other, addIn(1, "other-1", 0), "sq1")
	seedIn(other, addIn(1, "other-2", 10), "sq2")
	addIn(1, "other-near", 5)

	require.Equal(t, 2, rebuild(true))
	require.Equal(t, []string{"near", "close"}, picked(), "no more picks than the playlist has songs")
	require.Equal(t, exactPicks(playlist), picked())
	require.Equal(t, []string{"other-near"}, pickedIn(other))
	require.Zero(t, rebuild(true), "same input, no rebuild")

	// A song near no playlist song rebuilds nothing.
	add("later", 170)
	require.Zero(t, rebuild(false))
	require.Zero(t, rebuild(true))

	// A library change waits for a background run; then it rebuilds only
	// the playlists whose songs it is near.
	otherInput := input(other)
	add("fresh", 2)
	require.Zero(t, rebuild(false))
	require.Equal(t, 1, rebuild(true))
	require.Equal(t, []string{"fresh", "near"}, picked())
	require.Equal(t, exactPicks(playlist), picked())
	require.Equal(t, otherInput, input(other), "the other playlist was not rebuilt")

	addIn(1, "other-close", 3)
	require.Equal(t, 1, rebuild(true))
	require.Equal(t, []string{"other-close", "other-near"}, pickedIn(other))
	require.Equal(t, exactPicks(other), pickedIn(other))

	got, err := svc.Get(ctx, playlist)
	require.NoError(t, err)
	require.Equal(t, 2, got.PickCount)

	// An admin's removal lasts, and the next song takes the place, also
	// in a request: the removal is the playlist's own change.
	fresh := trackID("fresh")
	require.NoError(t, svc.RemovePick(ctx, playlist, fresh))
	require.ErrorIs(t, svc.RemovePick(ctx, playlist, fresh), playlists.ErrNotFound)
	require.Equal(t, 1, rebuild(false))
	require.Equal(t, []string{"near", "close"}, picked())
	require.Equal(t, exactPicks(playlist), picked())

	// A deleted song leaves the lists it was in.
	exec(`DELETE FROM tracks WHERE id = $1::uuid`, trackID("near"))
	require.Equal(t, 1, rebuild(true))
	require.Equal(t, []string{"close", "third"}, picked())
	require.Equal(t, exactPicks(playlist), picked())

	// A copy of a pick plays as that pick; a copy never is a pick itself.
	exec(`INSERT INTO track_copies (track_id, copy_of) VALUES ($1::uuid, $2::uuid)`, trackID("third"), trackID("close"))
	require.Equal(t, 1, rebuild(true))
	require.Equal(t, []string{"close"}, picked())
	require.Equal(t, exactPicks(playlist), picked())

	// A song analysed again moves to where its new embedding is.
	exec(`UPDATE track_audio_genres SET embedding = $2::vector, analyzed_at = NOW() WHERE track_id = $1::uuid`, trackID("later"), vec(12))
	require.Equal(t, 1, rebuild(true))
	require.Equal(t, []string{"later", "close"}, picked())
	require.Equal(t, exactPicks(playlist), picked())

	// New matches change the seeds: a pick that became a seed stops being a pick.
	seed(trackID("close"), "sp3")
	require.Equal(t, 1, rebuild(false), "new seeds rebuild even in a request")
	require.NotContains(t, picked(), "close")
	require.Equal(t, exactPicks(playlist), picked())

	// A playlist of songs that all sound alike: each song's nearest are the
	// playlist's own, so the lists of the first songs grow until they reach
	// songs that are not in the playlist.
	dense := newPlaylist("Dense")
	for i := range 24 {
		seedIn(dense, addIn(2, fmt.Sprintf("dense-%02d", i), float64(i)/10), fmt.Sprintf("sd%d", i))
	}
	addIn(2, "dense-pick", 3)
	addIn(2, "dense-pick-2", 3.2)
	require.Equal(t, 1, rebuild(true))
	require.Equal(t, []string{"dense-pick", "dense-pick-2"}, pickedIn(dense))
	require.Equal(t, exactPicks(dense), pickedIn(dense))
	var depth int
	require.NoError(t, pool.QueryRow(ctx, `SELECT depth FROM song_neighbour_lists WHERE song_id = $1::uuid`, trackID("dense-00")).Scan(&depth))
	require.Greater(t, depth, 20, "the list grew")
	require.Zero(t, rebuild(true), "a grown list rebuilds no other playlist")

	// The lists can be rebuilt from nothing, and new rules rebuild them all:
	// the picks stay the same.
	all := map[string][]string{}
	for _, p := range []string{playlist, other, dense} {
		all[p] = pickedIn(p)
	}
	exec(`DELETE FROM song_neighbour_lists`)
	require.Equal(t, 3, rebuild(true))
	exec(`UPDATE song_neighbour_lists SET version = 0`)
	rebuild(true)
	for p, want := range all {
		require.Equal(t, want, pickedIn(p))
	}
	require.Zero(t, rebuild(true))
}
