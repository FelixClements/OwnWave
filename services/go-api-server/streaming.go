package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func (h *Handler) serveMP3(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Transfer-Encoding", "chunked")

	cmd := exec.Command(
		h.ffmpegPath,
		"-i", path,
		"-map_metadata", "-1",
		"-c:a", "libmp3lame",
		"-b:a", "320k",
		"-f", "mp3",
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
