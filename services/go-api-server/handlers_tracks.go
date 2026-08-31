package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListTracks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 1000
	}
	tracks, err := h.db.ListTracks(r.Context(), limit, offset, q)
	if err != nil {
		writeInternalError(w, r, "list tracks", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tracks": tracks})
}
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results, err := h.db.Search(r.Context(), q)
	if err != nil {
		writeInternalError(w, r, "list tracks", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.db.ListAlbums(r.Context())
	if err != nil {
		writeInternalError(w, r, "list tracks", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"albums": albums})
}
func (h *Handler) ListArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := h.db.ListArtists(r.Context())
	if err != nil {
		writeInternalError(w, r, "list tracks", err)
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
	resp, err := h.analytics.GetSimilarTracks(id, limit)
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) RecordPlay(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	var req struct {
		StationID string `json:"station_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.playback.Record(r.Context(), trackID, req.StationID); err != nil {
		writeInternalError(w, r, "record play", err)
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
		writeInternalError(w, r, "record feedback", err)
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
		writeInternalError(w, r, "delete feedback", err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request) {
	entries, err := h.db.ListHistory(r.Context(), 50)
	if err != nil {
		writeInternalError(w, r, "list tracks", err)
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
		writeInternalError(w, r, "list tracks", err)
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

	fullPath, err := h.stream.ResolvePath(track.Path)
	if err != nil {
		http.Error(w, "no cover art", http.StatusNotFound)
		return
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
