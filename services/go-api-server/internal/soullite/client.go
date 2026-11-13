// Package soullite tells a SoulLite server which Spotify playlists to follow.
// SoulLite downloads the tracks a playlist lacks into the shared music folder
// and then asks OwnWave to rescan, so playlists added in OwnWave fill up on
// their own.
package soullite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ownwave/api/internal/spotify"
)

// Schedule is how often SoulLite syncs a playlist OwnWave asked it to follow,
// matching OwnWave's own daily playlist sync.
const Schedule = "daily"

// Client calls SoulLite's API with an admin-scope API Key.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New returns nil, nil when baseURL and apiKey are both empty: the
// integration is off. Setting only one of them is an error, as is a base URL
// that is not http(s).
func New(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" && apiKey == "" {
		return nil, nil
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("set both SOULLITE_URL and SOULLITE_API_KEY, or neither")
	}
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("SOULLITE_URL must be an http(s) URL, got %q", baseURL)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: httpClient}, nil
}

// BaseURL is the SoulLite server this client calls.
func (c *Client) BaseURL() string { return c.baseURL }

// PlaylistURL is the only playlist URL form SoulLite accepts.
func PlaylistURL(spotifyID string) string {
	return "https://open.spotify.com/playlist/" + spotifyID
}

type followed struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

// Follow makes SoulLite follow the playlist and starts its first Sync right
// away. A playlist SoulLite already follows is left as it is.
func (c *Client) Follow(ctx context.Context, spotifyID, name string) error {
	body := map[string]string{"url": PlaylistURL(spotifyID), "schedule": Schedule}
	if name != "" {
		body["name"] = name
	}
	resp, err := c.do(ctx, http.MethodPost, "/playlists", body)
	if err != nil {
		return err
	}
	defer drain(resp)
	switch resp.StatusCode {
	case http.StatusCreated:
	case http.StatusConflict:
		// "This Playlist is already followed": SoulLite syncs it on its own schedule.
		return nil
	default:
		return statusError("follow", resp)
	}
	var p followed
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&p); err != nil {
		return fmt.Errorf("soullite follow: read response: %w", err)
	}
	return c.sync(ctx, p.ID)
}

func (c *Client) sync(ctx context.Context, id int) error {
	resp, err := c.do(ctx, http.MethodPost, "/playlists/"+strconv.Itoa(id)+"/sync", nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusAccepted {
		return statusError("sync", resp)
	}
	return nil
}

// Unfollow stops SoulLite following the playlist. SoulLite keeps the files it
// downloaded. A playlist SoulLite does not follow is not an error.
func (c *Client) Unfollow(ctx context.Context, spotifyID string) error {
	resp, err := c.do(ctx, http.MethodGet, "/playlists", nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusOK {
		return statusError("list playlists", resp)
	}
	var list []followed
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&list); err != nil {
		return fmt.Errorf("soullite list playlists: read response: %w", err)
	}
	for _, p := range list {
		// SoulLite stores the URL it was given minus the query string; compare
		// Spotify ids so a playlist followed by hand in another form matches too.
		if id, err := spotify.ParsePlaylistID(p.URL); err != nil || id != spotifyID {
			continue
		}
		if err := c.delete(ctx, p.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) delete(ctx context.Context, id int) error {
	resp, err := c.do(ctx, http.MethodDelete, "/playlists/"+strconv.Itoa(id), nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return statusError("unfollow", resp)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("soullite %s %s: %w", method, path, err)
	}
	return resp, nil
}

// statusError keeps the start of SoulLite's error body ({"detail": ...}),
// which says why: a bad key (401), a read-only key (403) or a bad URL (422).
func statusError(op string, resp *http.Response) error {
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	return fmt.Errorf("soullite %s: HTTP %d: %s", op, resp.StatusCode, strings.TrimSpace(string(detail)))
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
}
