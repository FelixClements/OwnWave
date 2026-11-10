package streaming

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ErrNoCover means the file has no embedded cover art (or no longer exists).
// It is a normal outcome, not a failure.
var ErrNoCover = errors.New("no cover art")

// defaultMaxMisses bounds remembered "no cover" files. A miss holds no image
// bytes (~100 B per entry), so the limit is much higher than for hits.
const defaultMaxMisses = 5000

// coverMaxPx bounds a cover's width and height. Embedded art is often 1000-3000
// px and over 1 MB; the largest place the UI shows a cover is the full-screen
// player, and the OS media session asks for 512x512.
const coverMaxPx = 512

// coverVersion is part of every on-disk key. Change it whenever the extraction
// output changes (size, quality), so old files are no longer read.
const coverVersion = "v1-512"

// coverEntry is a cached result for one file. data == nil records a miss.
// The entry is valid only while the file's mtime and size are unchanged, so
// re-tagging a file (adding or replacing art) invalidates it on the next
// request.
type coverEntry struct {
	modTime time.Time
	size    int64
	data    []byte
}

func (e coverEntry) matches(info fs.FileInfo) bool {
	return e.size == info.Size() && e.modTime.Equal(info.ModTime())
}

// CoverCache caches extracted cover art bytes, and files without cover art,
// and limits concurrent ffmpeg extraction processes.
//
// Results live in memory and, when dir is set, on disk, so they survive
// restarts and a song on slow storage is read for its cover only once per
// file version. Disk files are keyed by path, size and mtime; a re-tagged file
// gets a new key and its old file is left behind. Covers are small thumbnails,
// so the dir is not size-capped; deleting it only costs re-extraction.
type CoverCache struct {
	mu        sync.RWMutex
	cache     map[string]coverEntry
	hits      int // entries with data
	misses    int // entries without data
	sem       chan struct{}
	maxSize   int
	maxMisses int
	stat      func(string) (fs.FileInfo, error)
	dir       string // "" = memory only
}

// NewCoverCache returns a cache that also persists covers under dir, if dir is
// non-empty. Disk errors are logged and fall back to extraction.
func NewCoverCache(dir string, maxEntries int, maxConcurrent int) *CoverCache {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &CoverCache{
		cache:     make(map[string]coverEntry),
		sem:       make(chan struct{}, maxConcurrent),
		maxSize:   maxEntries,
		maxMisses: defaultMaxMisses,
		stat:      os.Stat,
		dir:       dir,
	}
}

// GetOrExtract returns the embedded cover art of filePath. It returns
// ErrNoCover when the file has none (cached, so ffmpeg runs once per file
// version) or does not exist; any other error is a real failure, such as a
// cancelled context or a missing ffmpeg binary, and is never cached.
func (c *CoverCache) GetOrExtract(ctx context.Context, ffmpegPath, filePath string) ([]byte, error) {
	info, err := c.stat(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoCover
	}
	if err != nil {
		return nil, err
	}

	if data, ok := c.lookup(filePath, info); ok {
		return coverResult(data)
	}
	if data, ok := c.readDisk(filePath, info); ok {
		c.store(filePath, coverEntry{modTime: info.ModTime(), size: info.Size(), data: data})
		return coverResult(data)
	}

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Double check cache after acquiring semaphore slot
	if data, ok := c.lookup(filePath, info); ok {
		return coverResult(data)
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-i", filePath, "-an",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease", coverMaxPx, coverMaxPx),
		"-frames:v", "1", "-q:v", "4",
		"-vcodec", "mjpeg", "-f", "image2", "-", "-v", "0")
	var out bytes.Buffer
	cmd.Stdout = &out
	runErr := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		// Killed or never finished: says nothing about the file.
		return nil, ctxErr
	}
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		// ffmpeg did not run (not found, not executable, ...).
		return nil, runErr
	}

	// ffmpeg ran. A non-zero exit (e.g. "no video stream") or empty output
	// both mean the file has no usable cover art.
	var data []byte
	if runErr == nil && out.Len() > 0 {
		data = out.Bytes()
	}
	c.store(filePath, coverEntry{modTime: info.ModTime(), size: info.Size(), data: data})
	c.writeDisk(filePath, info, data)
	return coverResult(data)
}

// diskKey returns the on-disk path prefix for one version of filePath.
// "<key>.jpg" holds a cover; an empty "<key>.none" records a file without one.
func (c *CoverCache) diskKey(filePath string, info fs.FileInfo) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", coverVersion, filePath, info.Size(), info.ModTime().UnixNano())))
	key := hex.EncodeToString(sum[:16])
	return filepath.Join(c.dir, key[:2], key)
}

// readDisk returns a persisted result: (data, true) for a cover, (nil, true)
// for a known miss, (nil, false) when nothing is stored.
func (c *CoverCache) readDisk(filePath string, info fs.FileInfo) ([]byte, bool) {
	if c.dir == "" {
		return nil, false
	}
	key := c.diskKey(filePath, info)
	data, err := os.ReadFile(key + ".jpg")
	if err == nil && len(data) > 0 {
		return data, true
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("read cached cover", "path", key+".jpg", "error", err)
	}
	if _, err := os.Stat(key + ".none"); err == nil {
		return nil, true
	}
	return nil, false
}

// writeDisk persists a result. It writes a temp file and renames it, so a
// concurrent reader never sees a partial cover. Failures are only logged.
func (c *CoverCache) writeDisk(filePath string, info fs.FileInfo, data []byte) {
	if c.dir == "" {
		return
	}
	key := c.diskKey(filePath, info)
	target := key + ".none"
	if data != nil {
		target = key + ".jpg"
	}
	if err := writeFileAtomic(target, data); err != nil {
		slog.Warn("cache cover on disk", "path", target, "error", err)
	}
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func coverResult(data []byte) ([]byte, error) {
	if data == nil {
		return nil, ErrNoCover
	}
	return data, nil
}

func (c *CoverCache) lookup(filePath string, info fs.FileInfo) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.cache[filePath]
	if !ok || !e.matches(info) {
		return nil, false
	}
	return e.data, true
}

// store records e, replacing any stale entry for the same file. It is dropped
// once its kind (hit or miss) has reached its limit.
func (c *CoverCache) store(filePath string, e coverEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.cache[filePath]; ok {
		delete(c.cache, filePath)
		c.count(old, -1)
	}
	if !c.hasRoom(e) {
		return
	}
	c.cache[filePath] = e
	c.count(e, +1)
}

func (c *CoverCache) hasRoom(e coverEntry) bool {
	if e.data == nil {
		return c.misses < c.maxMisses
	}
	return c.hits < c.maxSize
}

func (c *CoverCache) count(e coverEntry, delta int) {
	if e.data == nil {
		c.misses += delta
	} else {
		c.hits += delta
	}
}
