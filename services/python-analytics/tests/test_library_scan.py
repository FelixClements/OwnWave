import shutil
from pathlib import Path

import numpy as np
import psycopg
import pytest
import soundfile as sf
from mutagen.flac import FLAC

from library_scan import scan_library
from library_scan.options import ScanOptions
from library_scan.prune import match_moved_tracks


def write_flac(path: Path, title: str, seconds: float = 1.0, freq: float = 440.0) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    t = np.linspace(0, seconds, int(8000 * seconds), endpoint=False)
    sf.write(path, 0.2 * np.sin(2 * np.pi * freq * t), 8000, format="FLAC")
    tags = FLAC(path)
    tags["title"] = title
    tags["artist"] = "Test Artist"
    tags.save()
    return path


def scan(root: Path):
    return scan_library(root, options=ScanOptions(analyze_audio=False))


def tracks(url: str) -> dict:
    with psycopg.connect(url) as conn:
        return {path: tid for tid, path in conn.execute("SELECT id, path FROM tracks").fetchall()}


def test_match_moved_tracks_requires_unambiguous_match():
    old = {"id": 1, "file_size": 100, "duration_seconds": 10.0}
    new = {"id": 2, "file_size": 100, "duration_seconds": 10.2}
    assert match_moved_tracks([old], [new]) == [(old, new)]

    copy = {"id": 3, "file_size": 100, "duration_seconds": 10.1}
    assert match_moved_tracks([old], [new, copy]) == []

    different = {"id": 4, "file_size": 101, "duration_seconds": 10.0}
    assert match_moved_tracks([old], [different]) == []


def test_scan_imports_loose_root_files_and_subfolders(db_url, tmp_path):
    files = [
        write_flac(tmp_path / "loose.flac", "Loose"),
        write_flac(tmp_path / "Artist" / "single.flac", "Single"),
        write_flac(tmp_path / "Artist" / "Album" / "song.flac", "Song"),
    ]
    scan(tmp_path)
    assert set(tracks(db_url)) == {str(f) for f in files}


def test_underscore_folders_do_not_delete_each_other(db_url, tmp_path):
    underscored = write_flac(tmp_path / "The_Band" / "a.flac", "A")
    spaced = write_flac(tmp_path / "The Band" / "b.flac", "B")
    scan(tmp_path)
    assert set(tracks(db_url)) == {str(underscored), str(spaced)}

    underscored.unlink()
    write_flac(tmp_path / "The_Band" / "c.flac", "C", freq=880.0, seconds=2.0)
    scan(tmp_path)
    assert str(spaced) in tracks(db_url)
    assert str(underscored) not in tracks(db_url)


def test_empty_library_deletes_nothing(db_url, tmp_path):
    write_flac(tmp_path / "Artist" / "song.flac", "Song")
    scan(tmp_path)
    shutil.rmtree(tmp_path / "Artist")

    scan(tmp_path)
    assert len(tracks(db_url)) == 1


@pytest.mark.parametrize("new_relpath", ["Artist/renamed.flac", "Elsewhere/song.flac"])
def test_moved_file_keeps_track_id_and_likes(db_url, tmp_path, new_relpath):
    original = write_flac(tmp_path / "Artist" / "song.flac", "Song")
    write_flac(tmp_path / "Artist" / "other.flac", "Other", freq=660.0, seconds=2.0)
    scan(tmp_path)
    track_id = tracks(db_url)[str(original)]

    with psycopg.connect(db_url) as conn:
        user_id = conn.execute(
            "INSERT INTO users (username, password_hash) VALUES ('u', 'x') RETURNING id"
        ).fetchone()[0]
        conn.execute(
            "INSERT INTO track_feedback (user_id, track_id, feedback) VALUES (%s, %s, 'like')",
            (user_id, track_id),
        )

    moved = tmp_path / new_relpath
    moved.parent.mkdir(parents=True, exist_ok=True)
    original.rename(moved)
    scan(tmp_path)

    after = tracks(db_url)
    assert after[str(moved)] == track_id
    assert str(original) not in after
    with psycopg.connect(db_url) as conn:
        likes = conn.execute("SELECT track_id FROM track_feedback WHERE feedback = 'like'").fetchall()
    assert likes == [(track_id,)]


def drain_queue():
    import db
    import task_queue

    with db.get_conn() as conn:
        while task_queue.run_one(conn):
            pass


def test_queued_scan_finalizes_job_and_prunes(db_url, tmp_path):
    import db
    import tasks

    loose = write_flac(tmp_path / "loose.flac", "Loose")
    nested = write_flac(tmp_path / "Artist" / "song.flac", "Song", freq=550.0, seconds=1.5)
    with db.get_conn() as conn:
        job_id = tasks.queue_library_scan(conn, str(tmp_path))
        conn.commit()

    drain_queue()

    assert set(tracks(db_url)) == {str(loose), str(nested)}
    with db.get_conn() as conn:
        status = db.get_scan_status(conn, job_id)
        leftover = conn.execute("SELECT task, status, error FROM task_queue").fetchall()
    assert status["status"] == "completed"
    assert status["folders"] == {"total": 2, "done": 2, "failed": 0}
    assert status["stats"]["total_files"] == 2
    assert leftover == []

    loose.unlink()
    with db.get_conn() as conn:
        tasks.queue_library_scan(conn, str(tmp_path))
        conn.commit()
    drain_queue()
    assert set(tracks(db_url)) == {str(nested)}


