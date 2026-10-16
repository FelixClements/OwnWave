package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

func httpAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"ms", time.Since(start).Milliseconds(),
		)
	})
}
