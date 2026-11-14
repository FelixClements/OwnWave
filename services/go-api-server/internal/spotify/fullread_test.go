package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	fullID   = "37i9dQZF1DX4o1oenSJRJd"
	hashOne  = "1111111111111111111111111111111111111111111111111111111111111111"
	hashTwo  = "2222222222222222222222222222222222222222222222222222222222222222"
	pageCap  = 2 // the fake query answers at most this many items per page
	theToken = "anon-token"
)

// fakeSpotify serves an embed page, the web player page and scripts, and the
// playlist query.
type fakeSpotify struct {
	token       string       // the embed page's access token ("" for none)
	hash        atomic.Value // the hash the query accepts
	hashInChunk bool         // hash only in a script chunk, not in the main script
	scriptHits  atomic.Int32 // fetches of the main script
	queryHits   atomic.Int32 // playlist queries
	webUA       atomic.Value // User-Agent of the last web player page fetch
	items       []map[string]any
}

func newFakeSpotify(t *testing.T) (*fakeSpotify, *httptest.Server) {
	f := &fakeSpotify{token: theToken}
	f.hash.Store(hashOne)
	f.items = []map[string]any{
		trackItem("spotify:track:t1", "One", 1000, "A"),
		trackItem("spotify:local:x:y:Local:1", "Local", 1000, "L"),
		trackItem("spotify:track:t2", "Two", 2000, "B", "C"),
		trackItem("spotify:episode:e1", "Episode", 3000, "P"),
		trackItem("spotify:track:t3", "Three", 3000, "D"),
		trackItem("spotify:track:t4", "  ", 4000, "E"), // no title
		trackItem("spotify:track:t5", "Five", 5000, "F"),
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

func trackItem(uri, name string, ms int, artists ...string) map[string]any {
	var as []any
	for _, a := range artists {
		as = append(as, map[string]any{"profile": map[string]any{"name": a}})
	}
	return map[string]any{"itemV2": map[string]any{"data": map[string]any{
		"uri": uri, "name": name,
		"artists":       map[string]any{"items": as},
		"trackDuration": map[string]any{"totalMilliseconds": ms},
	}}}
}

func (f *fakeSpotify) serve(w http.ResponseWriter, r *http.Request) {
	hash := f.hash.Load().(string)
	switch r.URL.Path {
	case "/embed/playlist/" + fullID:
		settings := ""
		if f.token != "" {
			settings = fmt.Sprintf(`"settings":{"session":{"accessToken":%q,"isAnonymous":true}},`, f.token)
		}
		fmt.Fprintf(w, `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{%s"data":{"entity":{
			"type":"playlist","id":%q,"name":"Big Mix",
			"trackList":[{"uri":"spotify:track:t1","title":"One","subtitle":"A","duration":1000},
				{"uri":"spotify:track:t2","title":"Two","subtitle":"B, C","duration":2000}]}}}}}}</script>`, settings, fullID)
	case "/":
		f.webUA.Store(r.UserAgent())
		fmt.Fprint(w, `<html><script src="/cdn/build/web-player/web-player.abc123.js"></script></html>`)
	case "/cdn/build/web-player/web-player.abc123.js":
		f.scriptHits.Add(1)
		if f.hashInChunk {
			fmt.Fprint(w, `var a={8:"backspace",9:"tab"};var n={1:"xpui-a",2:"xpui-b"}[e]+"."+{1:"aaaaaaaa",2:"bbbbbbbb"}[e]+".js";`)
			return
		}
		fmt.Fprintf(w, `x("fetchPlaylist","query","%s",null)`, hash)
	case "/cdn/build/web-player/xpui-b.bbbbbbbb.js":
		fmt.Fprintf(w, `x("fetchPlaylist","query","%s",null)`, hash)
	case "/pathfinder/v1/query":
		f.query(w, r, hash)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeSpotify) query(w http.ResponseWriter, r *http.Request, hash string) {
	f.queryHits.Add(1)
	if r.Header.Get("Authorization") != "Bearer "+theToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var ext struct {
		PersistedQuery struct {
			SHA256Hash string `json:"sha256Hash"`
		} `json:"persistedQuery"`
	}
	var vars struct {
		URI    string `json:"uri"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal([]byte(r.URL.Query().Get("extensions")), &ext)
	_ = json.Unmarshal([]byte(r.URL.Query().Get("variables")), &vars)
	if ext.PersistedQuery.SHA256Hash != hash {
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": "PersistedQueryNotFound"}}})
		return
	}
	if vars.URI != "spotify:playlist:"+fullID || r.URL.Query().Get("operationName") != "fetchPlaylist" {
		http.Error(w, "bad query", http.StatusBadRequest)
		return
	}
	end := min(vars.Offset+min(vars.Limit, pageCap), len(f.items))
	page := f.items[min(vars.Offset, end):end]
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"playlistV2": map[string]any{
		"__typename": "Playlist",
		"content":    map[string]any{"totalCount": len(f.items), "items": page},
	}}})
}

func fullClient(srv *httptest.Server) Client {
	c := NewClient(srv.Client())
	c.BaseURL, c.PartnerURL = srv.URL, srv.URL
	return c
}

func trackIDs(p Playlist) []string {
	var ids []string
	for _, t := range p.Tracks {
		ids = append(ids, t.ID)
	}
	return ids
}

func TestFetchPlaylistReadsEveryPage(t *testing.T) {
	f, srv := newFakeSpotify(t)

	p, err := fullClient(srv).FetchPlaylist(context.Background(), fullID)
	require.NoError(t, err)
	require.False(t, p.Partial)
	require.Equal(t, "Big Mix", p.Name)
	require.Equal(t, []string{"t1", "t2", "t3", "t5"}, trackIDs(p), "local files, episodes and untitled items are skipped")
	require.Equal(t, Track{ID: "t2", Title: "Two", Artists: []string{"B", "C"}, DurationMS: 2000}, p.Tracks[1])
	require.EqualValues(t, 4, f.queryHits.Load(), "7 items in pages of 2")
	require.True(t, strings.Contains(f.webUA.Load().(string), "Chrome/"), "desktop web player, not the mobile one")
}

func TestFetchPlaylistFindsHashInChunk(t *testing.T) {
	f, srv := newFakeSpotify(t)
	f.hashInChunk = true

	p, err := fullClient(srv).FetchPlaylist(context.Background(), fullID)
	require.NoError(t, err)
	require.False(t, p.Partial)
	require.Len(t, p.Tracks, 4)
}

func TestFetchPlaylistCachesHashAndRescrapesWhenStale(t *testing.T) {
	f, srv := newFakeSpotify(t)
	c := fullClient(srv)

	_, err := c.FetchPlaylist(context.Background(), fullID)
	require.NoError(t, err)
	_, err = c.FetchPlaylist(context.Background(), fullID)
	require.NoError(t, err)
	require.EqualValues(t, 1, f.scriptHits.Load(), "second read uses the cached hash")

	f.hash.Store(hashTwo) // Spotify shipped a new web player
	p, err := c.FetchPlaylist(context.Background(), fullID)
	require.NoError(t, err)
	require.False(t, p.Partial)
	require.Len(t, p.Tracks, 4)
	require.EqualValues(t, 2, f.scriptHits.Load(), "stale hash is scraped again")
}

func TestFetchPlaylistFallsBackToEmbedTracks(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		f, srv := newFakeSpotify(t)
		f.token = ""
		p, err := fullClient(srv).FetchPlaylist(context.Background(), fullID)
		require.NoError(t, err)
		require.True(t, p.Partial)
		require.Equal(t, []string{"t1", "t2"}, trackIDs(p))
		require.Zero(t, f.queryHits.Load())
	})
	t.Run("query refuses the token", func(t *testing.T) {
		f, srv := newFakeSpotify(t)
		f.token = "wrong"
		p, err := fullClient(srv).FetchPlaylist(context.Background(), fullID)
		require.NoError(t, err)
		require.True(t, p.Partial)
		require.Equal(t, []string{"t1", "t2"}, trackIDs(p))
	})
	t.Run("no hash anywhere", func(t *testing.T) {
		f, srv := newFakeSpotify(t)
		f.hashInChunk = true
		f.hash.Store("") // the chunk names no hash either
		p, err := fullClient(srv).FetchPlaylist(context.Background(), fullID)
		require.NoError(t, err)
		require.True(t, p.Partial)
		require.Len(t, p.Tracks, 2)
	})
}

func TestChunkFiles(t *testing.T) {
	script := []byte(`{8:"backspace",9:"tab"}{106:"*",107:"+"}` +
		`{1:"xpui-a",2:"xpui-b",3:"css-only"}{1:"aaaaaaaa",2:"bbbbbbbb"}` +
		`{1:"xpui-a",4:"xpui-d"}{1:"cccccccc",4:"dddddddd"}`)
	require.ElementsMatch(t, []string{
		"xpui-a.aaaaaaaa.js", "xpui-b.bbbbbbbb.js", "xpui-a.cccccccc.js", "xpui-d.dddddddd.js",
	}, chunkFiles(script))
}