def queue_scan(path: Path):
    import db
    import tasks

    with db.get_conn() as conn:
        job_id = tasks.queue_library_scan(conn, str(path))
        conn.commit()
    return job_id


def folder_jobs(url: str, master) -> list:
    with psycopg.connect(url) as conn:
        return [r[0] for r in conn.execute("SELECT path FROM scan_jobs WHERE parent_id = %s", (master,)).fetchall()]


def test_scan_with_no_changes_queues_nothing_and_sends_no_notify(db_url, tmp_path, monkeypatch):
    import db
    import task_queue
    import tasks

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    queue_scan(tmp_path)
    drain_queue()

    with psycopg.connect(db_url, autocommit=True) as listener:
        listener.execute(f"LISTEN {tasks.PIPELINE_CHANNEL}")
        job_id = queue_scan(tmp_path)
        with db.get_conn() as conn:
            assert task_queue.run_one(conn)  # trigger_library_scan
            queued = conn.execute("SELECT task FROM task_queue").fetchall()
            status = db.get_scan_status(conn, job_id)
        notices = [n.payload for n in listener.notifies(timeout=1, stop_after=1)]

    assert queued == []
    assert status["status"] == "completed"
    assert status["folders"] == {"total": 0, "done": 0, "failed": 0}
    assert status["stats"]["total_files"] == 2
    assert notices == []


def test_queued_scan_of_one_new_file_imports_one_folder(db_url, tmp_path, monkeypatch):
    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    queue_scan(tmp_path)
    drain_queue()

    new = write_flac(tmp_path / "C" / "c.flac", "C", freq=660.0)
    job_id = queue_scan(tmp_path)
    drain_queue()

    assert folder_jobs(db_url, job_id) == [str(tmp_path / "C")]
    assert str(new) in tracks(db_url)


def test_queued_scan_keeps_track_id_of_file_moved_between_folders(db_url, tmp_path, monkeypatch):
    import db

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    original = write_flac(tmp_path / "A" / "song.flac", "Song")
    write_flac(tmp_path / "A" / "other.flac", "Other", freq=660.0, seconds=2.0)
    queue_scan(tmp_path)
    drain_queue()
    track_id = tracks(db_url)[str(original)]

    moved = tmp_path / "B" / "song.flac"
    moved.parent.mkdir()
    original.rename(moved)
    job_id = queue_scan(tmp_path)
    drain_queue()

    after = tracks(db_url)
    assert after[str(moved)] == track_id
    assert str(original) not in after
    # Only the destination is imported; prune sees the source from the full listing.
    assert folder_jobs(db_url, job_id) == [str(tmp_path / "B")]
    with db.get_conn() as conn:
        assert db.get_scan_status(conn, job_id)["status"] == "completed"


def test_queued_scan_of_only_deleted_files_prunes_without_folder_tasks(db_url, tmp_path, monkeypatch):
    import db

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    gone = write_flac(tmp_path / "A" / "a.flac", "A")
    kept = write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    queue_scan(tmp_path)
    drain_queue()

    gone.unlink()
    job_id = queue_scan(tmp_path)
    drain_queue()

    assert set(tracks(db_url)) == {str(kept)}
    assert folder_jobs(db_url, job_id) == []
    with db.get_conn() as conn:
        assert db.get_scan_status(conn, job_id)["status"] == "completed"


def test_scan_whose_planned_folder_changes_nothing_skips_downstream(db_url, tmp_path, monkeypatch):
    """A file that fails analysis is planned again every scan (until #119),
    but when its folder changes nothing, no analysis refresh or NOTIFY follows."""
    import db
    import task_queue
    import tasks

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    broken = tmp_path / "A" / "broken.flac"
    broken.parent.mkdir()
    broken.write_bytes(b"not audio")
    queue_scan(tmp_path)
    drain_queue()

    with psycopg.connect(db_url, autocommit=True) as listener:
        listener.execute(f"LISTEN {tasks.PIPELINE_CHANNEL}")
        job_id = queue_scan(tmp_path)
        ran = []
        with db.get_conn() as conn:
            while True:
                task = conn.execute("SELECT task FROM task_queue ORDER BY id LIMIT 1").fetchone()
                if not task or not task_queue.run_one(conn):
                    break
                ran.append(task[0])
            status = db.get_scan_status(conn, job_id)
        notices = [n.payload for n in listener.notifies(timeout=1, stop_after=1)]

    assert ran == ["trigger_library_scan", "process_audio_folder", "finalize_scan"]
    assert status["status"] == "completed"
    assert status["stats"]["imported"] == 0
    assert status["stats"]["failed_paths"]
    assert notices == []
