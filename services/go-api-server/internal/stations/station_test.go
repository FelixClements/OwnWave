package stations

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	idA = "6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"
	idB = "0a1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"
)

func TestNormalizeCanonicalizesAndSorts(t *testing.T) {
	d, err := Definition{
		PlaylistIDs: []string{idA, " " + idB + " ", "6F1B8C6E-6A43-4E4B-9B1E-2B3F8D1A0C55"},
		Genres:      []string{"techno", "house", "techno"},
	}.Normalize()
	require.NoError(t, err)
	require.Equal(t, []string{idB, idA}, d.PlaylistIDs)
	require.Equal(t, []string{"house", "techno"}, d.Genres)

	d, err = Definition{PlaylistIDs: []string{idA}}.Normalize()
	require.NoError(t, err)
	require.Equal(t, []string{}, d.Genres, "never nil: stored as an empty array")
}

func TestNormalizeRejects(t *testing.T) {
	for name, d := range map[string]Definition{
		"no playlists": {},
		"bad playlist": {PlaylistIDs: []string{"nope"}},
		"bad genre":    {PlaylistIDs: []string{idA}, Genres: []string{"Hard Techno"}},
		"empty genre":  {PlaylistIDs: []string{idA}, Genres: []string{""}},
		"sql in genre": {PlaylistIDs: []string{idA}, Genres: []string{"x');drop"}},
	} {
		_, err := d.Normalize()
		require.True(t, errors.Is(err, ErrInvalid), name)
	}
}

func TestPoolKeyIgnoresOrder(t *testing.T) {
	require.Equal(t, PoolKey([]string{idA, idB}), PoolKey([]string{idB, idA}))
	require.Equal(t, idB+","+idA, PoolKey([]string{idA, idB}))
}

func TestWhereAddsGenreFilterOnlyWhenSet(t *testing.T) {
	var args Args
	sql := Definition{PlaylistIDs: []string{idA}}.Where(&args)
	require.Len(t, args, 1)
	require.NotContains(t, sql, "track_station_genres")

	args = Args{"user"}
	sql = Definition{PlaylistIDs: []string{idA}, Genres: []string{"techno"}}.Where(&args)
	require.Equal(t, Args{"user", []string{idA}, []string{"techno"}}, args)
	require.Contains(t, sql, "$2::uuid[]")
	require.Contains(t, sql, "$3::text[]")
}

func TestValidName(t *testing.T) {
	name, err := validName("  Techno  ")
	require.NoError(t, err)
	require.Equal(t, "Techno", name)
	_, err = validName(string(make([]rune, maxNameLen+1)))
	require.Error(t, err)
}

func TestNormalizeAllPlaylists(t *testing.T) {
	d, err := Definition{AllPlaylists: true, PlaylistIDs: []string{idA, "ignored"}, Genres: []string{"house"}}.Normalize()
	require.NoError(t, err)
	require.True(t, d.AllPlaylists)
	require.Equal(t, []string{}, d.PlaylistIDs, "every playlist: none stored")
	require.Equal(t, []string{"house"}, d.Genres)
}

func TestWhereAllPlaylistsHasNoPlaylistFilter(t *testing.T) {
	var args Args
	where := Definition{AllPlaylists: true, Genres: []string{"house"}}.Where(&args)
	require.NotContains(t, where, "playlist_id")
	require.Len(t, args, 1, "only the genres")

	args = nil
	where = Definition{PlaylistIDs: []string{idA}}.Where(&args)
	require.Contains(t, where, "pi.playlist_id = ANY($1::uuid[])")
}

func TestGenreStationShowsAndHidesWithAMargin(t *testing.T) {
	for _, c := range []struct {
		name    string
		a       autoStation
		enabled bool
		want    bool
	}{
		{"new, enough", autoStation{tracks: 25, stillAStationGenre: true}, true, true},
		{"new, too few", autoStation{tracks: 24, stillAStationGenre: true}, true, false},
		{"shown, in the margin", autoStation{tracks: 20, active: true, stillAStationGenre: true}, true, true},
		{"shown, below the margin", autoStation{tracks: 19, active: true, stillAStationGenre: true}, true, false},
		{"hidden, in the margin", autoStation{tracks: 22, stillAStationGenre: true}, true, false},
		{"excluded", autoStation{tracks: 99, excluded: true, stillAStationGenre: true}, true, false},
		{"switched off", autoStation{tracks: 99, active: true, stillAStationGenre: true}, false, false},
		{"genre gone from the taxonomy", autoStation{tracks: 99, active: true}, true, false},
	} {
		require.Equal(t, c.want, c.a.shouldShow(c.enabled), c.name)
	}
}
