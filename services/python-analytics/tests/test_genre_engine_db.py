"""The genre engine v2 against a database: loading sources, storing
decisions, the review queue, and the one switch (ADR 0008)."""

import numpy as np
import psycopg
import pytest
from psycopg.types.json import Jsonb

import genre_catalog as gc
import genre_engine as ge
import genre_scoring

DIMS = 1280


def vec(i: int, j: int = None, mix: float = 0.3) -> str:
    v = np.zeros(DIMS)
    v[i] = 1.0
    if j is not None:
        v[j] = mix
    return "[" + ",".join(f"{x:g}" for x in v) + "]"


class Library:
    def __init__(self, conn):
        self.conn = conn

    def artist(self, name):
        return self.conn.execute(
            "INSERT INTO artists (name) VALUES (%s) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id",
            (name,),
        ).fetchone()[0]

    def release(self, title, mb_release_id=None, group=None, compilation=False):
        return self.conn.execute(
            """
            INSERT INTO releases (folder, title, mb_release_id, mb_release_group_id, is_compilation)
            VALUES (%s, %s, %s, %s, %s) RETURNING id
            """,
            (f"/m/{title}", title, mb_release_id, group, compilation),
        ).fetchone()[0]

    def track(self, name, tag=None, artist=None, release=None, recording=None, embedding=None, copy_of=None):
        tid = self.conn.execute(
            """
            INSERT INTO tracks (path, title, genre_tag, artist_id, release_id, mb_recording_id)
            VALUES (%s, %s, %s, %s, %s, %s) RETURNING id
            """,
            (f"/m/{name}.flac", name, tag, self.artist(artist) if artist else None, release, recording),
        ).fetchone()[0]
        if embedding is not None:
            self.conn.execute(
                "INSERT INTO track_audio_genres (track_id, model_version, activations, embedding) "
                "VALUES (%s, 'test', '{}', %s)",
                (tid, embedding),
            )
        if copy_of is not None:
            self.conn.execute("INSERT INTO track_copies (track_id, copy_of) VALUES (%s, %s)", (tid, copy_of))
        return tid

    def outside(self, source, ref, genres, discogs_release_id=None, found=True):
        self.conn.execute(
            "INSERT INTO outside_genres (source, ref, found, genres, discogs_release_id) VALUES (%s, %s, %s, %s, %s)",
            (source, ref, found, Jsonb(genres), discogs_release_id),
        )

    def decisions(self):
        rows = self.conn.execute(
            "SELECT track_id, genre, style, source, confidence, unsure_reason, review_reason, why "
            "FROM track_genre_decisions"
        ).fetchall()
        return {r[0]: r[1:] for r in rows}


