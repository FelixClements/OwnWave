package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"ownwave/api/internal/stations"
)

func (h *Handler) ScanStatus(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.analytics.GetJob(r.Context(), jobID)
	proxyAnalytics(w, r, resp, err)
}

// ListScans returns the most recent library scans (?limit=1..50, default 10).
func (h *Handler) ListScans(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			http.Error(w, "limit must be 1-50", http.StatusBadRequest)
			return
		}
		limit = n
	}
	resp, err := h.analytics.ListJobs(r.Context(), limit)
	proxyAnalytics(w, r, resp, err)
}

// TriggerScan queues a scan of the whole music directory.
func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Force bool `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{"path": h.musicDir, "force": req.Force})
	resp, err := h.analytics.Scan(r.Context(), payload)
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
	resp, err := h.analytics.Health(r.Context())
	if err != nil {
		status["python"] = "error: " + err.Error()
	} else if resp.StatusCode == 200 {
		status["python"] = "ok"
	} else {
		status["python"] = "error: status " + strconv.Itoa(resp.StatusCode)
	}
	writeJSON(w, http.StatusOK, status)
}

// AdminStations lists stations with how many of their tracks the admin
// has played.
func (h *Handler) AdminStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	list, err := h.stations.List(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, r, "admin stations", err)
		return
	}
	type stationStatus struct {
		stations.Station
		PlayedCount int `json:"played_count"`
	}
	out := make([]stationStatus, 0, len(list))
	for _, st := range list {
		status := stationStatus{Station: st}
		if status.PlayedCount, err = h.db.PlayedCount(r.Context(), user.ID, st.ID); err != nil {
			writeInternalError(w, r, "admin stations played", err)
			return
		}
		out = append(out, status)
	}
	writeJSON(w, http.StatusOK, map[string]any{"stations": out})
}

// AdminRebuildAnalysis finishes analysis bookkeeping and queues a genre refresh.
func (h *Handler) AdminRebuildAnalysis(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildAnalysis(r.Context())
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.ListGenres(r.Context())
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) GetTrackGenres(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.analytics.GetTrackGenres(r.Context(), id)
	proxyAnalytics(w, r, resp, err)
}

// AdminGenreReport shows how well the audio genre model agrees with file tags.
func (h *Handler) AdminGenreReport(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.GenreReport(r.Context())
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) AdminRebuildGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.RebuildGenres(r.Context())
	proxyAnalytics(w, r, resp, err)
}
