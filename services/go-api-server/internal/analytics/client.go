package analytics

import "io"

// Client proxies OwnWave Python analytics endpoints.
type Client interface {
	GetSimilarTracks(trackID, limit string) (*Response, error)
	Scan(body []byte) (*Response, error)
	GetJob(jobID string) (*Response, error)
	CreateStation(body []byte) (*Response, error)
	UpdateStation(stationID string, body []byte) (*Response, error)
	Health() (*Response, error)
	RebuildVectors() (*Response, error)
	RebuildClusters() (*Response, error)
	ListGenres() (*Response, error)
	GetTrackGenres(trackID string) (*Response, error)
	RebuildGenres() (*Response, error)
	RebuildGenreStations() (*Response, error)
	SetupSummary() (*Response, error)
	SetupStations(body io.Reader) (*Response, error)
	GetStationTracklist(stationID string) (*Response, error)
}
