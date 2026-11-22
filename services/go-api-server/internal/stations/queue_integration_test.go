//go:build integration

package stations_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/stations"
	"ownwave/api/internal/testutil/testdb"
)

// These tests seed tracks, playlists, likes, bans, hides and history,
// call Queue, and check which songs come back and in what order.

type fixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
	user string
	svc  *stations.Service
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{t: t, pool: testdb.NewPool(t), ctx: context.Background()}
	f.user = f.newUser()
	f.svc = stations.NewService(f.pool)
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

type track struct {
	title, artist, album string
	genres               []string // station genres; the first is primary
	noFeatures           bool
}

func (f *fixture) addTrack(tr track) string {
	f.t.Helper()
	if tr.artist == "" {
		tr.artist = "Artist " + uuid.NewString()[:8]
	}
	if tr.album == "" {
		tr.album = "Album"
	}
	artistID := f.scalar(`INSERT INTO artists (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id::text`, tr.artist)
	albumID := f.scalar(`INSERT INTO albums (artist_id, title) VALUES ($1::uuid, $2)
		ON CONFLICT (artist_id, title) DO UPDATE SET title = EXCLUDED.title RETURNING id::text`, artistID, tr.album)
	id := f.scalar(`INSERT INTO tracks (path, artist_id, album_id, title, duration_seconds)
		VALUES ($1, $2::uuid, $3::uuid, $4, 180) RETURNING id::text`,
		fmt.Sprintf("/music/%s.flac", uuid.NewString()), artistID, albumID, tr.title)
	if !tr.noFeatures {
		f.exec(`INSERT INTO audio_features (track_id, bpm, key, energy, valence, loudness)
			VALUES ($1::uuid, 120, 'C', 0.5, 0.5, -10)`, id)
	}
	for i, g := range tr.genres {
		f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, $2, 0.5, $3)`,
			id, g, i == 0)
	}
	return id
}

// playlist stores a playlist whose items map Spotify track ids to local
// track ids ("" = no local file).
func (f *fixture) playlist(name string, items map[string]string) string {
	id := f.scalar(`INSERT INTO playlists (spotify_id, name) VALUES ($1, $2) RETURNING id::text`, uuid.NewString(), name)
	for spotifyID, trackID := range items {
		f.exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title) VALUES ($1::uuid, $2, $2)`, id, spotifyID)
		f.exec(`INSERT INTO spotify_track_matches (spotify_track_id, track_id, method) VALUES ($1, NULLIF($2, '')::uuid, 'exact')
			ON CONFLICT (spotify_track_id) DO NOTHING`, spotifyID, trackID)
	}
	return id
}

// playlistOf puts every given track in one new playlist.
func (f *fixture) playlistOf(trackIDs ...string) string {
	items := map[string]string{}
	for _, id := range trackIDs {
		items["sp-"+id] = id
	}
	return f.playlist(uuid.NewString(), items)
}

// station stores a custom station of the fixture's user.
func (f *fixture) station(def stations.Definition) string {
	id := f.scalar(`INSERT INTO stations (user_id, name, genres, all_playlists, enhance)
		VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`,
		f.user, uuid.NewString(), append([]string{}, def.Genres...), def.AllPlaylists, def.Enhance)
	for _, p := range def.PlaylistIDs {
		f.exec(`INSERT INTO station_playlists (station_id, playlist_id) VALUES ($1::uuid, $2::uuid)`, id, p)
	}
	return id
}

// genreStation stores a shared genre station; active is whether it shows.
func (f *fixture) genreStation(genre string, active bool) string {
	return f.scalar(`INSERT INTO stations (user_id, name, genres, auto_genre, auto_active)
		VALUES (NULL, $1, ARRAY[$1], $1, $2) RETURNING id::text`, genre, active)
}

func (f *fixture) queue(svc *stations.Service, userID, stationID string) []stations.QueueTrack {
	f.t.Helper()
	queue, err := svc.Queue(f.ctx, userID, stationID)
	require.NoError(f.t, err)
	for i, q := range queue {
		assert.Equal(f.t, i, q.Position)
	}
	return queue
}

func (f *fixture) queueIDs(stationID string) []string {
	f.t.Helper()
	return ids(f.queue(f.svc, f.user, stationID))
}

func ids(queue []stations.QueueTrack) []string {
	out := make([]string, len(queue))
	for i, q := range queue {
		out[i] = q.ID
	}
	return out
}

