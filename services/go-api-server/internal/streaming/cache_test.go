package streaming

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeFFmpeg writes "part1-part2-end" to its last argument in three slow
// steps (or fails when FAKE_FFMPEG_FAIL is set) and counts its runs.
func fakeFFmpeg(t *testing.T) (path, counter string) {
	t.Helper()
	dir := t.TempDir()
	counter = filepath.Join(dir, "runs")
	path = filepath.Join(dir, "ffmpeg")
	script := `#!/bin/sh
for out; do :; done
echo run >> "` + counter + `"
if [ -n "$FAKE_FFMPEG_FAIL" ]; then echo boom >&2; exit 1; fi
printf 'part1-' > "$out"; sleep 0.2
printf 'part2-' >> "$out"; sleep 0.2
printf 'end' >> "$out"
`
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path, counter
}

func runs(t *testing.T, counter string) int {
	b, err := os.ReadFile(counter)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return strings.Count(string(b), "run")
}

func newTestCache(t *testing.T, maxBytes int64, slots int) (*Cache, string) {
	ffmpeg, counter := fakeFFmpeg(t)
	c, err := NewCache(filepath.Join(t.TempDir(), "cache"), maxBytes, ffmpeg, slots)
	require.NoError(t, err)
	c.poll = 10 * time.Millisecond
	return c, counter
}

func src(name string) Source {
	return Source{Path: "/music/" + name + ".flac", Size: 100, ModTime: time.Unix(1700000000, 0)}
}

func get(c *Cache, s Source, rng string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	rec := httptest.NewRecorder()
	c.Serve(rec, req, s, Profiles["normal"])
	return rec
}

func TestCacheStreamsFirstPlayThenServesFileWithRanges(t *testing.T) {
	c, counter := newTestCache(t, 1<<20, 2)

	first := get(c, src("a"), "")
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, "part1-part2-end", first.Body.String(), "streamed while ffmpeg was writing")
	require.Equal(t, "audio/ogg", first.Header().Get("Content-Type"))

	second := get(c, src("a"), "")
	require.Equal(t, "part1-part2-end", second.Body.String())
	require.Equal(t, "15", second.Header().Get("Content-Length"))

	seek := get(c, src("a"), "bytes=6-10")
	require.Equal(t, http.StatusPartialContent, seek.Code)
	require.Equal(t, "part2", seek.Body.String())
	require.Equal(t, 1, runs(t, counter), "transcoded once")
}

func TestCacheKeysOnFileVersionAndGain(t *testing.T) {
	c, counter := newTestCache(t, 1<<20, 2)
	get(c, src("a"), "")
	edited := src("a")
	edited.ModTime = edited.ModTime.Add(time.Second)
	get(c, edited, "")
	louder := src("a")
	louder.GainDB = 2
	get(c, louder, "")
	require.Equal(t, 3, runs(t, counter))
}

func TestCacheSharesARunningTranscode(t *testing.T) {
	c, counter := newTestCache(t, 1<<20, 1)
	var wg sync.WaitGroup
	bodies := make([]string, 3)
	for i := range bodies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			bodies[i] = get(c, src("shared"), "").Body.String()
		}(i)
	}
	wg.Wait()
	for _, b := range bodies {
		require.Equal(t, "part1-part2-end", b)
	}
	require.Equal(t, 1, runs(t, counter))
}

func TestCacheSeekDuringFirstPlayWaitsForTheFile(t *testing.T) {
	c, _ := newTestCache(t, 1<<20, 2)
	rec := get(c, src("seek"), "bytes=12-")
	require.Equal(t, http.StatusPartialContent, rec.Code)
	require.Equal(t, "end", rec.Body.String())
}

func TestCacheBusyAndFailure(t *testing.T) {
	c, _ := newTestCache(t, 1<<20, 1)
	c.sem <- struct{}{} // every slot taken
	require.Equal(t, http.StatusServiceUnavailable, get(c, src("busy"), "").Code)
	<-c.sem

	t.Setenv("FAKE_FFMPEG_FAIL", "1")
	rec := get(c, src("broken"), "bytes=1-")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	matches, _ := filepath.Glob(filepath.Join(c.dir, "*", "*"))
	require.Empty(t, matches, "a failed transcode leaves no file behind")
}

func TestCacheEvictsLeastRecentlyPlayed(t *testing.T) {
	c, _ := newTestCache(t, 40, 2) // room for two 15-byte files
	get(c, src("old"), "")
	get(c, src("mid"), "")
	old := c.finalPath(c.key(src("old"), Profiles["normal"]), Profiles["normal"])
	past := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(old, past, past))

	get(c, src("new"), "") // 45 bytes > 40: evict down to 36
	_, err := os.Stat(old)
	require.True(t, os.IsNotExist(err), "oldest file evicted")
	_, err = os.Stat(c.finalPath(c.key(src("new"), Profiles["normal"]), Profiles["normal"]))
	require.NoError(t, err)
}

func TestOriginalContentType(t *testing.T) {
	require.Equal(t, "audio/flac", OriginalContentType("/m/a.FLAC"))
	require.Equal(t, "audio/mpeg", OriginalContentType("/m/a.mp3"))
	require.Equal(t, "application/octet-stream", OriginalContentType("/m/a.xyz"))
}
