package main

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"ownwave/api/internal/pipeline"
	"ownwave/api/internal/stations"
)

var genreParam = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// pipelineRunsShown is how many station refreshes the admin view lists.
const pipelineRunsShown = 20

// AdminGenreStations lists every station genre with its track count and
// genre station state.
func (h *Handler) AdminGenreStations(w http.ResponseWriter, r *http.Request) {
	settings, err := h.stations.GenreStations(r.Context())
	if err != nil {
		writeInternalError(w, r, "genre stations", err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// refreshAndList refreshes the genre stations after a settings change and
// answers with the new settings.
func (h *Handler) refreshAndList(w http.ResponseWriter, r *http.Request) {
	if _, err := h.pipeline.Run(r.Context(), pipeline.TriggerSettings, false); err != nil {
		writeInternalError(w, r, "refresh genre stations", err)
		return
	}
	h.AdminGenreStations(w, r)
}

// AdminSetGenreStations switches genre stations on or off: PUT {"enabled": bool}.
func (h *Handler) AdminSetGenreStations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		http.Error(w, "enabled is required", http.StatusBadRequest)
		return
	}
	if err := h.stations.SetGenreStationsEnabled(r.Context(), *req.Enabled); err != nil {
		writeInternalError(w, r, "set genre stations", err)
		return
	}
	h.refreshAndList(w, r)
}

// AdminUpdateGenreStation renames a genre's station or (un)excludes the
// genre: PUT {"name"?: string ("" = genre name), "excluded"?: bool}.
func (h *Handler) AdminUpdateGenreStation(w http.ResponseWriter, r *http.Request) {
	genre := chi.URLParam(r, "genre")
	if !genreParam.MatchString(genre) {
		http.Error(w, "unknown genre", http.StatusNotFound)
		return
	}
	var req stations.GenreStationUpdate
	if !decodeJSON(w, r, &req) {
		return
	}
	err := h.stations.UpdateGenreStation(r.Context(), genre, req)
	if errors.Is(err, stations.ErrNotFound) {
		http.Error(w, "unknown genre", http.StatusNotFound)
		return
	}
	if err != nil {
		writeStationError(w, r, "update genre station", err)
		return
	}
	h.refreshAndList(w, r)
}

// AdminPipeline reports the last scan, the last genre scoring and the
// recent station refreshes with what they changed.
func (h *Handler) AdminPipeline(w http.ResponseWriter, r *http.Request) {
	status, err := h.pipeline.Status(r.Context(), pipelineRunsShown)
	if err != nil {
		writeInternalError(w, r, "pipeline status", err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// AdminRefreshPipeline matches playlist tracks and refreshes the genre
// stations now.
func (h *Handler) AdminRefreshPipeline(w http.ResponseWriter, r *http.Request) {
	run, err := h.pipeline.Run(r.Context(), pipeline.TriggerAdmin, true)
	if err != nil {
		writeInternalError(w, r, "refresh stations", err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}
