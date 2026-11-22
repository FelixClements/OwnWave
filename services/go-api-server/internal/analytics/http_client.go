package analytics

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

func (c *HTTPClient) Health(ctx context.Context) error {
	status, err := c.do(ctx, http.MethodGet, "/health")
	if err == nil && status != http.StatusOK {
		err = fmt.Errorf("status %d", status)
	}
	return err
}

func (c *HTTPClient) RebuildGenres(ctx context.Context) error {
	status, err := c.do(ctx, http.MethodPost, "/rebuild-genres")
	if err == nil && status >= http.StatusBadRequest {
		err = fmt.Errorf("analytics answered %d", status)
	}
	return err
}

func (c *HTTPClient) do(ctx context.Context, method, path string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	if c.secret != "" {
		req.Header.Set("X-Internal-Token", c.secret)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
