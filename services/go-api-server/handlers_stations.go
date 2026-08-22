package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)
func (h *Handler) ListStations(w http.ResponseWriter, r *http.Request) {
	stations, err := h.db.ListStations(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"stations": stations})
}
func (h *Handler) GetStation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	station, err := h.db.GetStationByID(r.Context(), id)
	if err != nil {
		http.Error(w, "station not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(station)
}
func (h *Handler) UpdateStation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	resp, err := h.analytics.UpdateStation(id, body)
	proxyAnalytics(w, resp, err)
}
func (h *Handler) DeleteStation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteStation(r.Context(), id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) GetQueue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	queue, err := h.playback.BuildQueue(r.Context(), id, h.recentHours)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"queue": queue})
}
func (h *Handler) CreateStation(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	resp, err := h.analytics.CreateStation(body)
	proxyAnalytics(w, resp, err)
}
