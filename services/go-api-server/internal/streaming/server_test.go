package streaming

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ownwave/api/internal/playback"
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

func TestServeCrossfadedReturns503WhenSemaphoreFull(t *testing.T) {
	s := New(Config{
		MusicDir:                t.TempDir(),
		FFmpegPath:              "ffmpeg",
		MaxConcurrentTranscodes: 1,
	})

	// Fill the semaphore
	s.transcodeSem <- struct{}{}

	req := httptest.NewRequest(http.MethodGet, "/stations/123/crossfade?format=mp3", nil)
	rec := httptest.NewRecorder()

	queue := []playback.TrackWithFeatures{
		{Track: playback.Track{ID: "t1", Path: "track1.flac"}},
		{Track: playback.Track{ID: "t2", Path: "track2.flac"}},
	}

	s.ServeCrossfaded(rec, req, queue, "mp3", "320k", false, false)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestServeCrossfadedEmptyQueueReturnsBadRequest(t *testing.T) {
	s := New(Config{
		MusicDir:   t.TempDir(),
		FFmpegPath: "ffmpeg",
	})

	req := httptest.NewRequest(http.MethodGet, "/stations/123/crossfade?format=mp3", nil)
	rec := httptest.NewRecorder()

	s.ServeCrossfaded(rec, req, nil, "mp3", "320k", false, false)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
