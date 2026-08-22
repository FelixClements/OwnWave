package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/playback"
	"ownwave/api/internal/streaming"
)

type Handler struct {
	db          *DB
	playback    *playback.Service
	stream      *streaming.Server
	analytics   analytics.Client
	jwtSecret   []byte
	musicDir    string
	ffmpegPath  string
	recentHours int
}

func NewHandler(pool *pgxpool.Pool, jwtSecret []byte, musicDir, ffmpegPath, pythonURL string, recentHours int) *Handler {
	if recentHours <= 0 {
		recentHours = 24
	}
	return &Handler{
		db:          NewDB(pool),
		playback:    playback.NewService(pool),
		stream:      streaming.New(streaming.Config{MusicDir: musicDir, FFmpegPath: ffmpegPath}),
		analytics:   analytics.NewHTTPClient(pythonURL),
		jwtSecret:   jwtSecret,
		musicDir:    musicDir,
		ffmpegPath:  ffmpegPath,
		recentHours: recentHours,
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

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func getString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	if v, ok := claims[key].(float64); ok {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}
