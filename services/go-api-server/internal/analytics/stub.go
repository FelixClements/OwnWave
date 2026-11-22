package analytics

import "context"

// Stub records calls and returns canned errors for handler tests.
type Stub struct {
	Errors map[string]error
	Calls  []string
}

func NewStub() *Stub {
	return &Stub{Errors: make(map[string]error)}
}

func (s *Stub) Health(ctx context.Context) error {
	return s.call("Health")
}

func (s *Stub) RebuildGenres(ctx context.Context) error {
	return s.call("RebuildGenres")
}

func (s *Stub) call(name string) error {
	s.Calls = append(s.Calls, name)
	return s.Errors[name]
}
