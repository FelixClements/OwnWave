import psycopg
from psycopg.types.json import Jsonb

import db
import genre_scoring


def test_score_library_blends_sources_and_reports_distribution(db_url):
    labels = ["Electronic---Techno", "Rock---Punk"]
    with psycopg.connect(db_url) as conn:
        artist = conn.execute("INSERT INTO artists (name) VALUES ('The Punks') RETURNING id").fetchone()[0]

        def track(path, genre_tag=None, activations=None, artist_id=None):
            tid = conn.execute(
                "INSERT INTO tracks (path, title, genre_tag, artist_id) VALUES (%s, %s, %s, %s) RETURNING id",
                (path, path, genre_tag, artist_id),
            ).fetchone()[0]
            if activations:
                conn.execute(
                    "INSERT INTO track_audio_genres (track_id, model_version, activations) VALUES (%s, 'test', %s)",
                    (tid, activations),
                )
            return tid

        techno = track("/m/techno.flac", activations=[0.9, 0.05])
        punk = track("/m/punk.flac", artist_id=artist)
        tagged = track("/m/tagged.flac", genre_tag="Techno; Hard Techno")
        track("/m/nothing.flac")
        conn.execute(
            "INSERT INTO musicbrainz_artists (name_key, mbid, tags) VALUES ('the punks', 'x', %s)",
            (Jsonb([{"name": "punk", "count": 4}]),),
        )

        assert genre_scoring.score_library(conn, labels) == {"tracks": 4, "scored": 3}
        assert db.get_track_station_genres(conn, techno)[0]["genre"] == "techno"
        assert db.get_track_station_genres(conn, punk)[0]["genre"] == "punk"
        assert db.get_track_station_genres(conn, tagged) == [{"genre": "techno", "score": 1.0, "is_primary": True}]

        dist = genre_scoring.distribution(conn)
        by_genre = {g["genre"]: g for g in dist["genres"]}
        assert dist["scored_tracks"] == 3
        assert by_genre["techno"]["primary_tracks"] == 2
        assert dist["warnings"]  # techno holds 2 of 3 tracks


def test_agreement_report_compares_audio_with_tags(db_url):
    labels = ["Electronic---Techno", "Rock---Punk"]
    with psycopg.connect(db_url) as conn:
        for i, (tag, acts) in enumerate([("Techno", [0.9, 0.1]), ("Punk", [0.8, 0.2]), ("Punk", [0.1, 0.7])]):
            tid = conn.execute(
                "INSERT INTO tracks (path, title, genre_tag) VALUES (%s, 't', %s) RETURNING id", (f"/m/{i}", tag)
            ).fetchone()[0]
            conn.execute(
                "INSERT INTO track_audio_genres (track_id, model_version, activations) VALUES (%s, 'test', %s)",
                (tid, acts),
            )
        report = genre_scoring.agreement_report(conn, labels)
        assert report["tracks"] == 3
        assert report["audio_agreement"] == round(2 / 3, 3)
        assert report["top_confusions"] == [{"tag": "punk", "audio": "techno", "tracks": 1}]


def test_sync_catalog_writes_station_genres(db_url):
    import genre_taxonomy as tx

    with psycopg.connect(db_url) as conn:
        conn.execute("INSERT INTO station_genres (id, name, position) VALUES ('stale', 'Gone', 0)")
        genre_scoring.sync_catalog(conn)
        rows = conn.execute("SELECT id, name, is_station FROM station_genres ORDER BY position").fetchall()
        assert [r[0] for r in rows] == list(tx.STATION_GENRES)
        assert ("techno", "Techno", True) in rows
        assert ("other", "Other", False) in rows
