package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
)

// newTestHandler builds a Handler without a database; only use it for paths
// that fail validation or proxy to the analytics stub before touching the DB.
func newTestHandler(stub *analytics.Stub) *Handler {
	return &Handler{auth: auth.NewService(nil), analytics: stub}
}

func TestRegisterRejectsInvalidPasswords(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	cases := map[string]string{
		"short":    "short",
		"too long": strings.Repeat("a", 73),
	}
	for name, password := range cases {
		t.Run(name, func(t *testing.T) {
			body := `{"username":"alice","password":"` + password + `"}`
			rec := httptest.NewRecorder()
			h.Register(rec, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body)))
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "password must be")
		})
	}
}

func TestLoginRequiresCredentials(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	rec := httptest.NewRecorder()
	h.Login(rec, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"alice"}`)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInviteURL(t *testing.T) {
	cases := []struct {
		name, publicURL, domain, want string
	}{
		{"public url", "https://music.example.com/", "", "https://music.example.com/invite/tok"},
		{"domain only", "", "music.example.com", "https://music.example.com/invite/tok"},
		{"nothing configured", "", "", "/invite/tok"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PUBLIC_APP_URL", tc.publicURL)
			t.Setenv("OWNWAVE_DOMAIN", tc.domain)
			require.Equal(t, tc.want, inviteURL("tok"))
		})
	}
}

func TestProxyAnalyticsForwardsUpstreamStatus(t *testing.T) {
	stub := analytics.NewStub()
	stub.Responses["ListGenres"] = &analytics.Response{StatusCode: http.StatusServiceUnavailable, Body: []byte(`{"detail":"down"}`)}
	h := newTestHandler(stub)

	rec := httptest.NewRecorder()
	h.ListGenres(rec, httptest.NewRequest(http.MethodGet, "/genres", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"detail":"down"}`, rec.Body.String())
}

func TestMetricsUseRoutePattern(t *testing.T) {
	r := chi.NewRouter()
	r.Use(prometheusMiddleware)
	r.Get("/tracks/{id}", func(w http.ResponseWriter, r *http.Request) {})

	for _, id := range []string{"a", "b", "c"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tracks/"+id, nil))
	}

	labels := prometheus.Labels{"method": http.MethodGet, "route": "/tracks/{id}", "status": http.StatusText(http.StatusOK)}
	require.Equal(t, 3.0, testutil.ToFloat64(requestCount.With(labels)))
}
