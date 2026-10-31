package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"ownwave/api/internal/auth"
	"ownwave/api/internal/stations"
)

func (h *Handler) ListStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	list, err := h.db.ListStations(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, r, "list stations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"stations": list})
}

// loadStation fetches the user's station named by the {id} URL param and
// parses its seed, writing 404/500 itself when that fails.
func (h *Handler) loadStation(w http.ResponseWriter, r *http.Request, user auth.User) (Station, stations.Seed, bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return Station{}, stations.Seed{}, false
	}
	station, err := h.db.GetStationByID(r.Context(), user.ID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
		} else {
			writeInternalError(w, r, "get station", err)
		}
		return Station{}, stations.Seed{}, false
	}
	seed, err := stations.ParseStored(station.SeedFeatures)
	if err != nil {
		writeInternalError(w, r, "parse station seed", err)
		return Station{}, stations.Seed{}, false
	}
	return station, seed, true
}

func (h *Handler) GetStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	station, _, ok := h.loadStation(w, r, user)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, station)
}

// decodeStationRequest reads a create/update body, writing 400/413 on failure.
func decodeStationRequest(w http.ResponseWriter, r *http.Request) (stations.Request, bool) {
	var req stations.Request
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid request", http.StatusBadRequest)
		}
		return req, false
	}
	req.Name = strings.TrimSpace(req.Name)
	return req, true
}

// checkSeedPlaylists writes 400 unless every playlist of a playlists seed
// exists.
func (h *Handler) checkSeedPlaylists(w http.ResponseWriter, r *http.Request, seed stations.Seed) bool {
	if !seed.Shared() {
		return true
	}
	n, err := h.playlists.CountExisting(r.Context(), seed.PlaylistIDs)
	if err != nil {
		writeInternalError(w, r, "check station playlists", err)
		return false
	}
	if n != len(seed.PlaylistIDs) {
		http.Error(w, "unknown playlist", http.StatusBadRequest)
		return false
	}
	return true
}

// CreateStation creates a personal station, or a shared playlist station
// (admins only).
func (h *Handler) CreateStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	req, ok := decodeStationRequest(w, r)
	if !ok {
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	seed, err := req.Seed()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	owner := user.ID
	if seed.Shared() {
		if !user.IsAdmin {
			http.Error(w, "only admins create playlist stations", http.StatusForbidden)
			return
		}
		owner = ""
	}
	if !h.checkSeedPlaylists(w, r, seed) {
		return
	}
	raw, err := seed.JSON()
	if err != nil {
		writeInternalError(w, r, "encode station seed", err)
		return
	}
	id, err := h.db.CreateStation(r.Context(), owner, req.Name, raw)
	if err != nil {
		writeInternalError(w, r, "create station", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"station_id": id})
}

// canChange writes 403 when the user may not change the station: shared
// stations belong to the admins.
func canChange(w http.ResponseWriter, user auth.User, station Station) bool {
	if station.Shared && !user.IsAdmin {
		http.Error(w, "only admins change shared stations", http.StatusForbidden)
		return false
	}
	return true
}

func (h *Handler) UpdateStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	station, seed, ok := h.loadStation(w, r, user)
	if !ok || !canChange(w, user, station) {
		return
	}
	req, ok := decodeStationRequest(w, r)
	if !ok {
		return
	}

	name := req.Name
	if name == "" {
		name = station.Name
	}
	if req.HasSeed() {
		var err error
		if seed, err = req.Seed(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	// Ownership follows the seed type, so a station cannot switch between
	// personal and shared.
	if seed.Shared() != station.Shared {
		http.Error(w, "playlist stations are shared: create a new station instead", http.StatusBadRequest)
		return
	}
	if !h.checkSeedPlaylists(w, r, seed) {
		return
	}
	raw, err := seed.JSON()
	if err != nil {
		writeInternalError(w, r, "encode station seed", err)
		return
	}

	if err := h.db.UpdateStation(r.Context(), station.ID, name, raw); err != nil {
		writeInternalError(w, r, "update station", err)
		return
	}
	count, err := h.playback.PoolSize(r.Context(), user.ID, seed)
	if err != nil {
		writeInternalError(w, r, "count station tracks", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"station_id":  station.ID,
		"name":        name,
		"track_count": count,
	})
}

func (h *Handler) DeleteStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	station, _, ok := h.loadStation(w, r, user)
	if !ok || !canChange(w, user, station) {
		return
	}
	if err := h.db.DeleteStation(r.Context(), station.ID); err != nil {
		writeInternalError(w, r, "delete station", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetQueue(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	station, seed, ok := h.loadStation(w, r, user)
	if !ok {
		return
	}
	queue, err := h.playback.BuildQueue(r.Context(), user.ID, station.ID, seed, h.recentHours)
	if err != nil {
		writeInternalError(w, r, "build queue", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"queue": queue})
}
