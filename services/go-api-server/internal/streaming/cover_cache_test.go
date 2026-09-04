package streaming

import (
	"context"
	"testing"
	"time"
)

func TestCoverCacheHit(t *testing.T) {
	cache := NewCoverCache(10, 2)
	cache.cache["dummy.flac"] = []byte("image-data")

	data, err := cache.GetOrExtract(context.Background(), "ffmpeg", "dummy.flac")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "image-data" {
		t.Fatalf("expected 'image-data', got %q", string(data))
	}
}

func TestCoverCacheContextCancel(t *testing.T) {
	cache := NewCoverCache(10, 1)
	// Saturate semaphore
	cache.sem <- struct{}{}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := cache.GetOrExtract(ctx, "ffmpeg", "nonexistent.flac")
	if err == nil {
		t.Fatal("expected error due to canceled context, got nil")
	}
}
