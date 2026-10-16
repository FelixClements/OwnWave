package streaming

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeTranscodedReturns503WhenSemaphoreFull(t *testing.T) {
	s := New(Config{
		MusicDir:                t.TempDir(),
		FFmpegPath:              "ffmpeg",
		MaxConcurrentTranscodes: 1,
	})

	// Fill the semaphore
	s.transcodeSem <- struct{}{}

	req := httptest.NewRequest(http.MethodGet, "/stream/123?format=mp3", nil)
	rec := httptest.NewRecorder()

	s.ServeTranscoded(rec, req, "dummy.flac", "mp3", nil, false)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}
