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
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 1000
	}
	tracks, err := h.db.ListTracks(r.Context(), limit, offset, q)
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

func (h *Handler) GetSimilarTracks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "20"
	}
	resp, err := http.Get(h.pythonURL + "/tracks/" + id + "/similar?limit=" + limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
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

func (h *Handler) RecordPlay(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	var req struct {
		StationID string `json:"station_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.db.RecordPlay(r.Context(), trackID, req.StationID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) RecordFeedback(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	var req struct {
		Feedback string `json:"feedback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Feedback == "" {
		http.Error(w, "feedback required", 400)
		return
	}
	if err := h.db.RecordFeedback(r.Context(), trackID, req.Feedback); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) DeleteFeedback(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	var req struct {
		Feedback string `json:"feedback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Feedback == "" {
		http.Error(w, "feedback required", 400)
		return
	}
	if err := h.db.DeleteFeedback(r.Context(), trackID, req.Feedback); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request) {
	entries, err := h.db.ListHistory(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"history": entries})
}

func (h *Handler) ListFeedback(w http.ResponseWriter, r *http.Request) {
	feedback := r.URL.Query().Get("feedback")
	if feedback == "" {
		http.Error(w, "feedback param required", 400)
		return
	}
	tracks, err := h.db.ListFeedback(r.Context(), feedback, 100)
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
		Name       *string  `json:"name"`
		Length     *int     `json:"length"`
		MinBPM     *float64 `json:"min_bpm"`
		MaxBPM     *float64 `json:"max_bpm"`
		MinEnergy  *float64 `json:"min_energy"`
		MaxEnergy  *float64 `json:"max_energy"`
		MinValence *float64 `json:"min_valence"`
		MaxValence *float64 `json:"max_valence"`
		SeedType   *string  `json:"seed_type"`
		TrackID    *string  `json:"track_id"`
		ArtistID   *string  `json:"artist_id"`
		AlbumID    *string  `json:"album_id"`
		ClusterID  *int     `json:"cluster_id"`
		MainGenre  *string  `json:"main_genre"`
		SubGenre   *string  `json:"sub_genre"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Name == nil || *req.Name == "" {
		http.Error(w, "name required", 400)
		return
	}

	filters := map[string]interface{}{}
	if req.Length != nil {
		filters["length"] = *req.Length
	}
	if req.MinBPM != nil {
		filters["min_bpm"] = *req.MinBPM
	}
	if req.MaxBPM != nil {
		filters["max_bpm"] = *req.MaxBPM
	}
	if req.MinEnergy != nil {
		filters["min_energy"] = *req.MinEnergy
	}
	if req.MaxEnergy != nil {
		filters["max_energy"] = *req.MaxEnergy
	}
	if req.MinValence != nil {
		filters["min_valence"] = *req.MinValence
	}
	if req.MaxValence != nil {
		filters["max_valence"] = *req.MaxValence
	}
	if req.SeedType != nil && *req.SeedType != "" {
		filters["seed_type"] = *req.SeedType
	}
	if req.TrackID != nil && *req.TrackID != "" {
		filters["track_id"] = *req.TrackID
	}
	if req.ArtistID != nil && *req.ArtistID != "" {
		filters["artist_id"] = *req.ArtistID
	}
	if req.AlbumID != nil && *req.AlbumID != "" {
		filters["album_id"] = *req.AlbumID
	}
	if req.ClusterID != nil {
		filters["cluster_id"] = *req.ClusterID
	}
	if req.MainGenre != nil && *req.MainGenre != "" {
		filters["main_genre"] = *req.MainGenre
	}
	if req.SubGenre != nil && *req.SubGenre != "" {
		filters["sub_genre"] = *req.SubGenre
	}

	var seedFeatures string
	if len(filters) > 0 {
		b, _ := json.Marshal(filters)
		seedFeatures = string(b)
	}

	if err := h.db.UpdateStation(r.Context(), id, *req.Name, seedFeatures); err != nil {
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

func (h *Handler) AdminHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{
		"go":      "ok",
		"db":      "ok",
		"python":  "unknown",
		"version": "ok",
	}
	if err := h.db.pool.Ping(r.Context()); err != nil {
		status["db"] = "error: " + err.Error()
	}
	resp, err := http.Get(h.pythonURL + "/health")
	if err != nil {
		status["python"] = "error: " + err.Error()
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			status["python"] = "ok"
		} else {
			status["python"] = "error: " + resp.Status
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *Handler) AdminStations(w http.ResponseWriter, r *http.Request) {
	stations, err := h.db.ListStationsWithQueueStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"stations": stations})
}

func (h *Handler) AdminRebuildVectors(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Post(h.pythonURL+"/rebuild-vectors", "application/json", nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) AdminRebuildClusters(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Post(h.pythonURL+"/rebuild-clusters", "application/json", nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(h.pythonURL + "/genres")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) GetTrackGenres(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp, err := http.Get(h.pythonURL + "/tracks/" + id + "/genres")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) AdminRebuildGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Post(h.pythonURL+"/rebuild-genres", "application/json", nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) AdminRebuildGenreStations(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Post(h.pythonURL+"/rebuild-genre-stations", "application/json", nil)
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"full_name": user.FullName,
		"is_admin":  user.IsAdmin,
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}

	var req struct {
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	fullName := ""
	if req.FullName != nil {
		fullName = *req.FullName
	}

	if err := h.db.UpdateUserProfile(r.Context(), user.ID, email, fullName); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "current and new password required", 400)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "invalid current password", 401)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := h.db.UpdateUserPassword(r.Context(), user.ID, string(hash)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
