package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)
func (h *Handler) Rescan(w http.ResponseWriter, r *http.Request) {
	payload, _ := json.Marshal(map[string]interface{}{
		"path":  h.musicDir,
		"force": false,
	})
	resp, err := h.analytics.Scan(payload)
	proxyAnalytics(w, resp, err)
}
func (h *Handler) ScanStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	resp, err := h.analytics.GetJob(jobID)
	proxyAnalytics(w, resp, err)
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
	resp, err := h.analytics.Scan(payload)
	proxyAnalytics(w, resp, err)
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
	resp, err := h.analytics.Health()
	if err != nil {
		status["python"] = "error: " + err.Error()
	} else if resp.StatusCode == 200 {
		status["python"] = "ok"
	} else {
		status["python"] = "error: status " + strconv.Itoa(resp.StatusCode)
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
	resp, err := h.analytics.RebuildVectors()
	proxyAnalytics(w, resp, err)
}
func (h *Handler) AdminRebuildClusters(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildClusters()
	proxyAnalytics(w, resp, err)
}
func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.ListGenres()
	proxyAnalytics(w, resp, err)
}
func (h *Handler) GetTrackGenres(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp, err := h.analytics.GetTrackGenres(id)
	proxyAnalytics(w, resp, err)
}
func (h *Handler) AdminRebuildGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildGenres()
	proxyAnalytics(w, resp, err)
}
func (h *Handler) AdminRebuildGenreStations(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildGenreStations()
	proxyAnalytics(w, resp, err)
}
