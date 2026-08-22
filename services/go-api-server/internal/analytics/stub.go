package analytics

import "io"

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

func (s *Stub) GetSimilarTracks(trackID, limit string) (*Response, error) {
	return s.call("GetSimilarTracks:" + trackID + ":" + limit)
}

func (s *Stub) Scan(body []byte) (*Response, error) {
	return s.call("Scan")
}

func (s *Stub) GetJob(jobID string) (*Response, error) {
	return s.call("GetJob:" + jobID)
}

func (s *Stub) CreateStation(body []byte) (*Response, error) {
	return s.call("CreateStation")
}

func (s *Stub) UpdateStation(stationID string, body []byte) (*Response, error) {
	return s.call("UpdateStation:" + stationID)
}

func (s *Stub) Health() (*Response, error) {
	return s.call("Health")
}

func (s *Stub) RebuildVectors() (*Response, error) {
	return s.call("RebuildVectors")
}

func (s *Stub) RebuildClusters() (*Response, error) {
	return s.call("RebuildClusters")
}

func (s *Stub) ListGenres() (*Response, error) {
	return s.call("ListGenres")
}

func (s *Stub) GetTrackGenres(trackID string) (*Response, error) {
	return s.call("GetTrackGenres:" + trackID)
}

func (s *Stub) RebuildGenres() (*Response, error) {
	return s.call("RebuildGenres")
}

func (s *Stub) RebuildGenreStations() (*Response, error) {
	return s.call("RebuildGenreStations")
}

func (s *Stub) SetupSummary() (*Response, error) {
	return s.call("SetupSummary")
}

func (s *Stub) SetupStations(body io.Reader) (*Response, error) {
	return s.call("SetupStations")
}

func (s *Stub) GetStationTracklist(stationID string) (*Response, error) {
	return s.call("GetStationTracklist:" + stationID)
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
