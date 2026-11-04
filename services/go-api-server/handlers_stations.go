package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"ownwave/api/internal/stations"
	"ownwave/api/internal/streaming"
)

// writeStationError maps stations errors to HTTP statuses.
func writeStationError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, stations.ErrNotFound):
		http.Error(w, "station not found", http.StatusNotFound)
	case errors.Is(err, stations.ErrForbidden):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, stations.ErrInvalid):
		http.Error(w, strings.TrimPrefix(err.Error(), stations.ErrInvalid.Error()+": "), http.StatusBadRequest)
	default:
		writeInternalError(w, r, op, err)
	}
}

func (h *Handler) ListStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	list, err := h.stations.List(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, r, "list stations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stations": list})
}

func (h *Handler) GetStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	st, err := h.stations.Get(r.Context(), user.ID, id)
	if err != nil {
		writeStationError(w, r, "get station", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// decodeJSON reads a small JSON body into v, writing 400/413 on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid request", http.StatusBadRequest)
		}
		return false
	}
	return true
}

// CreateStation creates a station from playlists and optional genres.
// Admins create shared stations, other users personal ones.
func (h *Handler) CreateStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var req stations.Request
	if !decodeJSON(w, r, &req) {
		return
	}
	st, err := h.stations.Create(r.Context(), user, req)
	if err != nil {
		writeStationError(w, r, "create station", err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (h *Handler) UpdateStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req stations.Request
	if !decodeJSON(w, r, &req) {
		return
	}
	st, err := h.stations.Update(r.Context(), user, id, req)
	if err != nil {
		writeStationError(w, r, "update station", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) DeleteStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.stations.Delete(r.Context(), user, id); err != nil {
		writeStationError(w, r, "delete station", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetQueue(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	st, err := h.stations.Get(r.Context(), user.ID, id)
	if err != nil {
		writeStationError(w, r, "get station", err)
		return
	}
	queue, err := h.playback.BuildQueue(r.Context(), user.ID, st.ID, st.Definition(), h.recentHours)
	if err != nil {
		writeInternalError(w, r, "build queue", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queue": queue})
}

// ListStationGenres breaks a playlist pool down by station genre:
// GET /stations/genres?playlist_ids=a,b.
func (h *Handler) ListStationGenres(w http.ResponseWriter, r *http.Request) {
	var ids []string
	for _, id := range strings.Split(r.URL.Query().Get("playlist_ids"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	counts, err := h.stations.GenreCounts(r.Context(), ids)
	if err != nil {
		writeStationError(w, r, "station genres", err)
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

// ListStationSuggestions proposes stations from the synced playlists.
func (h *Handler) ListStationSuggestions(w http.ResponseWriter, r *http.Request) {
	list, err := h.stations.Suggestions(r.Context())
	if err != nil {
		writeInternalError(w, r, "station suggestions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": list, "min_tracks": stations.MinSuggestionTracks})
}

// DismissStationSuggestion hides a suggestion for good.
func (h *Handler) DismissStationSuggestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlaylistIDs []string `json:"playlist_ids"`
		Genre       string   `json:"genre"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.stations.Dismiss(r.Context(), req.PlaylistIDs, req.Genre); err != nil {
		writeStationError(w, r, "dismiss suggestion", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Browse returns a node of the media tree apps show (Android Auto):
// GET /browse?node=root|stations|recent|genres|playlists|genre:<id>|playlist:<id>.
func (h *Handler) Browse(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	node, err := h.stations.Browse(r.Context(), user.ID, r.URL.Query().Get("node"))
	if errors.Is(err, stations.ErrUnknownNode) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		writeInternalError(w, r, "browse", err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// SearchStations finds stations to play for a (voice) query such as
// "play techno radio": GET /search/stations?q=.
func (h *Handler) SearchStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if len(q) > 200 {
		http.Error(w, "query too long", http.StatusBadRequest)
		return
	}
	hits, err := h.stations.Search(r.Context(), user.ID, q)
	if err != nil {
		writeInternalError(w, r, "search stations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": stations.CleanQuery(q), "results": hits})
}

// coverCandidates bounds how many tracks are tried for a station cover.
const coverCandidates = 8

// GetStationCover returns the cover art of one of the station's tracks
// (the same one every time while the station's pool is unchanged).
func (h *Handler) GetStationCover(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	st, err := h.stations.Get(r.Context(), user.ID, id)
	if err != nil {
		writeStationError(w, r, "get station", err)
		return
	}
	ids, err := h.stations.CoverCandidates(r.Context(), user.ID, st, coverCandidates)
	if err != nil {
		writeInternalError(w, r, "station cover", err)
		return
	}
	var firstErr error
	for _, trackID := range ids {
		cover, err := h.trackCover(r.Context(), trackID)
		switch {
		case err == nil:
			writeCover(w, cover, "private, max-age=3600")
			return
		case errors.Is(err, streaming.ErrNoCover), errors.Is(err, pgx.ErrNoRows):
			// try the next candidate
		case firstErr == nil:
			firstErr = err
		}
	}
	if firstErr != nil {
		writeCoverError(w, r, "station cover", firstErr)
		return
	}
	// A station without any cover art is normal; see GetTrackCover.
	writeNoCover(w)
}
