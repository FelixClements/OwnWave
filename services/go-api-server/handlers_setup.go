package main

import (
	"net/http"

	"ownwave/api/internal/settings"
)

func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasUsers, err := h.auth.CountUsers(ctx)
	if err != nil {
		writeInternalError(w, r, "setup status users", err)
		return
	}
	state, _ := h.settings.Get(ctx, settings.SetupCompleted)
	completed := false
	if state != nil {
		if v, ok := state["completed"].(bool); ok {
			completed = v
		}
	}

	resp := map[string]interface{}{
		"setup_completed": completed,
		"has_users":       hasUsers > 0,
		// The first account needs the setup token from the server log or
		// OWNWAVE_SETUP_TOKEN.
		"setup_token_required": hasUsers == 0,
	}
	if hasUsers == 0 {
		// The database may have been emptied since startup.
		h.setup.announceOnce()
	}

	// This endpoint is public: only an admin finishing setup sees the
	// library size.
	if !completed {
		if user, ok := h.auth.UserFromRequest(r); ok && user.IsAdmin {
			if trackCount, err := h.library.CountTracks(ctx); err == nil {
				resp["track_count"] = trackCount
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
func (h *Handler) SetupComplete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasUsers, err := h.auth.CountUsers(ctx)
	if err != nil {
		writeInternalError(w, r, "setup complete check users", err)
		return
	}
	if hasUsers == 0 {
		http.Error(w, "cannot complete setup without registered users", http.StatusBadRequest)
		return
	}

	if err := h.settings.Set(ctx, settings.SetupCompleted, map[string]interface{}{"completed": true}); err != nil {
		writeInternalError(w, r, "setup complete", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"setup_completed": true})
}
