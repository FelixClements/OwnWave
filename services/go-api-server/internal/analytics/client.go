package analytics

// Client proxies OwnWave Python analytics endpoints (scanning, analysis, genres).
type Client interface {
	Scan(body []byte) (*Response, error)
	GetJob(jobID string) (*Response, error)
	Health() (*Response, error)
	RebuildVectors() (*Response, error)
	RebuildClusters() (*Response, error)
	ListGenres() (*Response, error)
	GetTrackGenres(trackID string) (*Response, error)
	RebuildGenres() (*Response, error)
	SetupSummary() (*Response, error)
}
