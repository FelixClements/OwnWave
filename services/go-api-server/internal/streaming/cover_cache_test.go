package streaming

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ownwave/api/internal/testffmpeg"
)

func TestCoverCacheHit(t *testing.T) {
	ff := testffmpeg.New(t)
	song := filepath.Join(t.TempDir(), "a.flac")
	testffmpeg.WriteFile(t, song, "cover")
	cache := NewCoverCache(10, 2)

	for i := 0; i < 3; i++ {
		data, err := cache.GetOrExtract(context.Background(), ff.Path, song)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != testffmpeg.JPEG {
			t.Fatalf("got %q", data)
		}
	}
	if n := ff.Runs(t); n != 1 {
		t.Fatalf("ffmpeg ran %d times, want 1 (cached)", n)
	}
}

func TestCoverCacheMissIsCached(t *testing.T) {
	ff := testffmpeg.New(t)
	song := filepath.Join(t.TempDir(), "a.flac")
	testffmpeg.WriteFile(t, song, "no art")
	cache := NewCoverCache(10, 2)

	for i := 0; i < 3; i++ {
		data, err := cache.GetOrExtract(context.Background(), ff.Path, song)
		if !errors.Is(err, ErrNoCover) || data != nil {
			t.Fatalf("got %q, %v; want ErrNoCover", data, err)
		}
	}
	if n := ff.Runs(t); n != 1 {
		t.Fatalf("ffmpeg ran %d times, want 1 (miss cached)", n)
	}
}

func TestCoverCacheMissInvalidatedWhenFileChanges(t *testing.T) {
	ff := testffmpeg.New(t)
	song := filepath.Join(t.TempDir(), "a.flac")
	testffmpeg.WriteFile(t, song, "no art")
	cache := NewCoverCache(10, 2)

	if _, err := cache.GetOrExtract(context.Background(), ff.Path, song); !errors.Is(err, ErrNoCover) {
		t.Fatalf("want ErrNoCover, got %v", err)
	}
	// Re-tagged with art: new contents and a new mtime.
	testffmpeg.WriteFile(t, song, "cover")
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(song, later, later); err != nil {
		t.Fatal(err)
	}
	data, err := cache.GetOrExtract(context.Background(), ff.Path, song)
	if err != nil || string(data) != testffmpeg.JPEG {
		t.Fatalf("got %q, %v; want the new cover", data, err)
	}
	if n := ff.Runs(t); n != 2 {
		t.Fatalf("ffmpeg ran %d times, want 2", n)
	}
	if cache.hits != 1 || cache.misses != 0 || len(cache.cache) != 1 {
		t.Fatalf("stale entry kept: hits=%d misses=%d entries=%d", cache.hits, cache.misses, len(cache.cache))
	}
}

func TestCoverCacheCancelledIsNotCached(t *testing.T) {
	ff := testffmpeg.New(t)
	song := filepath.Join(t.TempDir(), "a.flac")
	testffmpeg.WriteFile(t, song, "slow")
	cache := NewCoverCache(10, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := cache.GetOrExtract(ctx, ff.Path, song)
	if err == nil || errors.Is(err, ErrNoCover) {
		t.Fatalf("want context error, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
	if len(cache.cache) != 0 {
		t.Fatalf("cancelled extraction was cached: %v", cache.cache)
	}

	// ffmpeg runs again on the next request (same file, same mtime).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if _, err := cache.GetOrExtract(ctx2, ff.Path, song); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
	if n := ff.Runs(t); n != 2 {
		t.Fatalf("ffmpeg ran %d times, want 2 (no cached miss)", n)
	}
}

func TestCoverCacheContextCancel(t *testing.T) {
	ff := testffmpeg.New(t)
	song := filepath.Join(t.TempDir(), "a.flac")
	testffmpeg.WriteFile(t, song, "cover")
	cache := NewCoverCache(10, 1)
	// Saturate semaphore
	cache.sem <- struct{}{}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := cache.GetOrExtract(ctx, ff.Path, song)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context error, got %v", err)
	}
	if len(cache.cache) != 0 {
		t.Fatal("nothing should be cached")
	}
}

func TestCoverCacheMissingFFmpegIsAnError(t *testing.T) {
	song := filepath.Join(t.TempDir(), "a.flac")
	testffmpeg.WriteFile(t, song, "cover")
	cache := NewCoverCache(10, 1)

	_, err := cache.GetOrExtract(context.Background(), filepath.Join(t.TempDir(), "no-ffmpeg"), song)
	if err == nil || errors.Is(err, ErrNoCover) {
		t.Fatalf("want exec error, got %v", err)
	}
	if len(cache.cache) != 0 {
		t.Fatal("exec failure must not be cached")
	}
}

func TestCoverCacheMissingFileIsNoCover(t *testing.T) {
	ff := testffmpeg.New(t)
	cache := NewCoverCache(10, 1)

	_, err := cache.GetOrExtract(context.Background(), ff.Path, filepath.Join(t.TempDir(), "gone.flac"))
	if !errors.Is(err, ErrNoCover) {
		t.Fatalf("want ErrNoCover, got %v", err)
	}
	if ff.Runs(t) != 0 {
		t.Fatal("ffmpeg should not run for a missing file")
	}
}

func TestCoverCacheMissLimit(t *testing.T) {
	ff := testffmpeg.New(t)
	dir := t.TempDir()
	cache := NewCoverCache(10, 1)
	cache.maxMisses = 2

	for _, name := range []string{"a", "b", "c"} {
		song := filepath.Join(dir, name+".flac")
		testffmpeg.WriteFile(t, song, "no art")
		if _, err := cache.GetOrExtract(context.Background(), ff.Path, song); !errors.Is(err, ErrNoCover) {
			t.Fatalf("%s: want ErrNoCover, got %v", name, err)
		}
	}
	if cache.misses != 2 || len(cache.cache) != 2 {
		t.Fatalf("misses=%d entries=%d, want 2", cache.misses, len(cache.cache))
	}
}
