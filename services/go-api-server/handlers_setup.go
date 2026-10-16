package main

import (
	"encoding/json"
	"net/http"

	"ownwave/api/internal/stations"
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

// SetupStations creates one live genre station per selected main genre, plus
// "Uncategorized" for tracks without a confident genre prediction.
func (h *Handler) SetupStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		SelectedMainGenres []string `json:"selected_main_genres"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	counts, err := h.db.GenreCounts(r.Context(), h.minGenreConfidence)
	if err != nil {
		writeInternalError(w, r, "setup genre counts", err)
		return
	}
	hasTracks := make(map[string]bool)
	for _, g := range counts {
		if g.SubGenre == "" && g.TrackCount > 0 {
			hasTracks[g.MainGenre] = true
		}
	}

	created := 0
	ids := make([]string, 0, len(req.SelectedMainGenres)+1)
	ensure := func(name string, seed stations.Seed) bool {
		id, isNew, err := h.ensureGenreStation(r, user.ID, name, seed)
		if err != nil {
			writeInternalError(w, r, "setup station", err)
			return false
		}
		if isNew {
			created++
		}
		ids = append(ids, id)
		return true
	}

	for _, genre := range req.SelectedMainGenres {
		if !hasTracks[genre] {
			continue
		}
		if !ensure(stations.TitleCase(genre), stations.Seed{Type: "genre", MainGenre: genre}) {
			return
		}
	}

	uncategorized := stations.Seed{Type: "uncategorized"}
	uncovered, err := h.playback.PoolSize(r.Context(), user.ID, uncategorized)
	if err != nil {
		writeInternalError(w, r, "count uncategorized", err)
		return
	}
	if uncovered > 0 && !ensure("Uncategorized", uncategorized) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"created":     created,
		"station_ids": ids,
		"uncovered":   uncovered,
	})
}
