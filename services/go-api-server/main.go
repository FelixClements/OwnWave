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

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if f, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && f >= 0 {
		return f
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
		MusicDir:                 musicDir,
		FFmpegPath:               ffmpegPath,
		PythonURL:                pythonURL,
		RecentHours:              envInt("STATION_RECENT_HOURS", 24),
		MinGenreConfidence:       envFloat("GENRE_MIN_CONFIDENCE", 0.3),
		MinTracksPerGenreStation: envInt("GENRE_MIN_TRACKS_PER_STATION", 5),
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
	r.Use(middleware.RealIP)
	r.Use(securityHeaders(h.cookieSecure))
	r.Use(httpAccessLog)
	r.Use(middleware.Recoverer)
	r.Use(prometheusMiddleware)
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   loadAllowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler)

	r.Get("/health", h.Health)

	// The API lives under /api so it can share the origin with the web UI.
	r.Route("/api", func(r chi.Router) {

		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(10, time.Minute))
			r.Post("/login", h.Login)
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
			r.Post("/setup/stations", h.SetupStations)
		})

		r.Group(func(r chi.Router) {
			r.Use(h.auth.RequireUser)
			r.Get("/me", h.Me)
			r.Put("/me/profile", h.UpdateProfile)
			r.Post("/logout", h.Logout)
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
			})

			r.Get("/tracks", h.ListTracks)
			r.Get("/albums", h.ListAlbums)
			r.Get("/artists", h.ListArtists)
			r.Get("/search", h.Search)
			r.Get("/tracks/{id}", h.GetTrack)
			r.Get("/tracks/{id}/similar", h.GetSimilarTracks)
			r.Post("/tracks/{id}/played", h.RecordPlay)
			r.Post("/tracks/{id}/feedback", h.RecordFeedback)
			r.Delete("/tracks/{id}/feedback", h.DeleteFeedback)
			r.Get("/history", h.ListHistory)
			r.Get("/feedback", h.ListFeedback)
			r.Get("/genres", h.ListGenres)
			r.Get("/tracks/{id}/genres", h.GetTrackGenres)

			r.Get("/stations", h.ListStations)
			r.Get("/stations/{id}", h.GetStation)
			r.Put("/stations/{id}", h.UpdateStation)
			r.Delete("/stations/{id}", h.DeleteStation)
			r.Get("/stations/{id}/queue", h.GetQueue)
			r.Post("/stations", h.CreateStation)
		})

		r.Group(func(r chi.Router) {
			r.Use(h.auth.RequireAdmin)
			r.Post("/admin/scan", h.TriggerScan)
			r.Get("/admin/scan/{id}", h.ScanStatus)
			r.Get("/admin/scans", h.ListScans)
			r.Get("/admin/health", h.AdminHealth)
			r.Get("/admin/stations", h.AdminStations)
			r.Post("/admin/rebuild-analysis", h.AdminRebuildAnalysis)
			r.Post("/admin/rebuild-genres", h.AdminRebuildGenres)
			r.Post("/admin/rebuild-genre-stations", h.AdminRebuildGenreStations)
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
