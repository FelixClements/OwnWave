package main

import (
	"net/http"
	"os/exec"
	"strings"
)

func (h *Handler) StreamTrack(w http.ResponseWriter, r *http.Request) {
	trackID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}

	track, err := h.db.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "track not found", 404)
		return
	}

	fullPath, err := h.stream.ResolvePath(track.Path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if strings.ToLower(format) != "flac" {
		if _, err := exec.LookPath(h.ffmpegPath); err != nil {
			http.Error(w, "ffmpeg not available", 500)
			return
		}
	}

	normalize := r.URL.Query().Get("normalize") != "false"

	switch strings.ToLower(format) {
	case "flac":
		h.stream.ServeFLAC(w, r, fullPath)
	case "mp3", "opus", "aac":
		h.stream.ServeTranscoded(w, r, fullPath, format, track.Loudness, normalize)
	default:
		http.Error(w, "unsupported format", 400)
	}
}
