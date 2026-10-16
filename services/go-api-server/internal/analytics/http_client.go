package analytics

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
)

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
		httpClient: http.DefaultClient,
	}
}

func (c *HTTPClient) Scan(body []byte) (*Response, error) {
	return c.post("/scan", "application/json", bytes.NewReader(body))
}

func (c *HTTPClient) GetJob(jobID string) (*Response, error) {
	return c.get("/jobs/" + jobID)
}

func (c *HTTPClient) Health() (*Response, error) {
	return c.get("/health")
}

func (c *HTTPClient) RebuildVectors() (*Response, error) {
	return c.post("/rebuild-vectors", "application/json", nil)
}

func (c *HTTPClient) RebuildClusters() (*Response, error) {
	return c.post("/rebuild-clusters", "application/json", nil)
}

func (c *HTTPClient) ListGenres() (*Response, error) {
	return c.get("/genres")
}

func (c *HTTPClient) GetTrackGenres(trackID string) (*Response, error) {
	return c.get("/tracks/" + trackID + "/genres")
}

func (c *HTTPClient) RebuildGenres() (*Response, error) {
	return c.post("/rebuild-genres", "application/json", nil)
}

func (c *HTTPClient) SetupSummary() (*Response, error) {
	return c.get("/setup/summary")
}

func (c *HTTPClient) get(path string) (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}

func (c *HTTPClient) post(path, contentType string, body io.Reader) (*Response, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.setAuth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}

func (c *HTTPClient) setAuth(req *http.Request) {
	if c.secret != "" {
		req.Header.Set("X-Internal-Token", c.secret)
	}
}
