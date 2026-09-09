package main

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasUsers, err := h.db.CountUsers(ctx)
	if err != nil {
		writeInternalError(w, r, "setup status users", err)
		return
	}
	state, _ := h.db.GetAppState(ctx, "setup_completed")
	completed := false
	if state != nil {
		if v, ok := state["completed"].(bool); ok {
			completed = v
		}
	}

	resp := map[string]interface{}{
		"setup_completed": completed,
		"has_users":       hasUsers > 0,
	}

	// Only disclose track count during initial setup before completion
	if !completed {
		trackCount, err := h.db.CountTracks(ctx)
		if err == nil {
			resp["track_count"] = trackCount
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func (h *Handler) SetupComplete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasUsers, err := h.db.CountUsers(ctx)
	if err != nil {
		writeInternalError(w, r, "setup complete check users", err)
		return
	}
	if hasUsers == 0 {
		http.Error(w, "cannot complete setup without registered users", http.StatusBadRequest)
		return
	}

	if err := h.db.SetAppState(ctx, "setup_completed", map[string]interface{}{"completed": true}); err != nil {
		writeInternalError(w, r, "setup complete", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"setup_completed": true})
}
func (h *Handler) SetupSummary(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.SetupSummary()
	proxyAnalytics(w, r, resp, err)
}
func (h *Handler) SetupStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	resp, err := h.analyticsFor(user.ID).SetupStations(r.Body)
	proxyAnalytics(w, r, resp, err)
}
