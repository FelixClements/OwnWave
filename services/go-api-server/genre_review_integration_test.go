//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/stations"
)

// applyGenreV2Switch runs the pending genre v2 switch migration, as the
// release will (database/genre-v2/README.md).
func applyGenreV2Switch(f *stationsFixture) {
	f.t.Helper()
	sql, err := os.ReadFile("../../database/genre-v2/genre_v2_switch.up.sql")
	require.NoError(f.t, err)
	f.exec(string(sql))
}

func TestGenreReviewAfterTheSwitch(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	applyGenreV2Switch(f)
	stub := analytics.NewStub()
	f.h.analytics = stub

	artist := f.one(`INSERT INTO artists (name) VALUES ('Oxia') RETURNING id::text`)
	release := func(title string) string {
		return f.one(`INSERT INTO releases (folder, title, album_artist) VALUES ($1, $2, 'Oxia') RETURNING id::text`, "/m/"+title, title)
	}
	track := func(title, release string) string {
		return f.one(`INSERT INTO tracks (path, title, artist_id, duration_seconds, release_id, genre_tag)
			VALUES ($1, $2, $3::uuid, 200, NULLIF($4, '')::uuid, 'Techno') RETURNING id::text`, "/m/"+title, title, artist, release)
	}
	decide := func(track, genre, source string, confidence any, unsure, review, why string) {
		f.exec(`INSERT INTO track_genre_decisions
			(track_id, genre, source, confidence, unsure_reason, review_reason, fence, why, catalog_version)
			VALUES ($1::uuid, NULLIF($2, ''), $3, $4, NULLIF($5, ''), NULLIF($6, ''), ARRAY['house', 'techno'], $7::jsonb, 'test')`,
			track, genre, source, confidence, unsure, review, why)
	}
	albumA, albumB := release("Album A"), release("Album B")
	a1, a2, a3 := track("A1", albumA), track("A2", albumA), track("A3", albumA)
	b1 := track("B1", albumB)
	single := track("Single", "")
	f.refresh()
	decide(a1, "", "unsure", nil, "classifier_unsure", "", `{"classifier": {"genre": "house", "share": 0.5, "runner_ups": [["techno", 0.4]]}, "fence": ["house", "techno"]}`)
	decide(a2, "techno", "classifier", 0.65, "", "", `{"classifier": {"genre": "techno", "share": 0.65}}`)
	decide(a3, "techno", "strong_sources", 1, "", "", `{"agreeing": ["genre_tag"]}`)
	f.exec(`UPDATE track_genre_decisions SET style = 'acid' WHERE track_id = $1::uuid`, a3)
	decide(b1, "techno", "strong_sources", 1, "", "classifier_disagrees", `{"classifier": {"genre": "house", "share": 0.85}}`)
	decide(single, "techno", "classifier", 0.7, "", "", `{"classifier": {"genre": "techno", "share": 0.7}}`)
	f.exec(`INSERT INTO app_state (key, value) VALUES ('genre_test_score', '{"accuracy": 0.8, "labels": 3}')`)

	// The queue: grouped by release, ranked unsure → classifier vs tag →
	// least sure on a station. Strong-source songs are not on it.
	review := func(target string) stations.GenreReviewPage {
		rec := f.call(f.h.AdminGenreReview, admin, http.MethodGet, target, "", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decode[stations.GenreReviewPage](t, rec)
	}
	page := review("/r")
	require.True(t, page.Switched)
	require.Equal(t, 3, page.Total)
	require.Equal(t, 4, page.Songs)
	require.Len(t, page.Groups, 3)
	groupA := page.Groups[0]
	require.Equal(t, albumA, *groupA.ReleaseID)
	require.Equal(t, "Album A", *groupA.ReleaseTitle)
	require.Equal(t, "Oxia", *groupA.ReleaseArtist)
	require.Equal(t, 2, groupA.Priority)
	require.Len(t, groupA.Songs, 2)
	require.Equal(t, a1, groupA.Songs[0].TrackID)
	require.Nil(t, groupA.Songs[0].Genre, "unsure")
	require.Equal(t, "classifier_unsure", *groupA.Songs[0].UnsureReason)
	require.Equal(t, "house", *groupA.Songs[0].Suggestion)
	require.Equal(t, a2, groupA.Songs[1].TrackID)
	require.Nil(t, groupA.Songs[1].Suggestion, "the classifier picked it already")
	require.Equal(t, b1, page.Groups[1].Songs[0].TrackID)
	require.Equal(t, "classifier_disagrees", *page.Groups[1].Songs[0].ReviewReason)
	require.Equal(t, "house", *page.Groups[1].Songs[0].Suggestion)
	require.Nil(t, page.Groups[2].ReleaseID, "a song without a release is a group of its own")
	require.Equal(t, single, page.Groups[2].Songs[0].TrackID)
	second := review("/r?offset=1&limit=1")
	require.Len(t, second.Groups, 1)
	require.Equal(t, b1, second.Groups[0].Songs[0].TrackID)

	// The "why" breakdown of one song.
	rec := f.call(f.h.AdminGenreDecision, admin, http.MethodGet, "/d", a1, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	decision := decode[stations.GenreDecision](t, rec)
	require.Equal(t, "unsure", decision.Source)
	require.Equal(t, []string{"house", "techno"}, *decision.Fence)
	require.Equal(t, "Techno", *decision.GenreTag)
	require.Equal(t, "Album A", *decision.ReleaseTitle)
	var why struct {
		Classifier struct {
			Genre string `json:"genre"`
		} `json:"classifier"`
	}
	require.NoError(t, json.Unmarshal(decision.Why, &why))
	require.Equal(t, "house", why.Classifier.Genre)
	require.Equal(t, http.StatusNotFound, f.call(f.h.AdminGenreDecision, admin, http.MethodGet, "/d", albumA, "").Code)

	// Overview: songs per genre and style, unsure, flagged, the test score.
	rec = f.call(f.h.AdminGenreOverview, admin, http.MethodGet, "/o", "", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	o := decode[stations.GenreOverview](t, rec)
	require.True(t, o.Switched)
	require.Equal(t, 5, o.Songs)
	require.Equal(t, 4, o.Decided)
	require.Equal(t, 1, *o.Unsure)
	require.Equal(t, 1, *o.Flagged)
	require.Equal(t, 1, *o.WithStyle)
	require.Equal(t, 4, *o.ToReview)
	require.Equal(t, map[string]int{"unsure": 1, "classifier": 2, "strong_sources": 2}, o.BySource)
	require.JSONEq(t, `{"accuracy": 0.8, "labels": 3}`, string(o.TestScore))
	require.NotNil(t, o.TestScoreAt)
	var techno *stations.GenreCatalogEntry
	for i := range o.Genres {
		if o.Genres[i].ID == "techno" {
			techno = &o.Genres[i]
		}
		require.NotEqual(t, "acid", o.Genres[i].ID, "styles sit under their genre")
	}
	require.NotNil(t, techno)
	require.Equal(t, 4, techno.Songs)
	var acid int
	for _, s := range techno.Styles {
		if s.ID == "acid" {
			acid = s.Songs
		}
	}
	require.Equal(t, 1, acid)

	// Corrections: song, release or artist scope, with a style; each one
	// queues a rescore so it becomes a trusted label.
	type saved struct {
		Corrections []stations.GenreCorrection `json:"corrections"`
		SavedID     string                     `json:"saved_id"`
	}
	save := func(body string) (int, saved) {
		rec := f.call(f.h.AdminSaveGenreCorrection, admin, http.MethodPost, "/c", "", body)
		if rec.Code != http.StatusOK {
			return rec.Code, saved{}
		}
		return rec.Code, decode[saved](t, rec)
	}
	code, _ := save(`{"track_id":"` + a1 + `","genre":"house","style":"acid"}`)
	require.Equal(t, http.StatusBadRequest, code, "acid is a techno style")
	code, _ = save(`{"release_id":"` + a1 + `","genre":"techno"}`)
	require.Equal(t, http.StatusBadRequest, code, "unknown release")
	code, _ = save(`{"release_id":"` + albumA + `","artist":"Oxia","genre":"techno"}`)
	require.Equal(t, http.StatusBadRequest, code, "one scope only")
	code, _ = save(`{"track_id":"` + a1 + `","genre":"dance"}`)
	require.Equal(t, http.StatusBadRequest, code, "v1 ids are gone after the switch")
	require.Empty(t, stub.Calls)

	code, got := save(`{"release_id":"` + albumA + `","genre":"house"}`)
	require.Equal(t, http.StatusOK, code)
	code, got = save(`{"release_id":"` + albumA + `","genre":"techno","style":"acid"}`)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, got.Corrections, 1, "saving a release again replaces it")
	c := got.Corrections[0]
	require.Equal(t, got.SavedID, c.ID)
	require.Equal(t, "release", c.Scope)
	require.Equal(t, albumA, *c.ReleaseID)
	require.Equal(t, "Album A", *c.ReleaseTitle)
	require.Equal(t, "techno", c.Genre)
	require.Equal(t, "acid", *c.Style)
	require.Equal(t, "acid", f.one(`SELECT genre FROM genre_corrections WHERE release_id = $1::uuid`, albumA), "a style is stored by its id")
	code, got = save(`{"track_id":"` + a1 + `","genre":"house"}`)
	require.Equal(t, http.StatusOK, code)
	songFix := got.SavedID
	code, _ = save(`{"artist":"Oxia","genre":"techno"}`)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, []string{"RebuildGenres", "RebuildGenres", "RebuildGenres", "RebuildGenres"}, stub.Calls)

	// Search.
	search := func(q string) []stations.GenreCorrection {
		rec := f.call(f.h.AdminGenreCorrections, admin, http.MethodGet, "/c?q="+q, "", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decode[saved](t, rec).Corrections
	}
	require.Len(t, search(""), 3)
	require.Len(t, search("album%20a"), 1)
	require.Len(t, search("acid"), 1)
	require.Len(t, search("disco"), 1, "by genre name (House & Disco)")
	require.Len(t, search("oxia"), 3, "song artist, release artist and artist")
	require.Empty(t, search("%25"), "% matches literally")

	// Undo.
	rec = f.call(f.h.AdminDeleteGenreCorrection, admin, http.MethodDelete, "/c", songFix, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, decode[saved](t, rec).Corrections, 2)
	require.Len(t, stub.Calls, 5, "undo rescores too")
}
