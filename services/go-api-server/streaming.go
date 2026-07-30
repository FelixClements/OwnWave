package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (h *Handler) serveFLAC(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file not found", 404)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "audio/flac")
	http.ServeContent(w, r, filepath.Base(path), stat.ModTime(), f)
}

func (h *Handler) serveTranscoded(w http.ResponseWriter, r *http.Request, path string, format string) {
	format = strings.ToLower(format)

	var (
		encoder     string
		container   string
		contentType string
		defaultRate string
	)

	switch format {
	case "mp3":
		encoder = "libmp3lame"
		container = "mp3"
		contentType = "audio/mpeg"
		defaultRate = "320k"
	case "opus":
		encoder = "libopus"
		container = "opus"
		contentType = "audio/ogg"
		defaultRate = "192k"
	case "aac":
		encoder = "aac"
		container = "adts"
		contentType = "audio/aac"
		defaultRate = "192k"
	default:
		http.Error(w, "unsupported format", 400)
		return
	}

	bitrate := r.URL.Query().Get("bitrate")
	if bitrate == "" {
		bitrate = defaultRate
	} else {
		bitrate = normalizeBitrate(bitrate, defaultRate)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Transfer-Encoding", "chunked")

	cmd := exec.Command(
		h.ffmpegPath,
		"-i", path,
		"-map_metadata", "-1",
		"-c:a", encoder,
		"-b:a", bitrate,
		"-f", container,
		"-",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defer func() {
		_ = stdout.Close()
		_ = cmd.Wait()
	}()

	_, _ = io.Copy(w, stdout)
}

func normalizeBitrate(input, defaultRate string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if strings.HasSuffix(input, "k") {
		input = strings.TrimSuffix(input, "k")
	}
	kbps, err := strconv.Atoi(input)
	if err != nil || kbps <= 0 {
		return defaultRate
	}
	return fmt.Sprintf("%dk", kbps)
}
