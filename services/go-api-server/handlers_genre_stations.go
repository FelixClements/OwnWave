package main

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

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

const (
	genreReviewDefault = 20
	genreReviewMax     = 100
)

// queryInt reads an integer query parameter between lo and hi, or def when
// it is missing. It answers 400 itself when the value is out of range.
func queryInt(w http.ResponseWriter, r *http.Request, name string, def, lo, hi int) (int, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		http.Error(w, fmt.Sprintf("%s must be %d to %d", name, lo, hi), http.StatusBadRequest)
		return 0, false
	}
	return n, true
}

// AdminGenreOverview counts songs per genre and style, the unsure and
// flagged songs, and returns the classifier's test score.
func (h *Handler) AdminGenreOverview(w http.ResponseWriter, r *http.Request) {
	o, err := h.stations.GenreOverview(r.Context())
	if err != nil {
		writeInternalError(w, r, "genre overview", err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// AdminGenreReview returns one page of the review queue, grouped by release:
// ?offset= (groups to skip) and ?limit= (groups, up to 100).
func (h *Handler) AdminGenreReview(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryInt(w, r, "limit", genreReviewDefault, 1, genreReviewMax)
	if !ok {
		return
	}
	offset, ok := queryInt(w, r, "offset", 0, 0, 1_000_000)
	if !ok {
		return
	}
	page, err := h.stations.GenreReview(r.Context(), offset, limit)
	if err != nil {
		writeInternalError(w, r, "genre review", err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// AdminGenreDecision returns how the genre engine decided one song, with
// its "why" breakdown. 404 when it has not decided the song.
func (h *Handler) AdminGenreDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	d, err := h.stations.GenreDecision(r.Context(), id)
	if errors.Is(err, stations.ErrNotFound) {
		http.Error(w, "no genre decision for this song yet", http.StatusNotFound)
		return
	}
	if err != nil {
		writeInternalError(w, r, "genre decision", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// AdminGenreCorrections lists the genre corrections; ?q= searches them.
func (h *Handler) AdminGenreCorrections(w http.ResponseWriter, r *http.Request) {
	h.listCorrections(w, r, "")
}

func (h *Handler) listCorrections(w http.ResponseWriter, r *http.Request, saved string) {
	list, err := h.stations.GenreCorrections(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeInternalError(w, r, "genre corrections", err)
		return
	}
	body := map[string]any{"corrections": list}
	if saved != "" {
		body["saved_id"] = saved
	}
	writeJSON(w, http.StatusOK, body)
}

// AdminSaveGenreCorrection corrects a song, a release or an artist:
// POST {"track_id" | "release_id" | "artist", "genre", "style"?}. Saving
// twice replaces it. The answer names the saved correction, so a client can
// undo it.
func (h *Handler) AdminSaveGenreCorrection(w http.ResponseWriter, r *http.Request) {
	var req stations.GenreCorrectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := h.stations.SaveGenreCorrection(r.Context(), req)
	if err != nil {
		writeStationError(w, r, "save genre correction", err)
		return
	}
	h.rescoreAndListCorrections(w, r, id)
}

// AdminDeleteGenreCorrection takes a correction back (undo).
func (h *Handler) AdminDeleteGenreCorrection(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.stations.DeleteGenreCorrection(r.Context(), id); err != nil {
		writeInternalError(w, r, "delete genre correction", err)
		return
	}
	h.rescoreAndListCorrections(w, r, "")
}

// rescoreAndListCorrections queues genre scoring, which applies corrections
// and learns from them as trusted labels. The change is saved either way, so
// a client can retry when queueing fails.
func (h *Handler) rescoreAndListCorrections(w http.ResponseWriter, r *http.Request, saved string) {
	if err := h.analytics.RebuildGenres(r.Context()); err != nil {
		writeInternalError(w, r, "queue genre scoring", err)
		return
	}
	h.listCorrections(w, r, saved)
}
