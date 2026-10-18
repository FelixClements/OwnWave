package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

// parseUUIDParam returns the {name} URL param, writing 400 if it isn't a UUID.
func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	v := chi.URLParam(r, name)
	if _, err := uuid.Parse(v); err != nil {
		http.Error(w, "invalid "+name, http.StatusBadRequest)
		return "", false
	}
	return v, true
}

func writeInternalError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	slog.Error(msg, "error", err, "path", r.URL.Path, "method", r.Method)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// contentSecurityPolicy still allows inline scripts and styles, which the
// static Next.js export needs.
const contentSecurityPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"

// securityHeaders sets the browser security headers on every response, so they
// apply behind any reverse proxy. HSTS is only sent when the app is served over
// HTTPS, which OWNWAVE_COOKIE_SECURE already signals.
func securityHeaders(https bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", contentSecurityPolicy)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if https {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func httpAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		path := r.URL.Path
		if strings.HasPrefix(path, "/invite/") {
			path = "/invite/REDACTED" // invite tokens are credentials
		}
		slog.Info("http",
			"method", r.Method,
			"path", path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"ms", time.Since(start).Milliseconds(),
		)
	})
}
