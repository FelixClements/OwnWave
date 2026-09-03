package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) ListStations(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	stations, err := h.db.ListStations(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, r, "list stations", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"stations": stations})
}

func (h *Handler) GetStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	station, err := h.db.GetStationByID(r.Context(), user.ID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
		writeInternalError(w, r, "get station", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(station)
}

func (h *Handler) UpdateStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if _, err := h.db.GetStationByID(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
		writeInternalError(w, r, "get station", err)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.analyticsFor(user.ID).UpdateStation(id, body)
	proxyAnalytics(w, r, resp, err)
}

func (h *Handler) DeleteStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteStation(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
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
	id := chi.URLParam(r, "id")
	if _, err := h.db.GetStationByID(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
		writeInternalError(w, r, "get station", err)
		return
	}
	queue, err := h.playback.BuildQueue(r.Context(), user.ID, id, h.recentHours)
	if err != nil {
		writeInternalError(w, r, "build queue", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"queue": queue})
}

func (h *Handler) CreateStation(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.analyticsFor(user.ID).CreateStation(body)
	proxyAnalytics(w, r, resp, err)
}
