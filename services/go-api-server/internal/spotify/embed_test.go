package spotify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePlaylistID(t *testing.T) {
	const id = "37i9dQZF1DX6J5NfMJS675"
	for _, in := range []string{
		id,
		"spotify:playlist:" + id,
		"https://open.spotify.com/playlist/" + id,
		"https://open.spotify.com/playlist/" + id + "?si=abc",
		"https://open.spotify.com/intl-de/playlist/" + id,
		"https://open.spotify.com/embed/playlist/" + id + "?utm_source=generator&si=dc33",
		"  open.spotify.com/playlist/" + id + "/  ",
	} {
		got, err := ParsePlaylistID(in)
		require.NoError(t, err, in)
		require.Equal(t, id, got, in)
	}
	for _, in := range []string{"", "https://open.spotify.com/album/" + id, "https://example.com/playlist/short", "spotify:track:" + id} {
		_, err := ParsePlaylistID(in)
		require.ErrorIs(t, err, ErrInvalidURL, in)
	}
}

func TestParseEmbedPage(t *testing.T) {
	page, err := os.ReadFile("testdata/embed.html")
	require.NoError(t, err)

	p, err := Parse(page)
	require.NoError(t, err)
	require.Equal(t, "37i9dQZF1DX6J5NfMJS675", p.ID)
	require.Equal(t, "TECHNO BUNKER", p.Name)
	require.Len(t, p.Tracks, 3)
	require.Equal(t, Track{
		ID:         "2sksm6zC7U7fRmAGSQICiF",
		Title:      "Domino - Space 92 Remix",
		Artists:    []string{"Oxia", "Space 92"},
		DurationMS: 232500,
	}, p.Tracks[1])
}

func TestParseRejectsChangedPages(t *testing.T) {
	for name, page := range map[string]string{
		"no data":   `<html><body>nothing</body></html>`,
		"bad json":  `<script id="__NEXT_DATA__" type="application/json">{</script>`,
		"no entity": `<script id="__NEXT_DATA__" type="application/json">{"props":{}}</script>`,
		"no tracks": `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{"type":"playlist","id":"x","trackList":[]}}}}}}</script>`,
	} {
		_, err := Parse([]byte(page))
		require.Error(t, err, name)
	}
}

func TestFetchPlaylist(t *testing.T) {
	page, err := os.ReadFile("testdata/embed.html")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/playlist/37i9dQZF1DX6J5NfMJS675" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(page)
	}))
	defer srv.Close()

	c := Client{HTTP: srv.Client(), BaseURL: srv.URL}
	p, err := c.FetchPlaylist(context.Background(), "37i9dQZF1DX6J5NfMJS675")
	require.NoError(t, err)
	require.Len(t, p.Tracks, 3)

	_, err = c.FetchPlaylist(context.Background(), "0000000000000000000000")
	require.ErrorContains(t, err, "404")
	var fetchErr *FetchError
	require.ErrorAs(t, err, &fetchErr)
}
