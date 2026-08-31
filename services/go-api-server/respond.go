package main

import (
	"log/slog"
	"net/http"
)

func writeInternalError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	slog.Error(msg, "error", err, "path", r.URL.Path, "method", r.Method)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
