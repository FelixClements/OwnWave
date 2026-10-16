package analytics

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// requestTimeout bounds every call; long work (scans, rebuilds) is queued by
// the analytics service and returns immediately.
const requestTimeout = 30 * time.Second

// HTTPClient calls the Python analytics service over HTTP.
type HTTPClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return NewHTTPClientWithSecret(baseURL, os.Getenv("ANALYTICS_API_SECRET"))
}

func NewHTTPClientWithSecret(baseURL, secret string) *HTTPClient {
	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		secret:     secret,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

func (c *HTTPClient) Scan(ctx context.Context, body []byte) (*Response, error) {
	return c.do(ctx, http.MethodPost, "/scan", bytes.NewReader(body))
}

func (c *HTTPClient) GetJob(ctx context.Context, jobID string) (*Response, error) {
	return c.do(ctx, http.MethodGet, "/jobs/"+url.PathEscape(jobID), nil)
}

func (c *HTTPClient) Health(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, "/health", nil)
}

func (c *HTTPClient) RebuildVectors(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodPost, "/rebuild-vectors", nil)
}

func (c *HTTPClient) RebuildClusters(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodPost, "/rebuild-clusters", nil)
}

func (c *HTTPClient) ListGenres(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, "/genres", nil)
}

func (c *HTTPClient) GetTrackGenres(ctx context.Context, trackID string) (*Response, error) {
	return c.do(ctx, http.MethodGet, "/tracks/"+url.PathEscape(trackID)+"/genres", nil)
}

func (c *HTTPClient) RebuildGenres(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodPost, "/rebuild-genres", nil)
}

func (c *HTTPClient) SetupSummary(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, "/setup/summary", nil)
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body io.Reader) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		req.Header.Set("X-Internal-Token", c.secret)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}
