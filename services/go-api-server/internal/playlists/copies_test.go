package playlists

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindCopiesGroupsOneRecording(t *testing.T) {
	got := findCopies([]LocalTrack{
		{ID: "mp3", Title: "Domino", Artist: "Oxia", DurationSec: 300, SampleRate: 44100, FileSize: 8},
		{ID: "flac", Title: "Domino (2011 Remaster)", Artist: "OXIA feat. Someone", DurationSec: 301, SampleRate: 96000, FileSize: 40},
		{ID: "cd", Title: "domino", Artist: "Oxia", DurationSec: 299.5, SampleRate: 44100, FileSize: 30},
		{ID: "alone", Title: "Domino Effect", Artist: "Oxia", DurationSec: 300},
	})
	require.Equal(t, map[string]string{"mp3": "flac", "cd": "flac"}, got)
}

func TestFindCopiesKeepsDifferentRecordingsApart(t *testing.T) {
	for name, tracks := range map[string][]LocalTrack{
		"other length": {
			{ID: "a", Title: "Song", Artist: "X", DurationSec: 200},
			{ID: "b", Title: "Song", Artist: "X", DurationSec: 210},
		},
		"other artist": {
			{ID: "a", Title: "Song", Artist: "X", DurationSec: 200},
			{ID: "b", Title: "Song", Artist: "Y", DurationSec: 200},
		},
		"other version": {
			{ID: "a", Title: "Song", Artist: "X", DurationSec: 200},
			{ID: "b", Title: "Song (Live)", Artist: "X", DurationSec: 200},
		},
		"no artist": {
			{ID: "a", Title: "Song", DurationSec: 200},
			{ID: "b", Title: "Song", DurationSec: 200},
		},
		"no length": {
			{ID: "a", Title: "Song", Artist: "X"},
			{ID: "b", Title: "Song", Artist: "X"},
		},
		"no title": {
			{ID: "a", Title: "", Artist: "X", DurationSec: 200},
			{ID: "b", Title: "", Artist: "X", DurationSec: 200},
		},
	} {
		require.Empty(t, findCopies(tracks), name)
	}
}

func TestFindCopiesTieBreakIsStable(t *testing.T) {
	a := LocalTrack{ID: "a", Title: "Song", Artist: "X", DurationSec: 200}
	b := LocalTrack{ID: "b", Title: "Song", Artist: "X", DurationSec: 200}
	require.Equal(t, map[string]string{"b": "a"}, findCopies([]LocalTrack{a, b}))
	require.Equal(t, map[string]string{"b": "a"}, findCopies([]LocalTrack{b, a}))
}

func TestChoosePicksKeepsOneOfEachSongUpToTheLimit(t *testing.T) {
	seeds := []LocalTrack{{ID: "seed", Title: "Domino", Artist: "Oxia"}}
	cands := []candidate{
		{LocalTrack: LocalTrack{ID: "same-as-seed", Title: "Domino - Radio Edit", Artist: "Oxia"}},
		{LocalTrack: LocalTrack{ID: "first", Title: "Harvest", Artist: "Neil"}},
		{LocalTrack: LocalTrack{ID: "same-as-first", Title: "Harvest", Artist: "Neil Young, Neil"}},
		{LocalTrack: LocalTrack{ID: "same-title-other-artist", Title: "Harvest", Artist: "Other"}},
		{LocalTrack: LocalTrack{ID: "over-the-limit", Title: "Later", Artist: "Z"}},
	}
	var got []string
	for _, p := range choosePicks(seeds, cands, 2) {
		got = append(got, p.ID)
	}
	require.Equal(t, []string{"first", "same-title-other-artist"}, got)
}
