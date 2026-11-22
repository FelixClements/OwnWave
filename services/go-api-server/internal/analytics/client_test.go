package analytics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestStubRecordsCalls(t *testing.T) {
	stub := NewStub()
	stub.Errors["Health"] = errors.New("down")

	if err := stub.Health(context.Background()); err == nil || err.Error() != "down" {
		t.Fatalf("Health() error = %v, want down", err)
	}
	if err := stub.RebuildGenres(context.Background()); err != nil {
		t.Fatalf("RebuildGenres() error = %v", err)
	}
	if len(stub.Calls) != 2 || stub.Calls[0] != "Health" || stub.Calls[1] != "RebuildGenres" {
		t.Fatalf("calls = %#v", stub.Calls)
	}
}

func TestHTTPClientHealth(t *testing.T) {
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Internal-Token") != "s3cret" {
			t.Errorf("missing internal token: %q", r.Header.Get("X-Internal-Token"))
		}
		w.WriteHeader(status)
	}))
	defer server.Close()

	client := NewHTTPClientWithSecret(server.URL, "s3cret")
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := client.Health(context.Background()); err == nil || err.Error() != "status 503" {
		t.Fatalf("Health() error = %v, want status 503", err)
	}
}

func TestHTTPClientRebuildGenresFailsOnErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rebuild-genres" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := NewHTTPClientWithSecret(server.URL, "s").RebuildGenres(context.Background()); err == nil {
		t.Fatal("expected error for a 500 answer")
	}
}

func TestHTTPClientHonoursContextCancel(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer server.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewHTTPClientWithSecret(server.URL, "s").Health(ctx); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// pythonStub answers like the analytics service and records what it got.
type pythonStub struct {
	*httptest.Server
	method, path, query, body string
	header                    http.Header
}

func newPythonStub(t *testing.T) *pythonStub {
	s := &pythonStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.method, s.path, s.query, s.body, s.header = r.Method, r.URL.Path, r.URL.RawQuery, string(body), r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"j","status":"queued"}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func serveRoutes(p *Proxy) http.Handler {
	r := chi.NewRouter()
	for _, rt := range Routes {
		r.Method(rt.Method, rt.Public, p.Handler(rt))
	}
	return r
}

func TestProxyForwardsEveryRoute(t *testing.T) {
	py := newPythonStub(t)
	router := serveRoutes(NewProxyWithSecret(py.URL, "s3cret", nil))
	const id = "0b6f3c1e-8d2a-4a8e-9a57-3f1d2c4b5a69"

	for _, rt := range Routes {
		public := strings.ReplaceAll(rt.Public, "{id}", id)
		want := strings.ReplaceAll(rt.Python, "{id}", id)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(rt.Method, public, nil))

		if rec.Code != http.StatusAccepted || rec.Body.String() != `{"job_id":"j","status":"queued"}` {
			t.Fatalf("%s %s: got %d %q", rt.Method, public, rec.Code, rec.Body.String())
		}
		if py.method != rt.Method || py.path != want {
			t.Fatalf("%s %s: Python got %s %s, want %s", rt.Method, public, py.method, py.path, want)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("%s %s: content-type %q", rt.Method, public, got)
		}
	}
}

func TestProxyPassesBodyAndQueryAndDropsCallerAuth(t *testing.T) {
	py := newPythonStub(t)
	router := serveRoutes(NewProxyWithSecret(py.URL, "s3cret", nil))

	req := httptest.NewRequest(http.MethodPost, "/admin/scan", strings.NewReader(`{"force":true}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Cookie", "ownwave_session=abc")
	req.Header.Set("Authorization", "Bearer device-token")
	router.ServeHTTP(httptest.NewRecorder(), req)

	if py.body != `{"force":true}` {
		t.Fatalf("body = %q", py.body)
	}
	if got := py.header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if py.header.Get("X-Internal-Token") != "s3cret" {
		t.Fatalf("internal token = %q", py.header.Get("X-Internal-Token"))
	}
	if py.header.Get("Cookie") != "" || py.header.Get("Authorization") != "" {
		t.Fatalf("caller credentials forwarded: %v", py.header)
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/scans?limit=5", nil))
	if py.path != "/jobs" || py.query != "limit=5" {
		t.Fatalf("got %s?%s, want /jobs?limit=5", py.path, py.query)
	}
}

func TestProxyRejectsNonUUIDParam(t *testing.T) {
	py := newPythonStub(t)
	router := serveRoutes(NewProxyWithSecret(py.URL, "s", nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/scan/not-a-uuid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if py.path != "" {
		t.Fatalf("Python was called: %s", py.path)
	}
}

func TestProxyReportsUnreachablePython(t *testing.T) {
	py := newPythonStub(t)
	url := py.URL
	py.Close()

	var got error
	onError := func(w http.ResponseWriter, r *http.Request, err error) {
		got = err
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	rec := httptest.NewRecorder()
	serveRoutes(NewProxyWithSecret(url, "s", onError)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/genres", nil))
	if rec.Code != http.StatusInternalServerError || got == nil {
		t.Fatalf("status = %d, err = %v", rec.Code, got)
	}
}

func TestRoutesForSplitsByGuard(t *testing.T) {
	n := len(RoutesFor(AdminOnly)) + len(RoutesFor(AnyUser)) + len(RoutesFor(SetupAccess))
	if n != len(Routes) {
		t.Fatalf("guards cover %d of %d routes", n, len(Routes))
	}
}
