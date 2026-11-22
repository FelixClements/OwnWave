package main

import (
	"context"
	"io"
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
	return &Handler{
		auth:         auth.NewService(nil),
		analytics:    stub,
		analyticsAPI: newAnalyticsProxy("http://analytics.invalid"),
	}
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

// pythonStub stands in for the analytics service and records the last request.
func pythonStub(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// analyticsRouter serves the pass-through routes behind one guard, without
// the guard itself.
func analyticsRouter(h *Handler, access analytics.Access) http.Handler {
	r := chi.NewRouter()
	h.mountAnalytics(r, access)
	return r
}

func TestProxyAnalyticsForwardsUpstreamStatus(t *testing.T) {
	py, _ := pythonStub(t, http.StatusServiceUnavailable, `{"detail":"down"}`)
	h := newTestHandler(analytics.NewStub())
	h.analyticsAPI = newAnalyticsProxy(py.URL)

	rec := httptest.NewRecorder()
	analyticsRouter(h, analytics.AnyUser).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/genres", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"detail":"down"}`, rec.Body.String())
}

func TestProxyAnalyticsAnswers500WhenPythonIsDown(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	h.analyticsAPI = newAnalyticsProxy("http://127.0.0.1:1")

	rec := httptest.NewRecorder()
	analyticsRouter(h, analytics.SetupAccess).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/setup/summary", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
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

func TestCreateStationRejectsInvalidRequests(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	cases := map[string]string{
		"not json":        `{`,
		"missing name":    `{"playlist_ids":["6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"]}`,
		"blank name":      `{"name":"  ","playlist_ids":["6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"]}`,
		"no playlists":    `{"name":"x"}`,
		"bad playlist id": `{"name":"x","playlist_ids":["nope"]}`,
		"bad genre id":    `{"name":"x","playlist_ids":["6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"],"genres":["Hard Techno"]}`,
		"old seed format": `{"name":"x","seed_type":"mood","min_energy":0.5}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/stations", strings.NewReader(body))
			req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"}))
			rec := httptest.NewRecorder()
			h.CreateStation(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestFeedbackOnlyAcceptsLikeAndBan(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	user := auth.User{ID: "6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"}
	for _, body := range []string{`{"feedback":"skip"}`, `{"feedback":""}`, `{}`} {
		req := httptest.NewRequest(http.MethodPost, "/tracks/x/feedback", strings.NewReader(body))
		req = req.WithContext(auth.WithUser(req.Context(), user))
		rec := httptest.NewRecorder()
		h.RecordFeedback(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

// TestListScansPassesLimit: Python owns the 1-50 limit (FastAPI answers 422
// outside it); Go forwards the query unchanged.
func TestListScansPassesLimit(t *testing.T) {
	py, got := pythonStub(t, http.StatusOK, `{"jobs":[]}`)
	h := newTestHandler(analytics.NewStub())
	h.analyticsAPI = newAnalyticsProxy(py.URL)
	router := analyticsRouter(h, analytics.AdminOnly)

	for query, want := range map[string]string{"": "GET /jobs", "?limit=25": "GET /jobs?limit=25"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/scans"+query, nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, want, *got)
	}
}

// TestTriggerScanPassesForce: Python scans its own MUSIC_DIR when no path is
// given, so Go forwards the body as it came (SoulLite posts here, ADR 0003).
func TestTriggerScanPassesForce(t *testing.T) {
	var body, contentType string
	py := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body, contentType = string(b), r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"job_id":"j","status":"queued"}`))
	}))
	t.Cleanup(py.Close)
	h := newTestHandler(analytics.NewStub())
	h.analyticsAPI = newAnalyticsProxy(py.URL)
	router := analyticsRouter(h, analytics.AdminOnly)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/admin/scan", strings.NewReader(`{"force":true}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"force":true}`, body)
	require.Equal(t, "application/json", contentType)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/admin/scan", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, body)
}

func TestStreamRejectsUnknownQuality(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	for _, q := range []string{"ultra", "320k", "flac"} {
		req := httptest.NewRequest(http.MethodGet, "/stream/x?quality="+q, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		h.StreamTrack(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, q)
	}
}
