package main

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
	"ownwave/api/internal/playback"
	"ownwave/api/internal/streaming"
)

type Handler struct {
	db           *DB
	auth         *auth.Service
	playback     *playback.Service
	stream       *streaming.Server
	analytics    analytics.Client
	musicDir     string
	ffmpegPath   string
	recentHours  int
	cookieSecure bool
}

func NewHandler(pool *pgxpool.Pool, musicDir, ffmpegPath, pythonURL string, recentHours int) *Handler {
	if recentHours <= 0 {
		recentHours = 24
	}
	return &Handler{
		db:           NewDB(pool),
		auth:         auth.NewService(pool),
		playback:     playback.NewService(pool),
		stream:       streaming.New(streaming.Config{MusicDir: musicDir, FFmpegPath: ffmpegPath}),
		analytics:    analytics.NewHTTPClient(pythonURL),
		musicDir:     musicDir,
		ffmpegPath:   ffmpegPath,
		recentHours:  recentHours,
		cookieSecure: auth.CookieSecure(),
	}
}

func (h *Handler) currentUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return auth.User{}, false
	}
	return user, true
}

func (h *Handler) analyticsFor(userID string) analytics.Client {
	return h.analytics.WithUser(userID)
}

func proxyAnalytics(w http.ResponseWriter, r *http.Request, resp *analytics.Response, err error) {
	if err != nil {
		writeInternalError(w, r, "analytics proxy", err)
		return
	}
	resp.WriteJSON(w)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
