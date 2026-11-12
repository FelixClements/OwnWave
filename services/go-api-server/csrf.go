package main

import (
	"log/slog"
	"net/http"
)

// crossOriginProtection rejects state-changing requests (POST, PUT, PATCH,
// DELETE) that a browser sent from another origin. The session cookie is
// SameSite=Lax, which still sends it from sibling subdomains (same site), so
// this is the CSRF defence. Browsers mark requests with Sec-Fetch-Site (or
// Origin); same-origin requests and clients that send neither, such as the
// Android app and scripts using a bearer token, pass. The ALLOWED_ORIGINS
// list is trusted as well.
func crossOriginProtection(trusted []string) func(http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	for _, origin := range trusted {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			slog.Warn("ignoring invalid allowed origin for CSRF check", "origin", origin, "error", err)
		}
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Warn("cross-origin request rejected",
			"method", r.Method, "path", r.URL.Path,
			"origin", r.Header.Get("Origin"), "sec_fetch_site", r.Header.Get("Sec-Fetch-Site"))
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin request rejected"})
	}))
	return cop.Handler
}
