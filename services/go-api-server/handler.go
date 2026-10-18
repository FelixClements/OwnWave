package main

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
	"ownwave/api/internal/playback"
	"ownwave/api/internal/streaming"
)

type Handler struct {
	db                       *DB
	auth                     *auth.Service
	playback                 *playback.Service
	stream                   *streaming.Server
	coverCache               *streaming.CoverCache
	analytics                analytics.Client
	musicDir                 string
	ffmpegPath               string
	recentHours              int
	minTracksPerGenreStation int
	cookieSecure             bool
}

// Config holds the environment-derived settings for NewHandler.
type Config struct {
	MusicDir                 string
	FFmpegPath               string
	PythonURL                string
	RecentHours              int
	MinTracksPerGenreStation int
}

func NewHandler(pool *pgxpool.Pool, cfg Config) *Handler {
	if cfg.RecentHours <= 0 {
		cfg.RecentHours = 24
	}
	return &Handler{
		db:                       NewDB(pool),
		auth:                     auth.NewService(pool),
		playback:                 playback.NewService(pool),
		stream:                   streaming.New(streaming.Config{MusicDir: cfg.MusicDir, FFmpegPath: cfg.FFmpegPath}),
		coverCache:               streaming.NewCoverCache(500, 4),
		analytics:                analytics.NewHTTPClient(cfg.PythonURL),
		musicDir:                 cfg.MusicDir,
		ffmpegPath:               cfg.FFmpegPath,
		recentHours:              cfg.RecentHours,
		minTracksPerGenreStation: cfg.MinTracksPerGenreStation,
		cookieSecure:             auth.CookieSecure(),
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

func proxyAnalytics(w http.ResponseWriter, r *http.Request, resp *analytics.Response, err error) {
	if err != nil {
		writeInternalError(w, r, "analytics proxy", err)
		return
	}
	resp.WriteJSON(w)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
