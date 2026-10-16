import psycopg

import db


def test_uncategorized_count_matches_confident_genre_rule(db_url):
    with psycopg.connect(db_url) as conn:
        def track(path, genre_confidence=None, analysed=True):
            tid = conn.execute(
                "INSERT INTO tracks (path, title) VALUES (%s, %s) RETURNING id", (path, path)
            ).fetchone()[0]
            if analysed:
                conn.execute("INSERT INTO audio_features (track_id) VALUES (%s)", (tid,))
            if genre_confidence is not None:
                conn.execute(
                    "INSERT INTO track_genres (track_id, main_genre, sub_genre, confidence, source)"
                    " VALUES (%s, 'rock', 'punk', %s, 'test')",
                    (tid, genre_confidence),
                )

        track("/m/confident.flac", 0.9)
        track("/m/weak.flac", 0.1)
        track("/m/none.flac")
        track("/m/unanalysed.flac", analysed=False)

        assert db.count_uncategorized_tracks(conn, 0.3) == 2
        assert db.get_main_genre_counts(conn, 0.3) == [
            {"main_genre": "rock", "track_count": 1, "sub_count": 1}
        ]
