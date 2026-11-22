package analytics

import "context"

// Client is the part of the Python analytics service that Go handlers use
// themselves. Every other analytics route is a pass-through: see Routes.
type Client interface {
	// Health returns nil when the service answers 200.
	Health(ctx context.Context) error
	// RebuildGenres queues genre scoring, which also applies genre corrections.
	RebuildGenres(ctx context.Context) error
}
