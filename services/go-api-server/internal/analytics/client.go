package analytics

import "context"

// Client proxies OwnWave Python analytics endpoints (scanning, analysis, genres).
type Client interface {
	Scan(ctx context.Context, body []byte) (*Response, error)
	GetJob(ctx context.Context, jobID string) (*Response, error)
	Health(ctx context.Context) (*Response, error)
	RebuildVectors(ctx context.Context) (*Response, error)
	RebuildClusters(ctx context.Context) (*Response, error)
	ListGenres(ctx context.Context) (*Response, error)
	GetTrackGenres(ctx context.Context, trackID string) (*Response, error)
	RebuildGenres(ctx context.Context) (*Response, error)
	SetupSummary(ctx context.Context) (*Response, error)
}