func sorted(ids ...string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func sortedOrNil(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	return sorted(ids...)
}

func TestQueue_PlaylistPoolsAndGenres(t *testing.T) {
	f := newFixture(t)
	techno := f.addTrack(track{title: "Techno", genres: []string{"techno"}})
	technoHouse := f.addTrack(track{title: "Techno House", genres: []string{"house", "techno"}})
	house := f.addTrack(track{title: "House", genres: []string{"house"}})
	unscored := f.addTrack(track{title: "Unscored"})
	f.addTrack(track{title: "Not in a playlist", genres: []string{"techno"}})
	// Two playlists sharing a track; one item has no local file.
	a := f.playlist("a", map[string]string{"sp-1": techno, "sp-2": technoHouse, "sp-missing": ""})
	b := f.playlist("b", map[string]string{"sp-1": techno, "sp-3": house, "sp-4": unscored})

	cases := map[string]struct {
		def  stations.Definition
		want []string
	}{
		"one playlist":         {stations.Definition{PlaylistIDs: []string{a}}, sorted(techno, technoHouse)},
		"playlists union":      {stations.Definition{PlaylistIDs: []string{a, b}}, sorted(techno, technoHouse, house, unscored)},
		"genre incl secondary": {stations.Definition{PlaylistIDs: []string{a, b}, Genres: []string{"techno"}}, sorted(techno, technoHouse)},
		"several genres":       {stations.Definition{PlaylistIDs: []string{b}, Genres: []string{"house", "techno"}}, sorted(techno, house)},
		"genre not in pool":    {stations.Definition{PlaylistIDs: []string{a}, Genres: []string{"jazz"}}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, sortedOrNil(f.queueIDs(f.station(tc.def))))
		})
	}
}

func TestQueue_AllPlaylistsPlaysEveryPlaylistIncludingNewOnes(t *testing.T) {
	f := newFixture(t)
	first := f.addTrack(track{title: "First", genres: []string{"house"}})
	techno := f.addTrack(track{title: "Techno", genres: []string{"techno"}})
	f.playlistOf(first, techno)
	all := f.station(stations.Definition{AllPlaylists: true})
	allHouse := f.station(stations.Definition{AllPlaylists: true, Genres: []string{"house"}})
	assert.Equal(t, sorted(first, techno), sorted(f.queueIDs(all)...))
	assert.Equal(t, []string{first}, f.queueIDs(allHouse))

	later := f.addTrack(track{title: "Later", genres: []string{"house"}})
	f.playlistOf(later)
	assert.Equal(t, sorted(first, techno, later), sorted(f.queueIDs(all)...), "a playlist added later joins")
	assert.Equal(t, sorted(first, later), sorted(f.queueIDs(allHouse)...))
}

func TestQueue_HiddenPlaylistItemsLeaveTheStation(t *testing.T) {
	f := newFixture(t)
	kept := f.addTrack(track{title: "Kept"})
	hidden := f.addTrack(track{title: "Hidden"})
	p := f.playlistOf(kept, hidden)
	station := f.station(stations.Definition{PlaylistIDs: []string{p}})
	f.exec(`UPDATE playlist_items SET hidden = TRUE WHERE playlist_id = $1::uuid AND spotify_track_id = $2`, p, "sp-"+hidden)
	assert.Equal(t, []string{kept}, f.queueIDs(station))
}

func TestQueue_GenreStationPlaysTheLibraryByMainGenre(t *testing.T) {
	f := newFixture(t)
	main := f.addTrack(track{title: "Main", genres: []string{"techno"}})
	f.addTrack(track{title: "Secondary", genres: []string{"house", "techno"}})
	f.addTrack(track{title: "Other", genres: []string{"house"}})
	f.addTrack(track{title: "Unscored"})
	// In no playlist: a genre station does not need one.
	assert.Equal(t, []string{main}, f.queueIDs(f.genreStation("techno", true)))
}

func TestQueue_EnhancePlaysPicksOfItsOwnPlaylists(t *testing.T) {
	f := newFixture(t)
	listed := f.addTrack(track{title: "Listed"})
	pick := f.addTrack(track{title: "Pick"})
	otherPick := f.addTrack(track{title: "Other pick"})
	p := f.playlistOf(listed)
	other := f.playlistOf(f.addTrack(track{title: "Elsewhere"}))
	f.exec(`INSERT INTO playlist_library_picks (playlist_id, track_id, hits, similarity)
		VALUES ($1::uuid, $2::uuid, 3, 0.9), ($3::uuid, $4::uuid, 3, 0.9)`, p, pick, other, otherPick)

	plain := f.station(stations.Definition{PlaylistIDs: []string{p}})
	enhanced := f.station(stations.Definition{PlaylistIDs: []string{p}, Enhance: true})
	assert.Equal(t, []string{listed}, f.queueIDs(plain), "picks only with Enhance")
	assert.Equal(t, sorted(listed, pick), sorted(f.queueIDs(enhanced)...))

	all := f.station(stations.Definition{AllPlaylists: true, Enhance: true})
	assert.Contains(t, f.queueIDs(all), otherPick, "every playlist's picks")
}

