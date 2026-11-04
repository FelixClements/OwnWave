//go:build integration

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/stations"
	"ownwave/api/internal/streaming"
	"ownwave/api/internal/testffmpeg"
)

// useFakeCovers points the handler at a fake ffmpeg and a temp music dir,
// and moves every track's file there, each starting without cover art.
func useFakeCovers(f *stationsFixture) (*testffmpeg.Fake, string) {
	t := f.t
	ff := testffmpeg.New(t)
	music := t.TempDir()
	f.h.ffmpegPath = ff.Path
	f.h.stream = streaming.New(streaming.Config{MusicDir: music, FFmpegPath: ff.Path})
	f.h.coverCache = streaming.NewCoverCache(10, 2)
	f.exec(`UPDATE tracks SET path = replace(path, '/m/', $1)`, music+"/")
	for _, p := range strings.Fields(f.one(`SELECT string_agg(path, ' ') FROM tracks`)) {
		testffmpeg.WriteFile(t, p, "no art")
	}
	return ff, music
}

// retag gives a file new contents and a newer mtime, like a tag editor.
func retag(t *testing.T, path, contents string) {
	testffmpeg.WriteFile(t, path, contents)
	later := time.Now().Add(time.Minute)
	require.NoError(t, os.Chtimes(path, later, later))
}

func TestTrackCover(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	f.playlist("rock", 2, "")
	ff, music := useFakeCovers(f)
	withArt := f.one(`SELECT id::text FROM tracks WHERE path = $1`, filepath.Join(music, "rock/0"))
	noArt := f.one(`SELECT id::text FROM tracks WHERE path = $1`, filepath.Join(music, "rock/1"))
	retag(t, filepath.Join(music, "rock/0"), "cover")

	cover := func(id string) *httptest.ResponseRecorder {
		return f.call(f.h.GetTrackCover, admin, http.MethodGet, "/api/tracks/"+id+"/cover", id, "")
	}

	t.Run("with art", func(t *testing.T) {
		rec := cover(withArt)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
		require.Equal(t, "private, max-age=86400", rec.Header().Get("Cache-Control"))
		require.Equal(t, testffmpeg.JPEG, rec.Body.String())
	})

	t.Run("without art", func(t *testing.T) {
		before := ff.Runs(t)
		for i := 0; i < 3; i++ {
			rec := cover(noArt)
			require.Equal(t, http.StatusNoContent, rec.Code)
			require.Equal(t, "private, max-age=86400", rec.Header().Get("Cache-Control"))
			require.Empty(t, rec.Body.String())
		}
		require.Equal(t, before+1, ff.Runs(t), "miss is cached: ffmpeg runs once")
	})

	t.Run("audio file missing", func(t *testing.T) {
		require.NoError(t, os.Remove(filepath.Join(music, "rock/1")))
		require.Equal(t, http.StatusNoContent, cover(noArt).Code)
	})

	t.Run("unknown track", func(t *testing.T) {
		rec := cover("6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55")
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid id", func(t *testing.T) {
		require.Equal(t, http.StatusBadRequest, cover("nope").Code)
	})

	t.Run("path outside music dir is a server error", func(t *testing.T) {
		id := f.one(`INSERT INTO tracks (path, title) VALUES ('/etc/passwd', 'x') RETURNING id::text`)
		require.Equal(t, http.StatusInternalServerError, cover(id).Code)
	})
}

func TestStationCover(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	rock := f.playlist("rock", 3, "techno")
	_, music := useFakeCovers(f)

	rec := f.call(f.h.CreateStation, admin, http.MethodPost, "/stations", "", `{"name":"Techno","playlist_ids":["`+rock+`"],"genres":["techno"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	st := decode[stations.Station](t, rec)
	cover := func(id string) *httptest.ResponseRecorder {
		return f.call(f.h.GetStationCover, admin, http.MethodGet, "/api/stations/"+id+"/cover", id, "")
	}

	t.Run("no track has art", func(t *testing.T) {
		rec := cover(st.ID)
		require.Equal(t, http.StatusNoContent, rec.Code)
		require.Equal(t, "private, max-age=86400", rec.Header().Get("Cache-Control"))
		require.Empty(t, rec.Body.String())
	})

	t.Run("one track has art", func(t *testing.T) {
		retag(t, filepath.Join(music, "rock/2"), "cover")
		rec := cover(st.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
		require.Equal(t, testffmpeg.JPEG, rec.Body.String())
	})

	t.Run("unknown station", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, cover("6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55").Code)
	})
}

func TestPlaylistCover(t *testing.T) {
	f := newStationsFixture(t)
	user := f.user("listener", false)
	id := f.playlist("rock", 1, "")
	cover := func(id string) *httptest.ResponseRecorder {
		return f.call(f.h.GetPlaylistCover, user, http.MethodGet, "/api/playlists/"+id+"/cover", id, "")
	}

	rec := cover(id)
	require.Equal(t, http.StatusNoContent, rec.Code, "not synced since covers were added")
	require.Empty(t, rec.Body.String())

	f.exec(`UPDATE playlists SET cover_url = 'https://i.scdn.co/image/a', cover_image = 'png!', cover_type = 'image/png' WHERE id = $1::uuid`, id)
	rec = cover(id)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Equal(t, "private, max-age=3600", rec.Header().Get("Cache-Control"))
	require.Equal(t, "png!", rec.Body.String())

	require.Equal(t, http.StatusNotFound, cover("00000000-0000-0000-0000-000000000000").Code)
}
