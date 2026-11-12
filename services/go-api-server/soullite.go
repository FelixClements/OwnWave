package main

import (
	"context"
	"log/slog"
	"time"
)

// soulLiteTimeout bounds one background follow or unfollow (two calls each).
const soulLiteTimeout = 30 * time.Second

// followInSoulLite asks SoulLite to follow a playlist just added here. It
// runs in the background and only logs failures: SoulLite being down never
// fails the admin request, and the next startup catch-up retries.
func (h *Handler) followInSoulLite(spotifyID, name string) {
	if h.soullite == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), soulLiteTimeout)
		defer cancel()
		if err := h.soullite.Follow(ctx, spotifyID, name); err != nil {
			slog.Warn("soullite follow failed", "spotify_id", spotifyID, "error", err)
			return
		}
		slog.Info("soullite follows playlist", "spotify_id", spotifyID)
	}()
}

// unfollowInSoulLite asks SoulLite to stop following a playlist just deleted
// here, in the background. SoulLite keeps the files it downloaded.
func (h *Handler) unfollowInSoulLite(spotifyID string) {
	if h.soullite == nil || spotifyID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), soulLiteTimeout)
		defer cancel()
		if err := h.soullite.Unfollow(ctx, spotifyID); err != nil {
			slog.Warn("soullite unfollow failed", "spotify_id", spotifyID, "error", err)
			return
		}
		slog.Info("soullite unfollowed playlist", "spotify_id", spotifyID)
	}()
}

// followAllInSoulLite sends every playlist to SoulLite once. It runs at
// startup and catches up on follows lost while SoulLite was down, and on
// playlists added before the integration was turned on. SoulLite answers
// "already followed" for the rest, which leaves them as they are.
func (h *Handler) followAllInSoulLite(ctx context.Context) {
	if h.soullite == nil {
		return
	}
	list, err := h.playlists.List(ctx)
	if err != nil {
		slog.Error("soullite catch-up: list playlists", "error", err)
		return
	}
	failed := 0
	for _, p := range list {
		callCtx, cancel := context.WithTimeout(ctx, soulLiteTimeout)
		err := h.soullite.Follow(callCtx, p.SpotifyID, p.Name)
		cancel()
		if err != nil {
			failed++
			slog.Warn("soullite catch-up: follow failed", "spotify_id", p.SpotifyID, "error", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
	slog.Info("soullite catch-up done", "playlists", len(list), "failed", failed)
}
