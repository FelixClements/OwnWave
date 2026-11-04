package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
	"ownwave/api/internal/pipeline"
	"ownwave/api/internal/playback"
	"ownwave/api/internal/playlists"
	"ownwave/api/internal/spotify"
	"ownwave/api/internal/stations"
	"ownwave/api/internal/streaming"
)

type Handler struct {
	db           *DB
	auth         *auth.Service
	playback     *playback.Service
	playlists    *playlists.Service
	stations     *stations.Service
	pipeline     *pipeline.Runner
	stream       *streaming.Server
	transcodes   *streaming.Cache // nil: transcode live, uncached
	coverCache   *streaming.CoverCache
	analytics    analytics.Client
	musicDir     string
	ffmpegPath   string
	recentHours  int
	cookieSecure bool
}

// Config holds the environment-derived settings for NewHandler.
type Config struct {
	MusicDir    string
	FFmpegPath  string
	PythonURL   string
	RecentHours int
	// TranscodeCacheDir holds transcoded files ("" = no cache).
	TranscodeCacheDir string
	// TranscodeCacheMB caps the cache size; least recently played go first.
	TranscodeCacheMB int
}

func NewHandler(pool *pgxpool.Pool, cfg Config) *Handler {
	if cfg.RecentHours <= 0 {
		cfg.RecentHours = 24
	}
	var transcodes *streaming.Cache
	if cfg.TranscodeCacheDir != "" {
		var err error
		transcodes, err = streaming.NewCache(cfg.TranscodeCacheDir, int64(cfg.TranscodeCacheMB)<<20, cfg.FFmpegPath, 4)
		if err != nil {
			slog.Warn("transcode cache disabled", "dir", cfg.TranscodeCacheDir, "error", err)
			transcodes = nil
		}
	}
	playlistService := playlists.NewService(pool, spotify.Client{HTTP: &http.Client{Timeout: 20 * time.Second}})
	stationService := stations.NewService(pool)
	return &Handler{
		transcodes:   transcodes,
		db:           NewDB(pool),
		auth:         auth.NewService(pool),
		playback:     playback.NewService(pool),
		playlists:    playlistService,
		stations:     stationService,
		pipeline:     pipeline.New(pool, playlistService, stationService),
		stream:       streaming.New(streaming.Config{MusicDir: cfg.MusicDir, FFmpegPath: cfg.FFmpegPath}),
		coverCache:   streaming.NewCoverCache(500, 4),
		analytics:    analytics.NewHTTPClient(cfg.PythonURL),
		musicDir:     cfg.MusicDir,
		ffmpegPath:   cfg.FFmpegPath,
		recentHours:  cfg.RecentHours,
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
