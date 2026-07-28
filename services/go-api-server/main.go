package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := pgxpool.New(context.Background(), dsn)
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
	r.Get("/tracks/{id}", h.GetTrack)
	r.Get("/tracks/{id}/stream-url", h.StreamURL)
	r.Get("/stream/{id}", h.StreamTrack)
	r.Get("/stations", h.ListStations)
	r.Get("/stations/{id}", h.GetStation)
	r.Get("/stations/{id}/queue", h.GetQueue)
	r.Post("/stations", h.CreateStation)
	r.Post("/admin/scan", h.TriggerScan)

	log.Println("go-api listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
