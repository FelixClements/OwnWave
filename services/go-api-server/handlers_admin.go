package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ownwave/api/internal/stations"
)

func (h *Handler) Rescan(w http.ResponseWriter, r *http.Request) {
	payload, _ := json.Marshal(map[string]interface{}{
		"path":  h.musicDir,
		"force": false,
	})
	resp, err := h.analytics.Scan(payload)
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) ScanStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	resp, err := h.analytics.GetJob(jobID)
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path  string `json:"path"`
		Force bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Path = h.musicDir
	}
	req.Path = h.musicDir

	payload, _ := json.Marshal(req)
	resp, err := h.analytics.Scan(payload)
	proxyAnalytics(w, r, resp, err)
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
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	list, err := h.db.ListStations(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, r, "admin stations", err)
		return
	}
	type stationStatus struct {
		Station
		TrackCount  int `json:"track_count"`
		PlayedCount int `json:"played_count"`
	}
	out := make([]stationStatus, 0, len(list))
	for _, st := range list {
		seed, err := stations.ParseStored(st.SeedFeatures)
		if err != nil {
			writeInternalError(w, r, "admin stations seed", err)
			return
		}
		status := stationStatus{Station: st}
		if status.TrackCount, err = h.playback.PoolSize(r.Context(), user.ID, seed); err != nil {
			writeInternalError(w, r, "admin stations count", err)
			return
		}
		if status.PlayedCount, err = h.db.PlayedCount(r.Context(), user.ID, st.ID); err != nil {
			writeInternalError(w, r, "admin stations played", err)
			return
		}
		out = append(out, status)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"stations": out})
}
func (h *Handler) AdminRebuildVectors(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildVectors()
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) AdminRebuildClusters(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildClusters()
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.ListGenres()
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) GetTrackGenres(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp, err := h.analytics.GetTrackGenres(id)
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) AdminRebuildGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildGenres()
	proxyAnalytics(w, r, resp, err)
}

// AdminRebuildGenreStations creates any missing main- and sub-genre stations
// (with at least minTracksPerGenreStation tracks) for the calling admin.
func (h *Handler) AdminRebuildGenreStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	counts, err := h.db.GenreCounts(r.Context(), h.minGenreConfidence)
	if err != nil {
		writeInternalError(w, r, "genre counts", err)
		return
	}
	created := 0
	for _, g := range counts {
		if g.TrackCount < h.minTracksPerGenreStation || g.SubGenre == g.MainGenre {
			continue
		}
		name := stations.TitleCase(g.MainGenre)
		seed := stations.Seed{Type: "genre", MainGenre: g.MainGenre}
		if g.SubGenre != "" {
			name += " / " + stations.TitleCase(g.SubGenre)
			seed = stations.Seed{Type: "sub_genre", MainGenre: g.MainGenre, SubGenre: g.SubGenre}
		}
		_, isNew, err := h.ensureGenreStation(r, user.ID, name, seed)
		if err != nil {
			writeInternalError(w, r, "create genre station", err)
			return
		}
		if isNew {
			created++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "created": created})
}
