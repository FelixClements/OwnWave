package spotify

// The full read. The embed page lists only about the first 100 tracks, but it
// also carries an anonymous access token. With that token, the web player's own
// playlist query (pathfinder "fetchPlaylist") pages through every track.
// SoulLite reads playlists the same way (through SpotipyFree), so both apps see
// the same tracks.
//
// The query is named by a hash that changes when Spotify ships a new web
// player. It is scraped from the web player's JavaScript and cached; a query
// that fails with a cached hash scrapes it again once.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultPartnerURL = "https://api-partner.spotify.com"
	// pageSize is what SpotipyFree asks for per page.
	pageSize = 343
	// maxTracks is Spotify's playlist size limit; it bounds the paging loop.
	maxTracks = 10000
	// hashTTL is how long a scraped query hash is trusted before it is scraped again.
	hashTTL = 6 * time.Hour
	// The web player's main script is about 5 MB.
	maxScriptBytes = 16 << 20
	// maxChunkFetches bounds the search through the web player's script chunks.
	maxChunkFetches = 150
)

const userAgent = "Mozilla/5.0 (compatible; OwnWave playlist sync)"

// browserUserAgent is sent for the web player page: other user agents get the
// mobile web player, whose scripts lack the playlist query.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// queryHashCache keeps the scraped fetchPlaylist hash between reads.
type queryHashCache struct {
	mu      sync.Mutex
	hash    string
	fetched time.Time
}

// NewClient returns a Client that caches the playlist query hash between reads.
func NewClient(h *http.Client) Client {
	return Client{HTTP: h, hashes: &queryHashCache{}}
}

// readAllTracks reads every track of a playlist with the embed page's token.
func (c Client) readAllTracks(ctx context.Context, id, token string) ([]Track, error) {
	hash, cached, err := c.queryHash(ctx)
	if err != nil {
		return nil, err
	}
	tracks, err := c.pageTracks(ctx, id, token, hash)
	if err == nil || !cached {
		return tracks, err
	}
	// The web player may have changed since the hash was cached.
	c.forgetQueryHash(hash)
	hash, _, err = c.queryHash(ctx)
	if err != nil {
		return nil, err
	}
	return c.pageTracks(ctx, id, token, hash)
}

// queryHash returns the fetchPlaylist hash, and whether it came from the cache.
func (c Client) queryHash(ctx context.Context) (string, bool, error) {
	if c.hashes != nil {
		c.hashes.mu.Lock()
		hash, fetched := c.hashes.hash, c.hashes.fetched
		c.hashes.mu.Unlock()
		if hash != "" && time.Since(fetched) < hashTTL {
			return hash, true, nil
		}
	}
	hash, err := c.scrapeQueryHash(ctx)
	if err != nil {
		return "", false, err
	}
	if c.hashes != nil {
		c.hashes.mu.Lock()
		c.hashes.hash, c.hashes.fetched = hash, time.Now()
		c.hashes.mu.Unlock()
	}
	return hash, false, nil
}

func (c Client) forgetQueryHash(hash string) {
	if c.hashes == nil {
		return
	}
	c.hashes.mu.Lock()
	if c.hashes.hash == hash {
		c.hashes.hash = ""
	}
	c.hashes.mu.Unlock()
}

var (
	webPlayerScriptPattern = regexp.MustCompile(`src="([^"]*web-player/web-player[^"]*\.js)"`)
	fetchPlaylistPattern   = regexp.MustCompile(`"fetchPlaylist","query","([0-9a-f]{64})"`)
	chunkMapPattern        = regexp.MustCompile(`\{\d+:"[^"]+"(?:,\d+:"[^"]+")*\}`)
	chunkMapEntryPattern   = regexp.MustCompile(`(\d+):"([^"]+)"`)
	chunkHashPattern       = regexp.MustCompile(`^[0-9a-f]{8}$`)
)

// scrapeQueryHash finds the fetchPlaylist hash in the web player's main script,
// or else in its chunks.
func (c Client) scrapeQueryHash(ctx context.Context) (string, error) {
	home, err := url.Parse(c.base() + "/")
	if err != nil {
		return "", err
	}
	page, err := c.get(ctx, home.String(), 8<<20, http.Header{"User-Agent": {browserUserAgent}})
	if err != nil {
		return "", fmt.Errorf("web player page: %w", err)
	}
	m := webPlayerScriptPattern.FindSubmatch(page)
	if m == nil {
		return "", errors.New("web player page names no web-player script (page format changed?)")
	}
	ref, err := url.Parse(string(m[1]))
	if err != nil {
		return "", fmt.Errorf("web player script: %w", err)
	}
	pack := home.ResolveReference(ref)
	script, err := c.get(ctx, pack.String(), maxScriptBytes, nil)
	if err != nil {
		return "", fmt.Errorf("web player script: %w", err)
	}
	if m := fetchPlaylistPattern.FindSubmatch(script); m != nil {
		return string(m[1]), nil
	}

	for i, name := range chunkFiles(script) {
		if i == maxChunkFetches {
			break
		}
		chunk, err := c.get(ctx, pack.ResolveReference(&url.URL{Path: name}).String(), maxScriptBytes, nil)
		if err != nil {
			continue // a CSS-only chunk has no .js file
		}
		if m := fetchPlaylistPattern.FindSubmatch(chunk); m != nil {
			return string(m[1]), nil
		}
	}
	return "", errors.New("no fetchPlaylist query hash in the web player scripts (format changed?)")
}

