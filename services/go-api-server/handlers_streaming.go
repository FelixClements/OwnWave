package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) StreamURL(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	bitrate := r.URL.Query().Get("bitrate")
	normalize := r.URL.Query().Get("normalize") != "false"
	url := fmt.Sprintf("/stream/%s?format=%s", trackID, format)
	if bitrate != "" {
		url += "&bitrate=" + bitrate
	}
	if !normalize {
		url += "&normalize=false"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

func (h *Handler) StreamTrack(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
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

func (h *Handler) StationCrossfadeURL(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	stationID := chi.URLParam(r, "id")
	if _, err := h.db.GetStationByID(r.Context(), user.ID, stationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
		writeInternalError(w, r, "get station", err)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	bitrate := r.URL.Query().Get("bitrate")
	gapless := r.URL.Query().Get("gapless") == "true"
	normalize := r.URL.Query().Get("normalize") != "false"
	url := fmt.Sprintf("/stations/%s/crossfade?format=%s", stationID, format)
	if bitrate != "" {
		url += "&bitrate=" + bitrate
	}
	if gapless {
		url += "&gapless=true"
	}
	if !normalize {
		url += "&normalize=false"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

func (h *Handler) StationCrossfadeStream(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	stationID := chi.URLParam(r, "id")
	if _, err := h.db.GetStationByID(r.Context(), user.ID, stationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
		writeInternalError(w, r, "get station", err)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}

	queue, err := h.playback.BuildQueue(r.Context(), user.ID, stationID, h.recentHours)
	if err != nil {
		writeInternalError(w, r, "build crossfade queue", err)
		return
	}
	if len(queue) == 0 {
		http.Error(w, "queue empty", 404)
		return
	}

	format = strings.ToLower(format)
	if format != "flac" && format != "mp3" {
		http.Error(w, "unsupported format", 400)
		return
	}

	gapless := r.URL.Query().Get("gapless") == "true"
	normalize := r.URL.Query().Get("normalize") != "false"
	bitrate := r.URL.Query().Get("bitrate")
	h.stream.ServeCrossfaded(w, r, queue, format, bitrate, gapless, normalize)
}
