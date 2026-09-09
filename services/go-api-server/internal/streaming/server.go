package streaming

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ownwave/api/internal/playback"
)

var ErrPathOutsideMusicDir = errors.New("path outside music directory")

const maxCrossfadeTracks = 20

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

	defer func() {
		_ = stdout.Close()
		_ = cmd.Wait()
	}()

	_, _ = io.Copy(w, stdout)
}

func (s *Server) ServeCrossfaded(w http.ResponseWriter, r *http.Request, queue []playback.TrackWithFeatures, format, bitrate string, gapless, normalize bool) {
	if len(queue) == 0 {
		http.Error(w, "empty queue", http.StatusBadRequest)
		return
	}
	if len(queue) > maxCrossfadeTracks {
		queue = queue[:maxCrossfadeTracks]
	}
	if len(queue) == 1 {
		fullPath, err := s.ResolvePath(queue[0].Path)
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		if format == "flac" {
			s.ServeFLAC(w, r, fullPath)
		} else {
			s.ServeTranscoded(w, r, fullPath, format, queue[0].Loudness, normalize)
		}
		return
	}

	select {
	case s.transcodeSem <- struct{}{}:
		defer func() { <-s.transcodeSem }()
	default:
		http.Error(w, "transcoding server busy", http.StatusServiceUnavailable)
		return
	}

	const defaultCrossfade = 5.0

	intros := make([]float64, len(queue))
	outroStarts := make([]float64, len(queue))
	outroEnds := make([]float64, len(queue))
	ends := make([]float64, len(queue))
	crossfades := make([]float64, len(queue)-1)
	gains := make([]float64, len(queue))

	for i, q := range queue {
		duration := 0.0
		if q.DurationSeconds != nil && *q.DurationSeconds > 0 {
			duration = *q.DurationSeconds
		}

		intro := q.IntroStartSeconds
		if intro < 0 {
			intro = 0
		}
		if intro > duration {
			intro = 0
		}

		outroEnd := q.OutroEndSeconds
		if outroEnd <= 0 || outroEnd > duration {
			outroEnd = duration
		}
		if outroEnd < intro {
			outroEnd = duration
		}

		outroStart := q.OutroStartSeconds
		if outroStart <= intro || outroStart <= 0 || outroStart >= outroEnd {
			outroStart = outroEnd - defaultCrossfade
			if outroStart < intro {
				outroStart = intro
			}
		}

		intros[i] = intro
		outroStarts[i] = outroStart
		outroEnds[i] = outroEnd
		gains[i] = VolumeGainDb(q.Loudness, normalize)
	}

	if gapless {
		for i := range queue {
			ends[i] = outroEnds[i]
		}
	} else {
		for i := len(queue) - 2; i >= 0; i-- {
			d := queue[i].IdealCrossfadeSeconds
			if d <= 0 {
				d = defaultCrossfade
			}
			thisOutro := outroEnds[i] - outroStarts[i]
			nextLen := ends[i+1] - intros[i+1]
			nextOutro := outroEnds[i+1] - intros[i+1]
			if nextOutro > 0 && d > nextOutro {
				d = nextOutro
			}
			if nextLen > 0 && d > nextLen {
				d = nextLen
			}
			if thisOutro > 0 && d > thisOutro {
				d = thisOutro
			}
			crossfades[i] = d
			ends[i] = outroStarts[i] + d
			if ends[i] > outroEnds[i] {
				ends[i] = outroEnds[i]
			}
		}
		ends[len(queue)-1] = outroEnds[len(queue)-1]
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	for i, q := range queue {
		fullPath, err := s.ResolvePath(q.Path)
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		if intros[i] > 0 {
			args = append(args, "-ss", fmt.Sprintf("%f", intros[i]))
		}
		if ends[i] > 0 {
			args = append(args, "-to", fmt.Sprintf("%f", ends[i]))
		}
		args = append(args, "-i", fullPath)
	}

	var contentType, encoder, container, defaultRate string
	switch format {
	case "mp3":
		encoder = "libmp3lame"
		container = "mp3"
		contentType = "audio/mpeg"
		defaultRate = "320k"
	default:
		contentType = "audio/flac"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Transfer-Encoding", "chunked")

	if format != "flac" {
		if bitrate == "" {
			bitrate = defaultRate
		} else {
			bitrate = NormalizeBitrate(bitrate, defaultRate)
		}
	}

	filter := &strings.Builder{}
	for i := range queue {
		if i > 0 {
			fmt.Fprint(filter, ";")
		}
		fmt.Fprintf(filter, "[%d:a]volume=%.2fdB[v%d]", i, gains[i], i)
	}
	if gapless {
		for i := range queue {
			if i > 0 {
				fmt.Fprint(filter, ";")
			}
			fmt.Fprintf(filter, "[v%d]", i)
		}
		fmt.Fprintf(filter, "concat=n=%d:v=0:a=1[out]", len(queue))
	} else {
		for i := 0; i < len(queue)-1; i++ {
			d := crossfades[i]
			if i == 0 {
				fmt.Fprintf(filter, ";[v%d][v%d]acrossfade=d=%f:c1=tri:c2=tri", i, i+1, d)
			} else {
				fmt.Fprintf(filter, ";[a%d][v%d]acrossfade=d=%f:c1=tri:c2=tri", i, i+1, d)
			}
			if i == len(queue)-2 {
				fmt.Fprint(filter, "[out]")
			} else {
				fmt.Fprintf(filter, "[a%d]", i+1)
			}
		}
	}

	args = append(args, "-filter_complex", filter.String(), "-map", "[out]")
	if format == "flac" {
		args = append(args, "-f", "flac", "-compression_level", "5", "-")
	} else {
		args = append(args, "-c:a", encoder, "-b:a", bitrate, "-f", container, "-")
	}

	cmd := exec.CommandContext(r.Context(), s.ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("crossfade stdout pipe", "error", err)
		http.Error(w, "streaming error", 500)
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Error("crossfade start", "error", err)
		http.Error(w, "streaming error", 500)
		return
	}
	defer func() {
		_ = stdout.Close()
		_ = cmd.Wait()
	}()
	_, _ = io.Copy(w, stdout)
}
