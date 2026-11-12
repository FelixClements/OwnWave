package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrossOriginProtection(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := crossOriginProtection([]string{"https://music.example.com"})(ok)

	cases := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"same origin", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"sibling subdomain", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://evil.example.com"}, http.StatusForbidden},
		{"cross site", http.MethodDelete, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.test"}, http.StatusForbidden},
		{"origin only, mismatched host", http.MethodPut, map[string]string{"Origin": "https://evil.example.com"}, http.StatusForbidden},
		{"origin only, same host", http.MethodPost, map[string]string{"Origin": "https://app.example.com"}, http.StatusOK},
		{"trusted origin", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://music.example.com"}, http.StatusOK},
		{"non-browser client", http.MethodPost, map[string]string{"Authorization": "Bearer x"}, http.StatusOK},
		{"cross-site GET", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "https://app.example.com/api/logout", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
