package main

import (
	"net/http"
	"os"
	"os/exec"
	"strings"

	"ownwave/api/internal/streaming"
)

// StreamTrack streams a track.
//
// ?quality=original|high|normal|low|aac picks a quality (the transcoded
// ones are Opus 160/96/64 kbps and AAC 128 kbps, cached on disk after the
// first play). Without quality, the older ?format=flac|mp3|opus|aac
// (&bitrate=) live transcoding applies; the default is the original file.
// ?normalize=false turns off loudness normalisation.
func (h *Handler) StreamTrack(w http.ResponseWriter, r *http.Request) {
	trackID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	quality := strings.ToLower(q.Get("quality"))
	format := strings.ToLower(q.Get("format"))
	if quality == "" && (format == "" || format == "flac") {
		quality = "original"
	}
	profile, transcode := streaming.Profiles[quality]
	if quality != "" && quality != "original" && !transcode {
		http.Error(w, "quality must be one of "+strings.Join(streaming.Qualities, ", "), http.StatusBadRequest)
		return
	}

	track, err := h.db.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "track not found", http.StatusNotFound)
		return
	}
	fullPath, err := h.stream.ResolvePath(track.Path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if quality == "original" {
		h.stream.ServeFLAC(w, r, fullPath)
		return
	}
	if _, err := exec.LookPath(h.ffmpegPath); err != nil {
		http.Error(w, "ffmpeg not available", http.StatusInternalServerError)
		return
	}
	normalize := q.Get("normalize") != "false"

	if transcode && h.transcodes != nil {
		stat, err := os.Stat(fullPath)
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		h.transcodes.Serve(w, r, streaming.Source{
			Path: fullPath, Size: stat.Size(), ModTime: stat.ModTime(),
			GainDB: streaming.VolumeGainDb(track.Loudness, normalize),
		}, profile)
		return
	}
	switch format {
	case "mp3", "opus", "aac":
		h.stream.ServeTranscoded(w, r, fullPath, format, track.Loudness, normalize)
	case "":
		// quality given but no cache configured: live-transcode the profile's codec.
		r.URL.RawQuery = "bitrate=" + profile.Bitrate
		codec := "opus"
		if quality == "aac" {
			codec = "aac"
		}
		h.stream.ServeTranscoded(w, r, fullPath, codec, track.Loudness, normalize)
	default:
		http.Error(w, "unsupported format", http.StatusBadRequest)
	}
}
