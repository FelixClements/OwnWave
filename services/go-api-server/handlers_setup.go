package main

import (
	"encoding/json"
	"net/http"
)
func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasUsers, err := h.db.CountUsers(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	trackCount, err := h.db.CountTracks(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	state, _ := h.db.GetAppState(ctx, "setup_completed")
	completed := false
	if state != nil {
		if v, ok := state["completed"].(bool); ok {
			completed = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"setup_completed": completed,
		"has_users":       hasUsers > 0,
		"track_count":     trackCount,
	})
}
func (h *Handler) SetupComplete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.db.SetAppState(ctx, "setup_completed", map[string]interface{}{"completed": true}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"setup_completed": true})
}
func (h *Handler) SetupSummary(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.SetupSummary()
	proxyAnalytics(w, resp, err)
}
func (h *Handler) SetupStations(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.SetupStations(r.Body)
	proxyAnalytics(w, resp, err)
}
