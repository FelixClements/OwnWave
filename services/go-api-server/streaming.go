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

func (h *Handler) serveCrossfaded(w http.ResponseWriter, r *http.Request, queue []TrackWithFeatures, format, bitrate string) {
	if len(queue) == 1 {
		fullPath := queue[0].Path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(h.musicDir, fullPath)
		}
		if format == "flac" {
			h.serveFLAC(w, r, fullPath)
		} else {
			h.serveTranscoded(w, r, fullPath, format)
		}
		return
	}

	const defaultCrossfade = 5.0

	outros := make([]float64, len(queue))
	ends := make([]float64, len(queue))
	crossfades := make([]float64, len(queue)-1)

	for i, q := range queue {
		d := q.IdealCrossfadeSeconds
		if d <= 0 {
			d = defaultCrossfade
		}
		if i < len(queue)-1 {
			crossfades[i] = d
		}

		outro := q.OutroStartSeconds
		if outro <= 0 && q.DurationSeconds != nil && *q.DurationSeconds > 0 {
			outro = *q.DurationSeconds - d
			if outro < 0 {
				outro = 0
			}
		}
		if outro < 0 {
			outro = 0
		}
		outros[i] = outro

		if i == len(queue)-1 {
			if q.DurationSeconds != nil && *q.DurationSeconds > 0 {
				ends[i] = *q.DurationSeconds
			}
		} else {
			ends[i] = outro + d
		}
	}

	for i := len(queue) - 2; i >= 0; i-- {
		if ends[i+1] > 0 && crossfades[i] > ends[i+1] {
			crossfades[i] = ends[i+1]
			ends[i] = outros[i] + crossfades[i]
		}
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	for i, q := range queue {
		fullPath := q.Path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(h.musicDir, fullPath)
		}
		if ends[i] > 0 {
			args = append(args, "-t", fmt.Sprintf("%f", ends[i]))
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
	w.Header().Set("Transfer-Encoding", "chunked")

	if format != "flac" {
		if bitrate == "" {
			bitrate = defaultRate
		} else {
			bitrate = normalizeBitrate(bitrate, defaultRate)
		}
	}

	filter := &strings.Builder{}
	for i := 0; i < len(queue)-1; i++ {
		d := crossfades[i]
		if i == 0 {
			fmt.Fprintf(filter, "[%d:a][%d:a]acrossfade=d=%f:c1=tri:c2=tri", i, i+1, d)
		} else {
			fmt.Fprintf(filter, ";[a%d][%d:a]acrossfade=d=%f:c1=tri:c2=tri", i, i+1, d)
		}
		if i == len(queue)-2 {
			fmt.Fprint(filter, "[out]")
		} else {
			fmt.Fprintf(filter, "[a%d]", i+1)
		}
	}

	args = append(args, "-filter_complex", filter.String(), "-map", "[out]")
	if format == "flac" {
		args = append(args, "-f", "flac", "-compression_level", "5", "-")
	} else {
		args = append(args, "-c:a", encoder, "-b:a", bitrate, "-f", container, "-")
	}

	cmd := exec.Command(h.ffmpegPath, args...)
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
