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

func TestParseCoverArt(t *testing.T) {
	page := `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{
		"type":"playlist","id":"x","name":"Mix",
		"coverArt":{"sources":[
			{"url":"https://evil.example.com/big.jpg","width":2000},
			{"url":"https://i.scdn.co/image/small","width":60},
			{"url":"https://mosaic.scdn.co/640/abc","width":640}
		]},
		"trackList":[{"uri":"spotify:track:1","title":"Song","subtitle":"Band"}]}}}}}}</script>`
	p, err := Parse([]byte(page))
	require.NoError(t, err)
	require.Equal(t, "https://mosaic.scdn.co/640/abc", p.CoverURL, "largest source on a Spotify image host")

	page = `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{
		"type":"playlist","id":"x","name":"Mix","coverArt":{"sources":[{"url":"https://i.scdn.co/image/abc","width":null}]},
		"trackList":[{"uri":"spotify:track:1","title":"Song","subtitle":"Band"}]}}}}}}</script>`
	p, err = Parse([]byte(page))
	require.NoError(t, err)
	require.Equal(t, "https://i.scdn.co/image/abc", p.CoverURL, "width may be null")

	p, err = Parse(mustRead(t, "testdata/embed.html"))
	require.NoError(t, err)
	require.Empty(t, p.CoverURL, "no coverArt, no cover")
}

func TestIsImageURL(t *testing.T) {
	for _, u := range []string{"https://i.scdn.co/image/ab67", "https://mosaic.scdn.co/640/ab", "https://image-cdn-ak.spotifycdn.com/image/ab"} {
		require.True(t, IsImageURL(u), u)
	}
	for _, u := range []string{
		"", "http://i.scdn.co/image/ab", "https://scdn.co.evil.com/x", "https://i.scdn.co:8443/x",
		"https://user@i.scdn.co/x", "https://example.com/i.scdn.co", "file:///etc/passwd",
	} {
		require.False(t, IsImageURL(u), u)
	}
}

func TestFetchImage(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 64)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cover.jpg":
			_, _ = w.Write(jpeg)
		case "/page.html":
			_, _ = w.Write([]byte("<html>not an image</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := Client{HTTP: srv.Client(), imageURLOK: func(string) bool { return true }}

	body, kind, err := c.FetchImage(context.Background(), srv.URL+"/cover.jpg")
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", kind)
	require.Equal(t, jpeg, body)

	_, _, err = c.FetchImage(context.Background(), srv.URL+"/page.html")
	require.ErrorContains(t, err, "not an image")
	_, _, err = c.FetchImage(context.Background(), srv.URL+"/missing")
	require.ErrorContains(t, err, "404")

	_, _, err = Client{HTTP: srv.Client()}.FetchImage(context.Background(), srv.URL+"/cover.jpg")
	require.ErrorContains(t, err, "not a Spotify image URL")
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return b
}
