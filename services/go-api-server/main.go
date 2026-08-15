package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("dev-secret-change-in-production")
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

	recentHours := 24
	if v := os.Getenv("STATION_RECENT_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			recentHours = n
		}
	}

	h := NewHandler(db, jwtSecret, musicDir, ffmpegPath, pythonURL, recentHours)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(prometheusMiddleware)
	r.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}).Handler)

	r.Get("/health", h.Health)
	r.Get("/tracks", h.ListTracks)
	r.Get("/albums", h.ListAlbums)
	r.Get("/artists", h.ListArtists)
	r.Get("/search", h.Search)
	r.Post("/rescan", h.Rescan)
	r.Get("/tracks/{id}", h.GetTrack)
	r.Get("/tracks/{id}/similar", h.GetSimilarTracks)
	r.Post("/tracks/{id}/played", h.RecordPlay)
	r.Post("/tracks/{id}/feedback", h.RecordFeedback)
	r.Delete("/tracks/{id}/feedback", h.DeleteFeedback)
	r.Get("/history", h.ListHistory)
	r.Get("/feedback", h.ListFeedback)
	r.Get("/tracks/{id}/cover", h.GetTrackCover)
	r.Get("/tracks/{id}/stream-url", h.StreamURL)
	r.Get("/stream/{id}", h.StreamTrack)
	r.Get("/stations", h.ListStations)
	r.Get("/stations/{id}", h.GetStation)
	r.Put("/stations/{id}", h.UpdateStation)
	r.Delete("/stations/{id}", h.DeleteStation)
	r.Get("/stations/{id}/queue", h.GetQueue)
	r.Get("/stations/{id}/crossfade-url", h.StationCrossfadeURL)
	r.Get("/stations/{id}/crossfade", h.StationCrossfadeStream)
	r.Get("/setup/status", h.SetupStatus)
	r.Post("/setup/complete", h.SetupComplete)
	r.Get("/setup/summary", h.SetupSummary)
	r.Post("/setup/stations", h.SetupStations)
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Get("/me", h.Me)
	r.Put("/me/profile", h.UpdateProfile)
	r.Post("/logout", h.Logout)
	r.Post("/me/password", h.ChangePassword)
	r.Post("/stations", h.CreateStation)
	r.Post("/admin/scan", h.TriggerScan)
	r.Get("/admin/scan/{id}", h.ScanStatus)
	r.Get("/admin/health", h.AdminHealth)
	r.Get("/admin/stations", h.AdminStations)
	r.Post("/admin/rebuild-vectors", h.AdminRebuildVectors)
	r.Post("/admin/rebuild-clusters", h.AdminRebuildClusters)
	r.Post("/admin/rebuild-genres", h.AdminRebuildGenres)
	r.Post("/admin/rebuild-genre-stations", h.AdminRebuildGenreStations)
	r.Get("/genres", h.ListGenres)
	r.Get("/tracks/{id}/genres", h.GetTrackGenres)
	r.Mount("/metrics", metricsHandler())

	slog.Info("go-api listening", "port", "8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		slog.Error("server failed", "error", err)
		log.Fatal(err)
	}
}