func TestQueue_ExcludesBansAndFlagsLikes(t *testing.T) {
	f := newFixture(t)
	liked := f.addTrack(track{title: "Liked"})
	banned := f.addTrack(track{title: "Banned"})
	plain := f.addTrack(track{title: "Plain"})
	f.exec(`INSERT INTO track_feedback (user_id, track_id, feedback) VALUES ($1::uuid, $2::uuid, 'like'), ($1::uuid, $3::uuid, 'ban')`,
		f.user, liked, banned)
	station := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(liked, banned, plain)}})

	likedByID := map[string]bool{}
	for _, q := range f.queue(f.svc, f.user, station) {
		likedByID[q.ID] = q.Liked
	}
	assert.Equal(t, map[string]bool{liked: true, plain: false}, likedByID)
}

func TestQueue_HidesCountOnlyOnTheirStation(t *testing.T) {
	f := newFixture(t)
	kept := f.addTrack(track{title: "Kept"})
	hidden := f.addTrack(track{title: "Hidden"})
	p := f.playlistOf(kept, hidden)
	a := f.station(stations.Definition{PlaylistIDs: []string{p}})
	b := f.station(stations.Definition{PlaylistIDs: []string{p}})
	f.exec(`INSERT INTO station_hidden_tracks (user_id, station_id, track_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, f.user, a, hidden)
	assert.Equal(t, []string{kept}, f.queueIDs(a))
	assert.Equal(t, sorted(kept, hidden), sorted(f.queueIDs(b)...))
}

func TestQueue_RecentlyPlayedGoLastOldestFirst(t *testing.T) {
	f := newFixture(t)
	fresh := f.addTrack(track{title: "Fresh"})
	playedLongAgo := f.addTrack(track{title: "Long ago"})
	playedHourAgo := f.addTrack(track{title: "Hour ago"})
	playedJustNow := f.addTrack(track{title: "Just now"})
	playedElsewhere := f.addTrack(track{title: "Elsewhere"})
	p := f.playlistOf(fresh, playedLongAgo, playedHourAgo, playedJustNow, playedElsewhere)
	station := f.station(stations.Definition{PlaylistIDs: []string{p}})
	elsewhere := f.station(stations.Definition{PlaylistIDs: []string{p}})

	now := time.Now()
	for id, at := range map[string]time.Time{
		playedLongAgo: now.Add(-72 * time.Hour),
		playedHourAgo: now.Add(-time.Hour),
		playedJustNow: now.Add(-time.Minute),
	} {
		f.exec(`INSERT INTO listening_history (user_id, track_id, station_id, played_at) VALUES ($1::uuid, $2::uuid, $3::uuid, $4)`,
			f.user, id, station, at)
	}
	f.exec(`INSERT INTO listening_history (user_id, track_id, station_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		f.user, playedElsewhere, elsewhere)

	got := f.queueIDs(station)
	require.Len(t, got, 5)
	assert.ElementsMatch(t, []string{fresh, playedLongAgo, playedElsewhere}, got[:3],
		"outside the recent window, or played on another station, counts as fresh")
	assert.Equal(t, []string{playedHourAgo, playedJustNow}, got[3:])
}

func TestQueue_RecentHoursIsAServiceOption(t *testing.T) {
	f := newFixture(t)
	twoHours := f.addTrack(track{title: "Two hours ago"})
	threeHours := f.addTrack(track{title: "Three hours ago"})
	station := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(twoHours, threeHours)}})
	for id, ago := range map[string]time.Duration{twoHours: 2 * time.Hour, threeHours: 3 * time.Hour} {
		f.exec(`INSERT INTO listening_history (user_id, track_id, station_id, played_at) VALUES ($1::uuid, $2::uuid, $3::uuid, $4)`,
			f.user, id, station, time.Now().Add(-ago))
	}

	// Within the default 24 hours both are recent: least recent first, always.
	for run := 0; run < 5; run++ {
		require.Equal(t, []string{threeHours, twoHours}, f.queueIDs(station))
	}
	// Outside a one-hour window they share an age group, which is shuffled.
	oneHour := stations.NewService(f.pool, stations.WithRecentHours(1))
	for run := 0; run < 30; run++ {
		if ids(f.queue(oneHour, f.user, station))[0] == twoHours {
			return
		}
	}
	t.Fatal("with a one-hour window, the queue never varied")
}