// chunkFiles lists the web player's script chunks as "name.hash.js". The main
// script maps chunk ids to names in one object, followed by an object mapping
// the same ids to 8-hex-digit content hashes.
func chunkFiles(script []byte) []string {
	maps := chunkMapPattern.FindAll(script, -1)
	var files []string
	seen := map[string]bool{}
	for i := 0; i+1 < len(maps); i++ {
		names, hashes := chunkMap(maps[i]), chunkMap(maps[i+1])
		if !allHashes(hashes) || allHashes(names) {
			continue
		}
		for id, name := range names {
			if h, ok := hashes[id]; ok {
				f := name + "." + h + ".js"
				if !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}
	}
	return files
}

func chunkMap(obj []byte) map[string]string {
	out := map[string]string{}
	for _, e := range chunkMapEntryPattern.FindAllSubmatch(obj, -1) {
		out[string(e[1])] = string(e[2])
	}
	return out
}

func allHashes(m map[string]string) bool {
	for _, v := range m {
		if !chunkHashPattern.MatchString(v) {
			return false
		}
	}
	return len(m) > 0
}

// pageTracks pages through the playlist query until it has every item.
func (c Client) pageTracks(ctx context.Context, id, token, hash string) ([]Track, error) {
	var tracks []Track
	for offset := 0; offset < maxTracks; {
		content, err := c.fetchPlaylistPage(ctx, id, token, hash, offset)
		if err != nil {
			return nil, err
		}
		for _, item := range content.Items {
			if t, ok := item.track(); ok {
				tracks = append(tracks, t)
			}
		}
		offset += len(content.Items)
		if len(content.Items) == 0 || offset >= content.TotalCount {
			break
		}
	}
	if len(tracks) == 0 {
		return nil, errors.New("playlist query returned no tracks")
	}
	return tracks, nil
}

type playlistContent struct {
	TotalCount int            `json:"totalCount"`
	Items      []playlistItem `json:"items"`
}

type playlistItem struct {
	ItemV2 struct {
		Data struct {
			URI     string `json:"uri"`
			Name    string `json:"name"`
			Artists struct {
				Items []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"items"`
			} `json:"artists"`
			TrackDuration struct {
				TotalMilliseconds int `json:"totalMilliseconds"`
			} `json:"trackDuration"`
		} `json:"data"`
	} `json:"itemV2"`
}

// track keeps the same items the embed read keeps: Spotify tracks with a title,
// not podcast episodes or local files.
func (it playlistItem) track() (Track, bool) {
	d := it.ItemV2.Data
	id, ok := strings.CutPrefix(d.URI, "spotify:track:")
	title := strings.TrimSpace(d.Name)
	if !ok || id == "" || title == "" {
		return Track{}, false
	}
	t := Track{ID: id, Title: title, DurationMS: d.TrackDuration.TotalMilliseconds}
	for _, a := range d.Artists.Items {
		if name := strings.TrimSpace(a.Profile.Name); name != "" {
			t.Artists = append(t.Artists, name)
		}
	}
	return t, true
}

func (c Client) fetchPlaylistPage(ctx context.Context, id, token, hash string, offset int) (playlistContent, error) {
	variables, _ := json.Marshal(map[string]any{
		"uri":                       "spotify:playlist:" + id,
		"offset":                    offset,
		"limit":                     pageSize,
		"enableWatchFeedEntrypoint": false,
	})
	extensions, _ := json.Marshal(map[string]any{
		"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash},
	})
	q := url.Values{
		"operationName": {"fetchPlaylist"},
		"variables":     {string(variables)},
		"extensions":    {string(extensions)},
	}
	partner := c.PartnerURL
	if partner == "" {
		partner = defaultPartnerURL
	}
	body, err := c.get(ctx, partner+"/pathfinder/v1/query?"+q.Encode(), 32<<20, http.Header{
		"Authorization": {"Bearer " + token},
		"Accept":        {"application/json"},
	})
	if err != nil {
		return playlistContent{}, fmt.Errorf("playlist query: %w", err)
	}
	var resp struct {
		Data struct {
			PlaylistV2 struct {
				Typename string           `json:"__typename"`
				Content  *playlistContent `json:"content"`
			} `json:"playlistV2"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return playlistContent{}, fmt.Errorf("playlist query: %w", err)
	}
	if len(resp.Errors) > 0 {
		return playlistContent{}, fmt.Errorf("playlist query: %s", resp.Errors[0].Message)
	}
	if resp.Data.PlaylistV2.Content == nil {
		return playlistContent{}, fmt.Errorf("playlist query: no playlist content (%q)", resp.Data.PlaylistV2.Typename)
	}
	return *resp.Data.PlaylistV2.Content, nil
}

func (c Client) base() string {
	if c.BaseURL == "" {
		return "https://open.spotify.com"
	}
	return c.BaseURL
}

// get fetches u and returns at most limit bytes of a 200 response. header may
// replace the default User-Agent.
func (c Client) get(ctx context.Context, u string, limit int64, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en")
	for k, v := range header {
		req.Header[http.CanonicalHeaderKey(k)] = v
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify answered %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}
