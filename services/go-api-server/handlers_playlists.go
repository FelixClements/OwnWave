package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"ownwave/api/internal/playlists"
	"ownwave/api/internal/spotify"
)

func (h *Handler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	list, err := h.playlists.List(r.Context())
	if err != nil {
		writeInternalError(w, r, "list playlists", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"playlists": list})
}

// GetPlaylist returns a playlist with every track it has ever listed and
// the local file each one matched.
func (h *Handler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	p, err := h.playlists.Get(r.Context(), id)
	if err != nil {
		writePlaylistError(w, r, "get playlist", err)
		return
	}
	items, err := h.playlists.Items(r.Context(), id)
	if err != nil {
		writeInternalError(w, r, "playlist items", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"playlist": p, "items": items})
}

// AddPlaylist adds a Spotify playlist by URL and syncs it right away.
func (h *Handler) AddPlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	p, err := h.playlists.Add(r.Context(), req.URL)
	if err != nil {
		writePlaylistError(w, r, "add playlist", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) SyncPlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	p, err := h.playlists.Sync(r.Context(), id)
	if err != nil {
		writePlaylistError(w, r, "sync playlist", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	usedBy, err := h.playlists.Delete(r.Context(), id)
	if err != nil {
		writePlaylistError(w, r, "delete playlist", err)
		return
	}
	if len(usedBy) > 0 {
		http.Error(w, "playlist is used by stations: "+strings.Join(usedBy, ", "), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writePlaylistError maps bad input to 400, unknown playlists to 404 and
// Spotify fetch failures to 502 with their message.
func writePlaylistError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	var fetchErr *spotify.FetchError
	switch {
	case errors.Is(err, spotify.ErrInvalidURL):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, playlists.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.As(err, &fetchErr):
		http.Error(w, "Spotify: "+fetchErr.Error(), http.StatusBadGateway)
	default:
		writeInternalError(w, r, msg, err)
	}
}
