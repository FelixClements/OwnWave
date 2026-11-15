// Package spotify reads public Spotify playlists without a Spotify login.
//
// Spotify's Web API refuses Spotify-owned (editorial) playlists to new apps,
// but the public embed page carries the playlist's current tracks (up to
// about 100) as JSON, plus an anonymous access token that reads the rest (see
// fullread.go). Neither is a documented API: when the embed page's shape
// changes, Parse returns an error instead of an empty playlist, and when the
// full read fails, FetchPlaylist keeps the embed page's tracks (a partial read).
package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// ErrInvalidURL is returned for input that names no Spotify playlist.
var ErrInvalidURL = errors.New("not a Spotify playlist URL")

// FetchError is a failure to load or read a playlist from Spotify.
type FetchError struct{ Err error }

func (e *FetchError) Error() string { return e.Err.Error() }
func (e *FetchError) Unwrap() error { return e.Err }

// Playlist is the playlist as Spotify shows it right now.
type Playlist struct {
	ID     string
	Name   string
	Tracks []Track
	// Partial is true when only the embed page's tracks (about the first 100)
	// could be read.
	Partial bool
	// CoverURL is the playlist's cover image on Spotify's image CDN, or ""
	// when the page lists none (or one on a host CoverURL may not point to).
	CoverURL string
}

type Track struct {
	ID         string
	Title      string
	Artists    []string
	DurationMS int
}

var playlistIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)

// ParsePlaylistID accepts a playlist URL (open.spotify.com/playlist/…,
// …/embed/playlist/…, with or without an intl-xx segment or query), a
// spotify:playlist:… URI, or a bare id.
func ParsePlaylistID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if rest, ok := strings.CutPrefix(s, "spotify:playlist:"); ok {
		s = rest
	} else if i := strings.Index(s, "/playlist/"); i >= 0 {
		s = s[i+len("/playlist/"):]
		if j := strings.IndexAny(s, "/?#"); j >= 0 {
			s = s[:j]
		}
	}
	if !playlistIDPattern.MatchString(s) {
		return "", ErrInvalidURL
	}
	return s, nil
}

// Client reads playlists. NewClient makes one that caches the playlist query
// hash; a zero Client scrapes it on every read.
type Client struct {
	HTTP       *http.Client
	BaseURL    string // defaults to https://open.spotify.com
	PartnerURL string // defaults to https://api-partner.spotify.com
	hashes     *queryHashCache
	// imageURLOK replaces IsImageURL in tests, which serve images over http.
	imageURLOK func(string) bool
}

// FetchPlaylist reads every track of a public playlist: the embed page first,
// then the full read with its token. When the full read fails, it returns the
// embed page's tracks with Partial set. Failures are *FetchError.
func (c Client) FetchPlaylist(ctx context.Context, id string) (Playlist, error) {
	if !playlistIDPattern.MatchString(id) {
		return Playlist{}, ErrInvalidURL
	}
	p, err := c.fetch(ctx, id)
	if err != nil {
		return Playlist{}, &FetchError{Err: err}
	}
	return p, nil
}

func (c Client) fetch(ctx context.Context, id string) (Playlist, error) {
	page, err := c.get(ctx, c.base()+"/embed/playlist/"+id, 8<<20, nil)
	if err != nil {
		return Playlist{}, fmt.Errorf("fetch playlist: %w", err)
	}
	p, token, err := parseEmbed(page)
	if err != nil {
		return Playlist{}, err
	}
	if token == "" {
		err = errors.New("embed page has no access token")
	} else {
		var tracks []Track
		if tracks, err = c.readAllTracks(ctx, id, token); err == nil {
			p.Tracks = tracks
			return p, nil
		}
		if ctx.Err() != nil {
			return Playlist{}, ctx.Err()
		}
	}
	p.Partial = true
	slog.Warn("spotify full playlist read failed, using the embed page's tracks (about the first 100)",
		"playlist_id", id, "tracks", len(p.Tracks), "error", err)
	return p, nil
}

