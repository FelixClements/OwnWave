"""The worker tells the Go API (Postgres NOTIFY) when a scan finishes and
when genres are scored, so playlist matching and the genre stations follow
without waiting for the API's hourly pass."""

import psycopg

import tasks


def _listen(db_url):
    conn = psycopg.connect(db_url, autocommit=True)
    conn.execute(f"LISTEN {tasks.PIPELINE_CHANNEL}")
    return conn


def _payloads(conn):
    return [n.payload for n in conn.notifies(timeout=1, stop_after=1)]


def test_finished_scan_notifies_the_api(db_url, monkeypatch):
    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    with _listen(db_url) as listener:
        tasks.rebuild_analysis()
        assert _payloads(listener) == ["scan_finished"]
    with psycopg.connect(db_url) as conn:
        assert conn.execute("SELECT task FROM task_queue").fetchall() == [("score_genres",)]


def test_genre_scoring_records_its_run_and_notifies_the_api(db_url, monkeypatch, tmp_path):
    monkeypatch.setattr("config.GENRE_MODEL_DIR", str(tmp_path))  # no model: tags only
    with psycopg.connect(db_url) as conn:
        conn.execute("INSERT INTO tracks (path, title, genre_tag) VALUES ('/m/a.flac', 'a', 'Techno')")
        conn.commit()
    with _listen(db_url) as listener:
        assert tasks.score_genres() == {"tracks": 1, "scored": 1}
        assert _payloads(listener) == ["genres_scored"]
    with psycopg.connect(db_url) as conn:
        value = conn.execute("SELECT value FROM app_state WHERE key = 'genres_scored'").fetchone()[0]
    assert value == {"tracks": 1, "scored": 1}


def test_no_notice_when_the_work_rolls_back(db_url):
    with _listen(db_url) as listener:
        with psycopg.connect(db_url) as conn:
            tasks.notify_pipeline(conn, "genres_scored")
            conn.rollback()
        assert _payloads(listener) == []
