package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) StreamURL(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	bitrate := r.URL.Query().Get("bitrate")
	normalize := r.URL.Query().Get("normalize") != "false"
	token, err := h.streamTokens.SignTrack(trackID, format)
	if err != nil {
		writeInternalError(w, r, "sign stream token", err)
		return
	}
	url := fmt.Sprintf("/stream/%s?format=%s&token=%s", trackID, format, token)
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
	token := r.URL.Query().Get("token")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	if token == "" {
		http.Error(w, "missing token", 401)
		return
	}
	claimTrackID, _, err := h.streamTokens.ValidateTrack(token)
	if err != nil || claimTrackID != trackID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
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
	stationID := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	bitrate := r.URL.Query().Get("bitrate")
	gapless := r.URL.Query().Get("gapless") == "true"
	normalize := r.URL.Query().Get("normalize") != "false"
	token, err := h.streamTokens.SignStation(stationID, format)
	if err != nil {
		writeInternalError(w, r, "sign stream token", err)
		return
	}
	url := fmt.Sprintf("/stations/%s/crossfade?format=%s&token=%s", stationID, format, token)
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
	stationID := chi.URLParam(r, "id")
	token := r.URL.Query().Get("token")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	if token == "" {
		http.Error(w, "missing token", 401)
		return
	}
	claimStationID, _, err := h.streamTokens.ValidateStation(token)
	if err != nil || claimStationID != stationID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	queue, err := h.playback.BuildQueue(r.Context(), stationID, h.recentHours)
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