var nextDataPattern = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// Parse extracts the playlist from an embed page.
func Parse(page []byte) (Playlist, error) {
	p, _, err := parseEmbed(page)
	return p, err
}

// parseEmbed extracts the playlist and the anonymous access token ("" when the
// page has none) from an embed page.
func parseEmbed(page []byte) (Playlist, string, error) {
	m := nextDataPattern.FindSubmatch(page)
	if m == nil {
		return Playlist{}, "", errors.New("embed page has no playlist data (page format changed?)")
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Settings struct {
						Session struct {
							AccessToken string `json:"accessToken"`
						} `json:"session"`
					} `json:"settings"`
					Data struct {
						Entity struct {
							Type     string `json:"type"`
							ID       string `json:"id"`
							Name     string `json:"name"`
							CoverArt struct {
								Sources []struct {
									URL   string `json:"url"`
									Width *int   `json:"width"`
								} `json:"sources"`
							} `json:"coverArt"`
							TrackList []struct {
								URI        string `json:"uri"`
								Title      string `json:"title"`
								Subtitle   string `json:"subtitle"`
								Duration   int    `json:"duration"`
								EntityType string `json:"entityType"`
							} `json:"trackList"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return Playlist{}, "", fmt.Errorf("embed page data: %w", err)
	}
	e := data.Props.PageProps.State.Data.Entity
	if e.Type != "playlist" || e.ID == "" {
		return Playlist{}, "", errors.New("embed page holds no playlist (page format changed?)")
	}

	p := Playlist{ID: e.ID, Name: strings.TrimSpace(e.Name)}
	best := -1
	for _, src := range e.CoverArt.Sources {
		w := 0
		if src.Width != nil {
			w = *src.Width
		}
		if IsImageURL(src.URL) && w > best {
			p.CoverURL, best = src.URL, w
		}
	}
	for _, t := range e.TrackList {
		id, ok := strings.CutPrefix(t.URI, "spotify:track:")
		if !ok || strings.TrimSpace(t.Title) == "" {
			continue // podcast episodes, local files
		}
		p.Tracks = append(p.Tracks, Track{
			ID:         id,
			Title:      strings.TrimSpace(t.Title),
			Artists:    splitArtists(t.Subtitle),
			DurationMS: t.Duration,
		})
	}
	if len(p.Tracks) == 0 {
		return Playlist{}, "", errors.New("embed page lists no tracks (page format changed?)")
	}
	return p, strings.TrimSpace(data.Props.PageProps.State.Settings.Session.AccessToken), nil
}

// splitArtists splits the embed subtitle "A, B" into names.
func splitArtists(subtitle string) []string {
	var out []string
	for _, a := range strings.Split(subtitle, ",") {
		if a = strings.TrimSpace(strings.ReplaceAll(a, " ", " ")); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// IsImageURL reports whether u is an https URL on Spotify's image CDNs,
// the only place a playlist cover is downloaded from.
func IsImageURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, ".scdn.co") || strings.HasSuffix(host, ".spotifycdn.com")
}

// maxImageBytes caps a downloaded cover; Spotify's are well under 1 MB.
const maxImageBytes = 4 << 20

// FetchImage downloads a cover image from Spotify's image CDN and returns
// it with its media type (image/jpeg, image/png or image/webp).
func (c Client) FetchImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	ok := IsImageURL
	if c.imageURLOK != nil {
		ok = c.imageURLOK
	}
	if !ok(imageURL) {
		return nil, "", fmt.Errorf("fetch cover: %q is not a Spotify image URL", imageURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch cover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch cover: spotify answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("fetch cover: %w", err)
	}
	if len(body) > maxImageBytes {
		return nil, "", errors.New("fetch cover: image is too large")
	}
	switch kind := http.DetectContentType(body); kind {
	case "image/jpeg", "image/png", "image/webp":
		return body, kind, nil
	default:
		return nil, "", fmt.Errorf("fetch cover: not an image (%s)", kind)
	}
}
