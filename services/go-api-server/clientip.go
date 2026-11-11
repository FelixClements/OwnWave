package main

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// defaultTrustedProxies covers loopback and private networks, where a reverse
// proxy in front of the app (the compose Caddy, one on the LAN, or one reached
// over a Tailscale-style 100.64/10 overlay) lives.
var defaultTrustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10",
	"fc00::/7",
}

// loadTrustedProxies reads TRUSTED_PROXIES (comma-separated CIDRs or IPs).
// Unset means the private defaults; "none" trusts no proxy at all.
func loadTrustedProxies() []netip.Prefix {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if strings.EqualFold(raw, "none") {
		return nil
	}
	entries := defaultTrustedProxies
	if raw != "" {
		entries = strings.Split(raw, ",")
	}
	return parsePrefixes(entries)
}

func parsePrefixes(entries []string) []netip.Prefix {
	var out []netip.Prefix
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if p, err := netip.ParsePrefix(e); err == nil {
			out = append(out, p.Masked())
			continue
		}
		if a, err := netip.ParseAddr(e); err == nil {
			out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
			continue
		}
		slog.Warn("ignoring invalid TRUSTED_PROXIES entry", "entry", e)
	}
	return out
}

func isTrusted(trusted []netip.Prefix, a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// clientIP rewrites r.RemoteAddr to the real client address so the rate
// limiters and access log key on it. X-Forwarded-For is honoured only when
// the TCP peer is a trusted proxy, and then read right to left, skipping
// trusted hops, so a client cannot pick its own address. Other client-IP
// headers (X-Real-IP, True-Client-IP) are never trusted: a proxy may pass
// them through from the client unchanged.
func clientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := resolveClientIP(trusted, r); ip.IsValid() {
				r.RemoteAddr = net.JoinHostPort(ip.String(), "0")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolveClientIP(trusted []netip.Prefix, r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	peer = peer.Unmap()
	if !isTrusted(trusted, peer) {
		return peer
	}
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		client = a.Unmap()
		if !isTrusted(trusted, client) {
			break
		}
	}
	return client
}

// maxBodyBytes caps request bodies on every route; handlers that need less
// set their own smaller cap on top.
const maxBodyBytes = 1 << 20

// limitBody caps the size of request bodies so a client cannot exhaust
// memory with a huge JSON value.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
