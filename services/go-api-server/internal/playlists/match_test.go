package playlists

import (
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/spotify"
)

func TestTitleKey(t *testing.T) {
	cases := map[string]string{
		"Domino - Space 92 Remix":           "domino|92 remix space",
		"Domino (Space 92 Remix)":           "domino|92 remix space",
		"Obsidian - Extended Mix":           "obsidian|",
		"Obsidian (Original Mix)":           "obsidian|",
		"Obsidian [Radio Edit]":             "obsidian|",
		"Song - Remastered 2011":            "song|",
		"Song (feat. Someone)":              "song|",
		"Song feat. Someone":                "song|",
		"Song (Live)":                       "song|live",
		"Café del Mar":                      "cafe del mar|",
		"Don't Stop & Go":                   "dont stop and go|",
		"God Gave Us Techno":                "god gave us techno|",
		"Track (with Artist) - Club Mix":    "track|",
		"Body Language - Booka Shade Remix": "body language|booka remix shade",
	}
	for in, want := range cases {
		require.Equal(t, want, titleKey(in), in)
	}
}

func TestArtistNames(t *testing.T) {
	require.Equal(t, []string{"oxia", "space 92"}, artistNames("Oxia & Space 92"))
	require.Equal(t, []string{"a", "b", "c"}, artistNames("A, B feat. C"))
	require.Equal(t, []string{"beyonce"}, artistNames("Beyoncé"))
	require.Empty(t, artistNames(""))
}

func TestMatcher(t *testing.T) {
	m := NewMatcher([]LocalTrack{
		{ID: "domino-orig", Title: "Domino", Artist: "Oxia", DurationSec: 420},
		{ID: "domino-remix", Title: "Domino (Space 92 Remix)", Artist: "Oxia & Space 92", DurationSec: 400},
		{ID: "obsidian-ext", Title: "Obsidian (Extended Mix)", Artist: "Kevin de Vries", DurationSec: 360},
		{ID: "techno", Title: "God Gave Us Techno!", Artist: "Sarah de Warren", DurationSec: 190},
		{ID: "untagged", Title: "Mystery", Artist: "", DurationSec: 200},
		{ID: "other-artist", Title: "Obsidian", Artist: "Somebody Else", DurationSec: 202},
	})
	match := func(title string, artists []string, ms int) (string, string) {
		id, method, _ := m.Match(spotify.Track{Title: title, Artists: artists, DurationMS: ms})
		return id, method
	}

	id, method := match("Domino - Space 92 Remix", []string{"Oxia", "Space 92"}, 232500)
	require.Equal(t, "domino-remix", id, "remix must not match the original")
	require.Equal(t, MethodExact, method)

	id, _ = match("Domino", []string{"Oxia"}, 420000)
	require.Equal(t, "domino-orig", id)

	id, method = match("Obsidian", []string{"Kevin de Vries", "Juan Elvadin"}, 202597)
	require.Equal(t, "obsidian-ext", id, "same artist wins over a closer duration from another artist")
	require.Equal(t, MethodExact, method)

	id, method = match("God Gave Us Techno", []string{"Sarah de Warren", "Tao Andra"}, 188357)
	require.Equal(t, "techno", id)
	require.Equal(t, MethodExact, method)

	id, method = match("God Gave Us Tecno", []string{"Sarah de Warren"}, 188357)
	require.Equal(t, "techno", id)
	require.Equal(t, MethodFuzzy, method)

	id, _ = match("Mystery", []string{"Anyone"}, 201000)
	require.Equal(t, "untagged", id, "untagged artist matches on title + duration")
	_, method = match("Mystery", []string{"Anyone"}, 260000)
	require.Equal(t, MethodNone, method)

	_, method = match("Not In Library", []string{"Oxia"}, 200000)
	require.Equal(t, MethodNone, method)
}
