package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)
func (h *Handler) StreamURL(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	bitrate := r.URL.Query().Get("bitrate")
	normalize := r.URL.Query().Get("normalize") != "false"
	token, err := h.signStreamToken(trackID, format)
	if err != nil {
		http.Error(w, err.Error(), 500)
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
	if _, _, err := h.validateStreamToken(token); err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}

	track, err := h.db.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "track not found", 404)
		return
	}

	fullPath := track.Path
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(h.musicDir, fullPath)
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
		h.serveFLAC(w, r, fullPath)
	case "mp3", "opus", "aac":
		h.serveTranscoded(w, r, fullPath, format, track.Loudness, normalize)
	default:
		http.Error(w, "unsupported format", 400)
	}
}
func (h *Handler) signStreamToken(trackID, format string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"track_id": trackID,
		"format":   format,
		"exp":      time.Now().Add(10 * time.Minute).Unix(),
	})
	return token.SignedString(h.jwtSecret)
}
func (h *Handler) validateStreamToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid claims")
	}
	return getString(claims, "track_id"), getString(claims, "format"), nil
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
	token, err := h.signStationStreamToken(stationID, format)
	if err != nil {
		http.Error(w, err.Error(), 500)
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
	if _, _, err := h.validateStationStreamToken(token); err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}

	queue, err := h.playback.BuildQueue(r.Context(), stationID, h.recentHours)
	if err != nil {
		http.Error(w, err.Error(), 500)
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
	h.serveCrossfaded(w, r, queue, format, bitrate, gapless, normalize)
}
func (h *Handler) signStationStreamToken(stationID, format string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"station_id": stationID,
		"format":     format,
		"exp":        time.Now().Add(10 * time.Minute).Unix(),
	})
	return token.SignedString(h.jwtSecret)
}
func (h *Handler) validateStationStreamToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid claims")
	}
	return getString(claims, "station_id"), getString(claims, "format"), nil
}
