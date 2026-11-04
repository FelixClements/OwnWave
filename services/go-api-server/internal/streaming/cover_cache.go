package streaming

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ErrNoCover means the file has no embedded cover art (or no longer exists).
// It is a normal outcome, not a failure.
var ErrNoCover = errors.New("no cover art")

// defaultMaxMisses bounds remembered "no cover" files. A miss holds no image
// bytes (~100 B per entry), so the limit is much higher than for hits.
const defaultMaxMisses = 5000

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
type CoverCache struct {
	mu        sync.RWMutex
	cache     map[string]coverEntry
	hits      int // entries with data
	misses    int // entries without data
	sem       chan struct{}
	maxSize   int
	maxMisses int
	stat      func(string) (fs.FileInfo, error)
}

func NewCoverCache(maxEntries int, maxConcurrent int) *CoverCache {
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

	cmd := exec.CommandContext(ctx, ffmpegPath, "-i", filePath, "-an", "-vcodec", "mjpeg", "-f", "image2", "-", "-v", "0")
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
	return coverResult(data)
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
