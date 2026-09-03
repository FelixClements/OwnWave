package analytics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStubRecordsCalls(t *testing.T) {
	stub := NewStub()
	stub.Responses["ListGenres"] = &Response{StatusCode: 200, Body: []byte(`[{"main_genre":"Rock"}]`)}

	resp, err := stub.ListGenres()
	if err != nil {
		t.Fatalf("ListGenres() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(stub.Calls) != 1 || stub.Calls[0] != "ListGenres" {
		t.Fatalf("calls = %#v, want [ListGenres]", stub.Calls)
	}
}

func TestStubReturnsConfiguredError(t *testing.T) {
	stub := NewStub()
	stub.Errors["Health"] = errors.New("down")

	if _, err := stub.Health(); err == nil || err.Error() != "down" {
		t.Fatalf("Health() error = %v, want down", err)
	}
}

func TestResponseWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &Response{StatusCode: 201, Body: []byte(`{"ok":true}`)}
	resp.WriteJSON(rec)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHTTPClientProxiesPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/genres" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewHTTPClientWithSecret(server.URL, "test-secret")
	resp, err := client.ListGenres()
	if err != nil {
		t.Fatalf("ListGenres() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(resp.Body) != "[]" {
		t.Fatalf("resp = %#v", resp)
	}
}

func TestHTTPClientWithUserSendsHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-OwnWave-User-Id") != "user-1" {
			t.Fatalf("user id = %q", r.Header.Get("X-OwnWave-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClientWithSecret(server.URL, "s3cret").WithUser("user-1")
	if _, err := client.CreateStation([]byte(`{}`)); err != nil {
		t.Fatalf("CreateStation() error = %v", err)
	}
}

func TestHTTPClientSendsInternalToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "s3cret" {
			t.Fatalf("missing internal token: %q", r.Header.Get("X-Internal-Token"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClientWithSecret(server.URL, "s3cret")
	if _, err := client.Health(); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}
