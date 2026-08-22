package main

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
	"ownwave/api/internal/playback"
	"ownwave/api/internal/streamauth"
	"ownwave/api/internal/streaming"
)

type Handler struct {
	db           *DB
	auth         *auth.Service
	streamTokens *streamauth.StreamTokens
	playback     *playback.Service
	stream       *streaming.Server
	analytics    analytics.Client
	musicDir     string
	ffmpegPath   string
	recentHours  int
}

func NewHandler(pool *pgxpool.Pool, jwtSecret []byte, musicDir, ffmpegPath, pythonURL string, recentHours int) *Handler {
	if recentHours <= 0 {
		recentHours = 24
	}
	return &Handler{
		db:           NewDB(pool),
		auth:         auth.NewService(pool),
		streamTokens: streamauth.New(jwtSecret),
		playback:     playback.NewService(pool),
		stream:       streaming.New(streaming.Config{MusicDir: musicDir, FFmpegPath: ffmpegPath}),
		analytics:    analytics.NewHTTPClient(pythonURL),
		musicDir:     musicDir,
		ffmpegPath:   ffmpegPath,
		recentHours:  recentHours,
	}
}

func proxyAnalytics(w http.ResponseWriter, resp *analytics.Response, err error) {
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	resp.WriteJSON(w)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
