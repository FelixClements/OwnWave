package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/pipeline"
)

func waitForDB(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err := pool.Ping(ctx); err == nil {
				return pool, nil
			}
			lastErr = err
			pool.Close()
		} else {
			lastErr = err
		}
		slog.Info("waiting for db", "attempt", i+1, "max", 30)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func loadAllowedOrigins() []string {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		domain := os.Getenv("OWNWAVE_DOMAIN")
		if domain != "" && domain != "localhost" {
			return []string{"https://" + domain}
		}
		return []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:3000"}
	}
	return origins
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}
	if err := analytics.CheckSecret(os.Getenv("ANALYTICS_API_SECRET")); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	db, err := waitForDB(ctx, dsn)
	if err != nil {
		log.Fatalf("db connection: %v", err)
	}
	defer db.Close()

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	musicDir := os.Getenv("MUSIC_DIR")
	if musicDir == "" {
		musicDir = "/music"
	}

	ffmpegPath := os.Getenv("FFMPEG_PATH")
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	pythonURL := os.Getenv("PYTHON_API_URL")
	if pythonURL == "" {
		pythonURL = "http://localhost:8000"
	}

	h := NewHandler(db, Config{
		MusicDir:          musicDir,
		FFmpegPath:        ffmpegPath,
		PythonURL:         pythonURL,
		RecentHours:       envInt("STATION_RECENT_HOURS", 24),
		TranscodeCacheDir: envString("TRANSCODE_CACHE_DIR", "/cache/transcodes"),
		TranscodeCacheMB:  envInt("TRANSCODE_CACHE_MB", 10240),
		CoverCacheDir:     envString("COVER_CACHE_DIR", "/cache/covers"),
	})

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			purged, err := h.auth.PurgeExpired(context.Background())
			if err != nil {
				slog.Error("failed to purge expired auth records", "error", err)
			} else if purged > 0 {
				slog.Info("purged expired auth records", "count", purged)
			}
		}
	}()

	// Spotify rotates editorial playlists every week or two and the embed
	// page only shows the current tracks, so sync daily to catch them all.
	// Checking hourly picks up stale playlists soon after a restart and
	// matches files added by a library scan within the hour.
	// The hourly pass also refreshes the genre stations, which catches up
	// on anything a missed pipeline notice left behind.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for ; ; <-ticker.C {
			if err := h.playlists.SyncStale(context.Background(), 24*time.Hour); err != nil {
				slog.Error("playlist sync", "error", err)
			}
			h.pipeline.Trigger(pipeline.TriggerHourly)
		}
	}()

	// After a library scan or genre scoring the analytics worker sends a
	// NOTIFY; matching and the genre stations follow right away.
	pipelineCtx, stopPipeline := context.WithCancel(context.Background())
	defer stopPipeline()
	h.pipeline.Start(pipelineCtx, dsn)

	r := newRouter(h, "web")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// No WriteTimeout: audio streams stay open for the whole track.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "error", err)
		}
	}()

	slog.Info("go-api listening", "port", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		log.Fatal(err)
	}
	// ListenAndServe returns as soon as Shutdown starts; wait for open
	// requests to drain before the deferred db.Close runs.
	<-shutdownDone
}

