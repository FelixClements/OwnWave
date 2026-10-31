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

func TestCreateStationRejectsInvalidSeed(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	cases := map[string]string{
		"missing name":    `{"seed_type":"mood"}`,
		"genre is gone":   `{"name":"x","seed_type":"genre","main_genre":"rock"}`,
		"no playlists":    `{"name":"x","seed_type":"playlists"}`,
		"bad playlist id": `{"name":"x","seed_type":"playlists","playlist_ids":["nope"]}`,
		"bad track id":    `{"name":"x","seed_type":"track","track_id":"nope"}`,
		"unknown type":    `{"name":"x","seed_type":"vibes"}`,
		"energy range":    `{"name":"x","seed_type":"mood","min_energy":2}`,
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

func TestOnlyAdminsCreatePlaylistStations(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	body := `{"name":"Rock","seed_type":"playlists","playlist_ids":["6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"]}`
	req := httptest.NewRequest(http.MethodPost, "/stations", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"}))
	rec := httptest.NewRecorder()
	h.CreateStation(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
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

func TestListScansValidatesLimit(t *testing.T) {
	stub := analytics.NewStub()
	h := newTestHandler(stub)

	for _, tc := range []struct {
		query    string
		wantCode int
		wantCall string
	}{
		{"", http.StatusOK, "ListJobs:10"},
		{"?limit=25", http.StatusOK, "ListJobs:25"},
		{"?limit=0", http.StatusBadRequest, ""},
		{"?limit=51", http.StatusBadRequest, ""},
		{"?limit=abc", http.StatusBadRequest, ""},
	} {
		t.Run(tc.query, func(t *testing.T) {
			stub.Calls = nil
			rec := httptest.NewRecorder()
			h.ListScans(rec, httptest.NewRequest(http.MethodGet, "/admin/scans"+tc.query, nil))
			require.Equal(t, tc.wantCode, rec.Code)
			if tc.wantCall == "" {
				require.Empty(t, stub.Calls)
			} else {
				require.Equal(t, []string{tc.wantCall}, stub.Calls)
			}
		})
	}
}
