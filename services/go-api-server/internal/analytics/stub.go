package analytics

import (
	"context"
	"strconv"
)

// Stub records calls and returns canned responses for handler tests.
type Stub struct {
	Responses map[string]*Response
	Errors    map[string]error
	Calls     []string
}

func NewStub() *Stub {
	return &Stub{
		Responses: make(map[string]*Response),
		Errors:    make(map[string]error),
	}
}

func (s *Stub) Scan(ctx context.Context, body []byte) (*Response, error) {
	return s.call("Scan")
}

func (s *Stub) GetJob(ctx context.Context, jobID string) (*Response, error) {
	return s.call("GetJob:" + jobID)
}

func (s *Stub) ListJobs(ctx context.Context, limit int) (*Response, error) {
	return s.call("ListJobs:" + strconv.Itoa(limit))
}

func (s *Stub) Health(ctx context.Context) (*Response, error) {
	return s.call("Health")
}

func (s *Stub) RebuildAnalysis(ctx context.Context) (*Response, error) {
	return s.call("RebuildAnalysis")
}

func (s *Stub) ListGenres(ctx context.Context) (*Response, error) {
	return s.call("ListGenres")
}

func (s *Stub) GetTrackGenres(ctx context.Context, trackID string) (*Response, error) {
	return s.call("GetTrackGenres:" + trackID)
}

func (s *Stub) RebuildGenres(ctx context.Context) (*Response, error) {
	return s.call("RebuildGenres")
}

func (s *Stub) SetupSummary(ctx context.Context) (*Response, error) {
	return s.call("SetupSummary")
}

func (s *Stub) call(name string) (*Response, error) {
	s.Calls = append(s.Calls, name)
	if err := s.Errors[name]; err != nil {
		return nil, err
	}
	if resp, ok := s.Responses[name]; ok {
		return resp, nil
	}
	return &Response{StatusCode: 200, Body: []byte(`{"status":"ok"}`)}, nil
}
