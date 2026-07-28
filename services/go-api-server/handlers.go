package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db         *DB
	jwtSecret  []byte
	musicDir   string
	ffmpegPath string
	pythonURL  string
}

func NewHandler(pool *pgxpool.Pool, jwtSecret []byte, musicDir, ffmpegPath, pythonURL string) *Handler {
	return &Handler{
		db:         NewDB(pool),
		jwtSecret:  jwtSecret,
		musicDir:   musicDir,
		ffmpegPath: ffmpegPath,
		pythonURL:  strings.TrimRight(pythonURL, "/"),
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) ListTracks(w http.ResponseWriter, r *http.Request) {
	tracks, err := h.db.ListTracks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tracks": tracks})
}

func (h *Handler) GetTrack(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	track, err := h.db.GetTrackByID(r.Context(), id)
	if err != nil {
		http.Error(w, "track not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(track)
}

func (h *Handler) ListStations(w http.ResponseWriter, r *http.Request) {
	stations, err := h.db.ListStations(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"stations": stations})
}

func (h *Handler) GetStation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "use /stations/{id}/queue"})
}

func (h *Handler) GetQueue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	queue, err := h.db.GetStationQueue(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"queue": queue})
}

func (h *Handler) CreateStation(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	resp, err := http.Post(h.pythonURL+"/stations", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path  string `json:"path"`
		Force bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Path = h.musicDir
	}
	if req.Path == "" {
		req.Path = h.musicDir
	}

	payload, _ := json.Marshal(req)
	resp, err := http.Post(h.pythonURL+"/scan", "application/json", bytes.NewReader(payload))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) StreamURL(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "flac"
	}
	token, err := h.signStreamToken(trackID, format)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	url := fmt.Sprintf("/stream/%s?format=%s&token=%s", trackID, format, token)
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
	if _, err := exec.LookPath(h.ffmpegPath); err != nil && format == "mp3" {
		http.Error(w, "ffmpeg not available", 500)
		return
	}

	switch strings.ToLower(format) {
	case "mp3":
		h.serveMP3(w, r, fullPath)
	default:
		h.serveFLAC(w, r, fullPath)
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

func getString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	if v, ok := claims[key].(float64); ok {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}
