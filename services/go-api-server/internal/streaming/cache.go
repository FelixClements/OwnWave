package streaming

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Profile is a transcoding quality clients pick with ?quality=.
type Profile struct {
	Name        string
	Encoder     string
	Bitrate     string
	Format      string // ffmpeg muxer
	Ext         string
	ContentType string
}

// Profiles are the transcoded qualities. "original" (the file as stored)
// is not a profile: it is served directly.
var Profiles = map[string]Profile{
	"high":   {"high", "libopus", "160k", "opus", ".opus", "audio/ogg"},
	"normal": {"normal", "libopus", "96k", "opus", ".opus", "audio/ogg"},
	"low":    {"low", "libopus", "64k", "opus", ".opus", "audio/ogg"},
	// For clients without Opus support.
	"aac": {"aac", "aac", "128k", "adts", ".aac", "audio/aac"},
}

// Quality names accepted by ?quality=, best first.
var Qualities = []string{"original", "high", "normal", "low", "aac"}

// ErrBusy is returned when every transcoding slot is taken.
var ErrBusy = errors.New("transcoding server busy")

// Cache transcodes a file once per profile and keeps the result on disk,
// so replays and seeks cost nothing. The first request streams the file
// while ffmpeg is still writing it; later requests get the finished file
// with Range support. Least recently used files are evicted past maxBytes.
type Cache struct {
	dir      string
	maxBytes int64
	ffmpeg   string
	sem      chan struct{}
	timeout  time.Duration
	poll     time.Duration

	mu   sync.Mutex
	jobs map[string]*job

	evictMu sync.Mutex
}

type job struct {
	tmp   string
	final string
	done  chan struct{}
	err   error
}

// NewCache creates the cache directory. maxTranscodes bounds concurrent ffmpeg runs.
func NewCache(dir string, maxBytes int64, ffmpegPath string, maxTranscodes int) (*Cache, error) {
	if maxTranscodes <= 0 {
		maxTranscodes = 4
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{
		dir: dir, maxBytes: maxBytes, ffmpeg: ffmpegPath,
		sem: make(chan struct{}, maxTranscodes), timeout: 15 * time.Minute, poll: 50 * time.Millisecond,
		jobs: map[string]*job{},
	}, nil
}

// Source is the audio file to transcode. Size and ModTime are part of the
// cache key, so an edited file is transcoded again.
type Source struct {
	Path    string
	Size    int64
	ModTime time.Time
	GainDB  float64
}

func (c *Cache) key(src Source, p Profile) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%s|%s|%.2f", src.Path, src.Size, src.ModTime.UnixNano(), p.Name, p.Bitrate, src.GainDB)))
	return hex.EncodeToString(h[:])
}

func (c *Cache) finalPath(key string, p Profile) string {
	return filepath.Join(c.dir, key[:2], key+p.Ext)
}

// Serve writes src transcoded to p.
func (c *Cache) Serve(w http.ResponseWriter, r *http.Request, src Source, p Profile) {
	key := c.key(src, p)
	final := c.finalPath(key, p)
	if f, err := os.Open(final); err == nil {
		defer f.Close()
		now := time.Now()
		_ = os.Chtimes(final, now, now) // LRU: a hit refreshes the file
		c.serveFile(w, r, f, p)
		return
	}

	j, err := c.start(key, final, src, p)
	if errors.Is(err, ErrBusy) {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		slog.Error("transcode start", "error", err, "path", src.Path)
		http.Error(w, "transcoding error", http.StatusInternalServerError)
		return
	}

	// A seek needs the whole file: wait for it.
	if rng := r.Header.Get("Range"); rng != "" && rng != "bytes=0-" {
		select {
		case <-j.done:
		case <-r.Context().Done():
			return
		}
		if j.err != nil {
			http.Error(w, "transcoding error", http.StatusInternalServerError)
			return
		}
		f, err := os.Open(final)
		if err != nil {
			http.Error(w, "transcoding error", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		c.serveFile(w, r, f, p)
		return
	}
	c.streamGrowing(w, r, j, p)
}

func (c *Cache) serveFile(w http.ResponseWriter, r *http.Request, f *os.File, p Profile) {
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "transcoding error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", p.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, "", stat.ModTime(), f)
}

// streamGrowing copies the file ffmpeg is writing until the job ends.
func (c *Cache) streamGrowing(w http.ResponseWriter, r *http.Request, j *job, p Profile) {
	f, err := os.Open(j.tmp)
	if err != nil {
		// The job finished (and renamed tmp) between start and open.
		<-j.done
		if f, err = os.Open(j.final); err != nil || j.err != nil {
			http.Error(w, "transcoding error", http.StatusInternalServerError)
			return
		}
	}
	defer f.Close()

	w.Header().Set("Content-Type", p.ContentType)
	w.Header().Set("Cache-Control", "private, no-store")
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 64<<10)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			continue
		}
		if readErr != nil && readErr != io.EOF {
			return
		}
		select {
		case <-j.done:
			if j.err != nil {
				return // the client sees a truncated stream
			}
			// Drain what was written after our last read.
			if _, err := io.Copy(w, f); err != nil {
				return
			}
			return
		case <-r.Context().Done():
			return
		case <-time.After(c.poll):
		}
	}
}

// start returns the running job for key, or starts one. The tmp file
// exists when start returns.
func (c *Cache) start(key, final string, src Source, p Profile) (*job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if j, ok := c.jobs[key]; ok {
		return j, nil
	}
	select {
	case c.sem <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		<-c.sem
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(final), key+".*.part")
	if err != nil {
		<-c.sem
		return nil, err
	}
	tmp.Close()
	j := &job{tmp: tmp.Name(), final: final, done: make(chan struct{})}
	c.jobs[key] = j

	go func() {
		defer func() { <-c.sem }()
		j.err = c.transcode(src, p, j.tmp)
		if j.err == nil {
			j.err = os.Rename(j.tmp, final)
		}
		if j.err != nil {
			slog.Error("transcode", "error", j.err, "path", src.Path, "quality", p.Name)
			_ = os.Remove(j.tmp)
		}
		if j.err == nil {
			c.evict()
		}
		c.mu.Lock()
		delete(c.jobs, key)
		c.mu.Unlock()
		close(j.done)
	}()
	return j, nil
}

func (c *Cache) transcode(src Source, p Profile, out string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", src.Path, "-map", "0:a:0", "-vn", "-map_metadata", "-1"}
	if src.GainDB != 0 {
		args = append(args, "-af", fmt.Sprintf("volume=%.2fdB", src.GainDB))
	}
	args = append(args, "-c:a", p.Encoder, "-b:a", p.Bitrate, "-f", p.Format, out)
	cmd := exec.CommandContext(ctx, c.ffmpeg, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// evict deletes least recently used files until the cache is below 90% of
// maxBytes. Running transcodes (.part files) are never deleted.
func (c *Cache) evict() {
	if c.maxBytes <= 0 {
		return
	}
	c.evictMu.Lock()
	defer c.evictMu.Unlock()
	type entry struct {
		path  string
		size  int64
		mtime time.Time
	}
	var files []entry
	var total int64
	_ = filepath.WalkDir(c.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".part") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, entry{path, info.Size(), info.ModTime()})
		total += info.Size()
		return nil
	})
	if total <= c.maxBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.Before(files[j].mtime) })
	target := c.maxBytes * 9 / 10
	for _, f := range files {
		if total <= target {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
