package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
	"ownwave/api/internal/library"
	"ownwave/api/internal/listening"
	"ownwave/api/internal/pipeline"
	"ownwave/api/internal/playlists"
	"ownwave/api/internal/settings"
	"ownwave/api/internal/soullite"
	"ownwave/api/internal/spotify"
	"ownwave/api/internal/stations"
	"ownwave/api/internal/streaming"
)

type Handler struct {
	db           pinger
	auth         *auth.Service
	library      *library.Service
	listening    *listening.Service
	settings     *settings.Service
	playlists    *playlists.Service
	stations     *stations.Service
	pipeline     *pipeline.Runner
	stream       *streaming.Server
	transcodes   *streaming.Cache // nil: transcode live, uncached
	coverCache   *streaming.CoverCache
	analytics    analytics.Client
	analyticsAPI *analytics.Proxy // pass-through routes, see analytics.Routes
	ffmpegPath   string
	cookieSecure bool
	logins       *loginThrottle   // nil: no per-username limit
	soullite     *soullite.Client // nil: SoulLite integration off
	setup        *setupToken
}

// pinger checks that the database answers.
type pinger interface {
	Ping(ctx context.Context) error
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
	// CoverCacheDir holds cover thumbnails across restarts ("" = memory only).
	CoverCacheDir string
	// SoulLite follows the Spotify playlists added here (nil = off).
	SoulLite *soullite.Client
	// SetupToken is the secret that creates the first (admin) account
	// ("" = a random one for this run, written to the log).
	SetupToken string
}

func NewHandler(pool *pgxpool.Pool, cfg Config) *Handler {
	var transcodes *streaming.Cache
	if cfg.TranscodeCacheDir != "" {
		var err error
		transcodes, err = streaming.NewCache(cfg.TranscodeCacheDir, int64(cfg.TranscodeCacheMB)<<20, cfg.FFmpegPath, 4)
		if err != nil {
			slog.Warn("transcode cache disabled", "dir", cfg.TranscodeCacheDir, "error", err)
			transcodes = nil
		}
	}
	playlistService := playlists.NewService(pool, spotify.NewClient(&http.Client{Timeout: 20 * time.Second}))
	stationService := stations.NewService(pool, stations.WithRecentHours(cfg.RecentHours))
	setup := newSetupToken(cfg.SetupToken)
	return &Handler{
		transcodes:   transcodes,
		db:           pool,
		auth:         auth.NewService(pool, auth.WithSetupToken(setup.value)),
		library:      library.NewService(pool),
		listening:    listening.NewService(pool),
		settings:     settings.NewService(pool),
		playlists:    playlistService,
		stations:     stationService,
		pipeline:     pipeline.New(pool, playlistService, stationService),
		stream:       streaming.New(streaming.Config{MusicDir: cfg.MusicDir, FFmpegPath: cfg.FFmpegPath}),
		coverCache:   streaming.NewCoverCache(cfg.CoverCacheDir, 1000, 4),
		analytics:    analytics.NewHTTPClient(cfg.PythonURL),
		analyticsAPI: newAnalyticsProxy(cfg.PythonURL),
		ffmpegPath:   cfg.FFmpegPath,
		cookieSecure: auth.CookieSecure(),
		logins:       newLoginThrottle(10, 15*time.Minute),
		soullite:     cfg.SoulLite,
		setup:        setup,
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

func newAnalyticsProxy(pythonURL string) *analytics.Proxy {
	return analytics.NewProxy(pythonURL, func(w http.ResponseWriter, r *http.Request, err error) {
		writeInternalError(w, r, "analytics proxy", err)
	})
}

// mountAnalytics registers the pass-through analytics routes behind one guard.
// The caller's group applies that guard.
func (h *Handler) mountAnalytics(r chi.Router, access analytics.Access) {
	for _, rt := range analytics.RoutesFor(access) {
		r.Method(rt.Method, rt.Public, h.analyticsAPI.Handler(rt))
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
