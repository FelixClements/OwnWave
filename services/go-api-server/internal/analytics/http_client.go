package analytics

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// HTTPClient calls the Python analytics service over HTTP.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
	}
}

func (c *HTTPClient) GetSimilarTracks(trackID, limit string) (*Response, error) {
	return c.get("/tracks/" + trackID + "/similar?limit=" + limit)
}

func (c *HTTPClient) Scan(body []byte) (*Response, error) {
	return c.post("/scan", "application/json", bytes.NewReader(body))
}

func (c *HTTPClient) GetJob(jobID string) (*Response, error) {
	return c.get("/jobs/" + jobID)
}

func (c *HTTPClient) CreateStation(body []byte) (*Response, error) {
	return c.post("/stations", "application/json", bytes.NewReader(body))
}

func (c *HTTPClient) UpdateStation(stationID string, body []byte) (*Response, error) {
	return c.patch("/stations/"+stationID, "application/json", bytes.NewReader(body))
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

func (c *HTTPClient) RebuildGenreStations() (*Response, error) {
	return c.post("/rebuild-genre-stations", "application/json", nil)
}

func (c *HTTPClient) SetupSummary() (*Response, error) {
	return c.get("/setup/summary")
}

func (c *HTTPClient) SetupStations(body io.Reader) (*Response, error) {
	return c.post("/setup/stations", "application/json", body)
}

func (c *HTTPClient) GetStationTracklist(stationID string) (*Response, error) {
	return c.get("/stations/" + stationID + "/tracklist")
}

func (c *HTTPClient) get(path string) (*Response, error) {
	resp, err := c.httpClient.Get(c.baseURL + path)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}

func (c *HTTPClient) post(path, contentType string, body io.Reader) (*Response, error) {
	resp, err := c.httpClient.Post(c.baseURL+path, contentType, body)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}

func (c *HTTPClient) patch(path, contentType string, body io.Reader) (*Response, error) {
	req, err := http.NewRequest(http.MethodPatch, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}
