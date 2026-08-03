package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"golang.org/x/crypto/bcrypt"
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

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results, err := h.db.Search(r.Context(), q)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.db.ListAlbums(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"albums": albums})
}

func (h *Handler) ListArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := h.db.ListArtists(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"artists": artists})
}

func (h *Handler) Rescan(w http.ResponseWriter, r *http.Request) {
	payload, _ := json.Marshal(map[string]interface{}{
		"path":  h.musicDir,
		"force": false,
	})
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

func (h *Handler) GetTrackCover(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	track, err := h.db.GetTrackByID(r.Context(), id)
	if err != nil {
		http.Error(w, "track not found", 404)
		return
	}

	fullPath := track.Path
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(h.musicDir, fullPath)
	}

	cmd := exec.CommandContext(r.Context(), h.ffmpegPath, "-i", fullPath, "-an", "-vcodec", "mjpeg", "-f", "image2", "-", "-v", "0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || out.Len() == 0 {
		http.Error(w, "no cover art", 404)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(out.Bytes())
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
	station, err := h.db.GetStationByID(r.Context(), id)
	if err != nil {
		http.Error(w, "station not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(station)
}

func (h *Handler) UpdateStation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Name == "" {
		http.Error(w, "name required", 400)
		return
	}
	if err := h.db.UpdateStation(r.Context(), id, req.Name); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "ok"})
}

func (h *Handler) DeleteStation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteStation(r.Context(), id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
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

	queue, err := h.db.GetStationQueue(r.Context(), stationID)
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

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", 400)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	userID, err := h.db.CreateUser(r.Context(), req.Username, string(hash))
	if err != nil {
		http.Error(w, "username taken", 409)
		return
	}

	token, err := h.createSession(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":       userID,
			"username": req.Username,
		},
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", 400)
		return
	}

	user, err := h.db.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "invalid credentials", 401)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", 401)
		return
	}

	token, err := h.createSession(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", 401)
		return
	}
	if _, ok := h.authUser(r); !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if err := h.db.DeleteSession(r.Context(), hashToken(token)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":       user.ID,
		"username": user.Username,
	})
}

func (h *Handler) authUser(r *http.Request) (User, bool) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return User{}, false
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return User{}, false
	}
	user, err := h.db.GetUserByTokenHash(r.Context(), hashToken(token))
	if err != nil {
		return User{}, false
	}
	return user, true
}

func (h *Handler) createSession(ctx context.Context, userID string) (string, error) {
	token, err := generateSessionToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if _, err := h.db.CreateSession(ctx, userID, hashToken(token), expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
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
