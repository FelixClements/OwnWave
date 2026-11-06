package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"ownwave/api/internal/pipeline"
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
// the local file each one matched. ?items=missing lists only the tracks
// with no local file (the missing list), ?items=matched only the others.
func (h *Handler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	filter := playlists.ItemFilter(r.URL.Query().Get("items"))
	switch filter {
	case playlists.AllItems, playlists.MatchedItems, playlists.MissingItems:
	default:
		http.Error(w, "items must be matched or missing", http.StatusBadRequest)
		return
	}
	p, err := h.playlists.Get(r.Context(), id)
	if err != nil {
		writePlaylistError(w, r, "get playlist", err)
		return
	}
	items, err := h.playlists.Items(r.Context(), id, filter)
	if err != nil {
		writeInternalError(w, r, "playlist items", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"playlist": p, "items": items})
}

// ListPlaylistPicks lists the library songs the playlist's stations play on
// top of its matches.
func (h *Handler) ListPlaylistPicks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.playlists.Get(r.Context(), id); err != nil {
		writePlaylistError(w, r, "get playlist", err)
		return
	}
	picks, err := h.playlists.Picks(r.Context(), id)
	if err != nil {
		writeInternalError(w, r, "playlist picks", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"picks": picks})
}

// RemovePlaylistPick takes a song out of this playlist's picks for good.
func (h *Handler) RemovePlaylistPick(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	trackID, ok := parseUUIDParam(w, r, "trackId")
	if !ok {
		return
	}
	if err := h.playlists.RemovePick(r.Context(), id, trackID); err != nil {
		writePlaylistError(w, r, "remove playlist pick", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPlaylistCover returns the playlist's Spotify cover, downloaded at
// sync. 204 when it has none yet; 404 only for an unknown playlist.
func (h *Handler) GetPlaylistCover(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	img, mediaType, err := h.playlists.Cover(r.Context(), id)
	if err != nil {
		writePlaylistError(w, r, "playlist cover", err)
		return
	}
	if img == nil {
		writeNoCover(w)
		return
	}
	w.Header().Set("Content-Type", mediaType)
	// A sync may bring a new cover: cache for an hour, like station covers.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(img)
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
	h.refreshStations(r, pipeline.TriggerAdded)
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
	h.refreshStations(r, pipeline.TriggerSynced)
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
	h.refreshStations(r, pipeline.TriggerDeleted)
	w.WriteHeader(http.StatusNoContent)
}

// refreshStations brings the genre stations in line after a playlist
// change. The change itself already matched its tracks and is done, so a
// failed refresh is only logged (the hourly pass retries it).
func (h *Handler) refreshStations(r *http.Request, trigger string) {
	if _, err := h.pipeline.Run(r.Context(), trigger, false); err != nil {
		slog.Error("station refresh failed", "trigger", trigger, "error", err)
	}
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
