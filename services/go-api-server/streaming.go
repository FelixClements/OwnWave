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

func (h *Handler) serveTranscoded(w http.ResponseWriter, r *http.Request, path string, format string, loudness *float64, normalize bool) {
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

	gainDb := volumeGainDb(loudness, normalize)

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

const (
	targetLoudness = -14.0
	maxGainDb      = 20.0
	minGainDb      = -20.0
)

func volumeGainDb(loudness *float64, normalize bool) float64 {
	if !normalize || loudness == nil || *loudness == 0 {
		return 0
	}
	gain := targetLoudness - *loudness
	if gain > maxGainDb {
		return maxGainDb
	}
	if gain < minGainDb {
		return minGainDb
	}
	return gain
}

func (h *Handler) serveCrossfaded(w http.ResponseWriter, r *http.Request, queue []TrackWithFeatures, format, bitrate string, gapless, normalize bool) {
	if len(queue) == 1 {
		fullPath := queue[0].Path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(h.musicDir, fullPath)
		}
		if format == "flac" {
			h.serveFLAC(w, r, fullPath)
		} else {
			h.serveTranscoded(w, r, fullPath, format, queue[0].Loudness, normalize)
		}
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
		gains[i] = volumeGainDb(q.Loudness, normalize)
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
		fullPath := q.Path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(h.musicDir, fullPath)
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
	w.Header().Set("Transfer-Encoding", "chunked")

	if format != "flac" {
		if bitrate == "" {
			bitrate = defaultRate
		} else {
			bitrate = normalizeBitrate(bitrate, defaultRate)
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