def test_run_report_reads_every_source_and_stores_decisions(db_url):
    with psycopg.connect(db_url) as conn:
        lib = Library(conn)
        # Labels for the classifier: techno sounds like axis 0, metal like axis 1.
        for i in range(4):
            lib.track(f"techno{i}", tag="Techno", embedding=vec(0))
            lib.track(f"metal{i}", tag="Heavy Metal", embedding=vec(1))

        # Discogs through the MusicBrainz release's link: Electronic + Deep House.
        r1 = lib.release("Deep", mb_release_id="rel-1")
        lib.outside("mb_release", "rel-1", [], discogs_release_id="d-1")
        lib.outside("discogs_release", "d-1", [{"name": "Electronic", "count": None, "kind": "genre"},
                                               {"name": "Deep House", "count": None, "kind": "style"}])
        deep = lib.track("deep", tag="Electronic", release=r1)
        # A MusicBrainz recording genre.
        lib.outside("mb_recording", "rec-1", [{"name": "techno", "count": 3, "kind": "genre"}])
        by_recording = lib.track("by-recording", recording="rec-1")
        # Release genres fall back to the release group's.
        r3 = lib.release("Heavy", mb_release_id="rel-3", group="grp-3")
        lib.outside("mb_release", "rel-3", [])
        lib.outside("mb_release_group", "grp-3", [{"name": "heavy metal", "count": 1, "kind": "genre"}])
        by_group = lib.track("by-group", release=r3)
        # Untagged, sounds like techno, but its artist is a metal artist.
        conn.execute("INSERT INTO musicbrainz_artists (name_key, mbid, tags) VALUES ('metal band', 'x', %s)",
                     (Jsonb([{"name": "heavy metal", "count": 5}]),))
        fenced = lib.track("fenced", artist="Metal Band", embedding=vec(0, 1))
        # Last.fm fences to house: a Techno tag is outside the fence.
        lib.outside("lastfm_track", "DJ\x1fSong", [{"name": "house", "count": 100, "kind": "tag"}])
        outside = lib.track("Song", tag="Techno", artist="DJ")
        nothing = lib.track("nothing")

        # Corrections: song (stored on the song id, applies to its copies),
        # release, artist; the song's beats the release's.
        r2 = lib.release("Fixed")
        on_release = lib.track("on-release", tag="Techno", release=r2)
        song_beats_release = lib.track("song-beats-release", tag="Techno", release=r2)
        best = lib.track("best", tag="Techno")
        copy = lib.track("copy", copy_of=best)
        by_artist = lib.track("by-artist", tag="Techno", artist="Corrected Artist")
        unknown = lib.track("unknown", tag="Techno")
        conn.execute("INSERT INTO genre_corrections (release_id, genre) VALUES (%s, 'jazz')", (r2,))
        conn.execute("INSERT INTO genre_corrections (track_id, genre) VALUES (%s, 'rock')", (song_beats_release,))
        conn.execute("INSERT INTO genre_corrections (track_id, genre) VALUES (%s, 'deep_house')", (best,))
        conn.execute("INSERT INTO genre_corrections (artist, genre) VALUES ('corrected artist', 'ambient')")
        conn.execute("INSERT INTO genre_corrections (track_id, genre) VALUES (%s, 'gone')", (unknown,))
        conn.commit()

        conn.read_only = True
        report = ge.run_report(conn)
        conn.rollback()
        conn.read_only = False
        assert report["written"] is False
        assert lib.decisions() == {}
        summary = report["summary"]
        assert summary["tracks"] == 8 + 12
        assert summary["unsure_reasons"] == {ge.OUTSIDE_FENCE: 1, ge.NO_AUDIO: 1}
        assert report["test_score"]["labels"] == 8
        assert report["examples"][ge.OUTSIDE_FENCE][0]["title"] == "Song"

        report = ge.run_report(conn, write=True)
        conn.commit()
        assert report["written"] is True
        got = lib.decisions()
        assert got[deep][:3] == ("house", "deep_house", ge.STRONG_SOURCES)
        assert got[deep][6]["sources"]["discogs"]["genre"] == "house"
        assert got[by_recording][:3] == ("techno", None, ge.STRONG_SOURCES)
        assert got[by_group][:3] == ("metal", "heavy_metal", ge.STRONG_SOURCES)
        assert got[fenced][:3] == ("metal", None, ge.CLASSIFIER)
        assert got[fenced][6]["fence"] == ["metal"]
        assert got[outside][2:5] == (ge.UNSURE, None, ge.OUTSIDE_FENCE)
        assert got[nothing][4] == ge.NO_AUDIO
        assert got[on_release][:3] == ("jazz", None, ge.CORRECTION)
        assert got[on_release][6]["correction"]["scope"] == ge.RELEASE_SCOPE
        assert got[song_beats_release][:3] == ("rock", None, ge.CORRECTION)
        assert got[best][:3] == got[copy][:3] == ("house", "deep_house", ge.CORRECTION)
        assert got[by_artist][:3] == ("ambient", None, ge.CORRECTION)
        assert got[unknown][:3] == ("techno", None, ge.STRONG_SOURCES)

        score = conn.execute("SELECT value FROM app_state WHERE key = %s", (ge.TEST_SCORE_KEY,)).fetchone()[0]
        assert score["labels"] == 8 and score["accuracy"] == 1.0
        assert score["catalog_version"] == gc.catalog().version
        # The bulk run never touches the live stations.
        assert conn.execute("SELECT COUNT(*) FROM track_station_genres").fetchone()[0] == 0


def test_review_queue_is_grouped_by_release_and_ranked(db_url):
    with psycopg.connect(db_url) as conn:
        lib = Library(conn)
        genre_scoring.sync_catalog(conn)  # station genres (v1 ids include house)
        user = conn.execute("INSERT INTO users (username, password_hash) VALUES ('u', 'x') RETURNING id").fetchone()[0]
        station = conn.execute(
            "INSERT INTO stations (name, auto_genre) VALUES ('House', 'house') RETURNING id"
        ).fetchone()[0]
        r_hidden, r_unsure, r_sure = lib.release("Hidden"), lib.release("Unsure"), lib.release("Sure")
        hidden = lib.track("hidden", release=r_hidden)
        hidden_mate = lib.track("hidden-mate", release=r_hidden)
        unsure = lib.track("unsure", release=r_unsure)
        flagged = lib.track("flagged")
        least_sure = lib.track("least-sure", release=r_sure)
        surer = lib.track("surer", release=r_sure)
        strong = lib.track("strong")
        corrected = lib.track("corrected")

        def decision(track, genre, source, confidence=None, unsure_reason=None, review_reason=None):
            conn.execute(
                """
                INSERT INTO track_genre_decisions
                    (track_id, genre, source, confidence, unsure_reason, review_reason, catalog_version)
                VALUES (%s, %s, %s, %s, %s, %s, 'test')
                """,
                (track, genre, source, confidence, unsure_reason, review_reason),
            )

        decision(hidden, "house", "strong_sources", 1.0)
        decision(hidden_mate, "house", "classifier", 0.9)
        decision(unsure, None, "unsure", unsure_reason="classifier_unsure")
        decision(flagged, "techno", "strong_sources", 1.0, review_reason="classifier_disagrees")
        decision(least_sure, "house", "classifier", 0.61)
        decision(surer, "house", "release", 0.9)
        decision(strong, "house", "strong_sources", 1.0)
        decision(corrected, "jazz", "correction", 1.0)
        conn.execute("INSERT INTO station_hidden_tracks (user_id, station_id, track_id) VALUES (%s, %s, %s)",
                     (user, station, hidden))
        rows = conn.execute(
            "SELECT track_id, priority, release_priority, hidden_by FROM genre_review_queue ORDER BY position"
        ).fetchall()
        assert [r[0] for r in rows] == [hidden, hidden_mate, unsure, flagged, least_sure, surer]
        assert [r[1] for r in rows] == [1, 4, 2, 3, 4, 4]
        assert rows[1][2] == 1  # kept with its release, ranked by the hidden song
        assert rows[0][3] == 1


