package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ownwave/api/internal/auth"
)

func TestResolveClientIP(t *testing.T) {
	trusted := parsePrefixes(defaultTrustedProxies)
	cases := []struct {
		name   string
		remote string
		xff    []string
		other  map[string]string
		want   string
	}{
		{name: "direct public client ignores spoofed headers", remote: "203.0.113.7:5555",
			xff: []string{"1.2.3.4"}, other: map[string]string{"X-Real-IP": "5.6.7.8", "True-Client-IP": "9.9.9.9"},
			want: "203.0.113.7"},
		{name: "trusted proxy without XFF keeps peer", remote: "172.18.0.5:4000", want: "172.18.0.5"},
		{name: "trusted proxy uses XFF", remote: "172.18.0.5:4000", xff: []string{"198.51.100.2"}, want: "198.51.100.2"},
		{name: "rightmost untrusted hop wins over client-supplied prefix", remote: "172.18.0.5:4000",
			xff: []string{"1.1.1.1, 198.51.100.2"}, want: "198.51.100.2"},
		{name: "skips trusted hops", remote: "127.0.0.1:4000",
			xff: []string{"198.51.100.2, 10.0.0.3", "192.168.1.10"}, want: "198.51.100.2"},
		{name: "X-Real-IP ignored even from trusted proxy", remote: "172.18.0.5:4000",
			other: map[string]string{"X-Real-IP": "5.6.7.8", "True-Client-IP": "9.9.9.9"}, want: "172.18.0.5"},
		{name: "garbage hop stops the walk", remote: "172.18.0.5:4000",
			xff: []string{"1.1.1.1, not-an-ip, 198.51.100.2"}, want: "198.51.100.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			for k, v := range tc.other {
				r.Header.Set(k, v)
			}
			if got := resolveClientIP(trusted, r).String(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "none")
	if got := loadTrustedProxies(); len(got) != 0 {
		t.Fatalf("none: got %v", got)
	}
	t.Setenv("TRUSTED_PROXIES", "10.1.2.3, 192.168.0.0/24, bogus")
	got := loadTrustedProxies()
	if len(got) != 2 || got[0].String() != "10.1.2.3/32" || got[1].String() != "192.168.0.0/24" {
		t.Fatalf("custom: got %v", got)
	}
}

func TestRouterRateLimitIgnoresSpoofedHeaders(t *testing.T) {
	r := newRouter(&Handler{auth: auth.NewService(nil)}, t.TempDir())
	limited := false
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{}`))
		req.RemoteAddr = "203.0.113.9:1234"
		req.Header.Set("True-Client-IP", "198.51.100."+string(rune('0'+i%10)))
		req.Header.Set("X-Forwarded-For", "198.51.100.200")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
		}
	}
	if !limited {
		t.Fatal("expected spoofed headers not to escape the per-IP login limit")
	}
}

func TestLimitBodyRejectsHugeBodies(t *testing.T) {
	var readErr error
	h := limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.Copy(io.Discard, r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxBodyBytes+1)))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if readErr == nil {
		t.Fatal("expected body over the cap to fail")
	}
}

func TestLoginThrottle(t *testing.T) {
	now := time.Unix(0, 0)
	lt := newLoginThrottle(3, time.Minute)
	lt.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if lt.Blocked("Alice") {
			t.Fatalf("blocked after %d failures", i)
		}
		lt.Fail(" alice ")
	}
	if !lt.Blocked("ALICE") {
		t.Fatal("expected block after max failures, case-insensitive")
	}
	if lt.Blocked("bob") {
		t.Fatal("other users must not be blocked")
	}
	now = now.Add(time.Minute)
	if lt.Blocked("alice") {
		t.Fatal("block should expire after the window")
	}
	lt.Fail("alice")
	lt.Succeed("alice")
	if lt.Blocked("alice") {
		t.Fatal("success should clear failures")
	}
	var nilThrottle *loginThrottle
	nilThrottle.Fail("x")
	if nilThrottle.Blocked("x") {
		t.Fatal("nil throttle must allow")
	}
}