// newRouter serves the API under /api and, when webDir exists, the web UI at /.
func newRouter(h *Handler, webDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(clientIP(loadTrustedProxies()))
	r.Use(limitBody)
	r.Use(securityHeaders(h.cookieSecure))
	r.Use(httpAccessLog)
	r.Use(middleware.Recoverer)
	r.Use(prometheusMiddleware)
	origins := loadAllowedOrigins()
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler)
	r.Use(crossOriginProtection(origins))

	r.Get("/health", h.Health)

	// The API lives under /api so it can share the origin with the web UI.
	r.Route("/api", func(r chi.Router) {
		r.Get("/openapi.yaml", serveOpenAPI)

		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(10, time.Minute))
			r.Post("/login", h.Login)
			r.Post("/auth/device", h.LoginDevice)
		})
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(5, time.Minute))
			r.Post("/register", h.Register)
		})

		r.Get("/setup/status", h.SetupStatus)
		r.Group(func(r chi.Router) {
			r.Use(h.requireSetupAccess)
			r.Post("/setup/complete", h.SetupComplete)
			r.Get("/setup/summary", h.SetupSummary)
		})

		r.Group(func(r chi.Router) {
			r.Use(h.auth.RequireUser)
			r.Get("/me", h.Me)
			r.Put("/me/profile", h.UpdateProfile)
			r.Post("/logout", h.Logout)
			r.Get("/me/devices", h.ListDevices)
			r.Delete("/me/devices/{id}", h.RevokeDevice)
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(5, time.Minute))
				r.Post("/me/password", h.ChangePassword)
			})

			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(60, time.Minute))
				r.Get("/stream/{id}", h.StreamTrack)
			})
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(300, time.Minute))
				r.Get("/tracks/{id}/cover", h.GetTrackCover)
				r.Get("/stations/{id}/cover", h.GetStationCover)
			})

			r.Get("/tracks", h.ListTracks)
			r.Get("/albums", h.ListAlbums)
			r.Get("/artists", h.ListArtists)
			r.Get("/search", h.Search)
			r.Get("/search/stations", h.SearchStations)
			r.Get("/browse", h.Browse)
			r.Get("/tracks/{id}", h.GetTrack)
			r.Post("/tracks/{id}/played", h.RecordPlay)
			r.Post("/tracks/{id}/feedback", h.RecordFeedback)
			r.Delete("/tracks/{id}/feedback", h.DeleteFeedback)
			r.Get("/history", h.ListHistory)
			r.Get("/feedback", h.ListFeedback)
			r.Get("/genres", h.ListGenres)
			r.Get("/tracks/{id}/genres", h.GetTrackGenres)

			r.Get("/playlists", h.ListPlaylists)
			r.Get("/playlists/{id}", h.GetPlaylist)
			r.Get("/playlists/{id}/cover", h.GetPlaylistCover)
			r.Get("/playlists/{id}/picks", h.ListPlaylistPicks)

			r.Get("/stations", h.ListStations)
			r.Get("/stations/genres", h.ListStationGenres)
			r.Get("/stations/{id}", h.GetStation)
			r.Put("/stations/{id}", h.UpdateStation)
			r.Delete("/stations/{id}", h.DeleteStation)
			r.Get("/stations/{id}/queue", h.GetQueue)
			r.Get("/stations/{id}/hidden-tracks", h.ListHiddenTracks)
			r.Post("/stations/{id}/hidden-tracks", h.HideTrack)
			r.Delete("/stations/{id}/hidden-tracks/{trackId}", h.UnhideTrack)
			r.Post("/stations", h.CreateStation)
		})

		r.Group(func(r chi.Router) {
			r.Use(h.auth.RequireAdmin)
			r.Post("/admin/scan", h.TriggerScan)
			r.Get("/admin/scan/{id}", h.ScanStatus)
			r.Get("/admin/scans", h.ListScans)
			r.Get("/admin/health", h.AdminHealth)
			r.Get("/admin/stations", h.AdminStations)
			r.Get("/admin/station-suggestions", h.ListStationSuggestions)
			r.Get("/admin/genre-stations", h.AdminGenreStations)
			r.Put("/admin/genre-stations", h.AdminSetGenreStations)
			r.Put("/admin/genre-stations/{genre}", h.AdminUpdateGenreStation)
			r.Get("/admin/genre-review", h.AdminGenreReview)
			r.Get("/admin/genre-corrections", h.AdminGenreCorrections)
			r.Post("/admin/genre-corrections", h.AdminSaveGenreCorrection)
			r.Delete("/admin/genre-corrections/{id}", h.AdminDeleteGenreCorrection)
			r.Get("/admin/pipeline", h.AdminPipeline)
			r.Post("/admin/pipeline/refresh", h.AdminRefreshPipeline)
			r.Post("/admin/station-suggestions/dismiss", h.DismissStationSuggestion)
			r.Post("/admin/rebuild-analysis", h.AdminRebuildAnalysis)
			r.Post("/admin/rebuild-genres", h.AdminRebuildGenres)
			r.Get("/admin/genres/report", h.AdminGenreReport)
			r.Post("/admin/playlists", h.AddPlaylist)
			r.Post("/admin/playlists/{id}/sync", h.SyncPlaylist)
			r.Delete("/admin/playlists/{id}", h.DeletePlaylist)
			r.Delete("/admin/playlists/{id}/picks/{trackId}", h.RemovePlaylistPick)
			r.Put("/admin/playlists/{id}/items/{spotifyTrackId}/match", h.SetPlaylistMatch)
			r.Delete("/admin/playlists/{id}/items/{spotifyTrackId}/match", h.ResetPlaylistMatch)
			r.Put("/admin/playlists/{id}/items/{spotifyTrackId}/hidden", h.SetPlaylistItemHidden)
			r.Handle("/admin/metrics", metricsHandler())
			r.Get("/admin/users", h.ListUsers)
			r.Post("/admin/users", h.CreateUser)
			r.Delete("/admin/users/{id}", h.DeleteUser)
			r.Group(func(r chi.Router) {
				r.Use(httprate.LimitByIP(10, time.Minute))
				r.Post("/admin/invites", h.CreateInvite)
			})
		})
	})
	mountWeb(r, webDir)
	return r
}