def test_release_corrections_need_exactly_one_target(db_url):
    with psycopg.connect(db_url) as conn:
        lib = Library(conn)
        release = lib.release("R")
        track = lib.track("t", release=release)
        with pytest.raises(psycopg.errors.CheckViolation):
            conn.execute("INSERT INTO genre_corrections (track_id, release_id, genre) VALUES (%s, %s, 'jazz')",
                         (track, release))


def test_score_genres_keeps_the_v1_engine_until_the_switch(db_url):
    import tasks

    with psycopg.connect(db_url, autocommit=True) as conn:
        lib = Library(conn)
        genre_scoring.sync_catalog(conn)
        tagged = lib.track("tagged", tag="Techno; Hard Techno")
        unsure = lib.track("unsure", tag="Techno", artist="Metal Band")
        conn.execute("INSERT INTO musicbrainz_artists (name_key, mbid, tags) VALUES ('metal band', 'x', %s)",
                     (Jsonb([{"name": "heavy metal", "count": 5}]),))

        assert not ge.switched(conn)
        tasks.score_genres()
        # v1: the blend, which gives every tagged song a genre.
        assert conn.execute("SELECT COUNT(*) FROM track_station_genres WHERE is_primary").fetchone()[0] == 2
        assert conn.execute("SELECT COUNT(*) FROM track_genre_decisions").fetchone()[0] == 0

        up, _ = gc.switch_migration(gc.catalog())
        conn.execute(up)
        assert ge.switched(conn)
        summary = tasks.score_genres()
        assert summary == {"tracks": 2, "scored": 1, "unsure": 1}
        stations = conn.execute(
            "SELECT track_id, genre, score, is_primary, corrected FROM track_station_genres"
        ).fetchall()
        # The unsure song joins no genre station.
        assert stations == [(tagged, "techno", 1.0, True, False)]
        style = conn.execute("SELECT style FROM track_genre_decisions WHERE track_id = %s", (tagged,)).fetchone()[0]
        assert style == "hard_techno"
        assert conn.execute(
            "SELECT kind FROM station_genres WHERE id = 'hard_techno'"
        ).fetchone()[0] == "style"
        assert conn.execute("SELECT value FROM app_state WHERE key = %s", (ge.TEST_SCORE_KEY,)).fetchone()
        scored = conn.execute("SELECT value FROM app_state WHERE key = %s", (tasks.GENRES_SCORED_KEY,)).fetchone()[0]
        assert scored == {"tracks": 2, "scored": 1}
        # The API's startup catalog sync follows the switch too.
        ge.sync_catalog(conn)
        assert conn.execute("SELECT COUNT(*) FROM station_genres WHERE kind = 'style'").fetchone()[0] > 0


def test_migration_036_down_restores_the_corrections_table(db_url):
    from pathlib import Path

    down = (Path(__file__).resolve().parents[3] / "database" / "migrations" / "036_genre_decisions.down.sql")
    with psycopg.connect(db_url, autocommit=True) as conn:
        lib = Library(conn)
        release = lib.release("R")
        conn.execute("INSERT INTO genre_corrections (release_id, genre) VALUES (%s, 'jazz')", (release,))
        conn.execute("INSERT INTO genre_corrections (artist, genre) VALUES ('Someone', 'rock')")
        conn.execute(down.read_text())
        assert conn.execute("SELECT artist, genre FROM genre_corrections").fetchall() == [("Someone", "rock")]
        assert conn.execute("SELECT to_regclass('track_genre_decisions')").fetchone()[0] is None
        conn.execute(down.with_name("036_genre_decisions.up.sql").read_text())


def test_cli_reports_without_writing(db_url, monkeypatch, capsys):
    import json
    import sys

    import main

    with psycopg.connect(db_url, autocommit=True) as conn:
        Library(conn).track("tagged", tag="Techno")
    monkeypatch.setattr(sys, "argv", ["main.py", "genre-engine", "--examples", "1"])
    main.main()
    report = json.loads(capsys.readouterr().out)
    assert report["summary"]["by_source"] == {ge.STRONG_SOURCES: 1}
    assert report["written"] is False
    with psycopg.connect(db_url) as conn:
        assert conn.execute("SELECT COUNT(*) FROM track_genre_decisions").fetchone()[0] == 0
