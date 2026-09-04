package streaming

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
)

// CoverCache caches extracted cover art bytes and limits concurrent ffmpeg extraction processes.
type CoverCache struct {
	mu      sync.RWMutex
	cache   map[string][]byte
	sem     chan struct{}
	maxSize int
}

func NewCoverCache(maxEntries int, maxConcurrent int) *CoverCache {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &CoverCache{
		cache:   make(map[string][]byte),
		sem:     make(chan struct{}, maxConcurrent),
		maxSize: maxEntries,
	}
}

func (c *CoverCache) GetOrExtract(ctx context.Context, ffmpegPath, filePath string) ([]byte, error) {
	c.mu.RLock()
	if data, ok := c.cache[filePath]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Double check cache after acquiring semaphore slot
	c.mu.RLock()
	if data, ok := c.cache[filePath]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	cmd := exec.CommandContext(ctx, ffmpegPath, "-i", filePath, "-an", "-vcodec", "mjpeg", "-f", "image2", "-", "-v", "0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || out.Len() == 0 {
		return nil, errors.New("no cover art")
	}

	data := out.Bytes()
	c.mu.Lock()
	if len(c.cache) < c.maxSize {
		c.cache[filePath] = data
	}
	c.mu.Unlock()

	return data, nil
}
