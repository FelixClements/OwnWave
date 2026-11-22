package main

import (
	"net/http"

	"ownwave/api/internal/stations"
)

func (h *Handler) AdminHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{
		"go":      "ok",
		"db":      "ok",
		"python":  "unknown",
		"version": "ok",
	}
	if err := h.db.Ping(r.Context()); err != nil {
		status["db"] = "error: " + err.Error()
	}
	if err := h.analytics.Health(r.Context()); err != nil {
		status["python"] = "error: " + err.Error()
	} else {
		status["python"] = "ok"
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
		if status.PlayedCount, err = h.listening.PlayedCount(r.Context(), user.ID, st.ID); err != nil {
			writeInternalError(w, r, "admin stations played", err)
			return
		}
		out = append(out, status)
	}
	writeJSON(w, http.StatusOK, map[string]any{"stations": out})
}
