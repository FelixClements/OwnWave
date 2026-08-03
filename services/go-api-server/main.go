package main

import (
	"context"
	"log"
	"net/http"
	"os"
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
		log.Printf("waiting for db... (%d/30)", i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func main() {
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

	h := NewHandler(db, jwtSecret, musicDir, ffmpegPath, pythonURL)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
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
	r.Post("/tracks/{id}/played", h.RecordPlay)
	r.Post("/tracks/{id}/feedback", h.RecordFeedback)
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
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Get("/me", h.Me)
	r.Post("/logout", h.Logout)
	r.Post("/stations", h.CreateStation)
	r.Post("/admin/scan", h.TriggerScan)

	log.Println("go-api listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
