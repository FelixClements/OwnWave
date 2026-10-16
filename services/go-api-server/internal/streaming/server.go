package streaming

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrPathOutsideMusicDir = errors.New("path outside music directory")

type Config struct {
	MusicDir                string
	FFmpegPath              string
	MaxConcurrentTranscodes int
}

type Server struct {
	musicDir     string
	ffmpegPath   string
	transcodeSem chan struct{}
}

func New(cfg Config) *Server {
	maxTranscodes := cfg.MaxConcurrentTranscodes
	if maxTranscodes <= 0 {
		maxTranscodes = 8
	}
	return &Server{
		musicDir:     cfg.MusicDir,
		ffmpegPath:   cfg.FFmpegPath,
		transcodeSem: make(chan struct{}, maxTranscodes),
	}
}

func (s *Server) ResolvePath(path string) (string, error) {
	musicDir := filepath.Clean(s.musicDir)
	if resolvedMusic, err := filepath.EvalSymlinks(musicDir); err == nil {
		musicDir = resolvedMusic
	}

	var resolved string
	if filepath.IsAbs(path) {
		resolved = filepath.Clean(path)
	} else {
		resolved = filepath.Clean(filepath.Join(musicDir, path))
	}
	if err := assertUnderMusicDir(musicDir, resolved); err != nil {
		return "", err
	}

	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return resolved, nil
		}
		return "", err
	}
	if err := assertUnderMusicDir(musicDir, realPath); err != nil {
		return "", err
	}
	return realPath, nil
}

func assertUnderMusicDir(musicDir, path string) error {
	rel, err := filepath.Rel(musicDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ErrPathOutsideMusicDir
	}
	return nil
}

func (s *Server) ServeFLAC(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file not found", 404)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		slog.Error("serve flac stat", "error", err, "path", path)
		http.Error(w, "internal streaming error", 500)
		return
	}

	w.Header().Set("Content-Type", "audio/flac")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, filepath.Base(path), stat.ModTime(), f)
}

func (s *Server) ServeTranscoded(w http.ResponseWriter, r *http.Request, path string, format string, loudness *float64, normalize bool) {
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

	select {
	case s.transcodeSem <- struct{}{}:
		defer func() { <-s.transcodeSem }()
	default:
		http.Error(w, "transcoding server busy", http.StatusServiceUnavailable)
		return
	}

	bitrate := r.URL.Query().Get("bitrate")
	if bitrate == "" {
		bitrate = defaultRate
	} else {
		bitrate = NormalizeBitrate(bitrate, defaultRate)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Transfer-Encoding", "chunked")

	gainDb := VolumeGainDb(loudness, normalize)

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", path,
		"-map", "0:a:0",
		"-vn",
		"-map_metadata", "-1",
	}
	if gainDb != 0 {
		args = append(args, "-af", fmt.Sprintf("volume=%.2fdB", gainDb))
	}
	args = append(args,
		"-c:a", encoder,
		"-b:a", bitrate,
		"-f", container,
		"-",
	)

	cmd := exec.CommandContext(r.Context(), s.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("transcoding stdout pipe", "error", err, "path", path)
		http.Error(w, "transcoding error", 500)
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Error("transcoding start", "error", err, "path", path)
		http.Error(w, "transcoding error", 500)
		return
	}

	_, _ = io.Copy(w, stdout)
	_ = stdout.Close()
	// A client disconnect cancels the context and kills ffmpeg; only log
	// failures the client didn't cause.
	if err := cmd.Wait(); err != nil && r.Context().Err() == nil {
		slog.Error("transcoding", "error", err, "path", path, "stderr", stderr.String())
	}
}
