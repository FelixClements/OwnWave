package main

import (
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

	writeJSON(w, http.StatusOK, resp)
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
	writeJSON(w, http.StatusOK, map[string]interface{}{"setup_completed": true})
}
func (h *Handler) SetupSummary(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analytics.SetupSummary(r.Context())
	proxyAnalytics(w, r, resp, err)
}
