//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/auth"
	"ownwave/api/internal/testdb"
)

// TestStreamQualitiesWithFFmpeg streams a real file in every quality.
func TestStreamQualitiesWithFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	music := t.TempDir()
	song := filepath.Join(music, "tone.flac")
	out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=f=440:d=8", song).CombinedOutput()
	require.NoError(t, err, string(out))

	pool := testdb.NewPool(t)
	ctx := context.Background()
	h := NewHandler(pool, Config{MusicDir: music, FFmpegPath: ffmpeg, TranscodeCacheDir: t.TempDir(), TranscodeCacheMB: 50})
	var trackID, userID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, title) VALUES ($1, 'Tone') RETURNING id::text`, song).Scan(&trackID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, password_hash) VALUES ('u', 'x') RETURNING id::text`).Scan(&userID))

	stream := func(query, rng string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/stream/"+trackID+"?"+query, nil)
		if rng != "" {
			req.Header.Set("Range", rng)
		}
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", trackID)
		req = req.WithContext(context.WithValue(auth.WithUser(req.Context(), auth.User{ID: userID}), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		h.StreamTrack(rec, req)
		return rec
	}

	orig := stream("quality=original", "")
	require.Equal(t, http.StatusOK, orig.Code)
	require.Equal(t, "audio/flac", orig.Header().Get("Content-Type"))
	require.Equal(t, "fLaC", orig.Body.String()[:4])

	for quality, want := range map[string]struct{ contentType, magic string }{
		"high": {"audio/ogg", "OggS"}, "normal": {"audio/ogg", "OggS"}, "low": {"audio/ogg", "OggS"}, "aac": {"audio/aac", "\xff"},
	} {
		first := stream("quality="+quality, "")
		require.Equal(t, http.StatusOK, first.Code, quality)
		require.Equal(t, want.contentType, first.Header().Get("Content-Type"), quality)
		require.Equal(t, want.magic, first.Body.String()[:len(want.magic)], quality)

		cached := stream("quality="+quality, "")
		require.Equal(t, first.Body.Len(), cached.Body.Len(), quality)
		require.NotEmpty(t, cached.Header().Get("Content-Length"), "%s: second play is the cached file", quality)
		seek := stream("quality="+quality, "bytes=100-199")
		require.Equal(t, http.StatusPartialContent, seek.Code, quality)
		require.Equal(t, 100, seek.Body.Len(), quality)
	}
	low, normal := stream("quality=low", "").Body.Len(), stream("quality=normal", "").Body.Len()
	require.Less(t, low, normal, "lower quality, smaller stream")
}