func TestQueue_PagesAndDedupes(t *testing.T) {
	f := newFixture(t)
	var trackIDs []string
	for i := 0; i < stations.QueuePageSize+10; i++ {
		trackIDs = append(trackIDs, f.addTrack(track{title: fmt.Sprintf("Track %d", i), artist: "Same"}))
	}
	// Same title+artist as an existing track: played once.
	trackIDs = append(trackIDs, f.addTrack(track{title: "Track 0", artist: "Same", album: "Deluxe"}))
	station := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(trackIDs...)}})
	assert.Len(t, f.queueIDs(station), stations.QueuePageSize)
}

func TestQueue_SpacesOutArtists(t *testing.T) {
	f := newFixture(t)
	var trackIDs []string
	artists := []string{"A", "A", "A", "B", "C", "D", "E", "F", "G", "H"}
	for _, artist := range artists {
		trackIDs = append(trackIDs, f.addTrack(track{title: uuid.NewString(), artist: artist}))
	}
	station := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(trackIDs...)}})
	artistOf := map[string]string{}
	for i, id := range trackIDs {
		artistOf[id] = artists[i]
	}
	for run := 0; run < 20; run++ {
		got := f.queueIDs(station)
		last := -10
		for i, id := range got {
			if artistOf[id] == "A" {
				assert.Greater(t, i-last, 3, "artist A repeats within 3 tracks: %v", got)
				last = i
			}
		}
	}
}

func TestQueue_IncludesUnanalysedTracks(t *testing.T) {
	f := newFixture(t)
	analysed := f.addTrack(track{title: "Analysed"})
	unanalysed := f.addTrack(track{title: "Not yet", noFeatures: true})
	station := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(analysed, unanalysed)}})
	assert.Equal(t, sorted(analysed, unanalysed), sorted(f.queueIDs(station)...))
}

func TestQueue_CopiesPlayAsOneSong(t *testing.T) {
	f := newFixture(t)
	best := f.addTrack(track{title: "Song", artist: "X", genres: []string{"techno"}})
	other := f.addTrack(track{title: "Song (2011 Remaster)", artist: "X", genres: []string{"techno"}})
	f.exec(`INSERT INTO track_copies (track_id, copy_of) VALUES ($1::uuid, $2::uuid)`, other, best)
	station := f.genreStation("techno", true)

	queue := func(userID string) []stations.QueueTrack { return f.queue(f.svc, userID, station) }
	got := queue(f.user)
	require.Len(t, got, 1, "only the best copy plays")
	require.Equal(t, best, got[0].ID)

	// Feedback given on the other copy counts for the song.
	f.exec(`INSERT INTO track_feedback (user_id, track_id, feedback) VALUES ($1::uuid, $2::uuid, 'like')`, f.user, other)
	require.True(t, queue(f.user)[0].Liked)
	f.exec(`UPDATE track_feedback SET feedback = 'ban'`)
	require.Empty(t, queue(f.user))
	f.exec(`DELETE FROM track_feedback`)

	f.exec(`INSERT INTO station_hidden_tracks (user_id, station_id, track_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, f.user, station, other)
	require.Empty(t, queue(f.user))
	require.Len(t, queue(f.newUser()), 1, "another user still hears it")
}

func TestQueue_OnlyStationsTheUserMayPlay(t *testing.T) {
	f := newFixture(t)
	song := f.addTrack(track{title: "Song", genres: []string{"techno"}})
	own := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(song)}})
	shared := f.genreStation("techno", true)
	hiddenGenre := f.genreStation("house", false)
	stranger := f.newUser()

	assert.Equal(t, []string{song}, ids(f.queue(f.svc, stranger, shared)), "a shared station plays for everyone")
	for name, c := range map[string]struct{ user, station string }{
		"another user's station": {stranger, own},
		"hidden genre station":   {f.user, hiddenGenre},
		"no such station":        {f.user, uuid.NewString()},
	} {
		_, err := f.svc.Queue(f.ctx, c.user, c.station)
		assert.ErrorIs(t, err, stations.ErrNotFound, name)
	}
}
